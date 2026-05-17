package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeRedis is an in-memory RedisClient substitute for tests.
type fakeRedis struct {
	data map[string]fakeEntry
}

type fakeEntry struct {
	codeVerifier string
	provider     string
}

func newFakeRedis() *RedisClient {
	// We can't directly replace RedisClient fields, so use a thin test-double
	// by embedding a real client backed by a miniredis server… or just skip
	// integration-style tests that require Redis and unit-test what we can.
	return nil
}

// ── Unit tests that don't need Redis ─────────────────────────────────────────

func TestCodeChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// RFC 7636 known-good: SHA256(verifier) base64url-no-padding
	// = E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := codeChallenge(verifier)
	if got != want {
		t.Errorf("codeChallenge(%q) = %q, want %q", verifier, got, want)
	}
}

func TestGenerateCodeVerifierLength(t *testing.T) {
	v, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("generateCodeVerifier: %v", err)
	}
	// base64url without padding: 64 bytes → 86 chars (well within 43–128 PKCE range)
	if len(v) < 43 || len(v) > 128 {
		t.Errorf("verifier length %d not in [43,128]", len(v))
	}
}

func TestGoogleProviderConfig(t *testing.T) {
	cfg := googleProvider("my-client-id", "my-secret", "https://example.com/callback")
	if cfg.AuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Errorf("unexpected AuthURL: %s", cfg.AuthURL)
	}
	if cfg.JWKSURL != "https://www.googleapis.com/oauth2/v3/certs" {
		t.Errorf("unexpected JWKSURL: %s", cfg.JWKSURL)
	}
	if cfg.Issuer != "https://accounts.google.com" {
		t.Errorf("unexpected Issuer: %s", cfg.Issuer)
	}
}

func TestGithubProviderConfig(t *testing.T) {
	cfg := githubProvider("id", "secret", "https://example.com/callback")
	if cfg.AuthURL != "https://github.com/login/oauth/authorize" {
		t.Errorf("unexpected AuthURL: %s", cfg.AuthURL)
	}
	if cfg.JWKSURL != "" {
		t.Errorf("GitHub should have no JWKS URL, got %q", cfg.JWKSURL)
	}
}

func TestTelegramProviderConfig(t *testing.T) {
	cfg := telegramProvider("bot-id", "bot-secret", "https://example.com/callback")
	if cfg.AuthURL != "https://oauth.telegram.org/auth" {
		t.Errorf("unexpected AuthURL: %s", cfg.AuthURL)
	}
	if cfg.JWKSURL != "https://oauth.telegram.org/.well-known/jwks.json" {
		t.Errorf("unexpected JWKSURL: %s", cfg.JWKSURL)
	}
	if cfg.Issuer != "https://oauth.telegram.org" {
		t.Errorf("unexpected Issuer: %s", cfg.Issuer)
	}
}

// ── oauthURLHandler (requires Redis; uses a real fake server via miniredis or skip) ──

func TestOAuthURLHandlerReturnsUnknownProviderFor404(t *testing.T) {
	deps := OAuthDeps{
		Providers: map[string]OAuthProviderConfig{
			"google": googleProvider("cid", "csec", "http://localhost/cb"),
		},
		// Redis is nil; handler returns 404 before touching Redis
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// simulate path routing
		r.SetPathValue("provider", "notexist")
		oauthURLHandler(deps)(w, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestOAuthURLHandlerServiceUnavailableWhenUnconfigured(t *testing.T) {
	deps := OAuthDeps{
		Providers: map[string]OAuthProviderConfig{
			"google": googleProvider("", "", ""), // empty = unconfigured
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("provider", "google")
		oauthURLHandler(deps)(w, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

// ── oauthCallbackHandler ─────────────────────────────────────────────────────

func TestOAuthCallbackRejectsMissingBody(t *testing.T) {
	deps := OAuthDeps{
		Providers: map[string]OAuthProviderConfig{
			"github": githubProvider("cid", "csec", "http://localhost/cb"),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("provider", "github")
		oauthCallbackHandler(deps)(w, r)
	}))
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty code/state, got %d", resp.StatusCode)
	}
}

func TestOAuthCallbackRejectsUnknownProvider(t *testing.T) {
	deps := OAuthDeps{
		Providers: map[string]OAuthProviderConfig{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("provider", "facebook")
		oauthCallbackHandler(deps)(w, r)
	}))
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(`{"code":"x","state":"y"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// ── UpsertOAuthUser ───────────────────────────────────────────────────────────

func TestUpsertOAuthUserCreatesNewUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	profile := OAuthProfile{
		Provider:       "google",
		ProviderUserID: "g-111",
		Email:          "new@example.com",
		DisplayName:    "New User",
		AvatarURL:      "https://avatars.example.com/new.png",
	}

	user, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if user.DisplayName != "New User" {
		t.Errorf("expected display_name=%q, got %q", "New User", user.DisplayName)
	}
	if user.Email.String != "new@example.com" {
		t.Errorf("expected email=%q, got %q", "new@example.com", user.Email.String)
	}
	if user.PasswordHash.Valid {
		t.Error("expected no password_hash for OAuth user")
	}

	// Provider row must exist
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_providers WHERE provider=$1 AND provider_id=$2`, "google", "g-111").Scan(&count); err != nil {
		t.Fatalf("query user_providers: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user_providers row, got %d", count)
	}
}

func TestUpsertOAuthUserUpdatesExistingUserByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	hash, _ := HashPassword("password123")
	existing, err := CreateUser(db, "alice@example.com", hash, "Old Name")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	profile := OAuthProfile{
		Provider:       "google",
		ProviderUserID: "g-alice",
		Email:          "alice@example.com",
		DisplayName:    "Alice From Google",
	}

	user, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if user.ID != existing.ID {
		t.Errorf("expected same user ID, got new one")
	}
	if user.DisplayName != "Alice From Google" {
		t.Errorf("expected updated display_name, got %q", user.DisplayName)
	}
	// Password hash must be preserved
	if !user.PasswordHash.Valid {
		t.Error("expected password_hash to remain set on existing user")
	}
}

func TestUpsertOAuthUserTelegramNoEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	profile := OAuthProfile{
		Provider:       "telegram",
		ProviderUserID: "tg-999",
		// Email intentionally empty
		DisplayName: "Telegram User",
	}

	user, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if user.Email.Valid {
		t.Errorf("expected no email for Telegram user, got %q", user.Email.String)
	}
	if user.DisplayName != "Telegram User" {
		t.Errorf("expected display_name=%q, got %q", "Telegram User", user.DisplayName)
	}
}

func TestUpsertOAuthUserIdempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	profile := OAuthProfile{
		Provider:       "github",
		ProviderUserID: "gh-777",
		Email:          "bob@example.com",
		DisplayName:    "Bob",
	}

	first, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("first UpsertOAuthUser: %v", err)
	}

	profile.DisplayName = "Bob Updated"
	second, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("second UpsertOAuthUser: %v", err)
	}

	if first.ID != second.ID {
		t.Error("expected same user on second upsert")
	}
	if second.DisplayName != "Bob Updated" {
		t.Errorf("expected updated display name, got %q", second.DisplayName)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_providers WHERE user_id=$1`, first.ID).Scan(&count); err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 provider row (idempotent), got %d", count)
	}
}

// ── setRefreshCookie / clearRefreshCookie ─────────────────────────────────────

func TestSetRefreshCookieAttributes(t *testing.T) {
	rr := httptest.NewRecorder()
	setRefreshCookie(rr, "my-token", false)

	cookies := rr.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == refreshCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected refresh_token cookie")
	}
	if found.Value != "my-token" {
		t.Errorf("expected value=my-token, got %q", found.Value)
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if found.MaxAge <= 0 {
		t.Errorf("expected positive MaxAge, got %d", found.MaxAge)
	}
	if found.Path != "/" {
		t.Errorf("expected Path=/, got %q", found.Path)
	}
}

func TestClearRefreshCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	clearRefreshCookie(rr)

	cookies := rr.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == refreshCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected refresh_token cookie in clear response")
	}
	if found.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 to clear cookie, got %d", found.MaxAge)
	}
}

// ── exchangeCode – test that code_verifier is included in token request ───────

func TestExchangeCodeIncludesCodeVerifier(t *testing.T) {
	var capturedBody url.Values

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		capturedBody = r.Form
		_ = json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok-123"})
	}))
	defer ts.Close()

	cfg := OAuthProviderConfig{
		ClientID:     "cid",
		ClientSecret: "csec",
		RedirectURL:  "https://example.com/callback",
		TokenURL:     ts.URL,
	}

	tok, err := exchangeCode(context.Background(), ts.Client(), cfg, "auth-code", "my-verifier")
	if err != nil {
		t.Fatalf("exchangeCode: %v", err)
	}
	if tok.AccessToken != "tok-123" {
		t.Errorf("unexpected access_token: %q", tok.AccessToken)
	}
	if capturedBody.Get("code_verifier") != "my-verifier" {
		t.Errorf("expected code_verifier=my-verifier in request, got %q", capturedBody.Get("code_verifier"))
	}
	if capturedBody.Get("code") != "auth-code" {
		t.Errorf("expected code=auth-code, got %q", capturedBody.Get("code"))
	}
}
