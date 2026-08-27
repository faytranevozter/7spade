package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/faytranevozter/7spade/services/admin-api/internal/admin"
	_ "github.com/lib/pq"
)

func main() {
	databaseURL := required("DATABASE_URL")
	email := required("ADMIN_BOOTSTRAP_EMAIL")
	password := required("ADMIN_BOOTSTRAP_PASSWORD")
	name := required("ADMIN_BOOTSTRAP_NAME")
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := admin.NewPostgresStore(db, "").Bootstrap(context.Background(), email, password, name); err != nil {
		log.Fatal(err)
	}
	log.Printf("administrator bootstrap completed for %s", email)
}

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}
