package main

import (
	"context"
	"log"

	"github.com/faytranevozter/7spade/services/admin-api/internal/config"
	"github.com/faytranevozter/7spade/services/admin-api/internal/database"
	"github.com/faytranevozter/7spade/services/admin-api/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.ValidateBootstrap(); err != nil {
		log.Fatal(err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	store := repository.NewPostgresStore(db, cfg.Environment)
	if err := store.Bootstrap(context.Background(), cfg.BootstrapEmail, cfg.BootstrapPassword, cfg.BootstrapDisplayName); err != nil {
		log.Fatal(err)
	}
	log.Printf("administrator bootstrap completed for %s", cfg.BootstrapEmail)
}
