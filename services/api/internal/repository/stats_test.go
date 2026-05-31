package repository

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// UpsertUserStats issues the expected upsert inside a transaction, passing
// winInc=1 for winners and the penalty for both total_penalty and best_penalty.
func TestUpsertUserStats(t *testing.T) {
	cases := []struct {
		name     string
		isWinner bool
		penalty  int
		wantWin  int
	}{
		{name: "winner", isWinner: true, penalty: 7, wantWin: 1},
		{name: "loser", isWinner: false, penalty: 20, wantWin: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			id := uuid.New()
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO user_stats").
				WithArgs(id, tc.wantWin, tc.penalty).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			if err := UpsertUserStats(tx, id, tc.isWinner, tc.penalty); err != nil {
				t.Fatalf("UpsertUserStats: %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

// GetLeaderboard runs a count query then a windowed page query, mapping rows to
// entries and threading min_games / pagination args through.
func TestGetLeaderboard(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM user_stats WHERE games_played >= $1")).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	rows := sqlmock.NewRows([]string{"rank", "user_id", "display_name", "games_played", "wins", "win_rate", "avg_penalty", "best_penalty"}).
		AddRow(1, "11111111-1111-1111-1111-111111111111", "Alice", 10, 7, 0.7, 12.5, 3).
		AddRow(2, "22222222-2222-2222-2222-222222222222", "Bob", 8, 4, 0.5, 15.0, nil)

	mock.ExpectQuery("FROM user_stats").
		WithArgs(5, 10, 0).
		WillReturnRows(rows)

	entries, total, err := GetLeaderboard(db, 1, 10, 5)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Rank != 1 || entries[0].DisplayName != "Alice" || entries[0].WinRate != 0.7 {
		t.Fatalf("entry[0] = %+v", entries[0])
	}
	if entries[0].BestPenalty == nil || *entries[0].BestPenalty != 3 {
		t.Fatalf("entry[0].BestPenalty = %v, want 3", entries[0].BestPenalty)
	}
	if entries[1].BestPenalty != nil {
		t.Fatalf("entry[1].BestPenalty = %v, want nil", entries[1].BestPenalty)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GetUserStats derives win_rate / avg_penalty and a rank when the user
// qualifies, returning found=true.
func TestGetUserStatsQualified(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "display_name", "games_played", "wins", "total_penalty", "best_penalty"}).
			AddRow(id.String(), "Alice", 10, 7, int64(125), 3))
	// rank query: 2 users ahead -> rank 3. The target user's rates are
	// recomputed in SQL from the `me` row, so only id + minGames are bound.
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(id, 5).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	stats, found, err := GetUserStats(db, id, 5)
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if stats.WinRate != 0.7 {
		t.Fatalf("win_rate = %v, want 0.7", stats.WinRate)
	}
	if stats.AvgPenalty != 12.5 {
		t.Fatalf("avg_penalty = %v, want 12.5", stats.AvgPenalty)
	}
	if !stats.Qualified || stats.Rank == nil || *stats.Rank != 3 {
		t.Fatalf("qualified=%v rank=%v, want qualified + rank 3", stats.Qualified, stats.Rank)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GetUserStats returns found=false and no error when the user has no row.
func TestGetUserStatsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	stats, found, err := GetUserStats(db, id, 5)
	if err != nil {
		t.Fatalf("GetUserStats: unexpected error %v", err)
	}
	if found || stats != nil {
		t.Fatalf("found=%v stats=%v, want not found", found, stats)
	}
}

// GetUserStats below the threshold returns found=true, qualified=false, nil rank.
func TestGetUserStatsSubThreshold(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "display_name", "games_played", "wins", "total_penalty", "best_penalty"}).
			AddRow(id.String(), "Newbie", 2, 1, int64(30), 10))

	stats, found, err := GetUserStats(db, id, 5)
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if stats.Qualified || stats.Rank != nil {
		t.Fatalf("qualified=%v rank=%v, want not qualified", stats.Qualified, stats.Rank)
	}
}
