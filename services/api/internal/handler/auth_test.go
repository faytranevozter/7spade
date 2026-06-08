package handler

import (
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
)

func TestMeGuestResponse(t *testing.T) {
	h := AuthHandler{DB: nil, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString(), DisplayName: "Guest", IsGuest: true})
	c.Request = httptest.NewRequest(http.MethodGet, "/me", nil)

	h.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body struct {
		UserID      *string `json:"user_id"`
		Username    *string `json:"username"`
		DisplayName string  `json:"display_name"`
		CreatedAt   *string `json:"created_at"`
		IsGuest     bool    `json:"is_guest"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.IsGuest || body.DisplayName != "Guest" {
		t.Fatalf("unexpected guest payload: %+v", body)
	}
	if body.UserID != nil || body.Username != nil || body.CreatedAt != nil {
		t.Fatalf("guest payload should not expose account fields: %+v", body)
	}
}

func TestMeRegisteredResponse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	providerCreated := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

	userCols := []string{"id", "email", "password_hash", "display_name", "username", "created_at", "email_verified_at"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, display_name, username, created_at, email_verified_at FROM users WHERE id = $1")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(id, "a@b.com", "hash", "Alice", "alice", created, created))
	mock.ExpectQuery("FROM users u").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"avatar_url"}).AddRow("https://avatar.test/alice.png"))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT provider, avatar_url, created_at
		FROM user_providers
		WHERE user_id = $1
		ORDER BY created_at DESC
	`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"provider", "avatar_url", "created_at"}).AddRow("github", "https://avatar.test/alice.png", providerCreated))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String(), DisplayName: "Alice", IsGuest: false})
	c.Request = httptest.NewRequest(http.MethodGet, "/me", nil)

	h.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		UserID      *string `json:"user_id"`
		Username    *string `json:"username"`
		DisplayName string  `json:"display_name"`
		CreatedAt   *string `json:"created_at"`
		IsGuest     bool    `json:"is_guest"`
		Providers   []struct {
			Provider string `json:"provider"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.UserID == nil || *body.UserID != id.String() {
		t.Fatalf("user_id mismatch: %+v", body.UserID)
	}
	if body.Username == nil || *body.Username != "alice" || body.DisplayName != "Alice" || body.IsGuest {
		t.Fatalf("unexpected registered payload: %+v", body)
	}
	if len(body.Providers) != 1 || body.Providers[0].Provider != "github" {
		t.Fatalf("unexpected providers payload: %+v", body.Providers)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// UpdateMe rejects guests and tokenless requests before touching the database.
func TestUpdateMeRejectsNonRegisteredUsers(t *testing.T) {
	cases := []struct {
		name    string
		claims  *auth.Claims
		setNil  bool
		wantMsg string
	}{
		{name: "no claims", setNil: true, wantMsg: "Authentication required"},
		{name: "guest", claims: &auth.Claims{Sub: uuid.NewString(), IsGuest: true}, wantMsg: "Logged-in user required"},
		{name: "bad sub", claims: &auth.Claims{Sub: "not-a-uuid"}, wantMsg: "Logged-in user required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := AuthHandler{DB: nil, JWTSecret: "test-secret"}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"display_name":"New Name"}`))
			if !tc.setNil {
				c.Set(middleware.ClaimsKey, tc.claims)
			}

			h.UpdateMe(c)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			assertErrorBody(t, w, tc.wantMsg)
		})
	}
}

// UpdateMe validates the display name (non-empty, <= 50 chars) before the DB.
func TestUpdateMeRejectsInvalidDisplayName(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty", body: `{"display_name":"   "}`},
		{name: "too long", body: `{"display_name":"` + strings.Repeat("a", 51) + `"}`},
		{name: "malformed json", body: `{`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := AuthHandler{DB: nil, JWTSecret: "test-secret"}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(middleware.ClaimsKey, &auth.Claims{Sub: uuid.NewString(), DisplayName: "Old"})
			c.Request = httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(tc.body))

			h.UpdateMe(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

// UpdateMe updates the name, re-issues a JWT carrying the new name, and trims
// surrounding whitespace.
func TestUpdateMeSuccessReissuesToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	userCols := []string{"id", "email", "password_hash", "display_name", "username", "created_at"}
	mock.ExpectQuery("UPDATE users SET display_name").
		WithArgs("New Name", id).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(id, "a@b.com", "hash", "New Name", "alice", time.Now()))
	mock.ExpectQuery("FROM users u").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"avatar_url"}).AddRow(nil))

	h := AuthHandler{DB: db, JWTSecret: "test-secret"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String(), DisplayName: "Old"})
	c.Request = httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"display_name":"  New Name  "}`))

	h.UpdateMe(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		JWT string `json:"jwt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.JWT == "" {
		t.Fatal("expected a re-issued jwt")
	}
	claims, err := auth.ParseToken(body.JWT, "test-secret")
	if err != nil {
		t.Fatalf("parse re-issued token: %v", err)
	}
	if claims.DisplayName != "New Name" {
		t.Fatalf("re-issued token display name = %q, want %q", claims.DisplayName, "New Name")
	}
	if claims.Sub != id.String() || claims.IsGuest {
		t.Fatalf("re-issued token identity wrong: sub=%q guest=%v", claims.Sub, claims.IsGuest)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
