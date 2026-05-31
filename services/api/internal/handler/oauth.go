package handler

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

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type OAuthProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	JWKSURL      string
	Issuer       string
	FetchUser    func(ctx context.Context, httpClient *http.Client, tokenResp tokenResponse) (repository.OAuthProfile, error)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

type OAuthHandler struct {
	DB          *sql.DB
	Redis       *cache.RedisClient
	JWTSecret   string
	FrontendURL string
	HTTPClient  *http.Client
	Providers   map[string]OAuthProviderConfig
}

type oauthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func NewOAuthHandler(db *sql.DB, rdb *cache.RedisClient, cfg *config.Config) OAuthHandler {
	return OAuthHandler{
		DB:          db,
		Redis:       rdb,
		JWTSecret:   cfg.JWTSecret,
		FrontendURL: cfg.FrontendURL,
		Providers: map[string]OAuthProviderConfig{
			"google":   googleProvider(cfg.GoogleOAuth.ClientID, cfg.GoogleOAuth.ClientSecret, cfg.GoogleOAuth.RedirectURL),
			"github":   githubProvider(cfg.GitHubOAuth.ClientID, cfg.GitHubOAuth.ClientSecret, cfg.GitHubOAuth.RedirectURL),
			"telegram": telegramProvider(cfg.TelegramOAuth.ClientID, cfg.TelegramOAuth.ClientSecret, cfg.TelegramOAuth.RedirectURL),
		},
	}
}

func (h OAuthHandler) URL(c *gin.Context) {
	providerName := c.Param("provider")
	cfg, ok := h.Providers[providerName]
	if !ok {
		JSONError(c, http.StatusNotFound, "unknown provider")
		return
	}
	if cfg.ClientID == "" || cfg.RedirectURL == "" {
		JSONError(c, http.StatusServiceUnavailable, fmt.Sprintf("%s OAuth is not configured", cfg.Name))
		return
	}
	state, err := generateState()
	if err != nil {
		log.Printf("oauth url: generate state: %v", err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	verifier, err := generateCodeVerifier()
	if err != nil {
		log.Printf("oauth url: generate verifier: %v", err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Redis.StoreOAuthState(ctx, state, verifier, providerName, 10*time.Minute); err != nil {
		log.Printf("oauth url: store state: %v", err)
		JSONError(c, http.StatusInternalServerError, "internal error")
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
	c.JSON(http.StatusOK, gin.H{"url": cfg.AuthURL + "?" + params.Encode(), "state": state})
}

func (h OAuthHandler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	cfg, ok := h.Providers[providerName]
	if !ok {
		JSONError(c, http.StatusNotFound, "unknown provider")
		return
	}
	var req oauthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" || req.State == "" {
		JSONError(c, http.StatusBadRequest, "code and state are required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	codeVerifier, storedProvider, err := h.Redis.GetAndDeleteOAuthState(ctx, req.State)
	if err != nil {
		log.Printf("oauth callback %s: state validation: %v", providerName, err)
		JSONError(c, http.StatusUnauthorized, "invalid or expired state")
		return
	}
	if storedProvider != providerName {
		JSONError(c, http.StatusUnauthorized, "state provider mismatch")
		return
	}
	tokResp, err := exchangeCode(ctx, httpClientOrDefault(h.HTTPClient), cfg, req.Code, codeVerifier)
	if err != nil {
		log.Printf("oauth callback %s: token exchange: %v", providerName, err)
		JSONError(c, http.StatusBadGateway, "token exchange failed")
		return
	}
	profile, err := cfg.FetchUser(ctx, httpClientOrDefault(h.HTTPClient), tokResp)
	if err != nil {
		log.Printf("oauth callback %s: fetch user: %v", providerName, err)
		JSONError(c, http.StatusBadGateway, "profile fetch failed")
		return
	}
	profile.Provider = providerName
	user, err := repository.UpsertOAuthUser(h.DB, profile)
	if err != nil {
		log.Printf("oauth callback %s: upsert: %v", providerName, err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	appJWT, err := auth.GenerateUserToken(user.ID.String(), user.DisplayName, profile.AvatarURL, h.JWTSecret)
	if err != nil {
		log.Printf("oauth callback %s: jwt: %v", providerName, err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		log.Printf("oauth callback %s: refresh token: %v", providerName, err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	if err := repository.StoreRefreshToken(h.DB, user.ID, auth.HashRefreshToken(refreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
		log.Printf("oauth callback %s: store refresh token: %v", providerName, err)
		JSONError(c, http.StatusInternalServerError, "internal error")
		return
	}
	SetRefreshCookie(c, refreshToken)
	c.JSON(http.StatusOK, gin.H{"access_token": appJWT})
}

func generateCodeVerifier() (string, error) {
	b := make([]byte, 64)
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

func verifyIDToken(ctx context.Context, client *http.Client, jwksURL, issuer, audience, idToken string) (jwt.Token, error) {
	keySet, err := jwk.Fetch(ctx, jwksURL, jwk.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}
	tok, err := jwt.Parse([]byte(idToken), jwt.WithKeySet(keySet), jwt.WithValidate(true), jwt.WithIssuer(issuer), jwt.WithAudience(audience))
	if err != nil {
		return nil, fmt.Errorf("id_token verify: %w", err)
	}
	return tok, nil
}

func googleProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{Name: "google", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", Scopes: []string{"openid", "email", "profile"}, JWKSURL: "https://www.googleapis.com/oauth2/v3/certs", Issuer: "https://accounts.google.com", FetchUser: fetchGoogleProfile(clientID)}
}

func fetchGoogleProfile(clientID string) func(context.Context, *http.Client, tokenResponse) (repository.OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (repository.OAuthProfile, error) {
		if tok.IDToken == "" {
			return repository.OAuthProfile{}, errors.New("google: no id_token in response")
		}
		claims, err := verifyIDToken(ctx, client, "https://www.googleapis.com/oauth2/v3/certs", "https://accounts.google.com", clientID, tok.IDToken)
		if err != nil {
			return repository.OAuthProfile{}, fmt.Errorf("google: %w", err)
		}
		sub := claims.Subject()
		if sub == "" {
			return repository.OAuthProfile{}, errors.New("google: missing sub claim")
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
		return repository.OAuthProfile{ProviderUserID: sub, Email: strings.ToLower(emailStr), DisplayName: displayName, AvatarURL: avatarURL}, nil
	}
}

func githubProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{Name: "github", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", Scopes: []string{"read:user", "user:email"}, FetchUser: fetchGithubProfile()}
}

func fetchGithubProfile() func(context.Context, *http.Client, tokenResponse) (repository.OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (repository.OAuthProfile, error) {
		if tok.AccessToken == "" {
			return repository.OAuthProfile{}, errors.New("github: no access_token")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
		if err != nil {
			return repository.OAuthProfile{}, err
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil {
			return repository.OAuthProfile{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return repository.OAuthProfile{}, fmt.Errorf("github /user %d: %s", resp.StatusCode, body)
		}
		var user struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return repository.OAuthProfile{}, err
		}
		if user.ID == 0 {
			return repository.OAuthProfile{}, errors.New("github: missing id")
		}
		email := user.Email
		if email == "" {
			email = fetchGithubPrimaryEmail(ctx, client, tok.AccessToken)
		}
		displayName := user.Name
		if displayName == "" {
			displayName = user.Login
		}
		return repository.OAuthProfile{ProviderUserID: fmt.Sprintf("%d", user.ID), Email: strings.ToLower(email), DisplayName: displayName, Username: user.Login, AvatarURL: user.AvatarURL}, nil
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

func telegramProvider(clientID, clientSecret, redirectURL string) OAuthProviderConfig {
	return OAuthProviderConfig{Name: "telegram", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, AuthURL: "https://oauth.telegram.org/auth", TokenURL: "https://oauth.telegram.org/token", Scopes: []string{"openid", "profile"}, JWKSURL: "https://oauth.telegram.org/.well-known/jwks.json", Issuer: "https://oauth.telegram.org", FetchUser: fetchTelegramProfile(clientID)}
}

func fetchTelegramProfile(botID string) func(context.Context, *http.Client, tokenResponse) (repository.OAuthProfile, error) {
	return func(ctx context.Context, client *http.Client, tok tokenResponse) (repository.OAuthProfile, error) {
		if tok.IDToken == "" {
			return repository.OAuthProfile{}, errors.New("telegram: no id_token in response")
		}
		claims, err := verifyIDToken(ctx, client, "https://oauth.telegram.org/.well-known/jwks.json", "https://oauth.telegram.org", botID, tok.IDToken)
		if err != nil {
			return repository.OAuthProfile{}, fmt.Errorf("telegram: %w", err)
		}
		sub := claims.Subject()
		if sub == "" {
			return repository.OAuthProfile{}, errors.New("telegram: missing sub claim")
		}
		var displayName string
		if fn, _ := claims.Get("first_name"); fn != nil {
			displayName = fmt.Sprintf("%v", fn)
		}
		if ln, _ := claims.Get("last_name"); ln != nil && ln != "" {
			displayName = strings.TrimSpace(displayName + " " + fmt.Sprintf("%v", ln))
		}
		var username string
		if un, _ := claims.Get("preferred_username"); un != nil {
			username = fmt.Sprintf("%v", un)
		}
		if displayName == "" {
			displayName = username
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
		return repository.OAuthProfile{ProviderUserID: sub, DisplayName: displayName, Username: username, AvatarURL: avatarURL}, nil
	}
}

func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 10 * time.Second}
}
