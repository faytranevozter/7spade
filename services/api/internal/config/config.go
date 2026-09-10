package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// OAuthCredentials holds client credentials for a single OAuth provider.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// S3Config holds S3-compatible storage configuration.
type S3Config struct {
	Endpoint     string
	AccessKeyID  string
	SecretKey    string
	Bucket       string
	Region       string
	PublicURL    string
	UsePathStyle bool
}

// Config holds all application configuration loaded from the environment.
type Config struct {
	Port                string
	JWTSecret           string
	DatabaseURL         string
	RedisURL            string
	FrontendURL         string
	CORSAllowedOrigins  []string
	InternalSecret      string
	LeaderboardMinGames int
	GameDetailRetention int
	DailyLoginXPBase    int
	DailyLoginXPStep    int
	DailyLoginXPMax     int
	AppTimezone         string
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
	TelegramMobileRedirectURL    string
	S3Config                     S3Config
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
		InternalSecret:               os.Getenv("INTERNAL_API_SECRET"),
		LeaderboardMinGames:          getenvInt("LEADERBOARD_MIN_GAMES", 5),
		GameDetailRetention:          getenvInt("GAME_DETAIL_RETENTION", 20),
		DailyLoginXPBase:             getenvInt("DAILY_LOGIN_XP_BASE", 10),
		DailyLoginXPStep:             getenvInt("DAILY_LOGIN_XP_STEP", 5),
		DailyLoginXPMax:              getenvInt("DAILY_LOGIN_XP_MAX", 50),
		AppTimezone:                  getenv("APP_TIMEZONE", "UTC"),
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
		TelegramMobileRedirectURL: os.Getenv("TELEGRAM_MOBILE_REDIRECT_URL"),
		S3Config: S3Config{
			Endpoint:     os.Getenv("S3_ENDPOINT"),
			AccessKeyID:  os.Getenv("S3_ACCESS_KEY_ID"),
			SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
			Bucket:       os.Getenv("S3_BUCKET"),
			Region:       getenv("S3_REGION", "auto"),
			PublicURL:    getenv("S3_PUBLIC_URL", ""),
			UsePathStyle: getenvBool("S3_USE_PATH_STYLE"),
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
	if _, err := time.LoadLocation(cfg.AppTimezone); err != nil {
		log.Printf("config: invalid APP_TIMEZONE=%q, using UTC", cfg.AppTimezone)
		cfg.AppTimezone = "UTC"
	}
	if cfg.DailyLoginXPBase <= 0 || cfg.DailyLoginXPStep <= 0 || cfg.DailyLoginXPMax < cfg.DailyLoginXPBase {
		log.Printf("config: daily login XP values must be positive and max must be at least base, using defaults")
		cfg.DailyLoginXPBase = 10
		cfg.DailyLoginXPStep = 5
		cfg.DailyLoginXPMax = 50
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

func getenvBool(key string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("config: invalid boolean for %s=%q, defaulting to false", key, v)
		return false
	}
	return b
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
