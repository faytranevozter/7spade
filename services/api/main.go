package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type dependencyCheck func(context.Context) error

type healthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies"`
}

type guestRequest struct {
	DisplayName string `json:"display_name"`
}

type guestResponse struct {
	Token string `json:"token"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	JWT          string `json:"jwt"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type refreshResponse struct {
	JWT string `json:"jwt"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	cfg := LoadConfig()

	db, err := InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	rdb, err := NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to create Redis client: %v", err)
	}
	defer rdb.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("api", map[string]dependencyCheck{
		"postgres": postgresCheck(cfg.DatabaseURL),
		"redis":    redisCheck(cfg.RedisURL),
	}))
	mux.HandleFunc("POST /guest", guestHandler(cfg.JWTSecret))
	mux.HandleFunc("POST /register", registerHandler(db, cfg.JWTSecret))
	mux.HandleFunc("POST /login", loginHandler(db, cfg.JWTSecret))
	mux.HandleFunc("POST /refresh", refreshHandler(db, cfg.JWTSecret))
	mux.HandleFunc("DELETE /auth/logout", logoutHandler(db))
	mux.HandleFunc("POST /internal/games", saveGameHandler(db))

	mux.HandleFunc("POST /rooms", requireAuth(cfg.JWTSecret, createRoomHandler(db)))
	mux.HandleFunc("GET /rooms", listPublicRoomsHandler(db))
	mux.HandleFunc("POST /rooms/{code}/join", requireAuth(cfg.JWTSecret, joinRoomHandler(db)))
	mux.HandleFunc("GET /rooms/{id}", getRoomHandler(db))
	mux.HandleFunc("GET /history", requireAuth(cfg.JWTSecret, historyHandler(db)))

	registerOAuthRoutes(mux, db, rdb, cfg)

	log.Printf("API service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(service string, checks map[string]dependencyCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		statusCode := http.StatusOK
		deps := make(map[string]string, len(checks))
		for name, check := range checks {
			if err := check(ctx); err != nil {
				deps[name] = "unreachable"
				statusCode = http.StatusServiceUnavailable
				continue
			}
			deps[name] = "ok"
		}

		status := "ok"
		if statusCode != http.StatusOK {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(healthResponse{Status: status, Service: service, Dependencies: deps})
	}
}

func guestHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req guestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" {
			jsonError(w, "Display name is required", http.StatusBadRequest)
			return
		}
		if len(displayName) > 50 {
			jsonError(w, "Display name must be 50 characters or less", http.StatusBadRequest)
			return
		}

		token, err := GenerateGuestToken(displayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate guest token: %v", err)
			jsonError(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(guestResponse{Token: token})
	}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func registerHandler(db *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if !emailRegex.MatchString(email) {
			jsonError(w, "Invalid email format", http.StatusBadRequest)
			return
		}
		if len(req.Password) < 8 {
			jsonError(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" || len(displayName) > 50 {
			jsonError(w, "Display name must be 1-50 characters", http.StatusBadRequest)
			return
		}

		existingUser, err := GetUserByEmail(db, email)
		if err != nil {
			log.Printf("Failed to check existing user: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if existingUser != nil {
			jsonError(w, "Email already registered", http.StatusConflict)
			return
		}

		passwordHash, err := HashPassword(req.Password)
		if err != nil {
			log.Printf("Failed to hash password: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		user, err := CreateUser(db, email, passwordHash, displayName)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		jwtToken, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := StoreRefreshToken(db, user.ID, HashRefreshToken(refreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		setRefreshCookie(w, refreshToken, r.TLS != nil)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(authResponse{JWT: jwtToken})
	}
}

func loginHandler(db *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))

		user, err := GetUserByEmail(db, email)
		if err != nil {
			log.Printf("Failed to get user: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if user == nil || !user.PasswordHash.Valid || ComparePassword(user.PasswordHash.String, req.Password) != nil {
			jsonError(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		jwtToken, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := StoreRefreshToken(db, user.ID, HashRefreshToken(refreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		setRefreshCookie(w, refreshToken, r.TLS != nil)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(authResponse{JWT: jwtToken})
	}
}

func refreshHandler(db *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(refreshCookieName)
		if err != nil || cookie.Value == "" {
			jsonError(w, "Missing refresh token", http.StatusUnauthorized)
			return
		}

		tokenHash := HashRefreshToken(cookie.Value)
		userID, err := ValidateRefreshToken(db, tokenHash)
		if err != nil {
			jsonError(w, "Invalid or expired refresh token", http.StatusUnauthorized)
			return
		}

		// Rotate: revoke old, issue new
		if err := RevokeRefreshToken(db, tokenHash); err != nil {
			log.Printf("Failed to revoke old refresh token: %v", err)
		}

		user, err := GetUserByID(db, userID)
		if err != nil {
			log.Printf("Failed to get user: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			jsonError(w, "User not found", http.StatusUnauthorized)
			return
		}

		newRefreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := StoreRefreshToken(db, user.ID, HashRefreshToken(newRefreshToken), time.Now().Add(30*24*time.Hour)); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		setRefreshCookie(w, newRefreshToken, r.TLS != nil)

		jwtToken, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(refreshResponse{JWT: jwtToken})
	}
}

func logoutHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(refreshCookieName)
		if err == nil && cookie.Value != "" {
			tokenHash := HashRefreshToken(cookie.Value)
			if err := RevokeRefreshToken(db, tokenHash); err != nil {
				log.Printf("Failed to revoke refresh token on logout: %v", err)
			}
		}
		clearRefreshCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postgresCheck(databaseURL string) dependencyCheck {
	return tcpURLCheck(databaseURL)
}

func redisCheck(redisURL string) dependencyCheck {
	return tcpURLCheck(redisURL)
}

func tcpURLCheck(rawURL string) dependencyCheck {
	return func(ctx context.Context) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}

		dialer := net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func registerOAuthRoutes(mux *http.ServeMux, db *sql.DB, rdb *RedisClient, cfg Config) {
	providers := map[string]OAuthProviderConfig{
		"google": googleProvider(
			cfg.GoogleOAuth.ClientID,
			cfg.GoogleOAuth.ClientSecret,
			cfg.GoogleOAuth.RedirectURL,
		),
		"github": githubProvider(
			cfg.GitHubOAuth.ClientID,
			cfg.GitHubOAuth.ClientSecret,
			cfg.GitHubOAuth.RedirectURL,
		),
		"telegram": telegramProvider(
			cfg.TelegramOAuth.ClientID,
			cfg.TelegramOAuth.ClientSecret,
			cfg.TelegramOAuth.RedirectURL,
		),
	}

	deps := OAuthDeps{
		DB:        db,
		Redis:     rdb,
		JWTSecret: cfg.JWTSecret,
		FrontendURL: cfg.FrontendURL,
		Providers: providers,
	}

	mux.HandleFunc("GET /auth/{provider}/url", oauthURLHandler(deps))
	mux.HandleFunc("POST /auth/{provider}/callback", oauthCallbackHandler(deps))
}
