package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeOAuthFetchUser returns a static profile for tests.
func fakeOAuthFetchUser(profile OAuthProfile) func(ctx context.Context, accessToken string) (OAuthProfile, error) {
	return func(ctx context.Context, accessToken string) (OAuthProfile, error) {
		return profile, nil
	}
}

func newTestOAuthConfig(name string, fetch func(ctx context.Context, accessToken string) (OAuthProfile, error)) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         name,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/" + name + "/callback",
		AuthURL:      "https://provider.example.com/authorize",
		TokenURL:     "https://provider.example.com/token",
		Scopes:       []string{"email", "profile"},
		FetchUser:    fetch,
	}
}

func TestOAuthStartRedirectsToProviderWithStateCookie(t *testing.T) {
	cfg := newTestOAuthConfig("google", fakeOAuthFetchUser(OAuthProfile{}))
	deps := OAuthDeps{StateSecret: "state-secret"}

	handler := oauthStartHandler(cfg, deps)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}

	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, cfg.AuthURL+"?") {
		t.Fatalf("expected redirect to provider, got %q", loc)
	}

	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != cfg.ClientID {
		t.Errorf("expected client_id=%q, got %q", cfg.ClientID, q.Get("client_id"))
	}
	if q.Get("redirect_uri") != cfg.RedirectURL {
		t.Errorf("expected redirect_uri=%q, got %q", cfg.RedirectURL, q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("expected response_type=code, got %q", q.Get("response_type"))
	}
	if q.Get("state") == "" {
		t.Error("expected state to be set")
	}

	// Verify a state cookie was set.
	cookies := rr.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == oauthStateCookie+"_google" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("expected oauth_state_google cookie")
	}
	if stateCookie.Value != q.Get("state") {
		t.Errorf("cookie value %q doesn't match state %q", stateCookie.Value, q.Get("state"))
	}
}

func TestOAuthStartReturnsServiceUnavailableIfNotConfigured(t *testing.T) {
	cfg := OAuthProviderConfig{Name: "google"} // missing ClientID/RedirectURL
	deps := OAuthDeps{StateSecret: "state-secret"}

	handler := oauthStartHandler(cfg, deps)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestOAuthCallbackRejectsMissingState(t *testing.T) {
	cfg := newTestOAuthConfig("github", fakeOAuthFetchUser(OAuthProfile{}))
	deps := OAuthDeps{StateSecret: "state-secret", FrontendURL: "http://localhost:5173"}

	handler := oauthCallbackHandler(cfg, deps)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "error=missing_state_or_code") {
		t.Errorf("expected missing_state_or_code in redirect, got %q", loc)
	}
}

func TestOAuthCallbackRejectsStateMismatch(t *testing.T) {
	cfg := newTestOAuthConfig("github", fakeOAuthFetchUser(OAuthProfile{}))
	deps := OAuthDeps{StateSecret: "state-secret", FrontendURL: "http://localhost:5173"}

	handler := oauthCallbackHandler(cfg, deps)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state=foo", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie + "_github", Value: "different.value"})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "error=state_mismatch") {
		t.Errorf("expected state_mismatch in redirect, got %q", loc)
	}
}

func TestOAuthCallbackInvalidStateSignature(t *testing.T) {
	cfg := newTestOAuthConfig("github", fakeOAuthFetchUser(OAuthProfile{}))
	deps := OAuthDeps{StateSecret: "state-secret", FrontendURL: "http://localhost:5173"}

	handler := oauthCallbackHandler(cfg, deps)
	rr := httptest.NewRecorder()
	tampered := "raw." + signState("raw", "wrong-secret")
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state="+tampered, nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie + "_github", Value: tampered})
	handler.ServeHTTP(rr, req)

	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "error=state_invalid") {
		t.Errorf("expected state_invalid in redirect, got %q", loc)
	}
}

func TestOAuthCallbackPropagatesProviderError(t *testing.T) {
	cfg := newTestOAuthConfig("github", fakeOAuthFetchUser(OAuthProfile{}))
	deps := OAuthDeps{StateSecret: "state-secret", FrontendURL: "http://localhost:5173"}

	handler := oauthCallbackHandler(cfg, deps)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?error=access_denied", nil)
	handler.ServeHTTP(rr, req)

	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "error=access_denied") {
		t.Errorf("expected access_denied in redirect, got %q", loc)
	}
}

func TestOAuthCallbackSucceeds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	profile := OAuthProfile{
		ProviderUserID: "12345",
		Email:          "alice@example.com",
		DisplayName:    "Alice",
		AvatarURL:      "https://avatars.example.com/alice.png",
	}
	cfg := newTestOAuthConfig("google", fakeOAuthFetchUser(profile))

	deps := OAuthDeps{
		DB:          db,
		JWTSecret:   "jwt-secret",
		StateSecret: "state-secret",
		FrontendURL: "http://localhost:5173",
		ExchangeCodeToken: func(ctx context.Context, c OAuthProviderConfig, code string) (string, error) {
			return "fake-access-token", nil
		},
	}

	handler := oauthCallbackHandler(cfg, deps)

	// Build a valid state value and matching cookie.
	rawState := "raw-state-value"
	signed := rawState + "." + signState(rawState, deps.StateSecret)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state="+url.QueryEscape(signed), nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie + "_google", Value: signed})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "http://localhost:5173/auth/callback#") {
		t.Fatalf("expected redirect to /auth/callback, got %q", loc)
	}

	// Parse the fragment.
	hashIdx := strings.Index(loc, "#")
	frag, err := url.ParseQuery(loc[hashIdx+1:])
	if err != nil {
		t.Fatalf("parse fragment: %v", err)
	}
	if frag.Get("provider") != "google" {
		t.Errorf("expected provider=google, got %q", frag.Get("provider"))
	}
	if frag.Get("jwt") == "" {
		t.Error("expected jwt in fragment")
	}
	if frag.Get("refresh_token") == "" {
		t.Error("expected refresh_token in fragment")
	}
	if frag.Get("error") != "" {
		t.Errorf("unexpected error: %q", frag.Get("error"))
	}

	// Verify JWT contains the user's display name and is_guest=false.
	claims, err := ParseGuestToken(frag.Get("jwt"), deps.JWTSecret)
	if err != nil {
		t.Fatalf("parse jwt: %v", err)
	}
	if claims.DisplayName != profile.DisplayName {
		t.Errorf("expected display_name=%q, got %q", profile.DisplayName, claims.DisplayName)
	}
	if claims.IsGuest {
		t.Error("expected is_guest=false for OAuth user")
	}

	// Verify the user was inserted with provider info.
	user, err := GetUserByProvider(db, "google", "12345")
	if err != nil {
		t.Fatalf("GetUserByProvider: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to exist")
	}
	if user.Email != profile.Email {
		t.Errorf("expected email=%q, got %q", profile.Email, user.Email)
	}
	if user.PasswordHash.Valid {
		t.Error("expected password_hash to be NULL for OAuth user")
	}
	if !user.AvatarURL.Valid || user.AvatarURL.String != profile.AvatarURL {
		t.Errorf("expected avatar_url=%q, got %+v", profile.AvatarURL, user.AvatarURL)
	}
}

func TestOAuthCallbackUpdatesExistingUserByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Pre-create an email/password user.
	hash, _ := HashPassword("password123")
	existing, err := CreateUser(db, "alice@example.com", hash, "Old Display")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	profile := OAuthProfile{
		ProviderUserID: "12345",
		Email:          "alice@example.com",
		DisplayName:    "Alice From Google",
		AvatarURL:      "https://avatars.example.com/alice.png",
	}
	cfg := newTestOAuthConfig("google", fakeOAuthFetchUser(profile))

	deps := OAuthDeps{
		DB:          db,
		JWTSecret:   "jwt-secret",
		StateSecret: "state-secret",
		FrontendURL: "http://localhost:5173",
		ExchangeCodeToken: func(ctx context.Context, c OAuthProviderConfig, code string) (string, error) {
			return "fake-access-token", nil
		},
	}

	handler := oauthCallbackHandler(cfg, deps)
	rawState := "raw-state-value"
	signed := rawState + "." + signState(rawState, deps.StateSecret)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state="+url.QueryEscape(signed), nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie + "_google", Value: signed})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}

	updated, err := GetUserByID(db, existing.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if updated == nil {
		t.Fatal("expected user to exist")
	}
	if updated.DisplayName != profile.DisplayName {
		t.Errorf("expected display_name=%q, got %q", profile.DisplayName, updated.DisplayName)
	}
	if !updated.Provider.Valid || updated.Provider.String != "google" {
		t.Errorf("expected provider=google, got %+v", updated.Provider)
	}
	if !updated.ProviderUserID.Valid || updated.ProviderUserID.String != "12345" {
		t.Errorf("expected provider_user_id=12345, got %+v", updated.ProviderUserID)
	}
	// Original password hash should still be present so the user can also log in via email/password.
	if !updated.PasswordHash.Valid {
		t.Error("expected password_hash to remain set on existing user")
	}
}
