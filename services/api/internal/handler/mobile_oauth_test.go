package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/gin-gonic/gin"
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

func TestMobileTelegramStartReturnsProviderURL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb, err := cache.New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	defer rdb.Close()
	h := OAuthHandler{Redis: rdb, Providers: map[string]OAuthProviderConfig{
		"telegram": {
			ClientID: "bot-id", RedirectURL: "https://api.example.com/auth/mobile/telegram/callback",
			AuthURL: "https://oauth.telegram.org/auth", Scopes: []string{"openid", "profile"},
		},
	}, TelegramMobileRedirectURL: "https://api.example.com/auth/mobile/telegram/callback"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram/start", strings.NewReader(`{"redirect_uri":"sevenspade://spade/auth/callback","code_challenge":"app-proof"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.MobileTelegramStart(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct{ URL, State string }
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	authURL, err := url.Parse(body.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if got := authURL.Query().Get("redirect_uri"); got != "https://api.example.com/auth/mobile/telegram/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
	if authURL.Query().Get("nonce") == "" || authURL.Query().Get("code_challenge") == "" || body.State == "" {
		t.Fatalf("authorization URL missing security parameters: %s", body.URL)
	}
	_, nonce, challenge, redirectURI, err := rdb.GetAndDeleteMobileOAuthState(context.Background(), body.State)
	if err != nil || nonce == "" || challenge != "app-proof" || redirectURI != "sevenspade://spade/auth/callback" {
		t.Fatalf("stored state = nonce %q challenge %q redirect %q err %v", nonce, challenge, redirectURI, err)
	}
}

func TestMobileTelegramExchangeRejectsWrongVerifierAndConsumesCode(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb, err := cache.New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	defer rdb.Close()
	profile, _ := json.Marshal(map[string]string{"provider": "telegram", "provider_user_id": "42"})
	if err := rdb.StoreMobileOAuthHandoff(context.Background(), "one-time", codeChallenge("correct"), profile, time.Minute); err != nil {
		t.Fatalf("store handoff: %v", err)
	}
	h := OAuthHandler{Redis: rdb}
	for attempt := 1; attempt <= 2; attempt++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram/exchange", strings.NewReader(`{"code":"one-time","code_verifier":"wrong"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.MobileTelegramExchange(c)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", attempt, w.Code, http.StatusUnauthorized)
		}
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
