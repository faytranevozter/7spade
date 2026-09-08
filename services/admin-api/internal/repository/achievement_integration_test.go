package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func openAchievementIntegrationDB(t *testing.T) *sql.DB {
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

	schema := "admin_achievement_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE users (id UUID PRIMARY KEY);
		CREATE TABLE achievements (id TEXT PRIMARY KEY);
		CREATE TABLE user_achievements (
			user_id UUID NOT NULL REFERENCES users(id),
			achievement_id TEXT NOT NULL REFERENCES achievements(id),
			PRIMARY KEY (user_id, achievement_id)
		);
		CREATE TABLE user_achievement_entitlement_events (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id),
			achievement_id TEXT NOT NULL REFERENCES achievements(id),
			action TEXT NOT NULL,
			reason TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			admin_user_id UUID,
			occurred_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE admin_audit_events (
			id UUID PRIMARY KEY,
			admin_user_id UUID,
			session_id UUID,
			request_id TEXT,
			action TEXT NOT NULL,
			resource_type TEXT,
			resource_id TEXT,
			reason TEXT,
			outcome TEXT NOT NULL,
			before_state JSONB,
			after_state JSONB,
			metadata JSONB,
			ip_address TEXT,
			user_agent TEXT,
			occurred_at TIMESTAMPTZ NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create test tables: %v", err)
	}
	return db
}

func seedAchievementEntitlement(t *testing.T, db *sql.DB) (string, string, string) {
	t.Helper()
	userID, otherUserID := uuid.NewString(), uuid.NewString()
	if _, err := db.Exec(`INSERT INTO users(id) VALUES($1),($2)`, userID, otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO achievements(id) VALUES('first_win'),('veteran')`); err != nil {
		t.Fatal(err)
	}
	return userID, otherUserID, "first_win"
}

func entitlementAudit(action, userID, achievementID string) model.AuditEvent {
	return model.AuditEvent{
		Action:       "achievement.entitlement." + action,
		ResourceType: "user_achievement_entitlement",
		ResourceID:   userID + ":" + achievementID,
		Reason:       "support correction",
		Outcome:      "success",
	}
}

func TestChangeAchievementEntitlementRejectsIdempotencyKeyReuseForDifferentOperation(t *testing.T) {
	db := openAchievementIntegrationDB(t)
	store := NewPostgresStore(db, "test")
	userID, otherUserID, achievementID := seedAchievementEntitlement(t, db)
	key := "shared-key"
	if _, _, err := store.ChangeAchievementEntitlement(context.Background(), userID, achievementID, "grant", "support correction", key, entitlementAudit("grant", userID, achievementID)); err != nil {
		t.Fatalf("initial grant: %v", err)
	}

	for _, tc := range []struct {
		name, userID, achievementID, action string
	}{
		{"different user", otherUserID, achievementID, "grant"},
		{"different achievement", userID, "veteran", "grant"},
		{"different action", userID, achievementID, "revoke"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, replayed, err := store.ChangeAchievementEntitlement(context.Background(), tc.userID, tc.achievementID, tc.action, "another reason", key, entitlementAudit(tc.action, tc.userID, tc.achievementID))
			if !errors.Is(err, ErrConflict) || replayed {
				t.Fatalf("ChangeAchievementEntitlement error = %v, replayed = %v; want ErrConflict", err, replayed)
			}
		})
	}

	var events, audits, entitlements int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM user_achievement_entitlement_events), (SELECT count(*) FROM admin_audit_events), (SELECT count(*) FROM user_achievements)`).Scan(&events, &audits, &entitlements); err != nil {
		t.Fatal(err)
	}
	if events != 1 || audits != 1 || entitlements != 1 {
		t.Fatalf("events=%d audits=%d entitlements=%d, want 1 each", events, audits, entitlements)
	}
}

func TestChangeAchievementEntitlementConcurrentIdenticalRequestReplaysCommittedEvent(t *testing.T) {
	for _, action := range []string{"grant", "revoke"} {
		t.Run(action, func(t *testing.T) {
			db := openAchievementIntegrationDB(t)
			store := NewPostgresStore(db, "test")
			userID, _, achievementID := seedAchievementEntitlement(t, db)
			if action == "revoke" {
				if _, err := db.Exec(`INSERT INTO user_achievements(user_id,achievement_id) VALUES($1,$2)`, userID, achievementID); err != nil {
					t.Fatal(err)
				}
			}

			type outcome struct {
				event    model.AchievementEntitlementEvent
				replayed bool
				err      error
			}
			start := make(chan struct{})
			results := make(chan outcome, 2)
			var ready sync.WaitGroup
			ready.Add(2)
			for range 2 {
				go func() {
					ready.Done()
					<-start
					event, replayed, err := store.ChangeAchievementEntitlement(context.Background(), userID, achievementID, action, "support correction", "concurrent-"+action, entitlementAudit(action, userID, achievementID))
					results <- outcome{event: event, replayed: replayed, err: err}
				}()
			}
			ready.Wait()
			close(start)
			first, second := <-results, <-results
			if first.err != nil || second.err != nil {
				t.Fatalf("concurrent errors: %v, %v", first.err, second.err)
			}
			if first.event.ID == "" || first.event.ID != second.event.ID {
				t.Fatalf("event IDs = %q, %q; want same committed event", first.event.ID, second.event.ID)
			}
			if first.replayed == second.replayed {
				t.Fatalf("replayed flags = %v, %v; want one mutation and one replay", first.replayed, second.replayed)
			}

			var events, audits, entitlements int
			if err := db.QueryRow(`SELECT (SELECT count(*) FROM user_achievement_entitlement_events), (SELECT count(*) FROM admin_audit_events), (SELECT count(*) FROM user_achievements)`).Scan(&events, &audits, &entitlements); err != nil {
				t.Fatal(err)
			}
			wantEntitlements := 1
			if action == "revoke" {
				wantEntitlements = 0
			}
			if events != 1 || audits != 1 || entitlements != wantEntitlements {
				t.Fatalf("events=%d audits=%d entitlements=%d, want 1, 1, %d", events, audits, entitlements, wantEntitlements)
			}
		})
	}
}

func TestChangeAchievementEntitlementEnforcesCurrentStateWithoutSideEffects(t *testing.T) {
	for _, tc := range []struct {
		action   string
		entitled bool
	}{
		{"grant", true},
		{"revoke", false},
	} {
		t.Run(fmt.Sprintf("%s entitled=%v", tc.action, tc.entitled), func(t *testing.T) {
			db := openAchievementIntegrationDB(t)
			store := NewPostgresStore(db, "test")
			userID, _, achievementID := seedAchievementEntitlement(t, db)
			if tc.entitled {
				if _, err := db.Exec(`INSERT INTO user_achievements(user_id,achievement_id) VALUES($1,$2)`, userID, achievementID); err != nil {
					t.Fatal(err)
				}
			}
			_, replayed, err := store.ChangeAchievementEntitlement(context.Background(), userID, achievementID, tc.action, "support correction", "state-conflict", entitlementAudit(tc.action, userID, achievementID))
			if !errors.Is(err, ErrConflict) || replayed {
				t.Fatalf("error = %v, replayed = %v; want ErrConflict", err, replayed)
			}
			var events, audits int
			if err := db.QueryRow(`SELECT (SELECT count(*) FROM user_achievement_entitlement_events), (SELECT count(*) FROM admin_audit_events)`).Scan(&events, &audits); err != nil {
				t.Fatal(err)
			}
			if events != 0 || audits != 0 {
				t.Fatalf("events=%d audits=%d, want no side effects", events, audits)
			}
		})
	}
}
