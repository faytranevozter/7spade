package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/faytranevozter/7spade/services/admin-api/internal/admin"
	_ "github.com/lib/pq"
)

func main() {
	port := env("PORT", "8082")
	databaseURL := required("DATABASE_URL")
	jwtSecret := required("ADMIN_JWT_SECRET")
	origin := env("ADMIN_FRONTEND_ORIGIN", "http://localhost:3001")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	store := admin.NewPostgresStore(db, env("APP_ENV", "development"))

	router := admin.NewRouter(admin.Config{JWTSecret: jwtSecret, SecureCookies: strings.EqualFold(os.Getenv("ADMIN_SECURE_COOKIES"), "true"), AllowedOrigin: origin}, store)
	log.Printf("admin API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}
