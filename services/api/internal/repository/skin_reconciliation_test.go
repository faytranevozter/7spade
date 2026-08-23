package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReconcileProgressionSkinsBackfillsDurableRewardsAndReportsGameRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO user_skins").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO user_skins").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT r.id, r.metric, r.operator, r.value, r.retroactive, s.enabled").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value", "retroactive", "skin_enabled"}).
			AddRow("past-win", "is_winner", "eq", "true", true, true).
			AddRow("future-ace", "ace_closed", "eq", "true", false, true).
			AddRow("old-ace", "ace_closed", "eq", "true", true, true).
			AddRow("bad-penalty", "penalty", "gte", "many", true, true).
			AddRow("disabled", "wins", "gte", "1", true, false))
	mock.ExpectExec("INSERT INTO user_skins").
		WithArgs("past-win", true, "backfill:game_condition:past-win").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	report, err := ReconcileProgressionSkins(db)
	if err != nil {
		t.Fatalf("ReconcileProgressionSkins: %v", err)
	}
	if report.AchievementGrants != 2 || report.LevelGrants != 1 || report.GameConditionGrants != 1 {
		t.Fatalf("grant counts = %+v", report)
	}
	want := []struct{ status, reason string }{
		{"reconciled", ""},
		{"skipped", "prospective_only"},
		{"skipped", "unreconstructable_metric"},
		{"skipped", "malformed_value"},
		{"skipped", "disabled_skin"},
	}
	if len(report.GameRules) != len(want) {
		t.Fatalf("game rule outcomes = %+v", report.GameRules)
	}
	for i, expected := range want {
		if report.GameRules[i].Status != expected.status || report.GameRules[i].Reason != expected.reason {
			t.Errorf("outcome %d = %+v, want status=%q reason=%q", i, report.GameRules[i], expected.status, expected.reason)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileProgressionSkinsIsRerunnableAndDoesNotTouchEquippedSkins(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO user_skins").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_skins").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT r.id, r.metric, r.operator, r.value, r.retroactive, s.enabled").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value", "retroactive", "skin_enabled"}))
	mock.ExpectCommit()

	report, err := ReconcileProgressionSkins(db)
	if err != nil {
		t.Fatalf("ReconcileProgressionSkins: %v", err)
	}
	if report.TotalGrants() != 0 {
		t.Fatalf("report = %+v, want no grants on rerun", report)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
