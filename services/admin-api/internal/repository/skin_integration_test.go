package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

func openAdminSkinIntegrationDB(t *testing.T) *sql.DB {
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
	schema := "admin_skin_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA IF EXISTS ` + pq.QuoteIdentifier(schema) + ` CASCADE`) })
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("options", "-csearch_path="+schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	entries, err := os.ReadDir("../../../api/internal/database/migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migration, err := os.ReadFile("../../../api/internal/database/migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply migration %s: %v", entry.Name(), err)
		}
	}
	return db
}

func TestCreateSkinIntegrationPersistsRulesConditionsAndRollsBackAtomically(t *testing.T) {
	db := openAdminSkinIntegrationDB(t)
	level := 3
	skin := Skin{SkinType: "avatar_frame", Name: "Integrated", UnlockRules: []model.SkinUnlockRule{
		{Name: "level", RuleType: "minimum_level", MinimumLevel: &level, Enabled: true},
		{Name: "winner", RuleType: "game_condition", Enabled: true, Conditions: json.RawMessage(`[{"metric":"is_winner","operator":"eq","value":"true"}]`)},
	}}
	created, err := NewPostgresStore(db, "test").CreateSkin(context.Background(), skin, model.AuditEvent{ID: uuid.NewString(), Action: "skin.create", ResourceType: "skin", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.UnlockRules) != 2 || created.UnlockRules[0].ID == "" || created.UnlockRules[1].ID == "" {
		t.Fatalf("created rules = %+v", created.UnlockRules)
	}
	var rules, conditions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM skin_unlock_rules WHERE skin_id=$1`, created.ID).Scan(&rules); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM skin_unlock_rule_conditions c JOIN skin_unlock_rules r ON r.id=c.skin_unlock_rule_id WHERE r.skin_id=$1`, created.ID).Scan(&conditions); err != nil {
		t.Fatal(err)
	}
	if rules != 2 || conditions != 1 {
		t.Fatalf("persisted rules/conditions = %d/%d, want 2/1", rules, conditions)
	}

	bad := skin
	bad.Name = "Must Roll Back"
	_, err = NewPostgresStore(db, "test").CreateSkin(context.Background(), bad, model.AuditEvent{ID: "not-a-uuid", Action: "skin.create", ResourceType: "skin", Reason: "test", Outcome: "success"})
	if err == nil {
		t.Fatal("CreateSkin succeeded with invalid audit id")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM skins WHERE name='Must Roll Back'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back skins = %d, want 0", count)
	}
}

func TestEventRepublishIntegrationAppendsRuleRevisionAssociations(t *testing.T) {
	db := openAdminSkinIntegrationDB(t)
	store := NewPostgresStore(db, "test")
	now := time.Now().UTC()
	event, err := store.CreateEvent(context.Background(), model.Event{
		Slug: "republish-" + uuid.NewString(), Name: "First", Summary: "summary", Description: "description",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(2 * time.Hour), RewardConfig: json.RawMessage(`{}`),
	}, model.AuditEvent{ID: uuid.NewString(), Action: "event.create", ResourceType: "event", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	level := 2
	skin, err := store.CreateSkin(context.Background(), Skin{SkinType: "avatar_frame", Name: "Event Skin", UnlockRules: []model.SkinUnlockRule{{Name: "event-level", RuleType: "minimum_level", MinimumLevel: &level, EventID: event.ID, Enabled: true}}}, model.AuditEvent{ID: uuid.NewString(), Action: "skin.create", ResourceType: "skin", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	event, err = store.TransitionEvent(context.Background(), event.ID, event.Version, model.EventPublished, model.AuditEvent{ID: uuid.NewString(), Action: "event.publish", ResourceType: "event", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	event.Name = "Second"
	event, err = store.UpdateEvent(context.Background(), event.ID, event.Version, event, model.AuditEvent{ID: uuid.NewString(), Action: "event.update", ResourceType: "event", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	if event.Revision != 2 || event.State != model.EventDraft {
		t.Fatalf("edited event = %+v", event)
	}
	event, err = store.TransitionEvent(context.Background(), event.ID, event.Version, model.EventPublished, model.AuditEvent{ID: uuid.NewString(), Action: "event.publish", ResourceType: "event", Reason: "test", Outcome: "success"})
	if err != nil {
		t.Fatal(err)
	}
	var associations, currentRevision int
	if err := db.QueryRow(`SELECT COUNT(*) FROM skin_unlock_rule_event_versions rv JOIN skin_unlock_rules r ON r.id=rv.skin_unlock_rule_id WHERE r.skin_id=$1`, skin.ID).Scan(&associations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT event_revision FROM skin_unlock_rules WHERE skin_id=$1`, skin.ID).Scan(&currentRevision); err != nil {
		t.Fatal(err)
	}
	if associations != 2 || currentRevision != 2 {
		t.Fatalf("associations/current revision = %d/%d, want 2/2", associations, currentRevision)
	}
}
