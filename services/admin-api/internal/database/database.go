package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// Open connects to PostgreSQL. Schema migrations remain owned by services/api.
func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return db, nil
}
