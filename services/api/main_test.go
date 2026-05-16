package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHealthHandlerReportsDependencyStatus(t *testing.T) {
	handler := healthHandler("api", map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || response.Service != "api" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Dependencies["postgres"] != "ok" || response.Dependencies["redis"] != "ok" {
		t.Fatalf("unexpected dependencies: %+v", response.Dependencies)
	}
}

func TestHealthHandlerReturnsUnavailableWhenDependencyFails(t *testing.T) {
	handler := healthHandler("api", map[string]dependencyCheck{
		"postgres": func(context.Context) error { return errors.New("down") },
		"redis":    func(context.Context) error { return nil },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "degraded" || response.Dependencies["postgres"] != "unreachable" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestGuestHandlerReturnsTokenForValidDisplayName(t *testing.T) {
	secret := "test-secret"
	handler := guestHandler(secret)

	body := bytes.NewBufferString(`{"display_name": "TestUser"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response guestResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify the token is valid
	claims, err := ParseGuestToken(response.Token, secret)
	if err != nil {
		t.Fatalf("failed to parse returned token: %v", err)
	}

	if claims.DisplayName != "TestUser" {
		t.Errorf("expected display_name 'TestUser', got %q", claims.DisplayName)
	}

	if !claims.IsGuest {
		t.Error("expected is_guest to be true")
	}
}

func TestGuestHandlerReturns400ForEmptyDisplayName(t *testing.T) {
	handler := guestHandler("test-secret")

	body := bytes.NewBufferString(`{"display_name": ""}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestGuestHandlerReturns400ForDisplayNameTooLong(t *testing.T) {
	handler := guestHandler("test-secret")

	longName := strings.Repeat("a", 51) // 51 characters
	body := bytes.NewBufferString(`{"display_name": "` + longName + `"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.Contains(response.Error, "50 characters") {
		t.Errorf("expected error about 50 characters, got %q", response.Error)
	}
}

func TestGuestHandlerReturns400ForInvalidJSON(t *testing.T) {
	handler := guestHandler("test-secret")

	body := bytes.NewBufferString(`invalid json`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestGuestHandlerTrimsWhitespaceFromDisplayName(t *testing.T) {
	secret := "test-secret"
	handler := guestHandler(secret)

	body := bytes.NewBufferString(`{"display_name": "  TestUser  "}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response guestResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	claims, err := ParseGuestToken(response.Token, secret)
	if err != nil {
		t.Fatalf("failed to parse returned token: %v", err)
	}

	if claims.DisplayName != "TestUser" {
		t.Errorf("expected trimmed display_name 'TestUser', got %q", claims.DisplayName)
	}
}

// setupTestDB creates a test database connection for integration tests
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Skip if no DATABASE_URL is set
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration tests")
	}

	db, err := InitDB(databaseURL)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Clean up test data before each test
	_, _ = db.Exec("DELETE FROM refresh_tokens")
	_, _ = db.Exec("DELETE FROM users")

	return db
}

func TestRegisterHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"
	handler := registerHandler(db, secret)

	body := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "password123",
		"display_name": "Test User"
	}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/register", body))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	var response authResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.JWT == "" {
		t.Fatal("expected non-empty JWT")
	}

	if response.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}

	// Verify JWT claims
	claims, err := ParseGuestToken(response.JWT, secret)
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}

	if claims.DisplayName != "Test User" {
		t.Errorf("expected display_name 'Test User', got %q", claims.DisplayName)
	}

	if claims.IsGuest {
		t.Error("expected is_guest to be false for registered user")
	}
}

func TestRegisterHandlerRejectsInvalidEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := registerHandler(db, "test-secret")

	tests := []struct {
		name  string
		email string
	}{
		{"missing @", "invalidemail"},
		{"missing domain", "test@"},
		{"missing username", "@example.com"},
		{"invalid format", "test@example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{
				"email": "` + tt.email + `",
				"password": "password123",
				"display_name": "Test User"
			}`)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/register", body))

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
			}
		})
	}
}

func TestRegisterHandlerRejectsWeakPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := registerHandler(db, "test-secret")

	body := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "short",
		"display_name": "Test User"
	}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/register", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.Contains(strings.ToLower(response.Error), "8 characters") {
		t.Errorf("expected error about password length, got %q", response.Error)
	}
}

func TestRegisterHandlerRejectsDuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := registerHandler(db, "test-secret")

	// Register first user
	body1 := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "password123",
		"display_name": "Test User 1"
	}`)
	recorder1 := httptest.NewRecorder()
	handler.ServeHTTP(recorder1, httptest.NewRequest(http.MethodPost, "/register", body1))

	if recorder1.Code != http.StatusCreated {
		t.Fatalf("first registration failed with status %d", recorder1.Code)
	}

	// Try to register with same email
	body2 := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "password456",
		"display_name": "Test User 2"
	}`)
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, httptest.NewRequest(http.MethodPost, "/register", body2))

	if recorder2.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, recorder2.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder2.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.Contains(strings.ToLower(response.Error), "already registered") {
		t.Errorf("expected error about duplicate email, got %q", response.Error)
	}
}

func TestLoginHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"

	// First register a user
	passwordHash, _ := HashPassword("password123")
	_, err := CreateUser(db, "test@example.com", passwordHash, "Test User")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := loginHandler(db, secret)

	body := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "password123"
	}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/login", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response authResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.JWT == "" {
		t.Fatal("expected non-empty JWT")
	}

	if response.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestLoginHandlerRejectsWrongPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Register a user
	passwordHash, _ := HashPassword("password123")
	_, err := CreateUser(db, "test@example.com", passwordHash, "Test User")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := loginHandler(db, "test-secret")

	body := bytes.NewBufferString(`{
		"email": "test@example.com",
		"password": "wrongpassword"
	}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/login", body))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestLoginHandlerRejectsNonExistentUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := loginHandler(db, "test-secret")

	body := bytes.NewBufferString(`{
		"email": "nonexistent@example.com",
		"password": "password123"
	}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/login", body))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestRefreshHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secret := "test-secret"

	// Create a test user
	passwordHash, _ := HashPassword("password123")
	user, err := CreateUser(db, "test@example.com", passwordHash, "Test User")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a refresh token
	refreshToken, _ := GenerateRefreshToken()
	tokenHash := HashRefreshToken(refreshToken)
	expiresAt := testNow().Add(30 * 24 * time.Hour)
	if err := StoreRefreshToken(db, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("failed to store refresh token: %v", err)
	}

	handler := refreshHandler(db, secret)

	body := bytes.NewBufferString(`{"refresh_token": "` + refreshToken + `"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/refresh", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response refreshResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.JWT == "" {
		t.Fatal("expected non-empty JWT")
	}
}

func TestRefreshHandlerRejectsInvalidToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := refreshHandler(db, "test-secret")

	body := bytes.NewBufferString(`{"refresh_token": "invalid-token"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/refresh", body))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestTelegramAuthHandlerAcceptsValidPayloadAndRejectsTamperedPayload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	botToken := "123456:test-bot-token"
	jwtSecret := "test-secret"
	handler := telegramAuthHandler(db, jwtSecret, botToken)
	payload := signedTelegramPayload(t, botToken, map[string]string{
		"id":         "987654321",
		"first_name": "Ada",
		"last_name":  "Lovelace",
		"username":   "ada",
		"auth_date":  strconv.FormatInt(time.Now().Unix(), 10),
	})

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/auth/telegram", bytes.NewReader(body)))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response authResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.JWT == "" || response.RefreshToken == "" {
		t.Fatalf("expected jwt and refresh token, got %+v", response)
	}
	claims, err := ParseGuestToken(response.JWT, jwtSecret)
	if err != nil {
		t.Fatalf("parse jwt: %v", err)
	}
	if claims.DisplayName != "Ada Lovelace" || claims.IsGuest {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	payload["first_name"] = "Grace"
	tamperedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal tampered payload: %v", err)
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/auth/telegram", bytes.NewReader(tamperedBody)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func signedTelegramPayload(t *testing.T, botToken string, data map[string]string) map[string]string {
	t.Helper()
	checkParts := make([]string, 0, len(data))
	for key, value := range data {
		checkParts = append(checkParts, key+"="+value)
	}
	sort.Strings(checkParts)
	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(checkParts, "\n")))

	payload := make(map[string]string, len(data)+1)
	for key, value := range data {
		payload[key] = value
	}
	payload["hash"] = hex.EncodeToString(mac.Sum(nil))
	return payload
}
