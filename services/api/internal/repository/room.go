package repository

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Room struct {
	ID               uuid.UUID
	InviteCode       string
	Visibility       string
	TurnTimerSeconds int
	BotDifficulty    string
	PracticeMode     bool
	Status           string
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
}

type RoomWithPlayerCount struct {
	Room
	PlayerCount int
}

// LiveGamePlayer is one seated player in a live game, for spectator discovery.
type LiveGamePlayer struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

// LiveGame is an in-progress public room a spectator can watch.
type LiveGame struct {
	RoomID      string           `json:"room_id"`
	InviteCode  string           `json:"invite_code"`
	StartedAt   string           `json:"started_at"`
	PlayerCount int              `json:"player_count"`
	Players     []LiveGamePlayer `json:"players"`
}

// GetLiveGames returns in-progress public rooms with their seated players, for
// the spectator "watch live" discovery surface. Private rooms are excluded (not
// publicly discoverable). Rooms with no membership rows are skipped.
func GetLiveGames(db *sql.DB) ([]LiveGame, error) {
	rows, err := db.Query(`
		SELECT r.id, r.invite_code, r.created_at, rp.user_id, rp.display_name
		FROM rooms r
		JOIN room_players rp ON rp.room_id = r.id
		WHERE r.visibility = 'public' AND r.status = 'in_progress' AND r.practice_mode = false
		ORDER BY r.created_at DESC, rp.joined_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query live games: %w", err)
	}
	defer rows.Close()

	// Preserve room order while grouping players per room.
	order := []string{}
	byRoom := map[string]*LiveGame{}
	for rows.Next() {
		var (
			roomID      uuid.UUID
			inviteCode  string
			createdAt   time.Time
			userID      uuid.UUID
			displayName string
		)
		if err := rows.Scan(&roomID, &inviteCode, &createdAt, &userID, &displayName); err != nil {
			return nil, fmt.Errorf("scan live game: %w", err)
		}
		key := roomID.String()
		game, ok := byRoom[key]
		if !ok {
			game = &LiveGame{
				RoomID:     key,
				InviteCode: inviteCode,
				StartedAt:  createdAt.UTC().Format(time.RFC3339),
				Players:    []LiveGamePlayer{},
			}
			byRoom[key] = game
			order = append(order, key)
		}
		game.Players = append(game.Players, LiveGamePlayer{UserID: userID.String(), DisplayName: displayName})
		game.PlayerCount = len(game.Players)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate live games: %w", err)
	}

	games := make([]LiveGame, 0, len(order))
	for _, key := range order {
		games = append(games, *byRoom[key])
	}
	return games, nil
}

const inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var (
	ErrRoomNotAcceptingPlayers = errors.New("room is not accepting players")
	ErrRoomFull                = errors.New("room is full")
	ErrPlayerAlreadyInRoom     = errors.New("already in room")
)

func GenerateInviteCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteCodeChars))))
		if err != nil {
			return "", fmt.Errorf("generate invite code: %w", err)
		}
		code[i] = inviteCodeChars[n.Int64()]
	}
	return string(code), nil
}

func CreateRoom(db *sql.DB, visibility string, turnTimerSeconds int, botDifficulty string, practiceMode bool, createdBy uuid.UUID) (*Room, error) {
	inviteCode, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}
	room := &Room{ID: uuid.New(), InviteCode: inviteCode, Visibility: visibility, TurnTimerSeconds: turnTimerSeconds, BotDifficulty: botDifficulty, PracticeMode: practiceMode, Status: "waiting", CreatedBy: createdBy, CreatedAt: time.Now()}
	err = db.QueryRow(`
		INSERT INTO rooms (id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, status, created_by, created_at
	`, room.ID, room.InviteCode, room.Visibility, room.TurnTimerSeconds, room.BotDifficulty, room.PracticeMode, room.Status, room.CreatedBy, room.CreatedAt).
		Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, &room.Status, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

func GetPublicWaitingRooms(db *sql.DB) ([]RoomWithPlayerCount, error) {
	rows, err := db.Query(`
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.bot_difficulty, r.practice_mode, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		WHERE r.visibility = 'public' AND r.status = 'waiting' AND r.practice_mode = false
		GROUP BY r.id
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query public rooms: %w", err)
	}
	defer rows.Close()

	rooms := []RoomWithPlayerCount{}
	for rows.Next() {
		var room RoomWithPlayerCount
		if err := rows.Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount); err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public rooms: %w", err)
	}
	return rooms, nil
}

func GetRoomByInviteCode(db *sql.DB, code string) (*RoomWithPlayerCount, error) {
	return getRoom(db, `WHERE r.invite_code = $1`, code)
}

func GetRoomByID(db *sql.DB, id uuid.UUID) (*RoomWithPlayerCount, error) {
	return getRoom(db, `WHERE r.id = $1`, id)
}

func getRoom(db *sql.DB, where string, arg any) (*RoomWithPlayerCount, error) {
	var room RoomWithPlayerCount
	err := db.QueryRow(`
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.bot_difficulty, r.practice_mode, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		`+where+`
		GROUP BY r.id
	`, arg).Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return &room, nil
}

func AddPlayerToRoom(db *sql.DB, roomID, userID uuid.UUID, displayName string) (playerCount int, retErr error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, tx.Rollback())
		}
	}()

	var status string
	var currentPlayerCount int
	err = tx.QueryRow(`SELECT status, (SELECT COUNT(*) FROM room_players WHERE room_id = $1) FROM rooms WHERE id = $1 FOR UPDATE`, roomID).
		Scan(&status, &currentPlayerCount)
	if err != nil {
		return 0, fmt.Errorf("check room: %w", err)
	}
	if status != "waiting" {
		return 0, ErrRoomNotAcceptingPlayers
	}
	if currentPlayerCount >= 4 {
		return 0, ErrRoomFull
	}

	var existing int
	err = tx.QueryRow(`SELECT COUNT(*) FROM room_players WHERE room_id = $1 AND user_id = $2`, roomID, userID).Scan(&existing)
	if err != nil {
		return 0, fmt.Errorf("check existing player: %w", err)
	}
	if existing > 0 {
		return 0, ErrPlayerAlreadyInRoom
	}

	_, err = tx.Exec(`INSERT INTO room_players (id, room_id, user_id, display_name, joined_at) VALUES ($1, $2, $3, $4, $5)`, uuid.New(), roomID, userID, displayName, time.Now())
	if err != nil {
		return 0, fmt.Errorf("add player: %w", err)
	}

	newCount := currentPlayerCount + 1
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newCount, nil
}

// RemovePlayerFromRoom drops a player's membership row. When the room is left
// empty as a result, the room itself is deleted (ON DELETE CASCADE clears any
// remaining child rows) so it stops showing in the public lobby list. The
// delete is idempotent: removing a player who is already gone is not an error.
// Returns the number of players still in the room afterwards.
func RemovePlayerFromRoom(db *sql.DB, roomID, userID uuid.UUID) (remaining int, retErr error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, tx.Rollback())
		}
	}()

	// Lock the room row so concurrent leaves/joins serialize on it.
	var status string
	err = tx.QueryRow(`SELECT status FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&status)
	if err == sql.ErrNoRows {
		// Room already gone; nothing to remove.
		return 0, tx.Commit()
	}
	if err != nil {
		return 0, fmt.Errorf("lock room: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM room_players WHERE room_id = $1 AND user_id = $2`, roomID, userID); err != nil {
		return 0, fmt.Errorf("remove player: %w", err)
	}

	if err := tx.QueryRow(`SELECT COUNT(*) FROM room_players WHERE room_id = $1`, roomID).Scan(&remaining); err != nil {
		return 0, fmt.Errorf("count remaining players: %w", err)
	}

	// Delete the room once the last player leaves while it is still waiting, so
	// abandoned lobbies do not linger in the public list. Rooms that already
	// started (in_progress/finished) are left for history/reconnection.
	if remaining == 0 && status == "waiting" {
		if _, err := tx.Exec(`DELETE FROM rooms WHERE id = $1`, roomID); err != nil {
			return 0, fmt.Errorf("delete empty room: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return remaining, nil
}

// DeleteStaleWaitingRooms removes 'waiting' rooms that have no live presence on
// the WS server and were created before olderThan. activeRoomIDs is the set of
// room IDs the WS service is currently tracking in memory; any waiting room not
// in that set is a candidate. The olderThan cutoff protects the brief window
// between a room being created (DB row exists) and its host's WebSocket
// connecting (presence registered), and guards against transient WS restarts.
// ON DELETE CASCADE clears any orphaned room_players rows. Returns the number
// of rooms deleted.
func DeleteStaleWaitingRooms(db *sql.DB, activeRoomIDs []uuid.UUID, olderThan time.Time) (int64, error) {
	active := make([]string, 0, len(activeRoomIDs))
	for _, id := range activeRoomIDs {
		active = append(active, id.String())
	}
	// $1 is a text[] of active room IDs; a room survives if its id is present in
	// that array. Comparing as text avoids uuid[] driver encoding concerns.
	result, err := db.Exec(`
		DELETE FROM rooms
		WHERE status = 'waiting'
		  AND created_at < $2
		  AND NOT (id::text = ANY($1))
	`, pq.StringArray(active), olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete stale waiting rooms: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("stale rooms affected: %w", err)
	}
	return deleted, nil
}

// UpdateRoomStatus moves a room between lifecycle states. Allowed transitions:
//
//	waiting -> in_progress (once the host starts the game)
//	in_progress -> finished (once a round ends)
//
// Other transitions are rejected so the public lobby list cannot regress.
func UpdateRoomStatus(db *sql.DB, roomID uuid.UUID, newStatus string) error {
	if newStatus != "in_progress" && newStatus != "finished" {
		return fmt.Errorf("invalid room status: %s", newStatus)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	if err := tx.QueryRow(`SELECT status FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&current); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("load room status: %w", err)
	}
	if current == newStatus {
		return tx.Commit()
	}
	allowed := false
	switch current {
	case "waiting":
		allowed = newStatus == "in_progress" || newStatus == "finished"
	case "in_progress":
		allowed = newStatus == "finished"
	}
	if !allowed {
		return fmt.Errorf("cannot transition room status from %s to %s", current, newStatus)
	}
	if _, err := tx.Exec(`UPDATE rooms SET status = $1 WHERE id = $2`, newStatus, roomID); err != nil {
		return fmt.Errorf("update room status: %w", err)
	}
	return tx.Commit()
}
