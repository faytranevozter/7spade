package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// UpsertSeasonUserStats issues the per-season upsert inside the caller's
// transaction with winInc=1 for a winner and the penalty bound to both
// total_penalty and best_penalty.
func TestUpsertSeasonUserStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	season := "2026-06"
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO season_user_stats").
		WithArgs(season, id, 1, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := UpsertSeasonUserStats(tx, season, id, true, 7); err != nil {
		t.Fatalf("UpsertSeasonUserStats: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// EnsureActiveSeason takes the fast path (no rollover) when the open season's id
// already equals the current UTC month.
func TestEnsureActiveSeasonFastPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	current := time.Now().UTC().Format("2006-01")
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow(current, "Current", "2026-06-01T00:00:00Z", nil))

	s, err := EnsureActiveSeason(db)
	if err != nil {
		t.Fatalf("EnsureActiveSeason: %v", err)
	}
	if s.ID != current || !s.Active {
		t.Fatalf("season = %+v, want id=%s active", s, current)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// When the open season belongs to a past month (or none exists), EnsureActiveSeason
// rolls over: it closes stale seasons and opens the current month in one tx.
func TestEnsureActiveSeasonRollover(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	current := time.Now().UTC().Format("2006-01")
	// Open season is a stale month, triggering rollover.
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow("2025-01", "January 2025", "2025-01-01T00:00:00Z", nil))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE seasons SET ended_at = NOW()")).
		WithArgs(current).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO seasons").
		WithArgs(current, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery("SELECT started_at FROM seasons").
		WithArgs(current).
		WillReturnRows(sqlmock.NewRows([]string{"started_at"}).AddRow("2026-06-01T00:00:00Z"))

	s, err := EnsureActiveSeason(db)
	if err != nil {
		t.Fatalf("EnsureActiveSeason: %v", err)
	}
	if s.ID != current || !s.Active {
		t.Fatalf("season = %+v, want rolled over to %s", s, current)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ApplyRatingDelta and ApplySeasonRatingDelta issue clamped UPDATEs.
func TestApplyRatingDelta(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_stats SET rating = GREATEST(0, rating + $2) WHERE user_id = $1")).
		WithArgs(id, 12).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE season_user_stats SET rating = GREATEST(0, rating + $3)")).
		WithArgs("2026-06", id, -8).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, _ := db.Begin()
	if err := ApplyRatingDelta(tx, id, 12); err != nil {
		t.Fatalf("ApplyRatingDelta: %v", err)
	}
	if err := ApplySeasonRatingDelta(tx, "2026-06", id, -8); err != nil {
		t.Fatalf("ApplySeasonRatingDelta: %v", err)
	}
	_ = tx.Commit()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
