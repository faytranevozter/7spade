package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        sql.NullString // nullable: Telegram users have no email
	PasswordHash sql.NullString
	DisplayName  string
	CreatedAt    time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateUser inserts a new user into the database (email/password registration)
func CreateUser(db *sql.DB, email, passwordHash, displayName string) (*User, error) {
	user := &User{
		ID:           uuid.New(),
		Email:        sql.NullString{String: email, Valid: true},
		PasswordHash: sql.NullString{String: passwordHash, Valid: true},
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, display_name, created_at
	`

	err := db.QueryRow(query, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, created_at
		FROM users
		WHERE email = $1
	`

	err := db.QueryRow(query, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(db *sql.DB, id uuid.UUID) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, created_at
		FROM users
		WHERE id = $1
	`

	err := db.QueryRow(query, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// OAuthProfile holds the normalised user profile returned by an OAuth provider.
type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string // may be empty (e.g. Telegram)
	DisplayName    string
	AvatarURL      string
}

// UpsertOAuthUser creates or updates a user for the given OAuth profile.
// Lookup order: user_providers.provider_id → users.email (if email provided).
func UpsertOAuthUser(db *sql.DB, profile OAuthProfile) (*User, error) {
	if profile.Provider == "" || profile.ProviderUserID == "" {
		return nil, fmt.Errorf("provider and provider_user_id are required")
	}
	if profile.DisplayName == "" {
		if profile.Email != "" {
			profile.DisplayName = profile.Email
		} else {
			profile.DisplayName = profile.Provider + " user"
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Look up existing user by provider identity
	var userID uuid.UUID
	err = tx.QueryRow(`
		SELECT user_id FROM user_providers WHERE provider = $1 AND provider_id = $2
	`, profile.Provider, profile.ProviderUserID).Scan(&userID)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("lookup provider: %w", err)
	}

	if err == sql.ErrNoRows {
		// 2. Fall back to email lookup (existing email/password account)
		if profile.Email != "" {
			err2 := tx.QueryRow(`SELECT id FROM users WHERE email = $1`, profile.Email).Scan(&userID)
			if err2 != nil && err2 != sql.ErrNoRows {
				return nil, fmt.Errorf("lookup email: %w", err2)
			}
		}
	}

	var user User
	if userID == uuid.Nil {
		// 3. Create a brand-new user
		var emailVal sql.NullString
		if profile.Email != "" {
			emailVal = sql.NullString{String: profile.Email, Valid: true}
		}
		err = tx.QueryRow(`
			INSERT INTO users (id, email, display_name, created_at)
			VALUES ($1, $2, $3, NOW())
			RETURNING id, email, password_hash, display_name, created_at
		`, uuid.New(), emailVal, profile.DisplayName).
			Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}
	} else {
		// 4. Update existing user's display name
		err = tx.QueryRow(`
			UPDATE users SET display_name = $1 WHERE id = $2
			RETURNING id, email, password_hash, display_name, created_at
		`, profile.DisplayName, userID).
			Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	// 5. Upsert into user_providers
	var emailVal sql.NullString
	if profile.Email != "" {
		emailVal = sql.NullString{String: profile.Email, Valid: true}
	}
	var avatarVal sql.NullString
	if profile.AvatarURL != "" {
		avatarVal = sql.NullString{String: profile.AvatarURL, Valid: true}
	}
	_, err = tx.Exec(`
		INSERT INTO user_providers (user_id, provider, provider_id, email, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_id) DO UPDATE
			SET email = EXCLUDED.email,
			    avatar_url = EXCLUDED.avatar_url
	`, user.ID, profile.Provider, profile.ProviderUserID, emailVal, avatarVal)
	if err != nil {
		return nil, fmt.Errorf("upsert provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &user, nil
}

// StoreRefreshToken stores a refresh token in the database
func StoreRefreshToken(db *sql.DB, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := db.Exec(query, uuid.New(), userID, tokenHash, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

// ValidateRefreshToken validates a refresh token and returns the user ID
func ValidateRefreshToken(db *sql.DB, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	var expiresAt time.Time

	query := `
		SELECT user_id, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	err := db.QueryRow(query, tokenHash).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("invalid refresh token")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to validate refresh token: %w", err)
	}

	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("refresh token expired")
	}

	return userID, nil
}

// RevokeRefreshToken revokes a refresh token
func RevokeRefreshToken(db *sql.DB, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`
	_, err := db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// Room represents a game room
type Room struct {
	ID               uuid.UUID
	InviteCode       string
	Visibility       string
	TurnTimerSeconds int
	Status           string
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
}

// RoomPlayer represents a player in a room
type RoomPlayer struct {
	ID          uuid.UUID
	RoomID      uuid.UUID
	UserID      uuid.UUID
	DisplayName string
	JoinedAt    time.Time
}

// RoomWithPlayerCount is a room with its current player count
type RoomWithPlayerCount struct {
	Room
	PlayerCount int
}

const inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var (
	errRoomNotAcceptingPlayers = errors.New("room is not accepting players")
	errRoomFull                = errors.New("room is full")
	errPlayerAlreadyInRoom     = errors.New("already in room")
)

// GenerateInviteCode creates a random 6-character alphanumeric invite code
func GenerateInviteCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteCodeChars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate invite code: %w", err)
		}
		code[i] = inviteCodeChars[n.Int64()]
	}
	return string(code), nil
}

// CreateRoom inserts a new room
func CreateRoom(db *sql.DB, visibility string, turnTimerSeconds int, createdBy uuid.UUID) (*Room, error) {
	inviteCode, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}

	room := &Room{
		ID:               uuid.New(),
		InviteCode:       inviteCode,
		Visibility:       visibility,
		TurnTimerSeconds: turnTimerSeconds,
		Status:           "waiting",
		CreatedBy:        createdBy,
		CreatedAt:        time.Now(),
	}

	query := `
		INSERT INTO rooms (id, invite_code, visibility, turn_timer_seconds, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, invite_code, visibility, turn_timer_seconds, status, created_by, created_at
	`

	err = db.QueryRow(query, room.ID, room.InviteCode, room.Visibility, room.TurnTimerSeconds, room.Status, room.CreatedBy, room.CreatedAt).
		Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.Status, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	return room, nil
}

// GetPublicWaitingRooms returns public rooms with waiting status
func GetPublicWaitingRooms(db *sql.DB) ([]RoomWithPlayerCount, error) {
	query := `
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		WHERE r.visibility = 'public' AND r.status = 'waiting'
		GROUP BY r.id
		ORDER BY r.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query public rooms: %w", err)
	}
	defer rows.Close()

	var rooms []RoomWithPlayerCount
	for rows.Next() {
		var rwp RoomWithPlayerCount
		err := rows.Scan(&rwp.ID, &rwp.InviteCode, &rwp.Visibility, &rwp.TurnTimerSeconds, &rwp.Status, &rwp.CreatedBy, &rwp.CreatedAt, &rwp.PlayerCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, rwp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate public rooms: %w", err)
	}

	if rooms == nil {
		rooms = []RoomWithPlayerCount{}
	}

	return rooms, nil
}

// GetRoomByInviteCode retrieves a room by its invite code
func GetRoomByInviteCode(db *sql.DB, code string) (*RoomWithPlayerCount, error) {
	query := `
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		WHERE r.invite_code = $1
		GROUP BY r.id
	`

	var rwp RoomWithPlayerCount
	err := db.QueryRow(query, code).
		Scan(&rwp.ID, &rwp.InviteCode, &rwp.Visibility, &rwp.TurnTimerSeconds, &rwp.Status, &rwp.CreatedBy, &rwp.CreatedAt, &rwp.PlayerCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room by invite code: %w", err)
	}

	return &rwp, nil
}

// GetRoomByID retrieves a room by its ID
func GetRoomByID(db *sql.DB, id uuid.UUID) (*RoomWithPlayerCount, error) {
	query := `
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		WHERE r.id = $1
		GROUP BY r.id
	`

	var rwp RoomWithPlayerCount
	err := db.QueryRow(query, id).
		Scan(&rwp.ID, &rwp.InviteCode, &rwp.Visibility, &rwp.TurnTimerSeconds, &rwp.Status, &rwp.CreatedBy, &rwp.CreatedAt, &rwp.PlayerCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room by ID: %w", err)
	}

	return &rwp, nil
}

// AddPlayerToRoom adds a player to a room. Returns the updated player count.
func AddPlayerToRoom(db *sql.DB, roomID, userID uuid.UUID, displayName string) (playerCount int, retErr error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, tx.Rollback())
		}
	}()

	// Check room status
	var status string
	var currentPlayerCount int
	err = tx.QueryRow(`SELECT status, (SELECT COUNT(*) FROM room_players WHERE room_id = $1) FROM rooms WHERE id = $1 FOR UPDATE`, roomID).
		Scan(&status, &currentPlayerCount)
	if err != nil {
		return 0, fmt.Errorf("failed to check room: %w", err)
	}

	if status != "waiting" {
		return 0, errRoomNotAcceptingPlayers
	}

	if currentPlayerCount >= 4 {
		return 0, errRoomFull
	}

	// Check if player already in room
	var existing int
	err = tx.QueryRow(`SELECT COUNT(*) FROM room_players WHERE room_id = $1 AND user_id = $2`, roomID, userID).Scan(&existing)
	if err != nil {
		return 0, fmt.Errorf("failed to check existing player: %w", err)
	}
	if existing > 0 {
		return 0, errPlayerAlreadyInRoom
	}

	// Insert player
	_, err = tx.Exec(`INSERT INTO room_players (id, room_id, user_id, display_name, joined_at) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), roomID, userID, displayName, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to add player: %w", err)
	}

	newCount := currentPlayerCount + 1

	// Transition to in_progress when 4th player joins
	if newCount == 4 {
		_, err = tx.Exec(`UPDATE rooms SET status = 'in_progress' WHERE id = $1`, roomID)
		if err != nil {
			return 0, fmt.Errorf("failed to update room status: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
	}

	return newCount, nil
}
