package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/faytranevozter/7spade/services/api/internal/accountcleanup"
	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/database"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/faytranevozter/7spade/services/api/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	rdb, err := cache.New(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to create Redis client: %v", err)
	}
	defer rdb.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cleanup := &accountcleanup.Runner{
		DB:       db,
		Interval: accountcleanup.DefaultInterval,
		Grace:    repository.AccountDeletionGracePeriod,
	}
	cleanup.Start(ctx)

	router := server.NewRouter(cfg, db, rdb)
	log.Printf("API service listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
