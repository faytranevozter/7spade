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
	ID             uuid.UUID
	Email          string
	PasswordHash   sql.NullString
	DisplayName    string
	Provider       sql.NullString
	ProviderUserID sql.NullString
	AvatarURL      sql.NullString
	CreatedAt      time.Time
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
		Email:        email,
		PasswordHash: sql.NullString{String: passwordHash, Valid: true},
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
	`

	err := db.QueryRow(query, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE email = $1
	`

	err := db.QueryRow(query, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

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
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE id = $1
	`

	err := db.QueryRow(query, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// GetUserByProvider returns the user matching (provider, provider_user_id), or nil if not found.
func GetUserByProvider(db *sql.DB, provider, providerUserID string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE provider = $1 AND provider_user_id = $2
	`

	err := db.QueryRow(query, provider, providerUserID).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by provider: %w", err)
	}

	return user, nil
}

// OAuthProfile holds the normalised user profile information returned by an OAuth provider.
type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	AvatarURL      string
}

// UpsertOAuthUser creates a user for the given OAuth profile, or updates an existing matching
// user (matched first by provider+provider_user_id, then by email). Returns the persisted user.
func UpsertOAuthUser(db *sql.DB, profile OAuthProfile) (*User, error) {
	if profile.Provider == "" || profile.ProviderUserID == "" {
		return nil, fmt.Errorf("provider and provider_user_id are required")
	}
	if profile.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Email
	}

	// Match by provider identity first
	existing, err := GetUserByProvider(db, profile.Provider, profile.ProviderUserID)
	if err != nil {
		return nil, err
	}

	// Fall back to email (e.g. user previously registered with email/password)
	if existing == nil {
		existing, err = GetUserByEmail(db, profile.Email)
		if err != nil {
			return nil, err
		}
	}

	avatar := sql.NullString{String: profile.AvatarURL, Valid: profile.AvatarURL != ""}

	if existing != nil {
		// Update display name, avatar, and link the provider identity if missing
		query := `
			UPDATE users
			SET display_name = $1,
			    avatar_url = $2,
			    provider = $3,
			    provider_user_id = $4
			WHERE id = $5
			RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		`
		updated := &User{}
		err := db.QueryRow(query, profile.DisplayName, avatar, profile.Provider, profile.ProviderUserID, existing.ID).
			Scan(&updated.ID, &updated.Email, &updated.PasswordHash, &updated.DisplayName, &updated.Provider, &updated.ProviderUserID, &updated.AvatarURL, &updated.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to update oauth user: %w", err)
		}
		return updated, nil
	}

	// Create new OAuth user (no password)
	user := &User{
		ID:             uuid.New(),
		Email:          profile.Email,
		DisplayName:    profile.DisplayName,
		Provider:       sql.NullString{String: profile.Provider, Valid: true},
		ProviderUserID: sql.NullString{String: profile.ProviderUserID, Valid: true},
		AvatarURL:      avatar,
		CreatedAt:      time.Now(),
	}
	insert := `
		INSERT INTO users (id, email, display_name, provider, provider_user_id, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
	`
	err = db.QueryRow(insert, user.ID, user.Email, user.DisplayName, user.Provider, user.ProviderUserID, user.AvatarURL, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert oauth user: %w", err)
	}
	return user, nil
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
