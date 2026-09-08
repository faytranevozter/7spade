package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func openMigrationIntegrationDB(t *testing.T) (*sql.DB, *sql.DB) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	admin, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	var serverVersion int
	if err := admin.QueryRow(`SHOW server_version_num`).Scan(&serverVersion); err != nil {
		t.Fatalf("read PostgreSQL version: %v", err)
	}
	if serverVersion/10000 != 16 {
		t.Skipf("requires PostgreSQL 16, got server_version_num %d", serverVersion)
	}

	schema := "migrations_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema)); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`DROP SCHEMA IF EXISTS ` + pq.QuoteIdentifier(schema) + ` CASCADE`)
	})

	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	query := u.Query()
	query.Set("options", "-csearch_path="+schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return admin, db
}

func TestRunMigrationsFreshPostgres16AndCompositeEventRuleFK(t *testing.T) {
	_, db := openMigrationIntegrationDB(t)
	if err := runMigrations(db); err != nil {
		t.Fatalf("run fresh migrations: %v", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	var migrationCount int
	for _, entry := range entries {
		if !entry.IsDir() {
			migrationCount++
		}
	}
	var ledgerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&ledgerCount); err != nil {
		t.Fatalf("count migration ledger: %v", err)
	}
	if ledgerCount != migrationCount {
		t.Fatalf("migration ledger has %d rows, want %d", ledgerCount, migrationCount)
	}

	eventA, eventB, ruleID := uuid.New(), uuid.New(), uuid.New()
	for i, eventID := range []uuid.UUID{eventA, eventB} {
		slug := fmt.Sprintf("migration-fk-event-%d-%s", i, eventID)
		if _, err := db.Exec(`
			INSERT INTO events (id, slug, name, starts_at, ends_at)
			VALUES ($1, $2, $2, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 hour')
		`, eventID, slug); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
		if _, err := db.Exec(`
			INSERT INTO event_versions
				(event_id, revision, slug, name, summary, description, starts_at, ends_at, reward_config, published_at)
			VALUES ($1, 1, $2, $2, '', '', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 hour', '{}', NOW())
		`, eventID, slug); err != nil {
			t.Fatalf("insert event version %d: %v", i, err)
		}
	}

	if _, err := db.Exec(`
		INSERT INTO skin_unlock_rules
			(id, name, skin_id, rule_type, minimum_level, event_id, event_revision)
		VALUES ($1, 'migration composite FK', '42395ffa-fc5f-4700-bdb7-713a501f7305',
			'minimum_level', 1, $2, 1)
	`, ruleID, eventA); err != nil {
		t.Fatalf("insert event-bound skin rule: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO skin_unlock_rule_event_versions (skin_unlock_rule_id, event_id, event_revision)
		VALUES ($1, $2, 1)
	`, ruleID, eventA); err != nil {
		t.Fatalf("insert matching rule event version: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO skin_unlock_rule_event_versions (skin_unlock_rule_id, event_id, event_revision)
		VALUES ($1, $2, 1)
	`, ruleID, eventB); err == nil {
		t.Fatal("mismatched rule and event unexpectedly satisfied composite foreign key")
	}

	if _, err := db.Exec(`DELETE FROM skin_unlock_rules WHERE id = $1`, ruleID); err != nil {
		t.Fatalf("delete skin rule: %v", err)
	}
	var associationCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM skin_unlock_rule_event_versions WHERE skin_unlock_rule_id = $1
	`, ruleID).Scan(&associationCount); err != nil {
		t.Fatalf("count rule event versions: %v", err)
	}
	if associationCount != 0 {
		t.Fatalf("rule delete left %d event version associations, want 0", associationCount)
	}
}

func TestRunMigrationsRollsBackMigrationWhenLedgerInsertFails(t *testing.T) {
	_, db := openMigrationIntegrationDB(t)
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations (
			version TEXT PRIMARY KEY CHECK (version = 'reject every embedded migration'),
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		t.Fatalf("create rejecting migration ledger: %v", err)
	}

	if err := runMigrations(db); err == nil {
		t.Fatal("runMigrations unexpectedly succeeded with a rejecting ledger")
	}

	var usersTable *string
	if err := db.QueryRow(`SELECT to_regclass('users')::text`).Scan(&usersTable); err != nil {
		t.Fatalf("check first migration rollback: %v", err)
	}
	if usersTable != nil {
		t.Fatalf("first migration DDL was not rolled back: users table is %q", *usersTable)
	}
	var ledgerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&ledgerCount); err != nil {
		t.Fatalf("count migration ledger: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("migration ledger has %d rows after rollback, want 0", ledgerCount)
	}
}

func TestConcurrentRunMigrationsSerializeOnAdvisoryLock(t *testing.T) {
	admin, db := openMigrationIntegrationDB(t)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	blocker, err := admin.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err := blocker.Exec(`SELECT pg_advisory_xact_lock($1)`, migrationAdvisoryLockID); err != nil {
		t.Fatalf("hold migration advisory lock: %v", err)
	}

	results := make(chan error, 2)
	go func() { results <- runMigrations(db) }()
	go func() { results <- runMigrations(db) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		err := admin.QueryRow(`
			SELECT COUNT(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND wait_event = 'advisory'
			  AND query LIKE 'SELECT pg_advisory_xact_lock(%'
		`).Scan(&waiting)
		if err != nil {
			t.Fatalf("inspect advisory lock waiters: %v", err)
		}
		if waiting >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("saw %d migration runners waiting for advisory lock, want 2", waiting)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release migration advisory lock: %v", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("concurrent migration run %d: %v", i+1, err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("timed out waiting for concurrent migration runs")
		}
	}

	var duplicates int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT version FROM schema_migrations GROUP BY version HAVING COUNT(*) > 1
		) duplicate_versions
	`).Scan(&duplicates); err != nil {
		t.Fatalf("check migration ledger duplicates: %v", err)
	}
	if duplicates != 0 {
		t.Fatalf("migration ledger has %d duplicate versions", duplicates)
	}
}
