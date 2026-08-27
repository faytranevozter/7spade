package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	APIHealthURL         string
	WSHealthURL          string
	OperationsLinks      map[string]string
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
		APIHealthURL:         os.Getenv("API_HEALTH_URL"),
		WSHealthURL:          os.Getenv("WS_HEALTH_URL"),
		OperationsLinks: map[string]string{
			"metrics":     os.Getenv("OPERATIONS_METRICS_URL"),
			"logs":        os.Getenv("OPERATIONS_LOGS_URL"),
			"traces":      os.Getenv("OPERATIONS_TRACES_URL"),
			"deployments": os.Getenv("OPERATIONS_DEPLOYMENTS_URL"),
			"runbook":     os.Getenv("OPERATIONS_RUNBOOK_URL"),
		},
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if err := validateURLs(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateURLs(cfg *Config) error {
	for name, rawURL := range map[string]string{
		"API_HEALTH_URL": cfg.APIHealthURL,
		"WS_HEALTH_URL":  cfg.WSHealthURL,
	} {
		if rawURL == "" {
			continue
		}
		if err := validateURL(name, rawURL); err != nil {
			return err
		}
	}
	for name, rawURL := range cfg.OperationsLinks {
		if rawURL == "" {
			continue
		}
		if err := validateURL("OPERATIONS_"+strings.ToUpper(name)+"_URL", rawURL); err != nil {
			return err
		}
	}
	return nil
}

func validateURL(name, rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("config: %s must be an absolute HTTP(S) URL", name)
	}
	return nil
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
