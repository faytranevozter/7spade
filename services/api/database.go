package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// InitDB initializes database connection and runs migrations
func InitDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database initialized successfully")
	return db, nil
}

func runMigrations(db *sql.DB) error {
	// Create migrations table if not exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Read migration files
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// Apply each migration
	for _, filename := range files {
		// Check if migration already applied
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", filename).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if count > 0 {
			log.Printf("Migration %s already applied, skipping", filename)
			continue
		}

		// Read and execute migration
		content, err := migrationFiles.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filename, err)
		}

		// Record migration as applied
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		log.Printf("Applied migration: %s", filename)
	}

	return nil
}
