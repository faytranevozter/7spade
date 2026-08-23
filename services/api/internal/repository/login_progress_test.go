package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestRecordInteractiveLoginAdvancesUTCStreakAndReturnsNewGrants(t *testing.T) {
	tests := []struct {
		name        string
		lastLogin   any
		current     int
		best        int
		wantCurrent int
		wantBest    int
	}{
		{name: "first day", wantCurrent: 1, wantBest: 1},
		{name: "same day", lastLogin: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), current: 4, best: 7, wantCurrent: 4, wantBest: 7},
		{name: "next day", lastLogin: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), current: 4, best: 4, wantCurrent: 5, wantBest: 5},
		{name: "missed day", lastLogin: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), current: 4, best: 8, wantCurrent: 1, wantBest: 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			userID := uuid.New()
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO user_login_progress").WithArgs(userID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT current_streak, best_streak, last_login_date").WithArgs(userID).
				WillReturnRows(sqlmock.NewRows([]string{"current_streak", "best_streak", "last_login_date"}).AddRow(tc.current, tc.best, tc.lastLogin))
			sameDay, _ := tc.lastLogin.(time.Time)
			if !sameDay.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) {
				mock.ExpectExec("UPDATE user_login_progress").WithArgs(tc.wantCurrent, tc.wantBest, "2026-08-24", userID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, tc.wantCurrent).
				WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}).
					AddRow("skin-1", SkinTypeAvatarFrame, "Streak", "reward", "streak.svg", 1, "login_streak:1"))
			authenticatedAt := time.Date(2026, 8, 25, 0, 0, 0, 0, time.FixedZone("local", 3600))
			expiresAt := authenticatedAt.Add(30 * 24 * time.Hour)
			mock.ExpectExec("INSERT INTO refresh_tokens").WithArgs(sqlmock.AnyArg(), userID, "refresh-hash", expiresAt, authenticatedAt).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			progress, grants, err := RecordInteractiveLogin(db, userID, authenticatedAt, "refresh-hash", expiresAt)
			if err != nil {
				t.Fatalf("RecordInteractiveLogin: %v", err)
			}
			if progress.CurrentStreak != tc.wantCurrent || progress.BestStreak != tc.wantBest {
				t.Fatalf("progress = %+v", progress)
			}
			if len(grants) != 1 || grants[0].Source != "login_streak:1" {
				t.Fatalf("grants = %+v", grants)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
