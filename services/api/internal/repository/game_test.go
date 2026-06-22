package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestSaveGameUpdatesRegisteredPlayerStats(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	result := GameResult{
		RoomID:     "22222222-2222-2222-2222-222222222222",
		StartedAt:  time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC),
		Players: []GameResultPlayer{
			{UserID: userID.String(), DisplayName: "Alice", PenaltyPoints: 3, Rank: 1, IsWinner: true, IsBot: false},
			{DisplayName: "Bot 1", PenaltyPoints: 7, Rank: 2, IsWinner: false, IsBot: true},
		},
	}

	// EnsureActiveSeason runs before the save transaction.
	activeSeason := time.Now().UTC().Format("2006-01")
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow(activeSeason, "Current", "2026-06-01T00:00:00Z", nil))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO games").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO game_players").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO user_stats").
		WillReturnRows(sqlmock.NewRows([]string{"games_played", "wins", "current_streak", "current_top2_streak", "best_win_streak", "best_top2_streak", "first_place_count", "zero_penalty_games", "human_only_games", "xp"}).AddRow(1, 1, 1, 1, 1, 1, 1, 0, 0, 125))
	mock.ExpectExec("INSERT INTO player_xp_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO season_user_stats").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("FROM achievements a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "metric", "operator", "value"}).
			AddRow(AchievementFirstWin, "is_winner", "eq", "true").
			AddRow(AchievementPerfectRound, "penalty", "eq", "0"))
	mock.ExpectExec("INSERT INTO user_achievements").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO game_players").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT rating FROM user_stats").
		WillReturnRows(sqlmock.NewRows([]string{"rating"}).AddRow(1200))
	mock.ExpectExec("INSERT INTO player_rating_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := SaveGame(db, result); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// closeGamePenalties / flagsFor classify close-game stories from the table
// outcome. The margins must anchor to the *actual* winner/runner-up even when a
// bot or guest takes a podium spot, and must treat a zero-penalty winner as a
// real winner (not "unset").
func TestCloseGameStoryFlags(t *testing.T) {
	const human = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name    string
		players []GameResultPlayer
		// target is the registered human whose flags we assert.
		want closeGameStoryFlags
	}{
		{
			name: "bot winner, human close loss",
			players: []GameResultPlayer{
				{DisplayName: "Bot", PenaltyPoints: 2, Rank: 1, IsWinner: true, IsBot: true},
				{UserID: human, DisplayName: "Alice", PenaltyPoints: 4, Rank: 2, IsWinner: false},
				{DisplayName: "Bot2", PenaltyPoints: 9, Rank: 3, IsBot: true},
			},
			// margin to winner (2) is 2 <= 3 -> close loss; not last place, not blowout.
			want: closeGameStoryFlags{CloseLoss: true},
		},
		{
			name: "bot winner, human blowout loss in last place",
			players: []GameResultPlayer{
				{DisplayName: "Bot", PenaltyPoints: 2, Rank: 1, IsWinner: true, IsBot: true},
				{DisplayName: "Bot2", PenaltyPoints: 6, Rank: 2, IsBot: true},
				{UserID: human, DisplayName: "Alice", PenaltyPoints: 30, Rank: 3, IsWinner: false},
			},
			// margin from winner (2) is 28 >= 15 and is last place -> blowout loss.
			want: closeGameStoryFlags{BlowoutLoss: true},
		},
		{
			name: "zero-penalty human winner, close win",
			players: []GameResultPlayer{
				{UserID: human, DisplayName: "Alice", PenaltyPoints: 0, Rank: 1, IsWinner: true},
				{DisplayName: "Bot", PenaltyPoints: 2, Rank: 2, IsBot: true},
				{DisplayName: "Bot2", PenaltyPoints: 8, Rank: 3, IsBot: true},
			},
			// runner-up is 2; winner 0 <= 2+3 -> close win (winner penalty 0 is real).
			want: closeGameStoryFlags{CloseWin: true},
		},
		{
			name: "zero-penalty human winner, blowout win",
			players: []GameResultPlayer{
				{UserID: human, DisplayName: "Alice", PenaltyPoints: 0, Rank: 1, IsWinner: true},
				{DisplayName: "Bot", PenaltyPoints: 18, Rank: 2, IsBot: true},
				{DisplayName: "Bot2", PenaltyPoints: 25, Rank: 3, IsBot: true},
			},
			// runner-up is 18; winner 0 <= 18-15 -> blowout win.
			want: closeGameStoryFlags{BlowoutWin: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pen := closeGamePenalties(tc.players)
			var target GameResultPlayer
			for _, p := range tc.players {
				if p.UserID == human {
					target = p
				}
			}
			got := pen.flagsFor(target)
			if got != tc.want {
				t.Fatalf("flags = %+v, want %+v (penalties=%+v)", got, tc.want, pen)
			}
		})
	}
}
