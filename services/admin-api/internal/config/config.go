package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	MFAEncryptionKey     string
	FrontendOrigin       string
	SecureCookies        bool
	Environment          string
	BootstrapEmail       string
	BootstrapPassword    string
	BootstrapDisplayName string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("config: no .env file found, using environment variables")
	}

	cfg := &Config{
		Port:                 getenv("PORT", "8082"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		JWTSecret:            os.Getenv("ADMIN_JWT_SECRET"),
		MFAEncryptionKey:     os.Getenv("ADMIN_MFA_ENCRYPTION_KEY"),
		FrontendOrigin:       getenv("ADMIN_FRONTEND_ORIGIN", "http://localhost:5174"),
		SecureCookies:        getenvBool("ADMIN_SECURE_COOKIES"),
		Environment:          getenv("APP_ENV", "development"),
		BootstrapEmail:       os.Getenv("ADMIN_BOOTSTRAP_EMAIL"),
		BootstrapPassword:    os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),
		BootstrapDisplayName: os.Getenv("ADMIN_BOOTSTRAP_NAME"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	return cfg, nil
}

func (c *Config) ValidateServer() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("config: ADMIN_JWT_SECRET is required")
	}
	if c.MFAEncryptionKey == "" {
		return fmt.Errorf("config: ADMIN_MFA_ENCRYPTION_KEY is required")
	}
	return nil
}

func (c *Config) ValidateBootstrap() error {
	if c.BootstrapEmail == "" || c.BootstrapPassword == "" || c.BootstrapDisplayName == "" {
		return fmt.Errorf("config: ADMIN_BOOTSTRAP_EMAIL, ADMIN_BOOTSTRAP_PASSWORD, and ADMIN_BOOTSTRAP_NAME are required")
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string) bool {
	value := os.Getenv(key)
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
