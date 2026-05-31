package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// evaluateAchievementIDs maps a player's game + counters to the achievement IDs
// they qualify for. Idempotency downstream means re-emitting an earned id is OK.
func TestEvaluateAchievementIDs(t *testing.T) {
	cases := []struct {
		name string
		ctx  achievementContext
		want []string
	}{
		{
			name: "first win + perfect round",
			ctx:  achievementContext{IsWinner: true, Penalty: 0, GamesPlayed: 1, Streak: 1},
			want: []string{AchievementFirstWin, AchievementPerfectRound},
		},
		{
			name: "shared win",
			ctx:  achievementContext{IsWinner: true, SharedWin: true, Penalty: 4, GamesPlayed: 2, Streak: 1},
			want: []string{AchievementFirstWin, AchievementSharedWin},
		},
		{
			name: "games milestone picks highest only",
			ctx:  achievementContext{IsWinner: false, Penalty: 5, GamesPlayed: 100, Streak: 0},
			want: []string{AchievementGames100},
		},
		{
			name: "games 10 boundary",
			ctx:  achievementContext{IsWinner: false, Penalty: 5, GamesPlayed: 10, Streak: 0},
			want: []string{AchievementGames10},
		},
		{
			name: "streak 5 picks highest only",
			ctx:  achievementContext{IsWinner: true, Penalty: 3, GamesPlayed: 6, Streak: 5},
			want: []string{AchievementFirstWin, AchievementStreak5},
		},
		{
			name: "streak 3",
			ctx:  achievementContext{IsWinner: true, Penalty: 3, GamesPlayed: 4, Streak: 3},
			want: []string{AchievementFirstWin, AchievementStreak3},
		},
		{
			name: "loser with penalty earns nothing",
			ctx:  achievementContext{IsWinner: false, Penalty: 12, GamesPlayed: 5, Streak: 0},
			want: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateAchievementIDs(tc.ctx)
			if !sameSet(got, tc.want) {
				t.Fatalf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// AwardAchievements inserts each allowlisted id idempotently and skips unknown ids.
func TestAwardAchievements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectBegin()
	// Only the two valid ids should be inserted; "bogus" is skipped.
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
	if err := AwardAchievements(tx, id, []string{AchievementFirstWin, "bogus", AchievementPerfectRound}); err != nil {
		t.Fatalf("AwardAchievements: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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
