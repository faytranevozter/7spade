package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/database"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer db.Close()

	report, err := repository.ReconcileProgressionSkins(db)
	if err != nil {
		log.Fatalf("reconcile progression skins: %v", err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatalf("encode reconciliation report: %v", err)
	}
}
