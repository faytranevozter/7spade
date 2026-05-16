package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
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
	RefreshToken string `json:"refresh_token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	JWT string `json:"jwt"`
}

type telegramAuthRequest struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Initialize database and run migrations
	db, err := InitDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("api", map[string]dependencyCheck{
		"postgres": postgresCheck(databaseURL),
		"redis":    redisCheck(os.Getenv("REDIS_URL")),
	}))
	mux.HandleFunc("POST /guest", guestHandler(jwtSecret))
	mux.HandleFunc("POST /register", registerHandler(db, jwtSecret))
	mux.HandleFunc("POST /login", loginHandler(db, jwtSecret))
	mux.HandleFunc("POST /refresh", refreshHandler(db, jwtSecret))
	mux.HandleFunc("POST /auth/telegram", telegramAuthHandler(db, jwtSecret, os.Getenv("TELEGRAM_BOT_TOKEN")))
	mux.HandleFunc("POST /internal/games", saveGameHandler(db))

	// Room endpoints (authenticated)
	mux.HandleFunc("POST /rooms", requireAuth(jwtSecret, createRoomHandler(db)))
	mux.HandleFunc("GET /rooms", listPublicRoomsHandler(db))
	mux.HandleFunc("POST /rooms/{code}/join", requireAuth(jwtSecret, joinRoomHandler(db)))
	mux.HandleFunc("GET /rooms/{id}", getRoomHandler(db))
	mux.HandleFunc("GET /history", requireAuth(jwtSecret, historyHandler(db)))

	registerOAuthRoutes(mux, db, jwtSecret)

	log.Printf("API service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func telegramAuthHandler(db *sql.DB, jwtSecret string, botToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req telegramAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid request body"})
			return
		}

		if botToken == "" || !verifyTelegramPayload(req, botToken, time.Now()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid or expired Telegram payload"})
			return
		}

		displayName := telegramDisplayName(req)
		user, err := UpsertTelegramUser(db, req.ID, displayName, req.PhotoURL)
		if err != nil {
			log.Printf("Failed to upsert Telegram user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		jwt, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		tokenHash := HashRefreshToken(refreshToken)
		if err := StoreRefreshToken(db, user.ID, tokenHash, time.Now().Add(30*24*time.Hour)); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(authResponse{JWT: jwt, RefreshToken: refreshToken})
	}
}

func verifyTelegramPayload(req telegramAuthRequest, botToken string, now time.Time) bool {
	if req.ID == 0 || req.AuthDate == 0 || req.Hash == "" {
		return false
	}
	authTime := time.Unix(req.AuthDate, 0)
	if now.Sub(authTime) > 24*time.Hour || authTime.After(now.Add(5*time.Minute)) {
		return false
	}

	checkStringFields := map[string]string{
		"id":        fmt.Sprintf("%d", req.ID),
		"auth_date": fmt.Sprintf("%d", req.AuthDate),
	}
	if req.FirstName != "" {
		checkStringFields["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		checkStringFields["last_name"] = req.LastName
	}
	if req.Username != "" {
		checkStringFields["username"] = req.Username
	}
	if req.PhotoURL != "" {
		checkStringFields["photo_url"] = req.PhotoURL
	}

	checkStringParts := make([]string, 0, len(checkStringFields))
	for key, value := range checkStringFields {
		checkStringParts = append(checkStringParts, key+"="+value)
	}
	sort.Strings(checkStringParts)
	checkString := strings.Join(checkStringParts, "\n")

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(checkString))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(req.Hash)))
}

func telegramDisplayName(req telegramAuthRequest) string {
	displayName := strings.TrimSpace(strings.Join([]string{req.FirstName, req.LastName}, " "))
	if displayName == "" {
		displayName = strings.TrimSpace(req.Username)
	}
	if displayName == "" {
		displayName = fmt.Sprintf("Telegram %d", req.ID)
	}
	if len(displayName) > 50 {
		return displayName[:50]
	}
	return displayName
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid request body"})
			return
		}

		// Validate display name
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Display name is required"})
			return
		}
		if len(displayName) > 50 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Display name must be 50 characters or less"})
			return
		}

		// Generate JWT token
		token, err := GenerateGuestToken(displayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate guest token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Failed to generate token"})
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid request body"})
			return
		}

		// Validate email
		email := strings.TrimSpace(strings.ToLower(req.Email))
		if !emailRegex.MatchString(email) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid email format"})
			return
		}

		// Validate password
		if len(req.Password) < 8 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Password must be at least 8 characters"})
			return
		}

		// Validate display name
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" || len(displayName) > 50 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Display name must be 1-50 characters"})
			return
		}

		// Check if user already exists
		existingUser, err := GetUserByEmail(db, email)
		if err != nil {
			log.Printf("Failed to check existing user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}
		if existingUser != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Email already registered"})
			return
		}

		// Hash password
		passwordHash, err := HashPassword(req.Password)
		if err != nil {
			log.Printf("Failed to hash password: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Create user
		user, err := CreateUser(db, email, passwordHash, displayName)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Generate JWT
		jwt, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Generate refresh token
		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Store refresh token (hashed)
		tokenHash := HashRefreshToken(refreshToken)
		expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days
		if err := StoreRefreshToken(db, user.ID, tokenHash, expiresAt); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(authResponse{JWT: jwt, RefreshToken: refreshToken})
	}
}

func loginHandler(db *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid request body"})
			return
		}

		// Normalize email
		email := strings.TrimSpace(strings.ToLower(req.Email))

		// Get user by email
		user, err := GetUserByEmail(db, email)
		if err != nil {
			log.Printf("Failed to get user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Check if user exists, has a password set, and password matches
		if user == nil || !user.PasswordHash.Valid || ComparePassword(user.PasswordHash.String, req.Password) != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid email or password"})
			return
		}

		// Generate JWT
		jwt, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Generate refresh token
		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			log.Printf("Failed to generate refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		// Store refresh token (hashed)
		tokenHash := HashRefreshToken(refreshToken)
		expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days
		if err := StoreRefreshToken(db, user.ID, tokenHash, expiresAt); err != nil {
			log.Printf("Failed to store refresh token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(authResponse{JWT: jwt, RefreshToken: refreshToken})
	}
}

func refreshHandler(db *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid request body"})
			return
		}

		if req.RefreshToken == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Refresh token is required"})
			return
		}

		// Validate refresh token
		tokenHash := HashRefreshToken(req.RefreshToken)
		userID, err := ValidateRefreshToken(db, tokenHash)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Invalid or expired refresh token"})
			return
		}

		// Get user
		user, err := GetUserByID(db, userID)
		if err != nil {
			log.Printf("Failed to get user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}
		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "User not found"})
			return
		}

		// Generate new JWT
		jwt, err := GenerateUserToken(user.ID.String(), user.DisplayName, jwtSecret)
		if err != nil {
			log.Printf("Failed to generate JWT: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errorResponse{Error: "Internal server error"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(refreshResponse{JWT: jwt})
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// registerOAuthRoutes wires the Google and GitHub OAuth start/callback routes onto the given mux.
// Providers without a configured client ID are still registered so the frontend can detect
// unconfigured providers (the start handler returns 503).
func registerOAuthRoutes(mux *http.ServeMux, db *sql.DB, jwtSecret string) {
	stateSecret := os.Getenv("OAUTH_STATE_SECRET")
	if stateSecret == "" {
		stateSecret = jwtSecret
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	deps := OAuthDeps{
		DB:          db,
		JWTSecret:   jwtSecret,
		StateSecret: stateSecret,
		FrontendURL: frontendURL,
	}

	googleCfg := googleProvider(
		os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		nil,
	)
	mux.HandleFunc("GET /auth/google", oauthStartHandler(googleCfg, deps))
	mux.HandleFunc("GET /auth/google/callback", oauthCallbackHandler(googleCfg, deps))

	githubCfg := githubProvider(
		os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		os.Getenv("GITHUB_OAUTH_REDIRECT_URL"),
		nil,
	)
	mux.HandleFunc("GET /auth/github", oauthStartHandler(githubCfg, deps))
	mux.HandleFunc("GET /auth/github/callback", oauthCallbackHandler(githubCfg, deps))
}
