package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminAuthenticationAndAuthorization(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", SecureCookies: true}, store)

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

func TestMFAEnrollmentChallengeAndSingleUseRecovery(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", MFAEncryptionKey: "test-mfa-key-at-least-32-bytes!!"}, store)

	initial := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var initialAuth AuthResponse
	if err := json.Unmarshal(initial.Body.Bytes(), &initialAuth); err != nil {
		t.Fatal(err)
	}
	enroll := request(t, router, http.MethodPost, "/auth/mfa/enroll", `{}`, initialAuth.AccessToken)
	if enroll.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, body=%s", enroll.Code, enroll.Body.String())
	}
	var enrollment MFAEnrollmentResponse
	if err := json.Unmarshal(enroll.Body.Bytes(), &enrollment); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirm := request(t, router, http.MethodPost, "/auth/mfa/confirm", `{"code":"`+code+`"}`, initialAuth.AccessToken)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body=%s", confirm.Code, confirm.Body.String())
	}
	var confirmed MFAConfirmationResponse
	if err := json.Unmarshal(confirm.Body.Bytes(), &confirmed); err != nil {
		t.Fatal(err)
	}
	if len(confirmed.RecoveryCodes) != recoveryCodeCount {
		t.Fatalf("recovery code count = %d", len(confirmed.RecoveryCodes))
	}

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if login.Code != http.StatusAccepted {
		t.Fatalf("MFA login status = %d, body=%s", login.Code, login.Body.String())
	}
	var challenge MFAChallengeResponse
	if err := json.Unmarshal(login.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.ChallengeToken == "" {
		t.Fatal("missing challenge token")
	}

	completeCode, _ := totp.GenerateCode(enrollment.Secret, time.Now())
	complete := request(t, router, http.MethodPost, "/auth/mfa/challenge", `{"challenge_token":"`+challenge.ChallengeToken+`","code":"`+completeCode+`"}`, "")
	if complete.Code != http.StatusOK {
		t.Fatalf("TOTP challenge status = %d, body=%s", complete.Code, complete.Body.String())
	}

	recoveryLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if err := json.Unmarshal(recoveryLogin.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	recoveryBody := `{"challenge_token":"` + challenge.ChallengeToken + `","recovery_code":"` + confirmed.RecoveryCodes[0] + `"}`
	recovery := request(t, router, http.MethodPost, "/auth/mfa/challenge", recoveryBody, "")
	if recovery.Code != http.StatusOK {
		t.Fatalf("recovery challenge status = %d, body=%s", recovery.Code, recovery.Body.String())
	}

	reusedLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if err := json.Unmarshal(reusedLogin.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	reused := request(t, router, http.MethodPost, "/auth/mfa/challenge", `{"challenge_token":"`+challenge.ChallengeToken+`","recovery_code":"`+confirmed.RecoveryCodes[0]+`"}`, "")
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("reused recovery status = %d", reused.Code)
	}
}

func TestProductionWritesRequireMFAVerifiedSession(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", MFAEncryptionKey: "test-mfa-key-at-least-32-bytes!!", Environment: "production"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	enroll := request(t, router, http.MethodPost, "/auth/mfa/enroll", `{}`, auth.AccessToken)
	if enroll.Code != http.StatusOK {
		t.Fatalf("production MFA enrollment status = %d", enroll.Code)
	}
	response := request(t, router, http.MethodPost, "/auth/mfa/confirm", `{"code":"invalid"}`, auth.AccessToken)
	if response.Code == http.StatusForbidden {
		t.Fatalf("production MFA confirmation was blocked by policy")
	}
}

func TestRefreshRotatesSessionAndLogoutRevokesIt(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
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
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
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
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
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
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
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
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	response := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status = %d", response.Code)
	}
}

func TestAdministratorListsAndRevokesSessions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	first := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	second := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var firstAuth, secondAuth AuthResponse
	_ = json.Unmarshal(first.Body.Bytes(), &firstAuth)
	_ = json.Unmarshal(second.Body.Bytes(), &secondAuth)

	list := request(t, router, http.MethodGet, "/sessions", "", firstAuth.AccessToken)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "token_hash") || strings.Contains(list.Body.String(), "refresh") {
		t.Fatalf("unsafe sessions response: status=%d body=%s", list.Code, list.Body.String())
	}
	var sessions []SessionResponse
	if err := json.Unmarshal(list.Body.Bytes(), &sessions); err != nil || len(sessions) != 2 {
		t.Fatalf("sessions = %+v, err=%v", sessions, err)
	}
	var current, other SessionResponse
	for _, session := range sessions {
		if session.Current {
			current = session
		} else {
			other = session
		}
	}
	if current.ID == "" || other.ID == "" {
		t.Fatalf("current/other session not identified: %+v", sessions)
	}

	revokeOther := request(t, router, http.MethodDelete, "/sessions/"+other.ID, "", firstAuth.AccessToken)
	if revokeOther.Code != http.StatusNoContent {
		t.Fatalf("revoke other status=%d body=%s", revokeOther.Code, revokeOther.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", secondAuth.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("revoked access status=%d", got.Code)
	}
	if got := request(t, router, http.MethodDelete, "/sessions/"+current.ID, "", firstAuth.AccessToken); got.Code != http.StatusNoContent {
		t.Fatalf("revoke current status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", firstAuth.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("current access after revoke status=%d", got.Code)
	}

	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.AdminID != "admin-1" || last.SessionID != current.ID || last.RequestID != "test-request" || last.ResourceType != "admin_session" || last.ResourceID != current.ID {
		t.Fatalf("incomplete revoke audit: %+v", last)
	}
}

func TestAdministratorRevokesAllOtherSessions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := func() AuthResponse {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth
	}
	current, other := login(), login()

	revoked := request(t, router, http.MethodDelete, "/sessions/others", "", current.AccessToken)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke others status=%d body=%s", revoked.Code, revoked.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", current.AccessToken); got.Code != http.StatusOK {
		t.Fatalf("current session status=%d", got.Code)
	}
	if got := request(t, router, http.MethodGet, "/me", "", other.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("other session status=%d", got.Code)
	}
}

func newTestRouter(cfg Config, store Store) *gin.Engine {
	h := NewAdminHandler(cfg, store)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request")
		c.Next()
	})
	router.POST("/auth/login", h.Login)
	router.POST("/auth/mfa/challenge", h.MFAChallenge)
	router.POST("/auth/refresh", h.Refresh)
	router.DELETE("/auth/logout", h.Logout)
	authed := router.Group("")
	authed.Use(h.RequireAuth)
	authed.GET("/me", h.Me)
	authed.GET("/sessions", h.ListSessions)
	authed.DELETE("/sessions/others", h.RevokeOtherSessions)
	authed.DELETE("/sessions/:id", h.RevokeSession)
	authed.POST("/auth/mfa/enroll", h.EnrollMFA)
	authed.POST("/auth/mfa/confirm", h.ConfirmMFA)
	authed.GET("/dashboard", h.RequirePermission("dashboard.read"), h.Dashboard)
	return router
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
