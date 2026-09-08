package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func TestCreateSkinPersistsAndReturnsUnlockRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	level := 3
	rules := []model.SkinUnlockRule{
		{Name: "level-three", RuleType: "minimum_level", MinimumLevel: &level, Retroactive: true, Enabled: true},
		{Name: "win-cleanly", RuleType: "game_condition", Enabled: true, Conditions: json.RawMessage(`[{"metric":"is_winner","operator":"eq","value":"true"}]`)},
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO skins").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skin_unlock_rules").WithArgs(sqlmock.AnyArg(), "level-three", sqlmock.AnyArg(), "minimum_level", "", &level, (*int)(nil), "", (*int)(nil), (*int)(nil), true, true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skin_unlock_rules").WithArgs(sqlmock.AnyArg(), "win-cleanly", sqlmock.AnyArg(), "game_condition", "", (*int)(nil), (*int)(nil), "", (*int)(nil), (*int)(nil), false, true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skin_unlock_rule_conditions").WithArgs(sqlmock.AnyArg(), "is_winner", "eq", "true").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	created, err := NewPostgresStore(db, "test").CreateSkin(context.Background(), Skin{SkinType: "avatar_frame", Name: "Earned", UnlockRules: rules}, model.AuditEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.UnlockRules) != 2 || created.UnlockRules[1].Conditions == nil {
		t.Fatalf("created unlock rules = %+v, want submitted rules", created.UnlockRules)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSkinRollsBackMetadataWhenRulePersistenceFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	level := 2
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO skins").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO skin_unlock_rules").WillReturnError(errors.New("rule unavailable"))
	mock.ExpectRollback()

	_, err = NewPostgresStore(db, "test").CreateSkin(context.Background(), Skin{SkinType: "avatar_frame", Name: "Earned", UnlockRules: []model.SkinUnlockRule{{Name: "level", RuleType: "minimum_level", MinimumLevel: &level, Enabled: true}}}, model.AuditEvent{})
	if err == nil {
		t.Fatal("CreateSkin succeeded, want rule persistence failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSkinChecksRevisionAfterLockAndGuardsAvailability(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		enabled, visible, revision bool
	}{
		{"enable unpublished", true, false, false},
		{"show unpublished", false, true, false},
		{"hide unpublished", false, false, false},
		{"enable published", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT id FROM skins WHERE id=\$1 FOR UPDATE`).WithArgs("skin").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("skin"))
			mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM skins s JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled WHERE s.id=\$1\)`).WithArgs("skin").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(tc.revision))
			conflict := (tc.enabled || tc.visible) && !tc.revision
			if conflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec("UPDATE skins SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(false))
				mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(false))
				mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			_, err = NewPostgresStore(db, "test").UpdateSkin(context.Background(), "skin", Skin{Enabled: tc.enabled, CatalogVisible: tc.visible}, model.AuditEvent{})
			if conflict && !errors.Is(err, ErrConflict) || !conflict && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDisableSkinRevisionLocksSkinBeforeRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM skins WHERE id=\$1 FOR UPDATE`).WithArgs("skin").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("skin"))
	mock.ExpectExec("UPDATE skin_revisions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE skins").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := NewPostgresStore(db, "test").DisableSkinRevision(context.Background(), "skin", "revision", model.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSkinRetroactiveGrantUsesEnabledCurrentRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM skins WHERE id=\$1 FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("skin"))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
	mock.ExpectExec("UPDATE skins SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(false))
	mock.ExpectExec("DELETE FROM skin_unlock_rules").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO skin_unlock_rules").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO user_skins .*skin_revision_id.*SELECT us.user_id,\$1,\$2,r.event_id,r.event_revision,sr.id.*JOIN skins s ON s.id=\$1 AND s.enabled JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled`).WithArgs("skin", sqlmock.AnyArg(), 2).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(true))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	level := 2
	next, err := NewPostgresStore(db, "test").UpdateSkin(context.Background(), "skin", Skin{Enabled: true, UnlockRules: []model.SkinUnlockRule{{Name: "level", RuleType: "minimum_level", MinimumLevel: &level, Retroactive: true, Enabled: true}}}, model.AuditEvent{})
	if err != nil || !next.UnlockRulesLocked {
		t.Fatalf("UpdateSkin = %+v, %v", next, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
