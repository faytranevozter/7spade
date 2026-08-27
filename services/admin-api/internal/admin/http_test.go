package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminAuthenticationAndAuthorization(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", SecureCookies: true}, store)

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body=%s", login.Code, login.Body.String())
	}
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken == "" || auth.Admin.Email != "ops@example.com" {
		t.Fatalf("unexpected login response: %+v", auth)
	}
	cookies := login.Result().Cookies()
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/auth" {
		t.Fatalf("insecure refresh cookie: %+v", cookie)
	}
	csrf := cookies[1]
	if csrf.HttpOnly || !csrf.Secure || csrf.SameSite != http.SameSiteStrictMode || csrf.Path != "/" {
		t.Fatalf("invalid CSRF cookie: %+v", csrf)
	}

	dashboard := request(t, router, http.MethodGet, "/dashboard", "", auth.AccessToken)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body=%s", dashboard.Code, dashboard.Body.String())
	}

	store.SetPermissions("admin-1", nil)
	forbidden := request(t, router, http.MethodGet, "/dashboard", "", auth.AccessToken)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("dashboard without permission = %d", forbidden.Code)
	}
}

func TestRefreshRotatesSessionAndLogoutRevokesIt(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	oldCookie := login.Result().Cookies()[0]

	refreshReq := csrfRequest(http.MethodPost, "/auth/refresh", oldCookie, login.Result().Cookies()[1])
	refresh := httptest.NewRecorder()
	router.ServeHTTP(refresh, refreshReq)
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body=%s", refresh.Code, refresh.Body.String())
	}
	newCookie := refresh.Result().Cookies()[0]
	if newCookie.Value == oldCookie.Value {
		t.Fatal("refresh token was not rotated")
	}

	replayReq := csrfRequest(http.MethodPost, "/auth/refresh", oldCookie, login.Result().Cookies()[1])
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, replayReq)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh status = %d", replay.Code)
	}
	familyReq := csrfRequest(http.MethodPost, "/auth/refresh", newCookie, refresh.Result().Cookies()[1])
	family := httptest.NewRecorder()
	router.ServeHTTP(family, familyReq)
	if family.Code != http.StatusUnauthorized {
		t.Fatalf("token family after replay status = %d", family.Code)
	}

	logoutReq := csrfRequest(http.MethodDelete, "/auth/logout", newCookie, refresh.Result().Cookies()[1])
	logout := httptest.NewRecorder()
	router.ServeHTTP(logout, logoutReq)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.Code)
	}

	revokedReq := csrfRequest(http.MethodPost, "/auth/refresh", newCookie, refresh.Result().Cookies()[1])
	revoked := httptest.NewRecorder()
	router.ServeHTTP(revoked, revokedReq)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked refresh status = %d", revoked.Code)
	}
	var refreshed AuthResponse
	if err := json.Unmarshal(refresh.Body.Bytes(), &refreshed); err != nil {
		t.Fatal(err)
	}
	afterLogout := request(t, router, http.MethodGet, "/dashboard", "", refreshed.AccessToken)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("access token after logout status = %d", afterLogout.Code)
	}
	if len(store.AuditEvents()) < 3 {
		t.Fatalf("expected authentication audit events, got %d", len(store.AuditEvents()))
	}
}

func TestRefreshAndLogoutRequireCSRFToken(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	refreshCookie := login.Result().Cookies()[0]

	for _, testCase := range []struct{ method, path string }{{http.MethodPost, "/auth/refresh"}, {http.MethodDelete, "/auth/logout"}} {
		req := httptest.NewRequest(testCase.method, testCase.path, nil)
		req.AddCookie(refreshCookie)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s without CSRF status = %d", testCase.path, response.Code)
		}
	}
}

func TestLoginValidationAndThrottling(t *testing.T) {
	store := NewMemoryStore()
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	invalid := request(t, router, http.MethodPost, "/auth/login", `{"email":"not-an-email","password":""}`, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid login status = %d", invalid.Code)
	}
	for attempt := 0; attempt < 4; attempt++ {
		request(t, router, http.MethodPost, "/auth/login", `{"email":"none@example.com","password":"wrong"}`, "")
	}
	throttled := request(t, router, http.MethodPost, "/auth/login", `{"email":"none@example.com","password":"wrong"}`, "")
	if throttled.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled login status = %d", throttled.Code)
	}
}

func TestAdminAPIRejectsWrongIssuerAndAudience(t *testing.T) {
	store := NewMemoryStore(Admin{ID: "admin-1", Status: "active"})
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	for _, claims := range []jwt.RegisteredClaims{
		{Subject: "admin-1", Issuer: "seven-spade-player", Audience: jwt.ClaimStrings{"admin-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Subject: "admin-1", Issuer: "seven-spade-admin", Audience: jwt.ClaimStrings{"player-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	} {
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims{SessionID: "session", RegisteredClaims: claims}).SignedString([]byte("test-secret-at-least-32-bytes-long"))
		response := request(t, router, http.MethodGet, "/me", "", token)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("wrong token status = %d", response.Code)
		}
	}
}

func TestDisabledAdminCannotLogin(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "disabled"})
	router := NewRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	response := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status = %d", response.Code)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func csrfRequest(method, path string, refresh, csrf *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(refresh)
	req.AddCookie(csrf)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	return req
}
