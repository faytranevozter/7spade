package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Config struct {
	Port             string
	JWTSecret        string
	DatabaseURL      string
	RedisURL         string
	TelegramBotToken string
	FrontendURL      string
	OAuthStateSecret string
	GoogleOAuth      OAuthCredentials
	GitHubOAuth      OAuthCredentials
}

func LoadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file found, using environment variables")
	}

	cfg := Config{
		Port:             getenv("PORT", "8080"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisURL:         os.Getenv("REDIS_URL"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		FrontendURL:      getenv("FRONTEND_URL", "http://localhost:5173"),
		OAuthStateSecret: getenv("OAUTH_STATE_SECRET", os.Getenv("JWT_SECRET")),
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
	}

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
