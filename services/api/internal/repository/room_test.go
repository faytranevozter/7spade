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

func TestQuickPlayRoomJoinsOldestCompatibleRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	roomID := uuid.New()
	userID := uuid.New()
	createdBy := uuid.New()
	createdAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id, r.invite_code, r.visibility")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "status", "created_by", "created_at", "player_count"}).
			AddRow(roomID, "OLD123", "public", 60, "medium", false, "waiting", createdBy, createdAt, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO room_players")).
		WithArgs(sqlmock.AnyArg(), roomID, userID, "Alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	room, created, err := QuickPlayRoom(db, userID, "Alice")
	if err != nil {
		t.Fatalf("QuickPlayRoom: %v", err)
	}
	if created {
		t.Fatal("expected to join existing room, got created=true")
	}
	if room.ID != roomID || room.InviteCode != "OLD123" || room.PlayerCount != 3 {
		t.Fatalf("room = %+v", room)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQuickPlayRoomCreatesDefaultRoomWhenNoneMatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	createdAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id, r.invite_code, r.visibility")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "status", "created_by", "created_at", "player_count"}))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public", QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, false, "waiting", userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "NEW123", "public", 60, "medium", false, "waiting", userID, createdAt))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO room_players")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), userID, "Alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	room, created, err := QuickPlayRoom(db, userID, "Alice")
	if err != nil {
		t.Fatalf("QuickPlayRoom: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if room.InviteCode != "NEW123" || room.Visibility != "public" || room.TurnTimerSeconds != 60 || room.BotDifficulty != "medium" || room.PracticeMode || room.PlayerCount != 1 {
		t.Fatalf("room = %+v", room)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQuickPlayRoomPropagatesSelectionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id, r.invite_code, r.visibility")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID).
		WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	if _, _, err := QuickPlayRoom(db, userID, "Alice"); err == nil {
		t.Fatal("expected query error to propagate")
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

// CreateRoom persists practice_mode and returns the stored row, including the
// forced-private practice flag.
func TestCreateRoomPersistsPracticeMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	createdBy := uuid.New()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "private", 60, "hard", true, "waiting", createdBy, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "PRAC01", "private", 60, "hard", true, "waiting", createdBy, now))

	room, err := CreateRoom(db, "private", 60, "hard", true, createdBy)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if !room.PracticeMode {
		t.Fatalf("expected practice_mode true, got %+v", room)
	}
	if room.Visibility != "private" {
		t.Fatalf("expected private visibility, got %q", room.Visibility)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
