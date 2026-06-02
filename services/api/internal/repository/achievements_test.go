package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// EvaluateAchievementIDs maps DB-configured rules to all achievement IDs the
// player qualifies for. All matching tiers are emitted independently.
func TestEvaluateAchievementIDsAwardsAllMatchingTiers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("FROM achievements a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value"}).
			AddRow(AchievementGames10, "games_played", "gte", "10").
			AddRow(AchievementGames50, "games_played", "gte", "50").
			AddRow(AchievementGames100, "games_played", "gte", "100").
			AddRow(AchievementStreak3, "current_streak", "gte", "3").
			AddRow(AchievementStreak5, "current_streak", "gte", "5"))
	mock.ExpectRollback()

	tx, _ := db.Begin()
	got, err := EvaluateAchievementIDs(tx, achievementContext{GamesPlayed: 100, CurrentStreak: 5})
	if err != nil {
		t.Fatalf("EvaluateAchievementIDs: %v", err)
	}
	_ = tx.Rollback()

	want := []string{AchievementGames10, AchievementGames50, AchievementGames100, AchievementStreak3, AchievementStreak5}
	if !sameSet(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

func TestEvaluateAchievementIDsSupportsBooleanAndAndRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("FROM achievements a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value"}).
			AddRow(AchievementFirstWin, "is_winner", "eq", "true").
			AddRow(AchievementSharedWin, "shared_win", "eq", "true").
			AddRow("winner_zero", "is_winner", "eq", "true").
			AddRow("winner_zero", "penalty", "eq", "0"))
	mock.ExpectRollback()

	tx, _ := db.Begin()
	got, err := EvaluateAchievementIDs(tx, achievementContext{IsWinner: true, SharedWin: true, Penalty: 0})
	if err != nil {
		t.Fatalf("EvaluateAchievementIDs: %v", err)
	}
	_ = tx.Rollback()

	want := []string{AchievementFirstWin, AchievementSharedWin, "winner_zero"}
	if !sameSet(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

func TestEvaluateAchievementIDsRejectsInvalidRule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("FROM achievements a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value"}).
			AddRow(AchievementFirstWin, "is_winner", "gte", "true"))
	mock.ExpectRollback()

	tx, _ := db.Begin()
	_, err = EvaluateAchievementIDs(tx, achievementContext{IsWinner: true})
	_ = tx.Rollback()
	if err == nil {
		t.Fatal("expected invalid rule error")
	}
}

// AwardAchievements inserts each DB-enabled id idempotently.
func TestAwardAchievements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO user_achievements").
		WithArgs(id, AchievementFirstWin).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_achievements").
		WithArgs(id, AchievementPerfectRound).
		WillReturnResult(sqlmock.NewResult(0, 0)) // ON CONFLICT no-op
	mock.ExpectCommit()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := AwardAchievements(tx, id, []string{AchievementFirstWin, AchievementPerfectRound}); err != nil {
		t.Fatalf("AwardAchievements: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetAchievementCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM achievements")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "icon"}).
			AddRow(AchievementFirstWin, "First Blood", "Win your first game", "🏆").
			AddRow(AchievementGames10, "Regular", "Play 10 games", "🎴"))

	catalog, err := GetAchievementCatalog(db)
	if err != nil {
		t.Fatalf("GetAchievementCatalog: %v", err)
	}
	if len(catalog) != 2 || catalog[0].ID != AchievementFirstWin || catalog[0].Name == "" {
		t.Fatalf("catalog = %+v", catalog)
	}
}

func TestAwardAchievementsEmptyIsNoOp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, _ := db.Begin()
	if err := AwardAchievements(tx, uuid.New(), nil); err != nil {
		t.Fatalf("AwardAchievements: %v", err)
	}
	_ = tx.Commit()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GetUserAchievements maps rows to DTOs; empty result is a non-nil empty slice.
func TestGetUserAchievements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("FROM user_achievements")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"achievement_id", "earned_at"}).
			AddRow(AchievementFirstWin, timeFixture()).
			AddRow(AchievementGames10, timeFixture()))

	earned, err := GetUserAchievements(db, id)
	if err != nil {
		t.Fatalf("GetUserAchievements: %v", err)
	}
	if len(earned) != 2 || earned[0].AchievementID != AchievementFirstWin {
		t.Fatalf("earned = %+v", earned)
	}
	if earned[0].EarnedAt == "" {
		t.Fatal("expected RFC3339 earned_at to be set")
	}
}

func TestGetUserAchievementsEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("FROM user_achievements")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"achievement_id", "earned_at"}))

	earned, err := GetUserAchievements(db, id)
	if err != nil {
		t.Fatalf("GetUserAchievements: %v", err)
	}
	if earned == nil || len(earned) != 0 {
		t.Fatalf("expected empty non-nil slice, got %+v", earned)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}

func timeFixture() time.Time {
	return time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
}
