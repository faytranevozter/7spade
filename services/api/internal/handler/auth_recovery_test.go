package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// fakeEmailSender records the last link sent so tests can replay the token.
type fakeEmailSender struct {
	resetTo, resetLink   string
	verifyTo, verifyLink string
}

func (f *fakeEmailSender) SendPasswordReset(_ context.Context, to, link string) error {
	f.resetTo, f.resetLink = to, link
	return nil
}

func (f *fakeEmailSender) SendVerification(_ context.Context, to, link string) error {
	f.verifyTo, f.verifyLink = to, link
	return nil
}

func newTestRedisClient(t *testing.T) *cache.RedisClient {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client, err := cache.New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	idx := strings.Index(link, "token=")
	if idx < 0 {
		t.Fatalf("no token in link: %s", link)
	}
	raw := link[idx+len("token="):]
	// The link URL-encodes the token; a real client reads the decoded value.
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		t.Fatalf("unescape token: %v", err)
	}
	return decoded
}

const userSelectCols = "id, email, password_hash, display_name, username, created_at, email_verified_at"

func TestForgotPasswordAlwaysReturns200(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// Unknown email: handler looks it up, finds nothing, still returns 200.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+userSelectCols+" FROM users WHERE email = $1")).
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows(strings.Split(userSelectCols, ", ")))

	email := &fakeEmailSender{}
	h := AuthHandler{DB: db, JWTSecret: "s", Redis: newTestRedisClient(t), Email: email, FrontendURL: "https://app.test"}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"nobody@example.com"}`))

	h.ForgotPassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if email.resetLink != "" {
		t.Fatalf("no email should be sent for unknown account, got %q", email.resetLink)
	}
}

func TestForgotThenResetPasswordFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	cols := strings.Split(userSelectCols, ", ")
	// forgot-password: existing password account.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+userSelectCols+" FROM users WHERE email = $1")).
		WithArgs("a@b.com").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(id, "a@b.com", "old-hash", "Alice", "alice", time.Now(), nil))
	// reset-password: update hash + revoke refresh tokens.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET password_hash = $1 WHERE id = $2")).
		WithArgs(sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM refresh_tokens WHERE user_id = $1")).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 2))

	email := &fakeEmailSender{}
	h := AuthHandler{DB: db, JWTSecret: "s", Redis: newTestRedisClient(t), Email: email, FrontendURL: "https://app.test"}

	// 1. forgot-password
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"a@b.com"}`))
	h.ForgotPassword(c)
	if w.Code != http.StatusOK {
		t.Fatalf("forgot status = %d", w.Code)
	}
	if !strings.Contains(email.resetLink, "https://app.test/reset-password?token=") {
		t.Fatalf("unexpected reset link: %q", email.resetLink)
	}
	token := tokenFromLink(t, email.resetLink)

	// 2. reset-password with the emailed token
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"`+token+`","password":"newpassword1"}`))
	h.ResetPassword(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("reset status = %d (body %s)", w2.Code, w2.Body.String())
	}

	// 3. token is single-use: a second reset fails.
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"`+token+`","password":"another12345"}`))
	h.ResetPassword(c3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("reused token: status = %d, want 400", w3.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestForgotPasswordRateLimited(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	cols := strings.Split(userSelectCols, ", ")
	// The handler looks up the user on every request; allow up to 4 lookups.
	for i := 0; i < 4; i++ {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT "+userSelectCols+" FROM users WHERE email = $1")).
			WithArgs("a@b.com").
			WillReturnRows(sqlmock.NewRows(cols).AddRow(id, "a@b.com", "hash", "Alice", "alice", time.Now(), nil))
	}

	email := &fakeEmailSender{}
	h := AuthHandler{DB: db, JWTSecret: "s", Redis: newTestRedisClient(t), Email: email, FrontendURL: "https://app.test"}

	sends := 0
	for i := 0; i < 4; i++ {
		email.resetLink = ""
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"a@b.com"}`))
		h.ForgotPassword(c)
		// Always 200 (enumeration-safe) regardless of rate limiting.
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i, w.Code)
		}
		if email.resetLink != "" {
			sends++
		}
	}
	// Limit is 3/hr: the 4th request must not send an email.
	if sends != 3 {
		t.Fatalf("expected 3 emails sent under a 3/hr limit, got %d", sends)
	}
}

func TestResetPasswordRejectsShortPassword(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "s", Redis: newTestRedisClient(t)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"x","password":"short"}`))
	h.ResetPassword(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestResetPasswordRejectsUnknownToken(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "s", Redis: newTestRedisClient(t)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"does-not-exist","password":"longenough1"}`))
	h.ResetPassword(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestVerifyEmailFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	redis := newTestRedisClient(t)
	// Seed a verification token directly (as Register would).
	token, err := auth.GenerateURLToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if err := redis.StoreEmailVerifyToken(context.Background(), auth.HashToken(token), id.String(), time.Hour); err != nil {
		t.Fatalf("store: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET email_verified_at = NOW() WHERE id = $1")).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := AuthHandler{DB: db, JWTSecret: "s", Redis: redis}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/verify-email", strings.NewReader(`{"token":"`+token+`"}`))
	h.VerifyEmail(c)

	if w.Code != http.StatusOK {
		t.Fatalf("verify status = %d (body %s)", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}

	// Single-use: replaying the token fails.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/auth/verify-email", strings.NewReader(`{"token":"`+token+`"}`))
	h.VerifyEmail(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("reused verify token: status = %d, want 400", w2.Code)
	}
}

func TestResendVerificationRejectsGuest(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "s", Redis: newTestRedisClient(t), Email: &fakeEmailSender{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString(), IsGuest: true})
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/resend-verification", nil)
	h.ResendVerification(c)
	c.Writer.WriteHeaderNow() // flush gin's buffered status (no body written)
	// Guests get a no-op 204 (no enumeration / no email).
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}
