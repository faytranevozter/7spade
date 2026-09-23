package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestMobileGoogleRejectsMissingIDToken(t *testing.T) {
	h := OAuthHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/google", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileGoogle(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	assertErrorBody(t, w, "id_token is required")
}

func TestMobileTelegramRejectsMissingOrBlankIDToken(t *testing.T) {
	for _, body := range []string{`{}`, `{"id_token":"   "}`} {
		t.Run(body, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			(OAuthHandler{}).MobileTelegram(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
			}
			assertErrorBody(t, w, "id_token is required")
		})
	}
}

func TestMobileTelegramRejectsUnconfiguredProvider(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	(OAuthHandler{}).MobileTelegram(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	assertErrorBody(t, w, "telegram OAuth is not configured")
}

func TestMobileTelegramRejectsInvalidOrExpiredToken(t *testing.T) {
	var gotJWKSURL, gotIssuer, gotAudience, gotToken string
	h := OAuthHandler{
		Providers: map[string]OAuthProviderConfig{
			"telegram": {ClientID: "bot-id", JWKSURL: "https://oauth.telegram.org/.well-known/jwks.json", Issuer: "https://oauth.telegram.org"},
		},
		VerifyIDToken: func(_ context.Context, jwksURL, issuer, audience, token string) (map[string]any, error) {
			gotJWKSURL, gotIssuer, gotAudience, gotToken = jwksURL, issuer, audience, token
			return nil, errors.New("expired token")
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	if gotJWKSURL != "https://oauth.telegram.org/.well-known/jwks.json" || gotIssuer != "https://oauth.telegram.org" || gotAudience != "bot-id" || gotToken != "signed-token" {
		t.Fatalf("verification parameters = (%q, %q, %q, %q)", gotJWKSURL, gotIssuer, gotAudience, gotToken)
	}
	assertErrorBody(t, w, "invalid Telegram ID token")
}

func TestMobileTelegramRejectsMissingSubject(t *testing.T) {
	h := OAuthHandler{
		Providers: map[string]OAuthProviderConfig{"telegram": {ClientID: "bot-id"}},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{"first_name": "Ada"}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	assertErrorBody(t, w, "invalid Telegram ID token")
}

func TestMobileTelegramMapsClaimsAndIssuesMobileSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	userID := uuid.New()
	longName := strings.Repeat("A", 60)
	wantDisplayName := strings.Repeat("A", 50)
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("telegram:telegram-user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id FROM user_providers").WithArgs("telegram", "telegram-user").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))
	mock.ExpectQuery("UPDATE users SET display_name").WithArgs(wantDisplayName, userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "username", "created_at"}).
			AddRow(userID, nil, nil, wantDisplayName, "teleplayer", time.Now()))
	mock.ExpectExec("INSERT INTO user_providers").WithArgs(userID, "telegram", "telegram-user", nil, "https://example.com/avatar.png").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT suspended_at IS NOT NULL").WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"suspended"}).AddRow(false))
	mock.ExpectExec("INSERT INTO refresh_tokens").WillReturnResult(sqlmock.NewResult(0, 1))

	h := OAuthHandler{
		DB:        db,
		JWTSecret: "test-secret",
		Providers: map[string]OAuthProviderConfig{"telegram": {ClientID: "bot-id"}},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{
				"sub":                "telegram-user",
				"first_name":         longName,
				"last_name":          "Lovelace",
				"preferred_username": "ada",
				"picture":            "https://example.com/avatar.png",
			}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AccessToken == "" || body.RefreshToken == "" {
		t.Fatalf("mobile session = %+v, want access and refresh tokens", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileTelegramRejectsExistingEmailAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("telegram:telegram-user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id FROM user_providers").WithArgs("telegram", "telegram-user").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id FROM users WHERE email").WithArgs("player@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectRollback()

	h := OAuthHandler{
		DB:        db,
		Providers: map[string]OAuthProviderConfig{"telegram": {ClientID: "bot-id"}},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{"sub": "telegram-user", "email": "Player@Example.com"}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusConflict, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileTelegramRejectsDisabledRegistrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("telegram:telegram-user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id FROM user_providers").WithArgs("telegram", "telegram-user").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT .*value.* FROM feature_settings").WithArgs("new_registrations").
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
	mock.ExpectRollback()

	h := OAuthHandler{
		DB:        db,
		Providers: map[string]OAuthProviderConfig{"telegram": {ClientID: "bot-id"}},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{"sub": "telegram-user"}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileTelegramFallsBackToProviderIdentityAndRejectsSuspendedAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	userID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("telegram:telegram-user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id FROM user_providers").WithArgs("telegram", "telegram-user").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))
	mock.ExpectQuery("UPDATE users SET display_name").WithArgs("Telegram telegram-user", userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "username", "created_at"}).
			AddRow(userID, nil, nil, "Telegram telegram-user", "teleplayer", time.Now()))
	mock.ExpectExec("INSERT INTO user_providers").WithArgs(userID, "telegram", "telegram-user", nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT suspended_at IS NOT NULL").WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"suspended"}).AddRow(true))

	h := OAuthHandler{
		DB:        db,
		Providers: map[string]OAuthProviderConfig{"telegram": {ClientID: "bot-id"}},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{"sub": "telegram-user"}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegram(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusForbidden, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileGoogleRejectsUnverifiedEmail(t *testing.T) {
	h := OAuthHandler{
		Providers: map[string]OAuthProviderConfig{
			"google": {ClientID: "server-client"},
		},
		VerifyIDToken: func(_ context.Context, _ string, _ string, _ string, _ string) (map[string]any, error) {
			return map[string]any{
				"sub":            "google-user",
				"email":          "player@example.com",
				"email_verified": false,
			}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/google", strings.NewReader(`{"id_token":"signed-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileGoogle(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	assertErrorBody(t, w, "Google email is not verified")
}
