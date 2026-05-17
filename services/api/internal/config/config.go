package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// OAuthCredentials holds client credentials for a single OAuth provider.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Config holds all application configuration loaded from the environment.
type Config struct {
	Port               string
	JWTSecret          string
	DatabaseURL        string
	RedisURL           string
	FrontendURL        string
	CORSAllowedOrigins []string
	OAuthStateSecret   string
	GoogleOAuth        OAuthCredentials
	GitHubOAuth        OAuthCredentials
	TelegramOAuth      OAuthCredentials
}

// Load reads configuration from a .env file (if present) and environment variables.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("config: no .env file found, using environment variables")
	}

	cfg := &Config{
		Port:               getenv("PORT", "8080"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           getenv("REDIS_URL", "redis://localhost:6379"),
		FrontendURL:        getenv("FRONTEND_URL", "http://localhost:5173"),
		CORSAllowedOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173,http://127.0.0.1:3000")),
		OAuthStateSecret:   getenv("OAUTH_STATE_SECRET", os.Getenv("JWT_SECRET")),
		GoogleOAuth: OAuthCredentials{
			ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		},
		GitHubOAuth: OAuthCredentials{
			ClientID:     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GITHUB_OAUTH_REDIRECT_URL"),
		},
		TelegramOAuth: OAuthCredentials{
			ClientID:     os.Getenv("TELEGRAM_OAUTH_CLIENT_ID"),
			ClientSecret: os.Getenv("TELEGRAM_OAUTH_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("TELEGRAM_OAUTH_REDIRECT_URL"),
		},
	}

	if cfg.JWTSecret == "" {
		log.Fatal("config: JWT_SECRET environment variable is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("config: DATABASE_URL environment variable is required")
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
