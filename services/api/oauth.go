package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// ── Provider config ───────────────────────────────────────────────────────────

// OAuthProviderConfig describes a single OAuth2/OIDC provider.
type OAuthProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	// JWKS-based id_token verification (Google, Telegram). Empty = GitHub (no id_token).
	JWKSURL string
	// For OIDC providers: expected iss claim value.
	Issuer string
	// FetchUser extracts a normalised profile from the token response.
	// tokenResp is the raw JSON body returned by the token endpoint.
	FetchUser func(ctx context.Context, httpClient *http.Client, tokenResp tokenResponse) (OAuthProfile, error)
}

// tokenResponse is the parsed response from a provider's token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

// ── Deps ──────────────────────────────────────────────────────────────────────

// OAuthDeps bundles everything the OAuth handlers need.
type OAuthDeps struct {
	DB          *sql.DB
	Redis       *RedisClient
	JWTSecret   string
	FrontendURL string
	HTTPClient  *http.Client
	Providers   map[string]OAuthProviderConfig
}

// ── PKCE helpers ─────────────────────────────────────────────────────────────

func generateCodeVerifier() (string, error) {
	b := make([]byte, 64) // 86-char base64url output → well within 43–128 range
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ── GET /auth/{provider}/url ──────────────────────────────────────────────────

type oauthURLResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

func oauthURLHandler(deps OAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := r.PathValue("provider")
		cfg, ok := deps.Providers[providerName]
		if !ok {
			jsonError(w, "unknown provider", http.StatusNotFound)
			return
		}
		if cfg.ClientID == "" || cfg.RedirectURL == "" {
			jsonError(w, fmt.Sprintf("%s OAuth is not configured", cfg.Name), http.StatusServiceUnavailable)
			return
		}

		state, err := generateState()
		if err != nil {
			log.Printf("oauth url: generate state: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		verifier, err := generateCodeVerifier()
		if err != nil {
			log.Printf("oauth url: generate verifier: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := deps.Redis.StoreOAuthState(ctx, state, verifier, providerName, 10*time.Minute); err != nil {
			log.Printf("oauth url: store state: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		params := url.Values{}
		params.Set("client_id", cfg.ClientID)
		params.Set("redirect_uri", cfg.RedirectURL)
		params.Set("response_type", "code")
		params.Set("state", state)
		params.Set("code_challenge", codeChallenge(verifier))
		params.Set("code_challenge_method", "S256")
		if len(cfg.Scopes) > 0 {
			params.Set("scope", strings.Join(cfg.Scopes, " "))
		}

		authURL := cfg.AuthURL + "?" + params.Encode()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oauthURLResponse{URL: authURL, State: state})
	}
}

// ── POST /auth/{provider}/callback ───────────────────────────────────────────

type oauthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func oauthCallbackHandler(deps OAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := r.PathValue("provider")
		cfg, ok := deps.Providers[providerName]
		if !ok {
			jsonError(w, "unknown provider", http.StatusNotFound)
			return
		}

		var req oauthCallbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Code == "" || req.State == "" {
			jsonError(w, "code and state are required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Validate state + retrieve code_verifier from Redis (one-time)
		codeVerifier, storedProvider, err := deps.Redis.GetAndDeleteOAuthState(ctx, req.State)
		if err != nil {
			log.Printf("oauth callback %s: state validation: %v", providerName, err)
			jsonError(w, "invalid or expired state", http.StatusUnauthorized)
			return
		}
		if storedProvider != providerName {
			jsonError(w, "state provider mismatch", http.StatusUnauthorized)
			return
		}

		// Exchange code + PKCE verifier for tokens
		tokResp, err := exchangeCode(ctx, httpClientOrDefault(deps.HTTPClient), cfg, req.Code, codeVerifier)
		if err != nil {
			log.Printf("oauth callback %s: token exchange: %v", providerName, err)
			jsonError(w, "token exchange failed", http.StatusBadGateway)
			return
		}

		// Fetch normalised profile
		profile, err := cfg.FetchUser(ctx, httpClientOrDefault(deps.HTTPClient), tokResp)
		if err != nil {
			log.Printf("oauth callback %s: fetch user: %v", providerName, err)
			jsonError(w, "profile fetch failed", http.StatusBadGateway)
			return
		}
		profile.Provider = providerName

		// Upsert user + provider record
		user, err := UpsertOAuthUser(deps.DB, profile)
		if err != nil {
			log.Printf("oauth callback %s: upsert: %v", providerName, err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Issue app JWT
		appJWT, err := GenerateUserToken(user.ID.String(), user.DisplayName, deps.JWTSecret)
		if err != nil {
			log.Printf("oauth callback %s: jwt: %v", providerName, err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Issue refresh token → HttpOnly cookie
		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("oauth callback %s: refresh token: %v", providerName, err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := StoreRefreshToken(deps.DB, user.ID, HashRefreshToken(refreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
			log.Printf("oauth callback %s: store refresh token: %v", providerName, err)
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}

		setRefreshCookie(w, refreshToken, r.TLS != nil)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": appJWT})
	}
}

// ── Token exchange ────────────────────────────────────────────────────────────

func exchangeCode(ctx context.Context, client *http.Client, cfg OAuthProviderConfig, code, codeVerifier string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		// GitHub may return form-encoded
		if vals, qerr := url.ParseQuery(string(body)); qerr == nil {
			if v := vals.Get("access_token"); v != "" {
				tok.AccessToken = v
				return tok, nil
			}
			if e := vals.Get("error"); e != "" {
				return tokenResponse{}, fmt.Errorf("token endpoint error: %s", e)
			}
		}
		return tokenResponse{}, fmt.Errorf("invalid token response: %w", err)
	}
	if tok.AccessToken == "" && tok.IDToken == "" {
		return tokenResponse{}, errors.New("token endpoint returned no tokens")
	}
	return tok, nil
}

// ── JWKS id_token verification ────────────────────────────────────────────────

// verifyIDToken fetches JWKS, parses + verifies the id_token JWT, and returns the claims.
func verifyIDToken(ctx context.Context, client *http.Client, jwksURL, issuer, audience, idToken string) (jwt.Token, error) {
	keySet, err := jwk.Fetch(ctx, jwksURL, jwk.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}

	tok, err := jwt.Parse([]byte(idToken),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
	)
	if err != nil {
		return nil, fmt.Errorf("id_token verify: %w", err)
	}
	return tok, nil
}

// ── Google ────────────────────────────────────────────────────────────────────

func googleProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		Scopes:       []string{"openid", "email", "profile"},
		JWKSURL:      "https://www.googleapis.com/oauth2/v3/certs",
		Issuer:       "https://accounts.google.com",
		FetchUser:    fetchGoogleProfile(clientID),
	}
}

func fetchGoogleProfile(clientID string) func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
		if tok.IDToken == "" {
			return OAuthProfile{}, errors.New("google: no id_token in response")
		}
		claims, err := verifyIDToken(ctx, client,
			"https://www.googleapis.com/oauth2/v3/certs",
			"https://accounts.google.com",
			clientID,
			tok.IDToken,
		)
		if err != nil {
			return OAuthProfile{}, fmt.Errorf("google: %w", err)
		}

		sub := claims.Subject()
		if sub == "" {
			return OAuthProfile{}, errors.New("google: missing sub claim")
		}

		email, _ := claims.Get("email")
		emailStr, _ := email.(string)

		name, _ := claims.Get("name")
		displayName, _ := name.(string)
		if displayName == "" {
			displayName = emailStr
		}

		picture, _ := claims.Get("picture")
		avatarURL, _ := picture.(string)

		return OAuthProfile{
			ProviderUserID: sub,
			Email:          strings.ToLower(emailStr),
			DisplayName:    displayName,
			AvatarURL:      avatarURL,
		}, nil
	}
}

// ── GitHub ────────────────────────────────────────────────────────────────────

func githubProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		Scopes:       []string{"read:user", "user:email"},
		// No JWKS — GitHub is plain OAuth 2.0, not OIDC
		FetchUser: fetchGithubProfile(),
	}
}

func fetchGithubProfile() func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
		if tok.AccessToken == "" {
			return OAuthProfile{}, errors.New("github: no access_token")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
		if err != nil {
			return OAuthProfile{}, err
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := client.Do(req)
		if err != nil {
			return OAuthProfile{}, err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return OAuthProfile{}, fmt.Errorf("github /user %d: %s", resp.StatusCode, body)
		}

		var user struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return OAuthProfile{}, err
		}
		if user.ID == 0 {
			return OAuthProfile{}, errors.New("github: missing id")
		}

		email := user.Email
		if email == "" {
			email = fetchGithubPrimaryEmail(ctx, client, tok.AccessToken)
		}

		displayName := user.Name
		if displayName == "" {
			displayName = user.Login
		}

		return OAuthProfile{
			ProviderUserID: fmt.Sprintf("%d", user.ID),
			Email:          strings.ToLower(email),
			DisplayName:    displayName,
			AvatarURL:      user.AvatarURL,
		}, nil
	}
}

func fetchGithubPrimaryEmail(ctx context.Context, client *http.Client, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}

// ── Telegram ──────────────────────────────────────────────────────────────────

func telegramProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         "telegram",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://oauth.telegram.org/auth",
		TokenURL:     "https://oauth.telegram.org/token",
		Scopes:       []string{"openid", "profile"},
		JWKSURL:      "https://oauth.telegram.org/.well-known/jwks.json",
		Issuer:       "https://oauth.telegram.org",
		FetchUser:    fetchTelegramProfile(clientID),
	}
}

func fetchTelegramProfile(botID string) func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (OAuthProfile, error) {
		if tok.IDToken == "" {
			return OAuthProfile{}, errors.New("telegram: no id_token in response")
		}
		claims, err := verifyIDToken(ctx, client,
			"https://oauth.telegram.org/.well-known/jwks.json",
			"https://oauth.telegram.org",
			botID,
			tok.IDToken,
		)
		if err != nil {
			return OAuthProfile{}, fmt.Errorf("telegram: %w", err)
		}

		sub := claims.Subject()
		if sub == "" {
			return OAuthProfile{}, errors.New("telegram: missing sub claim")
		}

		// Build display name from first_name + last_name or preferred_username
		var displayName string
		if fn, _ := claims.Get("first_name"); fn != nil {
			displayName = fmt.Sprintf("%v", fn)
		}
		if ln, _ := claims.Get("last_name"); ln != nil && ln != "" {
			displayName = strings.TrimSpace(displayName + " " + fmt.Sprintf("%v", ln))
		}
		if displayName == "" {
			if un, _ := claims.Get("preferred_username"); un != nil {
				displayName = fmt.Sprintf("%v", un)
			}
		}
		if displayName == "" {
			displayName = "Telegram " + sub
		}
		if len(displayName) > 50 {
			displayName = displayName[:50]
		}

		var avatarURL string
		if pic, _ := claims.Get("picture"); pic != nil {
			avatarURL = fmt.Sprintf("%v", pic)
		}

		return OAuthProfile{
			ProviderUserID: sub,
			// Telegram does not provide email — left empty intentionally
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}, nil
	}
}

// ── Cookie helpers ────────────────────────────────────────────────────────────

const refreshCookieName = "refresh_token"
const refreshCookieMaxAge = 30 * 24 * 60 * 60 // 30 days

func setRefreshCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   refreshCookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ── Misc ──────────────────────────────────────────────────────────────────────

func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: message})
}
