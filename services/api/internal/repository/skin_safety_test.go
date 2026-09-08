package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// Check the SQL actually executed by every repository automatic grant path,
// rather than a copy of the statement or the trigger's fallback behavior.
func TestAutomaticSkinGrantsSelectEnabledCurrentRevision(t *testing.T) {
	for _, path := range []string{"achievement", "level", "login", "event", "game", "reconciliation"} {
		t.Run(path, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
				if expected != "grant" {
					return sqlmock.QueryMatcherRegexp.Match(expected, actual)
				}
				for _, fragment := range []string{"INSERT INTO user_skins", "skin_revision_id", "sr.id", "sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled"} {
					if !strings.Contains(actual, fragment) {
						return fmt.Errorf("grant SQL missing %q: %s", fragment, actual)
					}
				}
				return nil
			})))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			userID := uuid.New()
			if path == "reconciliation" {
				mock.ExpectExec("grant").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("grant").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery("SELECT r.id").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "metric", "operator", "value", "retroactive", "enabled"}).AddRow("rule", "rule", "wins", "gte", "1", true, true))
				mock.ExpectExec("grant").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
				_, err = ReconcileProgressionSkins(db)
			} else {
				if path == "game" {
					mock.ExpectQuery("SELECT r.id").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "skin", "event", "revision", "metric", "operator", "value"}).AddRow("rule", "rule", "skin", nil, nil, "is_winner", "eq", "false"))
				}
				mock.ExpectQuery("grant").WillReturnRows(sqlmock.NewRows([]string{"id", "type", "name", "description", "asset", "order", "source"}))
				mock.ExpectRollback()
				tx, beginErr := db.Begin()
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				switch path {
				case "achievement":
					_, err = GrantAchievementSkins(tx, userID, []string{"winner"}, time.Now())
				case "level":
					_, err = GrantMinimumLevelSkins(tx, userID, 2, time.Now())
				case "login":
					_, err = grantLoginStreakSkins(tx, userID, 2)
				case "event":
					_, err = grantEventCheckInSkins(tx, uuid.NewString(), 1, userID, 2)
				case "game":
					_, err = GrantGameConditionSkins(tx, userID, achievementContext{}, time.Now())
				}
				_ = tx.Rollback()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSkinSafetyIntegrationSkipsUnavailableAndPinsCurrent(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	userID := insertSkinRuleTestUser(t, db, "skin safety")
	if _, err := db.Exec(`UPDATE skin_unlock_rules SET enabled=FALSE`); err != nil {
		t.Fatal(err)
	}
	for _, ruleType := range []string{"minimum_level", "login_streak"} {
		for _, state := range []string{"unpublished", "disabled_revision", "disabled_skin", "available"} {
			t.Run(ruleType+"/"+state, func(t *testing.T) {
				skinID, revisionID := uuid.New(), uuid.New()
				if _, err := db.Exec(`INSERT INTO skins(id,skin_type,name,description,asset_key,enabled) VALUES($1,'avatar_frame',$2,'integration test','current.svg',$3)`, skinID, state, state != "disabled_skin"); err != nil {
					t.Fatal(err)
				}
				// An older enabled revision must never be used as a fallback.
				if _, err := db.Exec(`INSERT INTO skin_revisions(skin_id,version,asset_key,content_type) VALUES($1,1,'old.svg','image/svg+xml')`, skinID); err != nil {
					t.Fatal(err)
				}
				if state != "unpublished" {
					if _, err := db.Exec(`INSERT INTO skin_revisions(id,skin_id,version,asset_key,content_type,enabled,disabled_at) VALUES($1,$2,2,'current.svg','image/svg+xml',$3,CASE WHEN $3 THEN NULL ELSE NOW() END)`, revisionID, skinID, state != "disabled_revision"); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.Exec(`INSERT INTO skin_unlock_rules(name,skin_id,rule_type,minimum_level,login_streak_days) VALUES($1,$2,$3,1,1)`, ruleType+state, skinID, ruleType); err != nil {
					t.Fatal(err)
				}
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				grant := func(tx *sql.Tx, userID uuid.UUID, threshold int) ([]SkinGrant, error) {
					return GrantMinimumLevelSkins(tx, userID, threshold, time.Now())
				}
				if ruleType == "login_streak" {
					grant = grantLoginStreakSkins
				}
				if _, err = grant(tx, userID, 1); err != nil {
					t.Fatal(err)
				}
				if _, err = grant(tx, userID, 1); err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(); err != nil {
					t.Fatal(err)
				}
				var pinned uuid.UUID
				err = db.QueryRow(`SELECT skin_revision_id FROM user_skins WHERE user_id=$1 AND skin_id=$2`, userID, skinID).Scan(&pinned)
				if state == "available" {
					if err != nil || pinned != revisionID {
						t.Fatalf("pin=%s err=%v, want %s", pinned, err, revisionID)
					}
				} else if err != sql.ErrNoRows {
					t.Fatalf("unavailable skin granted: pin=%s err=%v", pinned, err)
				}
			})
		}
	}
}

func TestSkinSafetyIntegrationStarterTrigger(t *testing.T) {
	db := openSkinRuleIntegrationDB(t)
	if _, err := db.Exec(`UPDATE skins SET is_starter=FALSE`); err != nil {
		t.Fatal(err)
	}
	expected := map[uuid.UUID]uuid.UUID{}
	for _, state := range []string{"no_revision", "missing_current", "disabled_revision", "disabled_skin", "not_starter", "available"} {
		skinID, revisionID := uuid.New(), uuid.New()
		if _, err := db.Exec(`INSERT INTO skins(id,skin_type,name,description,asset_key,is_starter,enabled,catalog_visible) VALUES($1,'avatar_frame',$2,'integration test','current.svg',$3,$4,FALSE)`, skinID, state, state != "not_starter", state != "disabled_skin"); err != nil {
			t.Fatal(err)
		}
		if state != "no_revision" {
			if _, err := db.Exec(`INSERT INTO skin_revisions(skin_id,version,asset_key,content_type) VALUES($1,1,'old.svg','image/svg+xml')`, skinID); err != nil {
				t.Fatal(err)
			}
		}
		if state != "no_revision" && state != "missing_current" {
			if _, err := db.Exec(`INSERT INTO skin_revisions(id,skin_id,version,asset_key,content_type,enabled,disabled_at) VALUES($1,$2,2,'current.svg','image/svg+xml',$3,CASE WHEN $3 THEN NULL ELSE NOW() END)`, revisionID, skinID, state != "disabled_revision"); err != nil {
				t.Fatal(err)
			}
		}
		if state == "available" {
			expected[skinID] = revisionID
		}
	}
	// Disable fallback pinning so a successful insert proves the starter trigger
	// supplies the revision itself. All changes are confined to the test schema.
	if _, err := db.Exec(`ALTER TABLE user_skins DISABLE TRIGGER user_skins_pin_revision`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		userID := insertSkinRuleTestUser(t, db, "starter safety")
		rows, err := db.Query(`SELECT skin_id,skin_revision_id,source FROM user_skins WHERE user_id=$1`, userID)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var skinID, revisionID uuid.UUID
			var source string
			if err := rows.Scan(&skinID, &revisionID, &source); err != nil {
				t.Fatal(err)
			}
			if expected[skinID] != revisionID || source != "starter" {
				t.Fatalf("unexpected starter grant: skin=%s revision=%s source=%s", skinID, revisionID, source)
			}
			count++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if count != len(expected) {
			t.Fatalf("got %d starter grants, want %d", count, len(expected))
		}
	}
}
