package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// GetLiveGames groups player rows by room, preserving room order and per-room
// player order, and only returns in-progress public rooms (the WHERE clause).
func TestGetLiveGames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	room1 := uuid.New()
	room2 := uuid.New()
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("FROM rooms r")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "created_at", "user_id", "display_name"}).
			AddRow(room1, "ABC123", now, u1, "Alice").
			AddRow(room1, "ABC123", now, u2, "Bob").
			AddRow(room2, "XYZ789", now, u3, "Carol"))

	games, err := GetLiveGames(db)
	if err != nil {
		t.Fatalf("GetLiveGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("games = %d, want 2", len(games))
	}
	if games[0].RoomID != room1.String() || games[0].PlayerCount != 2 {
		t.Fatalf("game[0] = %+v", games[0])
	}
	if len(games[0].Players) != 2 || games[0].Players[0].DisplayName != "Alice" || games[0].Players[1].DisplayName != "Bob" {
		t.Fatalf("game[0] players = %+v", games[0].Players)
	}
	if games[1].RoomID != room2.String() || games[1].PlayerCount != 1 {
		t.Fatalf("game[1] = %+v", games[1])
	}
	if games[0].StartedAt == "" {
		t.Fatal("expected RFC3339 started_at")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GetLiveGames returns a non-nil empty slice when there are no live games.
func TestGetLiveGamesEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM rooms r")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "created_at", "user_id", "display_name"}))

	games, err := GetLiveGames(db)
	if err != nil {
		t.Fatalf("GetLiveGames: %v", err)
	}
	if games == nil || len(games) != 0 {
		t.Fatalf("expected empty non-nil slice, got %+v", games)
	}
}
