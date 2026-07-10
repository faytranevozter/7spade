package repository

import (
	"database/sql"
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

	// Idempotency guard: the game has not been saved yet.
	mock.ExpectQuery("SELECT EXISTS").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

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
	mock.ExpectExec("INSERT INTO game_result_details").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO game_result_details").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM game_moves").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM game_initial_hands").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM game_result_details").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if _, err := SaveGame(db, result); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetGameResultsReturnsRetainedDetailsForViewerWithGuestParticipant(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gameID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	viewerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)

	mock.ExpectQuery("WITH viewer_history").
		WithArgs(gameID, viewerID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "room_name", "started_at", "finished_at", "replay_available"}).
			AddRow(gameID.String(), "room-1", "Friday table", startedAt, finishedAt, true))
	mock.ExpectQuery("SELECT gp.player_index, gp.user_id, grd.subject_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"player_index", "user_id", "subject_id", "is_guest", "display_name", "penalty_points", "rank", "is_winner", "is_bot",
			"team", "face_down_cards", "rating_delta", "rating_after", "xp_delta", "xp_after",
		}).AddRow(
			0, viewerID.String(), viewerID.String(), false, "Alice", 3, 1, true, false,
			nil, []byte(`[{"suit":"hearts","rank":14,"points":1}]`), 12, 1212, 125, int64(125),
		).AddRow(
			1, nil, "guest-sub", true, "Guest", 8, 2, false, false,
			nil, []byte(`[]`), nil, nil, nil, nil,
		))

	results, ok, err := GetGameResults(db, gameID, viewerID, 7)
	if err != nil {
		t.Fatalf("GetGameResults: %v", err)
	}
	if !ok {
		t.Fatal("GetGameResults ok=false, want true")
	}
	if results.GameID != gameID.String() || results.RoomName != "Friday table" || !results.ReplayAvailable {
		t.Fatalf("metadata = %+v", results)
	}
	if len(results.Players) != 2 {
		t.Fatalf("players len = %d, want 2", len(results.Players))
	}
	player := results.Players[0]
	if player.DisplayName != "Alice" || !player.IsWinner || player.PenaltyPoints != 3 {
		t.Fatalf("player = %+v", player)
	}
	if player.UserID == nil || *player.UserID != viewerID.String() || !player.IsMe || player.IsGuest {
		t.Fatalf("viewer identity = %+v", player)
	}
	if len(player.FaceDownCards) != 1 || player.FaceDownCards[0].Suit != "hearts" || player.FaceDownCards[0].Points != 1 {
		t.Fatalf("face-down cards = %+v", player.FaceDownCards)
	}
	if player.RatingDelta == nil || *player.RatingDelta != 12 || player.XPDelta == nil || *player.XPDelta != 125 {
		t.Fatalf("deltas = rating %+v xp %+v", player.RatingDelta, player.XPDelta)
	}
	guest := results.Players[1]
	if !guest.IsGuest || guest.IsMe || guest.UserID == nil || *guest.UserID != "guest-sub" {
		t.Fatalf("guest identity = %+v", guest)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetGameResultsUnavailableWithoutRetainedDetails(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gameID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	viewerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)

	mock.ExpectQuery("WITH viewer_history").
		WithArgs(gameID, viewerID, DefaultGameDetailRetention).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "room_name", "started_at", "finished_at", "replay_available"}).
			AddRow(gameID.String(), "room-1", "Friday table", startedAt, finishedAt, false))
	mock.ExpectQuery("SELECT gp.player_index, gp.user_id, grd.subject_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"player_index", "user_id", "subject_id", "is_guest", "display_name", "penalty_points", "rank", "is_winner", "is_bot",
			"team", "face_down_cards", "rating_delta", "rating_after", "xp_delta", "xp_after",
		}))

	_, ok, err := GetGameResults(db, gameID, viewerID, DefaultGameDetailRetention)
	if err != nil {
		t.Fatalf("GetGameResults: %v", err)
	}
	if ok {
		t.Fatal("GetGameResults ok=true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetGameResultsUnavailableOutsideViewerRetention(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gameID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	viewerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	mock.ExpectQuery("WITH viewer_history").
		WithArgs(gameID, viewerID, 3).
		WillReturnError(sql.ErrNoRows)

	_, ok, err := GetGameResults(db, gameID, viewerID, 3)
	if err != nil {
		t.Fatalf("GetGameResults: %v", err)
	}
	if ok {
		t.Fatal("GetGameResults ok=true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetReplayUnavailableOutsideViewerRetention(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gameID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	viewerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	mock.ExpectQuery("WITH viewer_history").
		WithArgs(gameID, viewerID, 3).
		WillReturnError(sql.ErrNoRows)

	_, ok, err := GetReplay(db, gameID, viewerID, 3)
	if err != nil {
		t.Fatalf("GetReplay: %v", err)
	}
	if ok {
		t.Fatal("GetReplay ok=true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetReplayReturnsRetainedReplayForViewer(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gameID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	viewerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)

	mock.ExpectQuery("WITH viewer_history").
		WithArgs(gameID, viewerID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_name", "started_at", "finished_at"}).
			AddRow(gameID.String(), "Friday table", startedAt, finishedAt))
	mock.ExpectQuery("SELECT display_name, is_bot, is_winner, rank, player_index").
		WithArgs(gameID).
		WillReturnRows(sqlmock.NewRows([]string{"display_name", "is_bot", "is_winner", "rank", "player_index"}).
			AddRow("Alice", false, true, 1, 0).
			AddRow("Guest", false, false, 2, 1))
	mock.ExpectQuery("SELECT player_index, hand FROM game_initial_hands").
		WithArgs(gameID).
		WillReturnRows(sqlmock.NewRows([]string{"player_index", "hand"}).
			AddRow(0, []byte(`[{"suit":"spades","rank":7}]`)))
	mock.ExpectQuery("SELECT move_index, player_index, card_rank, card_suit, move_type, ace_close_direction").
		WithArgs(gameID).
		WillReturnRows(sqlmock.NewRows([]string{"move_index", "player_index", "card_rank", "card_suit", "move_type", "ace_close_direction"}).
			AddRow(0, 0, 7, 0, "play", nil))

	replay, ok, err := GetReplay(db, gameID, viewerID, 7)
	if err != nil {
		t.Fatalf("GetReplay: %v", err)
	}
	if !ok {
		t.Fatal("GetReplay ok=false, want true")
	}
	if replay.GameID != gameID.String() || replay.RoomName != "Friday table" {
		t.Fatalf("metadata = %+v", replay)
	}
	if len(replay.Players) != 2 || replay.Players[0].DisplayName != "Alice" || !replay.Players[0].IsWinner {
		t.Fatalf("players = %+v", replay.Players)
	}
	if len(replay.InitialHands) != 1 || replay.InitialHands[0][0].Suit != "spades" || replay.InitialHands[0][0].Rank != 7 {
		t.Fatalf("initial hands = %+v", replay.InitialHands)
	}
	if len(replay.Moves) != 1 || replay.Moves[0].PlayerIndex != 0 || replay.Moves[0].Suit != "spades" || replay.Moves[0].Rank != 7 {
		t.Fatalf("moves = %+v", replay.Moves)
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
