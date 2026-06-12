package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	JWTSecret      string
	DatabaseURL    string
	RedisURL       string
	WSRedisURL     string
	APIURL         string
	InternalSecret string
}

func LoadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file found, using environment variables")
	}

	cfg := Config{
		Port:           getenv("PORT", "8081"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
		WSRedisURL:     os.Getenv("WS_REDIS_URL"),
		APIURL:         os.Getenv("API_URL"),
		InternalSecret: os.Getenv("INTERNAL_API_SECRET"),
	}

	// WS_REDIS_URL is the dedicated Redis for the cross-replica relay (pub/sub,
	// owner leases, room snapshots). It falls back to REDIS_URL so single-Redis
	// and single-replica deployments keep working with no extra config.
	if cfg.WSRedisURL == "" {
		cfg.WSRedisURL = cfg.RedisURL
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
