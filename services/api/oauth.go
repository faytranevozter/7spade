package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthProviderConfig describes a single OAuth2 provider (Google or GitHub).
type OAuthProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	// FetchUser exchanges an access token for a normalised OAuthProfile.
	// The Provider field of the returned profile is populated by the handler.
	FetchUser func(ctx context.Context, accessToken string) (OAuthProfile, error)
}

// OAuthDeps bundles the dependencies the OAuth handlers need.
type OAuthDeps struct {
	DB                *sql.DB
	JWTSecret         string
	StateSecret       string
	FrontendURL       string
	HTTPClient        *http.Client
	Now               func() time.Time
	ExchangeCodeToken func(ctx context.Context, cfg OAuthProviderConfig, code string) (string, error)
}

const (
	oauthStateCookie = "oauth_state"
	oauthStateMaxAge = 10 * 60 // 10 minutes
)

// generateRandomState returns a URL-safe random state string.
func generateRandomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// signState produces a deterministic HMAC of the state value using the configured secret.
func signState(state, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(state))
	return hex.EncodeToString(mac.Sum(nil))
}

// oauthStartHandler redirects the user to the provider's consent screen.
func oauthStartHandler(cfg OAuthProviderConfig, deps OAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.ClientID == "" || cfg.RedirectURL == "" {
			http.Error(w, fmt.Sprintf("%s OAuth is not configured", cfg.Name), http.StatusServiceUnavailable)
			return
		}

		state, err := generateRandomState()
		if err != nil {
			log.Printf("oauth: failed to generate state: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		signedState := state + "." + signState(state, deps.StateSecret)

		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie + "_" + cfg.Name,
			Value:    signedState,
			Path:     "/",
			MaxAge:   oauthStateMaxAge,
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})

		params := url.Values{}
		params.Set("client_id", cfg.ClientID)
		params.Set("redirect_uri", cfg.RedirectURL)
		params.Set("response_type", "code")
		params.Set("state", signedState)
		if len(cfg.Scopes) > 0 {
			params.Set("scope", strings.Join(cfg.Scopes, " "))
		}

		http.Redirect(w, r, cfg.AuthURL+"?"+params.Encode(), http.StatusFound)
	}
}

// oauthCallbackHandler validates state, exchanges the code for tokens, fetches the user
// profile, upserts the user, and redirects the SPA to the configured frontend URL with
// the JWT and refresh token in the URL fragment.
func oauthCallbackHandler(cfg OAuthProviderConfig, deps OAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if errParam := query.Get("error"); errParam != "" {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", errParam)
			return
		}

		state := query.Get("state")
		code := query.Get("code")
		if state == "" || code == "" {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "missing_state_or_code")
			return
		}

		cookie, err := r.Cookie(oauthStateCookie + "_" + cfg.Name)
		if err != nil || cookie.Value == "" {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "missing_state_cookie")
			return
		}
		if cookie.Value != state {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "state_mismatch")
			return
		}
		parts := strings.SplitN(state, ".", 2)
		if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(signState(parts[0], deps.StateSecret))) {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "state_invalid")
			return
		}

		// Clear the state cookie now that it has been used.
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie + "_" + cfg.Name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		exchange := deps.ExchangeCodeToken
		if exchange == nil {
			exchange = defaultExchangeCodeToken(deps.HTTPClient)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		accessToken, err := exchange(ctx, cfg, code)
		if err != nil {
			log.Printf("oauth %s: token exchange failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "token_exchange_failed")
			return
		}

		profile, err := cfg.FetchUser(ctx, accessToken)
		if err != nil {
			log.Printf("oauth %s: profile fetch failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "profile_fetch_failed")
			return
		}
		profile.Provider = cfg.Name

		if profile.Email == "" {
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "no_email")
			return
		}

		user, err := UpsertOAuthUser(deps.DB, profile)
		if err != nil {
			log.Printf("oauth %s: upsert failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "upsert_failed")
			return
		}

		jwtToken, err := GenerateUserToken(user.ID.String(), user.DisplayName, deps.JWTSecret)
		if err != nil {
			log.Printf("oauth %s: jwt failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "jwt_failed")
			return
		}

		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("oauth %s: refresh token failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "refresh_failed")
			return
		}
		now := time.Now
		if deps.Now != nil {
			now = deps.Now
		}
		if err := StoreRefreshToken(deps.DB, user.ID, HashRefreshToken(refreshToken), now().Add(30*24*time.Hour)); err != nil {
			log.Printf("oauth %s: store refresh token failed: %v", cfg.Name, err)
			redirectFrontend(w, r, deps.FrontendURL, cfg.Name, "", "", "store_refresh_failed")
			return
		}

		redirectFrontend(w, r, deps.FrontendURL, cfg.Name, jwtToken, refreshToken, "")
	}
}

// redirectFrontend sends the browser back to the SPA with the auth result encoded in the URL fragment.
// Tokens go in the fragment so that they don't appear in server logs or referrers.
func redirectFrontend(w http.ResponseWriter, r *http.Request, frontendURL, provider, jwtToken, refreshToken, errorCode string) {
	target := frontendURL
	if target == "" {
		target = "/"
	}
	if !strings.Contains(target, "/auth/callback") {
		target = strings.TrimRight(target, "/") + "/auth/callback"
	}

	frag := url.Values{}
	frag.Set("provider", provider)
	if jwtToken != "" {
		frag.Set("jwt", jwtToken)
	}
	if refreshToken != "" {
		frag.Set("refresh_token", refreshToken)
	}
	if errorCode != "" {
		frag.Set("error", errorCode)
	}

	http.Redirect(w, r, target+"#"+frag.Encode(), http.StatusFound)
}

// defaultExchangeCodeToken performs the standard OAuth2 authorization-code -> access token
// exchange via POST application/x-www-form-urlencoded.
func defaultExchangeCodeToken(client *http.Client) func(ctx context.Context, cfg OAuthProviderConfig, code string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context, cfg OAuthProviderConfig, code string) (string, error) {
		form := url.Values{}
		form.Set("client_id", cfg.ClientID)
		form.Set("client_secret", cfg.ClientSecret)
		form.Set("code", code)
		form.Set("grant_type", "authorization_code")
		form.Set("redirect_uri", cfg.RedirectURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
		}

		var parsed struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			// GitHub may return application/x-www-form-urlencoded by default; parse that too.
			if values, qerr := url.ParseQuery(string(body)); qerr == nil {
				if tok := values.Get("access_token"); tok != "" {
					return tok, nil
				}
				if e := values.Get("error"); e != "" {
					return "", fmt.Errorf("token endpoint error: %s", e)
				}
			}
			return "", fmt.Errorf("invalid token response: %w", err)
		}
		if parsed.Error != "" {
			return "", fmt.Errorf("token endpoint error: %s", parsed.Error)
		}
		if parsed.AccessToken == "" {
			return "", errors.New("token endpoint returned empty access_token")
		}
		return parsed.AccessToken, nil
	}
}

// googleProvider builds an OAuthProviderConfig for Google with sensible defaults.
func googleProvider(clientID, clientSecret, redirectURL string, httpClient *http.Client) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		Scopes:       []string{"openid", "email", "profile"},
		FetchUser:    fetchGoogleProfile(httpClient),
	}
}

// githubProvider builds an OAuthProviderConfig for GitHub with sensible defaults.
func githubProvider(clientID, clientSecret, redirectURL string, httpClient *http.Client) OAuthProviderConfig {
	return OAuthProviderConfig{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		Scopes:       []string{"read:user", "user:email"},
		FetchUser:    fetchGithubProfile(httpClient),
	}
}

func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// fetchGoogleProfile retrieves the user's profile from Google's userinfo endpoint.
func fetchGoogleProfile(client *http.Client) func(ctx context.Context, accessToken string) (OAuthProfile, error) {
	c := httpClientOrDefault(client)
	return func(ctx context.Context, accessToken string) (OAuthProfile, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
		if err != nil {
			return OAuthProfile{}, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			return OAuthProfile{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return OAuthProfile{}, fmt.Errorf("google userinfo returned %d: %s", resp.StatusCode, string(body))
		}
		var data struct {
			Sub     string `json:"sub"`
			Email   string `json:"email"`
			Name    string `json:"name"`
			Picture string `json:"picture"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return OAuthProfile{}, err
		}
		if data.Sub == "" {
			return OAuthProfile{}, errors.New("google userinfo missing sub")
		}
		display := data.Name
		if display == "" {
			display = data.Email
		}
		return OAuthProfile{
			ProviderUserID: data.Sub,
			Email:          strings.ToLower(data.Email),
			DisplayName:    display,
			AvatarURL:      data.Picture,
		}, nil
	}
}

// fetchGithubProfile retrieves the user's profile (including a primary verified email) from GitHub.
func fetchGithubProfile(client *http.Client) func(ctx context.Context, accessToken string) (OAuthProfile, error) {
	c := httpClientOrDefault(client)
	return func(ctx context.Context, accessToken string) (OAuthProfile, error) {
		// /user
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
		if err != nil {
			return OAuthProfile{}, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := c.Do(req)
		if err != nil {
			return OAuthProfile{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return OAuthProfile{}, fmt.Errorf("github /user returned %d: %s", resp.StatusCode, string(body))
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
			return OAuthProfile{}, errors.New("github /user missing id")
		}

		email := user.Email
		if email == "" {
			// Fetch the user's primary verified email
			req2, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
			if err != nil {
				return OAuthProfile{}, err
			}
			req2.Header.Set("Authorization", "Bearer "+accessToken)
			req2.Header.Set("Accept", "application/vnd.github+json")
			emailResp, err := c.Do(req2)
			if err != nil {
				return OAuthProfile{}, err
			}
			defer emailResp.Body.Close()
			if emailResp.StatusCode >= 200 && emailResp.StatusCode < 300 {
				var emails []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if err := json.NewDecoder(emailResp.Body).Decode(&emails); err == nil {
					for _, e := range emails {
						if e.Primary && e.Verified {
							email = e.Email
							break
						}
					}
					if email == "" {
						for _, e := range emails {
							if e.Verified {
								email = e.Email
								break
							}
						}
					}
				}
			}
		}

		display := user.Name
		if display == "" {
			display = user.Login
		}
		return OAuthProfile{
			ProviderUserID: fmt.Sprintf("%d", user.ID),
			Email:          strings.ToLower(email),
			DisplayName:    display,
			AvatarURL:      user.AvatarURL,
		}, nil
	}
}
