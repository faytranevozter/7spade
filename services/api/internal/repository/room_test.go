package repository

import (
	"errors"
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
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
	mock.ExpectQuery(regexp.QuoteMeta("WITH candidate AS")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID, false, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at", "player_count"}).
			AddRow(roomID, "OLD123", "Room #1", "public", 60, "medium", false, nil, nil, "waiting", createdBy, createdAt, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO room_players")).
		WithArgs(sqlmock.AnyArg(), roomID, userID, "Alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	room, created, err := QuickPlayRoom(db, QuickPlayOptions{UserID: userID, DisplayName: "Alice"})
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
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
	mock.ExpectQuery(regexp.QuoteMeta("WITH candidate AS")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID, false, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at", "player_count"}))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public", QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, false, nil, nil, "waiting", userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "NEW123", "Room #2", "public", 60, "medium", false, nil, nil, "waiting", userID, createdAt))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO room_players")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), userID, "Alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	room, created, err := QuickPlayRoom(db, QuickPlayOptions{UserID: userID, DisplayName: "Alice"})
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

func TestQuickPlayRoomRankedCreatesRatingBoundedRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	rating := 1234
	createdAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
	mock.ExpectQuery(regexp.QuoteMeta("WITH candidate AS")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID, true, rating).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at", "player_count"}))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public", QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, false, 1034, 1434, "waiting", userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "RANKED", "Room #3", "public", 60, "medium", false, 1034, 1434, "waiting", userID, createdAt))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO room_players")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), userID, "Alice", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	room, created, err := QuickPlayRoom(db, QuickPlayOptions{UserID: userID, DisplayName: "Alice", Rating: &rating, Ranked: true})
	if err != nil {
		t.Fatalf("QuickPlayRoom: %v", err)
	}
	if !created || room.MinElo == nil || room.MaxElo == nil || *room.MinElo != 1034 || *room.MaxElo != 1434 {
		t.Fatalf("room = %+v created=%v", room, created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQuickPlayRoomRankedRequiresRating(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	if _, _, err := QuickPlayRoom(db, QuickPlayOptions{UserID: uuid.New(), DisplayName: "Guest", Ranked: true}); err != ErrRoomRatingRestricted {
		t.Fatalf("err = %v, want ErrRoomRatingRestricted", err)
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
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
	mock.ExpectQuery(regexp.QuoteMeta("WITH candidate AS")).
		WithArgs(QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, userID, false, nil).
		WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	if _, _, err := QuickPlayRoom(db, QuickPlayOptions{UserID: userID, DisplayName: "Alice"}); err == nil {
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
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "", "private", 60, "hard", true, nil, nil, "classic", 4, 1, "rank_value", nil, "ffa", "waiting", createdBy, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "game_mode", "max_players", "deck_count", "scoring_mode", "team_mode", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "PRAC01", "Room #5", "private", 60, "hard", true, nil, nil, "classic", 4, 1, "rank_value", "ffa", "waiting", createdBy, now))

	room, err := CreateRoom(db, "", "private", 60, "hard", true, nil, nil, createdBy)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if !room.PracticeMode {
		t.Fatalf("expected practice_mode true, got %+v", room)
	}
	if room.Visibility != "private" {
		t.Fatalf("expected private visibility, got %q", room.Visibility)
	}
	if room.Name != "Room #5" {
		t.Fatalf("expected default name, got %q", room.Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRoomPersistsEloRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	createdBy := uuid.New()
	minElo, maxElo := 1000, 1400
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO rooms")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Friends only", "public", 60, "medium", false, minElo, maxElo, "classic", 4, 1, "rank_value", nil, "ffa", "waiting", createdBy, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "game_mode", "max_players", "deck_count", "scoring_mode", "team_mode", "status", "created_by", "created_at"}).
			AddRow(uuid.New(), "ELO123", "Friends only", "public", 60, "medium", false, minElo, maxElo, "classic", 4, 1, "rank_value", "ffa", "waiting", createdBy, now))

	room, err := CreateRoom(db, "Friends only", "public", 60, "medium", false, &minElo, &maxElo, createdBy)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if room.MinElo == nil || room.MaxElo == nil || *room.MinElo != minElo || *room.MaxElo != maxElo {
		t.Fatalf("elo range = %v-%v", room.MinElo, room.MaxElo)
	}
	if room.Name != "Friends only" {
		t.Fatalf("expected custom name, got %q", room.Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetUserRatingDefaultsWhenNoStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT rating FROM user_stats")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"rating"}))

	rating, err := GetUserRating(db, userID)
	if err != nil {
		t.Fatalf("GetUserRating: %v", err)
	}
	if rating != DefaultRating {
		t.Fatalf("rating = %d, want %d", rating, DefaultRating)
	}
}

// UpdateRoomStatus allows finished -> in_progress (a unanimous rematch reuses
// the room for a new game) and finished -> waiting (a partial rematch drops the
// voters back to the waiting room). Both are needed so a mid-game refresh of a
// rematched room reconnects instead of being bounced to history.
func TestUpdateRoomStatusAllowsRematchTransitions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		next    string
	}{
		{"finished to in_progress", "finished", "in_progress"},
		{"finished to waiting", "finished", "waiting"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			roomID := uuid.New()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM rooms WHERE id = $1 FOR UPDATE")).
				WithArgs(roomID).
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(tc.current))
			mock.ExpectExec(regexp.QuoteMeta("UPDATE rooms SET status = $1 WHERE id = $2")).
				WithArgs(tc.next, roomID).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			if err := UpdateRoomStatus(db, roomID, tc.next); err != nil {
				t.Fatalf("UpdateRoomStatus(%s -> %s): %v", tc.current, tc.next, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

// A finished room cannot regress straight back to finished from a state machine
// that no longer matches; the only forbidden case worth guarding is an unknown
// target, which is rejected before any DB work.
func TestUpdateRoomStatusRejectsInvalidTarget(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	if err := UpdateRoomStatus(db, uuid.New(), "archived"); err == nil {
		t.Fatal("expected invalid status to be rejected")
	}
}

func TestGetActiveRoomForUserReturnsRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	roomID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}).
			AddRow(roomID, "ABC123", "in_progress", false))

	active, err := GetActiveRoomForUser(db, userID)
	if err != nil {
		t.Fatalf("GetActiveRoomForUser: %v", err)
	}
	if active == nil || active.ID != roomID || active.Status != "in_progress" {
		t.Fatalf("active = %+v", active)
	}
}

func TestGetActiveRoomForUserReturnsNilWhenNone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))

	active, err := GetActiveRoomForUser(db, userID)
	if err != nil {
		t.Fatalf("GetActiveRoomForUser: %v", err)
	}
	if active != nil {
		t.Fatalf("expected nil active room, got %+v", active)
	}
}

func TestAddPlayerToRoomBlocksWhenInAnotherRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	targetRoom := uuid.New()
	otherRoom := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}).
			AddRow(otherRoom, "OTHER1", "in_progress", false))
	mock.ExpectRollback()

	_, err = AddPlayerToRoom(db, targetRoom, JoinRoomPlayer{UserID: userID, DisplayName: "Alice"})
	var inAnother PlayerInAnotherRoomError
	if !errors.As(err, &inAnother) {
		t.Fatalf("expected PlayerInAnotherRoomError, got %v", err)
	}
	if inAnother.Room.ID != otherRoom {
		t.Fatalf("expected active room %s, got %+v", otherRoom, inAnother.Room)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAddPlayerToRoomBlocksKickedPlayer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	roomID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	// Not in any other active room.
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
	// But they were kicked from the target room.
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_kicked_players WHERE room_id = $1 AND user_id = $2")).
		WithArgs(roomID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = AddPlayerToRoom(db, roomID, JoinRoomPlayer{UserID: userID, DisplayName: "Alice"})
	if !errors.Is(err, ErrPlayerKicked) {
		t.Fatalf("expected ErrPlayerKicked, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAddPlayerToRoomAllowsReentryToSameRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	roomID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	// Already a member of the target room: the active-room guard sees the same
	// room and lets the join proceed to the existing per-room checks.
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}).
			AddRow(roomID, "SAME01", "waiting", false))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_kicked_players WHERE room_id = $1 AND user_id = $2")).
		WithArgs(roomID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM rooms WHERE id = $1 FOR UPDATE")).
		WithArgs(roomID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "min_elo", "max_elo", "max_players", "count"}).
			AddRow("waiting", nil, nil, 4, 1))
	// Existing-membership check returns 1 -> ErrPlayerAlreadyInRoom (same-room
	// re-entry is reported distinctly from the cross-room block).
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM room_players WHERE room_id = $1 AND user_id = $2")).
		WithArgs(roomID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = AddPlayerToRoom(db, roomID, JoinRoomPlayer{UserID: userID, DisplayName: "Alice"})
	if !errors.Is(err, ErrPlayerAlreadyInRoom) {
		t.Fatalf("expected ErrPlayerAlreadyInRoom, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestQuickPlayRoomBlocksWhenInAnotherRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	otherRoom := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("pg_advisory_xact_lock")).WithArgs(userID.String()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM room_players rp")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}).
			AddRow(otherRoom, "OTHER1", "waiting", false))
	mock.ExpectRollback()

	_, _, err = QuickPlayRoom(db, QuickPlayOptions{UserID: userID, DisplayName: "Alice"})
	var inAnother PlayerInAnotherRoomError
	if !errors.As(err, &inAnother) {
		t.Fatalf("expected PlayerInAnotherRoomError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
