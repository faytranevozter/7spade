package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationAdvisoryLockID int64 = 734726331926081

// Open connects to PostgreSQL and runs all pending migrations.
func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("database: migrations: %w", err)
	}
	log.Println("database: initialized successfully")
	return db, nil
}

func runMigrations(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback()

	// Serialize migrations across API replicas and keep schema changes atomic
	// with their schema_migrations records.
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var applied []string
	for _, filename := range files {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", filename).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if count > 0 {
			log.Printf("database: migration %s already applied, skipping", filename)
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}
		applied = append(applied, filename)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	for _, filename := range applied {
		log.Printf("database: applied migration %s", filename)
	}
	return nil
}
