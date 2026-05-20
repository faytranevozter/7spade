package repository

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

type Room struct {
	ID               uuid.UUID
	InviteCode       string
	Visibility       string
	TurnTimerSeconds int
	Status           string
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
}

type RoomWithPlayerCount struct {
	Room
	PlayerCount int
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

func CreateRoom(db *sql.DB, visibility string, turnTimerSeconds int, createdBy uuid.UUID) (*Room, error) {
	inviteCode, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}
	room := &Room{ID: uuid.New(), InviteCode: inviteCode, Visibility: visibility, TurnTimerSeconds: turnTimerSeconds, Status: "waiting", CreatedBy: createdBy, CreatedAt: time.Now()}
	err = db.QueryRow(`
		INSERT INTO rooms (id, invite_code, visibility, turn_timer_seconds, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, invite_code, visibility, turn_timer_seconds, status, created_by, created_at
	`, room.ID, room.InviteCode, room.Visibility, room.TurnTimerSeconds, room.Status, room.CreatedBy, room.CreatedAt).
		Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.Status, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

func GetPublicWaitingRooms(db *sql.DB) ([]RoomWithPlayerCount, error) {
	rows, err := db.Query(`
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		WHERE r.visibility = 'public' AND r.status = 'waiting'
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
		if err := rows.Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount); err != nil {
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
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		`+where+`
		GROUP BY r.id
	`, arg).Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount)
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

// UpdateRoomStatus moves a room between lifecycle states. Allowed transitions:
//   waiting -> in_progress (once the host starts the game)
//   in_progress -> finished (once a round ends)
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
