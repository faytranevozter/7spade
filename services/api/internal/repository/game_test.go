package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestSaveGameUpdatesRegisteredPlayerStats(t *testing.T) {
	db, mock, err := sqlmock.New()
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
			{UserID: userID.String(), DisplayName: "Alice", PenaltyPoints: 3, Rank: 1, IsWinner: true},
			{DisplayName: "Bot 1", PenaltyPoints: 7, Rank: 2, IsWinner: false},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO games (id, room_id, started_at, finished_at) VALUES ($1, $2, $3, $4)")).
		WithArgs(sqlmock.AnyArg(), result.RoomID, result.StartedAt, result.FinishedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO game_players").
		WithArgs(sqlmock.AnyArg(), &userID, "Alice", 3, 1, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO user_stats").
		WithArgs(userID, 1, 3).
		WillReturnRows(sqlmock.NewRows([]string{"games_played", "wins", "current_streak"}).AddRow(1, 1, 1))
	mock.ExpectExec("INSERT INTO user_achievements").
		WithArgs(userID, AchievementFirstWin).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO game_players").
		WithArgs(sqlmock.AnyArg(), nil, "Bot 1", 7, 2, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := SaveGame(db, result); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
