package repository

import (
	"database/sql"
	"errors"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	appdb "github.com/faytranevozter/7spade/services/api/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func openSkinRuleIntegrationDB(t *testing.T) *sql.DB {
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

	schema := "skin_rules_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

	db, err := appdb.Open(u.String())
	if err != nil {
		t.Fatalf("open migrated test schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertSkinRuleTestUser(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, email, password_hash, display_name, username)
		VALUES ($1, $2, 'test-hash', $3, $4)
	`, id, id.String()+"@test.example", name, "test_"+strings.ReplaceAll(id.String(), "-", "")[:20])
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertSkinRuleTestRule(t *testing.T, db *sql.DB, name string, retroactive bool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ruleID, skinID := uuid.New(), uuid.New()
	if _, err := db.Exec(`
		INSERT INTO skins (id, skin_type, name, description, asset_key, display_order)
		VALUES ($1, 'avatar_frame', $2, 'integration test', 'skins/frames/gold-spade.svg', 9999)
	`, skinID, name+" skin"); err != nil {
		t.Fatalf("insert skin: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO skin_revisions (skin_id,version,asset_key,content_type) VALUES ($1,1,'skins/frames/gold-spade.svg','image/svg+xml')`, skinID); err != nil {
		t.Fatalf("insert revision: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO skin_unlock_rules (id, name, skin_id, rule_type, retroactive)
		VALUES ($1, $2, $3, 'game_condition', $4)
	`, ruleID, name, skinID, retroactive); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	return ruleID, skinID
}

func insertSkinRuleTestCondition(t *testing.T, db *sql.DB, ruleID uuid.UUID, metric, operator, value string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO skin_unlock_rule_conditions (skin_unlock_rule_id, metric, operator, value)
		VALUES ($1, $2, $3, $4)
	`, ruleID, metric, operator, value); err != nil {
		t.Fatalf("insert condition: %v", err)
	}
}

func TestSkinRuleIntegrationMixedConditionsRequireSameGameAndAggregateMatch(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, skinID := insertSkinRuleTestRule(t, db, "mixed-rule", true)
	insertSkinRuleTestCondition(t, db, ruleID, "is_winner", "eq", "true")
	insertSkinRuleTestCondition(t, db, ruleID, "penalty", "lte", "5")
	insertSkinRuleTestCondition(t, db, ruleID, "wins", "gte", "3")

	eligible := insertSkinRuleTestUser(t, db, "Eligible")
	ineligible := insertSkinRuleTestUser(t, db, "Split Games")
	for _, userID := range []uuid.UUID{eligible, ineligible} {
		if _, err := db.Exec(`INSERT INTO user_stats (user_id, games_played, wins) VALUES ($1, 10, 3)`, userID); err != nil {
			t.Fatal(err)
		}
	}

	insertGame := func(userID uuid.UUID, displayName string, winner bool, penalty int) {
		gameID := uuid.New()
		if _, err := db.Exec(`INSERT INTO games (id, room_id, started_at, finished_at) VALUES ($1, $2, $3, $4)`, gameID, gameID.String(), time.Now().Add(-time.Minute), time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO game_players (game_id, user_id, display_name, penalty_points, rank, is_winner) VALUES ($1, $2, $3, $4, 1, $5)`, gameID, userID, displayName, penalty, winner); err != nil {
			t.Fatal(err)
		}
	}
	insertGame(eligible, "Eligible", true, 4)
	insertGame(ineligible, "Split Winner", true, 20)
	insertGame(ineligible, "Split Low Penalty", false, 2)

	report, err := ReconcileProgressionSkins(db)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.GameConditionGrants != 1 {
		t.Fatalf("game condition grants = %d, want 1", report.GameConditionGrants)
	}

	var owners []uuid.UUID
	rows, err := db.Query(`SELECT user_id FROM user_skins WHERE skin_id = $1 AND skin_unlock_rule_id = $2`, skinID, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		owners = append(owners, id)
	}
	if len(owners) != 1 || owners[0] != eligible {
		t.Fatalf("owners = %v, want only %s", owners, eligible)
	}
}

func TestSkinRuleIntegrationCatalogReturnsEveryCondition(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, skinID := insertSkinRuleTestRule(t, db, "rising-champion", false)
	insertSkinRuleTestCondition(t, db, ruleID, "is_winner", "eq", "true")
	insertSkinRuleTestCondition(t, db, ruleID, "games_played", "gte", "3")
	insertSkinRuleTestCondition(t, db, ruleID, "wins", "gte", "2")

	catalog, err := GetSkinCatalog(db)
	if err != nil {
		t.Fatalf("get skin catalog: %v", err)
	}
	for _, skin := range catalog {
		if skin.ID == skinID.String() {
			if len(skin.UnlockRules) != 1 {
				t.Fatalf("unlock rules = %+v, want one", skin.UnlockRules)
			}
			want := []SkinUnlockCondition{
				{Metric: "is_winner", Operator: "eq", Value: "true"},
				{Metric: "games_played", Operator: "gte", Value: "3"},
				{Metric: "wins", Operator: "gte", Value: "2"},
			}
			if !reflect.DeepEqual(skin.UnlockRules[0].Conditions, want) {
				t.Fatalf("conditions = %+v, want %+v", skin.UnlockRules[0].Conditions, want)
			}
			return
		}
	}
	t.Fatalf("test skin %s not found in catalog", skinID)
}

func TestSkinRuleIntegrationCatalogExcludesPrivateAndDisabledSkins(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	visibleID, privateID, disabledID := uuid.New(), uuid.New(), uuid.New()
	for _, skin := range []struct {
		id             uuid.UUID
		name           string
		enabled        bool
		catalogVisible bool
	}{
		{visibleID, "Visible Skin", true, true},
		{privateID, "Private Skin", true, false},
		{disabledID, "Disabled Skin", false, true},
	} {
		if _, err := db.Exec(`
			INSERT INTO skins (id, skin_type, name, description, asset_key, display_order, enabled, catalog_visible)
			VALUES ($1, 'avatar_frame', $2, 'integration test', $3, 9999, $4, $5)
		`, skin.id, skin.name, skin.id.String()+".svg", skin.enabled, skin.catalogVisible); err != nil {
			t.Fatalf("insert %s: %v", skin.name, err)
		}
	}

	catalog, err := GetSkinCatalog(db)
	if err != nil {
		t.Fatalf("get skin catalog: %v", err)
	}
	found := make(map[string]bool)
	for _, skin := range catalog {
		found[skin.ID] = true
	}
	if !found[visibleID.String()] {
		t.Error("visible skin is missing from catalog")
	}
	if found[privateID.String()] {
		t.Error("private skin is present in catalog")
	}
	if found[disabledID.String()] {
		t.Error("disabled skin is present in catalog")
	}
}

func TestSkinRuleIntegrationRejectsDuplicateConditions(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, _ := insertSkinRuleTestRule(t, db, "duplicate-rule", false)
	insertSkinRuleTestCondition(t, db, ruleID, "wins", "gte", "3")

	_, err := db.Exec(`INSERT INTO skin_unlock_rule_conditions (skin_unlock_rule_id, metric, operator, value) VALUES ($1, 'wins', 'gte', '3')`, ruleID)
	var pqErr *pq.Error
	if err == nil || !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		t.Fatalf("duplicate insert error = %v, want PostgreSQL unique violation", err)
	}
}

func TestSkinRuleIntegrationSkipsInvalidConditionValue(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, _ := insertSkinRuleTestRule(t, db, "invalid-value-rule", false)
	insertSkinRuleTestCondition(t, db, ruleID, "wins", "gte", "many")
	userID := insertSkinRuleTestUser(t, db, "Invalid Rule User")

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	grants, err := GrantGameConditionSkins(tx, userID, achievementContext{Wins: 10}, time.Now())
	if err != nil {
		t.Fatalf("grant game-condition skins: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants = %+v, want invalid rule skipped", grants)
	}
}

func TestSkinRuleIntegrationUsesGameTimeAndHistoricalEventRevision(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, skinID := insertSkinRuleTestRule(t, db, "event-winner", false)
	insertSkinRuleTestCondition(t, db, ruleID, "is_winner", "eq", "true")
	userID := insertSkinRuleTestUser(t, db, "Event Winner")
	eventID := uuid.New()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	publishedAt := start.Add(-time.Hour)
	if _, err := db.Exec(`
		INSERT INTO events (id, slug, name, summary, description, starts_at, ends_at, enabled, lifecycle_state, revision, resource_version)
		VALUES ($1, $2, 'Event', '', '', $3, $4, TRUE, 'published', 2, 2)
	`, eventID, "event-"+eventID.String(), start.Add(7*24*time.Hour), end.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO event_versions (event_id, revision, slug, name, summary, description, starts_at, ends_at, reward_config, published_at)
		VALUES ($1, 1, $2, 'Event', '', '', $3, $4, '{}'::jsonb, $5),
		       ($1, 2, $2, 'Event', '', '', $6, $7, '{}'::jsonb, $8)
	`, eventID, "event-"+eventID.String(), start, end, publishedAt, start.Add(7*24*time.Hour), end.Add(7*24*time.Hour), end.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE skin_unlock_rules SET event_id = $2, event_revision = 1 WHERE id = $1`, ruleID, eventID); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	grants, err := GrantGameConditionSkins(tx, userID, achievementContext{IsWinner: true}, end.Add(-time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].ID != skinID.String() {
		t.Fatalf("grants = %+v", grants)
	}
	var grantedEvent uuid.UUID
	var revision int
	if err := db.QueryRow(`SELECT event_id, event_revision FROM user_skins WHERE user_id = $1 AND skin_id = $2`, userID, skinID).Scan(&grantedEvent, &revision); err != nil {
		t.Fatal(err)
	}
	if grantedEvent != eventID || revision != 1 {
		t.Fatalf("provenance = %s revision %d, want %s revision 1", grantedEvent, revision, eventID)
	}

	if _, err := db.Exec(`UPDATE events SET enabled = FALSE, lifecycle_state = 'archived' WHERE id = $1`, eventID); err != nil {
		t.Fatal(err)
	}
	delayedUser := insertSkinRuleTestUser(t, db, "Delayed User")
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	delayedGrants, err := GrantGameConditionSkins(tx, delayedUser, achievementContext{IsWinner: true}, start)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(delayedGrants) != 1 {
		t.Fatalf("delayed grants = %+v, want historical event grant", delayedGrants)
	}

	secondUser := insertSkinRuleTestUser(t, db, "Boundary User")
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	grants, err = GrantGameConditionSkins(tx, secondUser, achievementContext{IsWinner: true}, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("end-boundary grants = %+v, want none", grants)
	}
}

func TestSkinRuleIntegrationDeletingRuleCascadesConditions(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	ruleID, _ := insertSkinRuleTestRule(t, db, "cascade-rule", false)
	insertSkinRuleTestCondition(t, db, ruleID, "wins", "gte", "1")
	insertSkinRuleTestCondition(t, db, ruleID, "penalty", "lte", "5")

	if _, err := db.Exec(`DELETE FROM skin_unlock_rules WHERE id = $1`, ruleID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM skin_unlock_rule_conditions WHERE skin_unlock_rule_id = $1`, ruleID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("remaining conditions = %d, want 0", count)
	}
}
