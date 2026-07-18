package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return string(h)
}

func userRowCols() []string {
	return []string{"id", "email", "password_hash", "display_name", "username", "created_at", "email_verified_at", "deletion_scheduled_at"}
}

func expectGetUserByID(mock sqlmock.Sqlmock, id uuid.UUID, email, hash, display, username string, deletionAt interface{}) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, display_name, username, created_at, email_verified_at, deletion_scheduled_at FROM users WHERE id = $1")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(userRowCols()).AddRow(id, email, hash, display, username, time.Now(), nil, deletionAt))
}

func TestDeleteAccountRejectsGuest(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString(), IsGuest: true})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{}`))

	h.DeleteAccount(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	assertErrorBody(t, w, "Account deletion is only available for registered users")
}

func TestDeleteAccountPasswordRequiredAndWrong(t *testing.T) {
	hash := mustHash(t, "correct-password")
	id := uuid.New()

	t.Run("missing password", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		defer db.Close()
		expectGetUserByID(mock, id, "a@b.com", hash, "Alice", "alice", nil)

		h := AuthHandler{DB: db, JWTSecret: "test-secret"}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
		c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{}`))

		h.DeleteAccount(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
		}
		assertErrorBody(t, w, "Password is required")
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		defer db.Close()
		expectGetUserByID(mock, id, "a@b.com", hash, "Alice", "alice", nil)

		h := AuthHandler{DB: db, JWTSecret: "test-secret"}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
		c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{"password":"nope"}`))

		h.DeleteAccount(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
		assertErrorBody(t, w, "Invalid password")
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestDeleteAccountSuccessWithPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	hash := mustHash(t, "secret")
	scheduled := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	expectGetUserByID(mock, id, "a@b.com", hash, "Alice", "alice", nil)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NOW()
		WHERE id = $1 AND deletion_scheduled_at IS NULL
		RETURNING deletion_scheduled_at
	`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(scheduled))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM refresh_tokens WHERE user_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 2))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{"password":"secret"}`))

	h.DeleteAccount(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		DeletionScheduledAt string `json:"deletion_scheduled_at"`
		GraceDays           int    `json:"grace_days"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.DeletionScheduledAt != scheduled.Format(time.RFC3339) {
		t.Fatalf("deletion_scheduled_at = %q", body.DeletionScheduledAt)
	}
	if body.GraceDays != 7 {
		t.Fatalf("grace_days = %d, want 7", body.GraceDays)
	}
	// Refresh cookie cleared.
	foundClear := false
	for _, c := range w.Result().Cookies() {
		if c.Name == RefreshCookieName && c.MaxAge < 0 {
			foundClear = true
		}
	}
	if !foundClear {
		t.Fatalf("expected cleared refresh cookie, got %v", w.Result().Cookies())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteAccountOAuthOnlyNoPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	scheduled := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	// password_hash NULL via empty string + driver null — use nil for sqlmock
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, display_name, username, created_at, email_verified_at, deletion_scheduled_at FROM users WHERE id = $1")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(userRowCols()).AddRow(id, nil, nil, "OAuth User", "oauthuser", time.Now(), nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NOW()
		WHERE id = $1 AND deletion_scheduled_at IS NULL
		RETURNING deletion_scheduled_at
	`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(scheduled))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM refresh_tokens WHERE user_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{}`))

	h.DeleteAccount(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteAccountIdempotentWhenAlreadyPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	hash := mustHash(t, "secret")
	existing := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	expectGetUserByID(mock, id, "a@b.com", hash, "Alice", "alice", existing)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(existing))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM refresh_tokens WHERE user_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/delete", strings.NewReader(`{"password":"secret"}`))

	h.DeleteAccount(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		DeletionScheduledAt string `json:"deletion_scheduled_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.DeletionScheduledAt != existing.Format(time.RFC3339) {
		t.Fatalf("deletion_scheduled_at = %q, want existing", body.DeletionScheduledAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelDeletionSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NULL
		WHERE id = $1 AND deletion_scheduled_at IS NOT NULL
	`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/cancel-deletion", nil)

	h.CancelDeletion(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelDeletionNotPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NULL
		WHERE id = $1 AND deletion_scheduled_at IS NOT NULL
	`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String()})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/cancel-deletion", nil)

	h.CancelDeletion(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
	assertErrorBody(t, w, "No pending account deletion")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCancelDeletionRejectsGuest(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString(), IsGuest: true})
	c.Request = httptest.NewRequest(http.MethodPost, "/me/cancel-deletion", nil)

	h.CancelDeletion(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// Ensure sql.NullString path compiles for oauth-only mock (nil password_hash).
var _ = sql.NullString{}
