package config

import (
	"log"
	"os"
	"strconv"
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
	Port                string
	JWTSecret           string
	DatabaseURL         string
	RedisURL            string
	FrontendURL         string
	CORSAllowedOrigins  []string
	OAuthStateSecret    string
	InternalSecret      string
	LeaderboardMinGames int
	GameDetailRetention int
	// Rate-limit tiers (requests per window). Window is RateLimitWindowSeconds.
	RateLimitAuthPerMinute       int
	RateLimitRoomsWritePerMinute int
	RateLimitSocialPerMinute     int
	RateLimitGeneralPerMinute    int
	RateLimitWindowSeconds       int
	RateLimitQuickPlayCooldownMs int
	SMTPHost                     string
	SMTPPort                     int
	SMTPUser                     string
	SMTPPass                     string
	SMTPFrom                     string
	SMTPFromName                 string
	SMTPReplyTo                  string
	SMTPEncryption               string
	GoogleOAuth                  OAuthCredentials
	GitHubOAuth                  OAuthCredentials
	TelegramOAuth                OAuthCredentials
}

// Load reads configuration from a .env file (if present) and environment variables.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("config: no .env file found, using environment variables")
	}

	cfg := &Config{
		Port:                         getenv("PORT", "8080"),
		JWTSecret:                    os.Getenv("JWT_SECRET"),
		DatabaseURL:                  os.Getenv("DATABASE_URL"),
		RedisURL:                     getenv("REDIS_URL", "redis://localhost:6379"),
		FrontendURL:                  getenv("FRONTEND_URL", "http://localhost:5173"),
		CORSAllowedOrigins:           splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173,http://127.0.0.1:3000")),
		OAuthStateSecret:             getenv("OAUTH_STATE_SECRET", os.Getenv("JWT_SECRET")),
		InternalSecret:               os.Getenv("INTERNAL_API_SECRET"),
		LeaderboardMinGames:          getenvInt("LEADERBOARD_MIN_GAMES", 5),
		GameDetailRetention:          getenvInt("GAME_DETAIL_RETENTION", 20),
		RateLimitAuthPerMinute:       getenvInt("RATE_LIMIT_AUTH_PER_MINUTE", 10),
		RateLimitRoomsWritePerMinute: getenvInt("RATE_LIMIT_ROOMS_WRITE_PER_MINUTE", 5),
		RateLimitSocialPerMinute:     getenvInt("RATE_LIMIT_SOCIAL_PER_MINUTE", 30),
		RateLimitGeneralPerMinute:    getenvInt("RATE_LIMIT_GENERAL_PER_MINUTE", 60),
		RateLimitWindowSeconds:       getenvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		RateLimitQuickPlayCooldownMs: getenvInt("RATE_LIMIT_QUICK_PLAY_COOLDOWN_MS", 3000),
		SMTPHost:                     os.Getenv("SMTP_HOST"),
		SMTPPort:                     getenvInt("SMTP_PORT", 587),
		SMTPUser:                     os.Getenv("SMTP_USER"),
		SMTPPass:                     os.Getenv("SMTP_PASS"),
		SMTPFrom:                     getenv("SMTP_FROM", "no-reply@sevenspade.local"),
		SMTPFromName:                 getenv("SMTP_FROM_NAME", "Seven Spade"),
		SMTPReplyTo:                  os.Getenv("SMTP_REPLY_TO"),
		SMTPEncryption:               getenv("SMTP_ENCRYPTION", "auto"),
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
	if cfg.InternalSecret == "" {
		log.Fatal("config: INTERNAL_API_SECRET environment variable is required (the /internal/* endpoints are otherwise unauthenticated)")
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
		log.Printf("config: invalid %s=%q, using default %d", key, v, fallback)
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
