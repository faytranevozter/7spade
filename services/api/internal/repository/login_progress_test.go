package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestDailyLoginXPIncreasesAndCaps(t *testing.T) {
	cfg := DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: time.UTC}
	for _, tc := range []struct {
		day  int
		want int
	}{{1, 10}, {5, 30}, {9, 50}, {20, 50}} {
		if got := DailyLoginXP(tc.day, cfg); got != tc.want {
			t.Fatalf("DailyLoginXP(%d) = %d, want %d", tc.day, got, tc.want)
		}
	}
}

func TestGetLoginProgressUsesConfiguredTimezoneBoundary(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := uuid.New()
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
	mock.ExpectQuery("SELECT current_streak, best_streak, last_login_date").WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"current_streak", "best_streak", "last_login_date"}).
			AddRow(4, 4, time.Date(2026, 8, 24, 16, 30, 0, 0, time.UTC)))
	mock.ExpectQuery("SELECT COALESCE").WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"xp"}).AddRow(100))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(userID, 4).WillReturnRows(sqlmock.NewRows([]string{"exists", "next"}).AddRow(false, nil))

	result, err := GetLoginProgress(db, userID, time.Date(2026, 8, 24, 17, 30, 0, 0, time.UTC), DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: jakarta})
	if err != nil {
		t.Fatal(err)
	}
	if result.Progress.ClaimedToday || result.Progress.CurrentStreak != 4 || result.AppTimezone != "+07:00" {
		t.Fatalf("result = %+v", result)
	}
}

func TestGetLoginProgressReportsExpiredStreakAsZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := uuid.New()
	mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
	mock.ExpectQuery("SELECT current_streak, best_streak, last_login_date").WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"current_streak", "best_streak", "last_login_date"}).
			AddRow(4, 8, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)))

	mock.ExpectQuery("SELECT COALESCE").WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"xp"}).AddRow(0))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(userID, 0).WillReturnRows(sqlmock.NewRows([]string{"exists", "next"}).AddRow(false, nil))
	result, err := GetLoginProgress(db, userID, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC), DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if result.Progress.CurrentStreak != 0 || result.Progress.BestStreak != 8 || result.Progress.ClaimedToday {
		t.Fatalf("result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimDailyLoginUsesProgressTransactionWithoutRefreshToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := uuid.New()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
	mock.ExpectExec("INSERT INTO user_login_progress").WithArgs(userID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT current_streak, best_streak, last_login_date").WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"current_streak", "best_streak", "last_login_date"}).AddRow(4, 4, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)))
	mock.ExpectExec("UPDATE user_login_progress").WithArgs(5, 5, "2026-08-25", userID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_stats").WithArgs(userID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("UPDATE user_stats").WithArgs(30, userID).WillReturnRows(sqlmock.NewRows([]string{"xp"}).AddRow(130))
	mock.ExpectExec("INSERT INTO daily_login_xp_events").WithArgs(userID, "2026-08-25", 5, int64(100), int64(130), 30).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}))
	mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(userID, 5).WillReturnRows(sqlmock.NewRows([]string{"exists", "next"}).AddRow(false, nil))
	mock.ExpectCommit()

	result, err := ClaimDailyLogin(db, userID, now, DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if result.Progress.CurrentStreak != 5 || !result.Progress.ClaimedToday || result.XPDelta != 30 || result.XPAfter != 130 || len(result.SkinGrants) != 0 {
		t.Fatalf("result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimDailyLoginStopsBeforeMutationWhenFeatureIsDisabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
	mock.ExpectRollback()

	_, err = ClaimDailyLogin(db, uuid.New(), time.Now(), DailyLoginConfig{})
	if err != ErrDailyLoginDisabled {
		t.Fatalf("ClaimDailyLogin error = %v, want ErrDailyLoginDisabled", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimDailyLoginAdvancesStreakAndReturnsNewGrants(t *testing.T) {
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
			mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
			mock.ExpectExec("INSERT INTO user_login_progress").WithArgs(userID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT current_streak, best_streak, last_login_date").WithArgs(userID).
				WillReturnRows(sqlmock.NewRows([]string{"current_streak", "best_streak", "last_login_date"}).AddRow(tc.current, tc.best, tc.lastLogin))
			sameDay, _ := tc.lastLogin.(time.Time)
			if !sameDay.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) {
				mock.ExpectExec("UPDATE user_login_progress").WithArgs(tc.wantCurrent, tc.wantBest, "2026-08-24", userID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("INSERT INTO user_stats").WithArgs(userID).WillReturnResult(sqlmock.NewResult(0, 1))
			if sameDay.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) {
				mock.ExpectQuery("SELECT xp FROM user_stats").WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"xp"}).AddRow(100))
			} else {
				xpDelta := DailyLoginXP(tc.wantCurrent, DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: time.UTC})
				mock.ExpectQuery("UPDATE user_stats").WithArgs(xpDelta, userID).WillReturnRows(sqlmock.NewRows([]string{"xp"}).AddRow(100 + xpDelta))
				mock.ExpectExec("INSERT INTO daily_login_xp_events").WithArgs(userID, "2026-08-24", tc.wantCurrent, int64(100), int64(100+xpDelta), xpDelta).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, tc.wantCurrent).
				WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}).
					AddRow("skin-1", SkinTypeAvatarFrame, "Streak", "reward", "streak.svg", 1, "login_streak:1"))
			mock.ExpectQuery("INSERT INTO user_skins").WithArgs(userID, sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "skin_type", "name", "description", "asset_key", "display_order", "source"}))
			mock.ExpectQuery("SELECT EXISTS").WithArgs(userID, tc.wantCurrent).WillReturnRows(sqlmock.NewRows([]string{"exists", "next"}).AddRow(true, nil))
			mock.ExpectCommit()

			claimedAt := time.Date(2026, 8, 25, 0, 0, 0, 0, time.FixedZone("local", 3600))
			result, err := ClaimDailyLogin(db, userID, claimedAt, DailyLoginConfig{XPBase: 10, XPStep: 5, XPMax: 50, Timezone: time.UTC})
			if err != nil {
				t.Fatalf("ClaimDailyLogin: %v", err)
			}
			if result.Progress.CurrentStreak != tc.wantCurrent || result.Progress.BestStreak != tc.wantBest {
				t.Fatalf("result = %+v", result)
			}
			if len(result.SkinGrants) != 1 || result.SkinGrants[0].Source != "login_streak:1" {
				t.Fatalf("grants = %+v", result.SkinGrants)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
