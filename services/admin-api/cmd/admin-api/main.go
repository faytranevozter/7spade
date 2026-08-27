package main

import (
	"log"

	"github.com/faytranevozter/7spade/services/admin-api/internal/config"
	"github.com/faytranevozter/7spade/services/admin-api/internal/database"
	"github.com/faytranevozter/7spade/services/admin-api/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.ValidateServer(); err != nil {
		log.Fatal(err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	router := server.NewRouter(cfg, db)
	log.Printf("Admin API service listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
