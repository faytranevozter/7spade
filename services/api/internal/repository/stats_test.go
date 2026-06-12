package repository

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// UpsertUserStats issues the expected upsert inside a transaction, passing
// the full UpsertUserStatsParams and returning the post-update counters.
func TestUpsertUserStats(t *testing.T) {
	cases := []struct {
		name      string
		isWinner  bool
		rank      int
		penalty   int
		hasBot    bool
		retGames  int
		retWins   int
		retStreak int
	}{
		{name: "winner", isWinner: true, rank: 1, penalty: 7, hasBot: false, retGames: 3, retWins: 2, retStreak: 2},
		{name: "loser rank 3", isWinner: false, rank: 3, penalty: 20, hasBot: true, retGames: 4, retWins: 2, retStreak: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			id := uuid.New()
			mock.ExpectBegin()
			mock.ExpectQuery("INSERT INTO user_stats").
				WillReturnRows(sqlmock.NewRows([]string{"games_played", "wins", "current_streak", "current_top2_streak", "best_win_streak", "best_top2_streak"}).
					AddRow(tc.retGames, tc.retWins, tc.retStreak, 0, 0, 0))
			mock.ExpectCommit()

			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			params := UpsertUserStatsParams{
				UserID:      id,
				IsWinner:    tc.isWinner,
				Penalty:     tc.penalty,
				Rank:        tc.rank,
				HasBot:      tc.hasBot,
				CloseWin:    false,
				CloseLoss:   false,
				BlowoutWin:  false,
				BlowoutLoss: false,
			}
			snap, err := UpsertUserStats(tx, params)
			if err != nil {
				t.Fatalf("UpsertUserStats: %v", err)
			}
			if snap.GamesPlayed != tc.retGames || snap.Wins != tc.retWins || snap.CurrentStreak != tc.retStreak {
				t.Fatalf("snapshot = %+v, want games=%d wins=%d streak=%d", snap, tc.retGames, tc.retWins, tc.retStreak)
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

	cols := []string{"rank", "user_id", "display_name", "avatar_url", "games_played", "wins", "win_rate", "avg_penalty", "best_penalty", "rating", "avg_rank", "top2_rate", "first_place_count", "human_only_games", "bot_mixed_games"}
	rows := sqlmock.NewRows(cols).
		AddRow(1, "11111111-1111-1111-1111-111111111111", "Alice", "https://cdn/a.png", 10, 7, 0.7, 12.5, 3, 1300, 1.5, 0.8, 5, 8, 2).
		AddRow(2, "22222222-2222-2222-2222-222222222222", "Bob", nil, 8, 4, 0.5, 15.0, nil, 1180, 2.1, 0.5, 2, 5, 3)

	mock.ExpectQuery("FROM user_stats").
		WithArgs(5, 10, 0).
		WillReturnRows(rows)

	entries, total, appliedSort, err := GetLeaderboard(db, 1, 10, 5, "win_rate", "")
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if appliedSort != "win_rate" {
		t.Fatalf("appliedSort = %q, want win_rate", appliedSort)
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
	if entries[0].AvatarURL == nil || *entries[0].AvatarURL != "https://cdn/a.png" {
		t.Fatalf("entry[0].AvatarURL = %v, want https://cdn/a.png", entries[0].AvatarURL)
	}
	if entries[1].AvatarURL != nil {
		t.Fatalf("entry[1].AvatarURL = %v, want nil", entries[1].AvatarURL)
	}
	if entries[0].BestPenalty == nil || *entries[0].BestPenalty != 3 {
		t.Fatalf("entry[0].BestPenalty = %v, want 3", entries[0].BestPenalty)
	}
	if entries[1].BestPenalty != nil {
		t.Fatalf("entry[1].BestPenalty = %v, want nil", entries[1].BestPenalty)
	}
	if entries[0].Rating != 1300 || entries[1].Rating != 1180 {
		t.Fatalf("ratings = %d/%d, want 1300/1180", entries[0].Rating, entries[1].Rating)
	}
	if entries[0].HumanOnlyGames != 8 || entries[1].BotMixedGames != 3 {
		t.Fatalf("human/bot = %d/%d, want 8/3", entries[0].HumanOnlyGames, entries[1].BotMixedGames)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GetLeaderboard applies the requested sort's ORDER BY fragment and echoes the
// normalized key back; an unknown sort falls back to the win_rate default.
func TestGetLeaderboardSort(t *testing.T) {
	cases := []struct {
		name      string
		sort      string
		wantSort  string
		wantOrder string
	}{
		{name: "total_wins", sort: "total_wins", wantSort: "total_wins", wantOrder: "ORDER BY wins DESC"},
		{name: "avg_penalty", sort: "avg_penalty", wantSort: "avg_penalty", wantOrder: "ORDER BY (total_penalty::float8 / games_played) ASC"},
		{name: "best_penalty", sort: "best_penalty", wantSort: "best_penalty", wantOrder: "ORDER BY best_penalty ASC NULLS LAST"},
		{name: "games_played", sort: "games_played", wantSort: "games_played", wantOrder: "ORDER BY games_played DESC"},
		{name: "unknown falls back to win_rate", sort: "bogus", wantSort: "win_rate", wantOrder: "ORDER BY (wins::float8 / games_played) DESC"},
		{name: "empty falls back to win_rate", sort: "", wantSort: "win_rate", wantOrder: "ORDER BY (wins::float8 / games_played) DESC"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM user_stats WHERE games_played >= $1")).
				WithArgs(5).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			cols := []string{"rank", "user_id", "display_name", "avatar_url", "games_played", "wins", "win_rate", "avg_penalty", "best_penalty", "rating", "avg_rank", "top2_rate", "first_place_count", "human_only_games", "bot_mixed_games"}
			mock.ExpectQuery(regexp.QuoteMeta(tc.wantOrder)).
				WithArgs(5, 10, 0).
				WillReturnRows(sqlmock.NewRows(cols))

			_, _, appliedSort, err := GetLeaderboard(db, 1, 10, 5, tc.sort, "")
			if err != nil {
				t.Fatalf("GetLeaderboard: %v", err)
			}
			if appliedSort != tc.wantSort {
				t.Fatalf("appliedSort = %q, want %q", appliedSort, tc.wantSort)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
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
	cols := []string{"user_id", "display_name", "avatar_url", "games_played", "wins", "total_penalty", "best_penalty", "worst_penalty", "rating", "rank_sum", "first_place_count", "second_place_count", "third_place_count", "fourth_place_count", "zero_penalty_games", "low_penalty_games", "high_penalty_games", "human_only_games", "bot_mixed_games", "current_streak", "best_win_streak", "current_top2_streak", "best_top2_streak", "close_wins", "close_losses", "blowout_wins", "blowout_losses"}
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(id.String(), "Alice", "https://cdn/a.png", 10, 7, int64(125), 3, 20, 1250, int64(15), 5, 3, 2, 0, 1, 5, 4, 8, 2, 3, 3, 2, 2, 2, 3, 1, 2))
	// rank query: 2 users ahead -> rank 3.
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(id, 5).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	stats, found, err := GetUserStats(db, id, 5, "")
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if stats.AvatarURL == nil || *stats.AvatarURL != "https://cdn/a.png" {
		t.Fatalf("avatar = %v, want https://cdn/a.png", stats.AvatarURL)
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
	if stats.Rating != 1250 {
		t.Fatalf("rating = %d, want 1250", stats.Rating)
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

	stats, found, err := GetUserStats(db, id, 5, "")
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
	cols := []string{"user_id", "display_name", "avatar_url", "games_played", "wins", "total_penalty", "best_penalty", "worst_penalty", "rating", "rank_sum", "first_place_count", "second_place_count", "third_place_count", "fourth_place_count", "zero_penalty_games", "low_penalty_games", "high_penalty_games", "human_only_games", "bot_mixed_games", "current_streak", "best_win_streak", "current_top2_streak", "best_top2_streak", "close_wins", "close_losses", "blowout_wins", "blowout_losses"}
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(id.String(), "Newbie", nil, 2, 1, int64(30), 10, 30, 1200, int64(5), 1, 1, 0, 0, 1, 1, 0, 2, 0, 1, 1, 1, 1, 1, 1, 0, 0))

	stats, found, err := GetUserStats(db, id, 5, "")
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if stats.AvatarURL != nil {
		t.Fatalf("avatar = %v, want nil", stats.AvatarURL)
	}
	if stats.Qualified || stats.Rank != nil {
		t.Fatalf("qualified=%v rank=%v, want not qualified", stats.Qualified, stats.Rank)
	}
}

// GetUserAvatar returns the resolved avatar, or nil when the LATERAL select
// yields SQL NULL (no provider avatar).
func TestGetUserAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM users u").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"avatar_url"}).AddRow("https://cdn/pic.png"))

	avatar, err := GetUserAvatar(db, id)
	if err != nil {
		t.Fatalf("GetUserAvatar: %v", err)
	}
	if avatar == nil || *avatar != "https://cdn/pic.png" {
		t.Fatalf("avatar = %v, want https://cdn/pic.png", avatar)
	}

	// NULL avatar (e.g. email/password-only user) -> nil, no error.
	mock.ExpectQuery("FROM users u").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"avatar_url"}).AddRow(nil))
	avatar, err = GetUserAvatar(db, id)
	if err != nil {
		t.Fatalf("GetUserAvatar (null): %v", err)
	}
	if avatar != nil {
		t.Fatalf("avatar = %v, want nil", avatar)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
