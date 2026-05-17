package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	JWTSecret   string
	DatabaseURL string
	RedisURL    string
	APIURL      string
}

func LoadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file found, using environment variables")
	}

	cfg := Config{
		Port:        getenv("PORT", "8081"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		APIURL:      os.Getenv("API_URL"),
	}

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
