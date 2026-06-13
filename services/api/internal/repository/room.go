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
	MinElo           *int
	MaxElo           *int
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

const (
	QuickPlayTurnTimerSeconds = 60
	QuickPlayBotDifficulty    = "medium"
	RankedQuickPlayEloRange   = 200
)

var (
	ErrRoomNotAcceptingPlayers = errors.New("room is not accepting players")
	ErrRoomFull                = errors.New("room is full")
	ErrPlayerAlreadyInRoom     = errors.New("already in room")
	ErrRoomRatingRestricted    = errors.New("room rating restricted")
)

type JoinRoomPlayer struct {
	UserID      uuid.UUID
	DisplayName string
	Rating      *int
}

type QuickPlayOptions struct {
	UserID      uuid.UUID
	DisplayName string
	Rating      *int
	Ranked      bool
}

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

func CreateRoom(db *sql.DB, visibility string, turnTimerSeconds int, botDifficulty string, practiceMode bool, minElo, maxElo *int, createdBy uuid.UUID) (*Room, error) {
	inviteCode, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}
	room := &Room{ID: uuid.New(), InviteCode: inviteCode, Visibility: visibility, TurnTimerSeconds: turnTimerSeconds, BotDifficulty: botDifficulty, PracticeMode: practiceMode, MinElo: minElo, MaxElo: maxElo, Status: "waiting", CreatedBy: createdBy, CreatedAt: time.Now()}
	err = db.QueryRow(`
		INSERT INTO rooms (id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, min_elo, max_elo, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, min_elo, max_elo, status, created_by, created_at
	`, room.ID, room.InviteCode, room.Visibility, room.TurnTimerSeconds, room.BotDifficulty, room.PracticeMode, nullableInt(minElo), nullableInt(maxElo), room.Status, room.CreatedBy, room.CreatedAt).
		Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, scanIntPtr(&room.MinElo), scanIntPtr(&room.MaxElo), &room.Status, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

// QuickPlayRoom places a player into the oldest compatible public waiting room,
// or creates a default public room when none is available. The selection and join
// happen in one transaction so concurrent quick-play calls cannot overfill a
// room. It returns created=true when it had to create a fallback room.
func QuickPlayRoom(db *sql.DB, opts QuickPlayOptions) (room RoomWithPlayerCount, created bool, retErr error) {
	if opts.Ranked && opts.Rating == nil {
		return room, false, ErrRoomRatingRestricted
	}
	tx, err := db.Begin()
	if err != nil {
		return room, false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, tx.Rollback())
		}
	}()

	var (
		ratingArg any
		minElo    *int
		maxElo    *int
	)
	if opts.Ranked {
		ratingArg = *opts.Rating
		lo := *opts.Rating - RankedQuickPlayEloRange
		if lo < 0 {
			lo = 0
		}
		hi := *opts.Rating + RankedQuickPlayEloRange
		minElo = &lo
		maxElo = &hi
	}

	err = tx.QueryRow(`
		WITH candidate AS (
			SELECT r.id
			FROM rooms r
			WHERE r.visibility = 'public'
			  AND r.status = 'waiting'
			  AND r.practice_mode = false
			  AND r.turn_timer_seconds = $1
			  AND r.bot_difficulty = $2
			  AND (SELECT COUNT(*) FROM room_players rp WHERE rp.room_id = r.id) < 4
			  AND (
			      ($4::boolean = false AND r.min_elo IS NULL AND r.max_elo IS NULL)
			      OR ($4::boolean = true AND r.min_elo IS NOT NULL AND r.max_elo IS NOT NULL AND r.min_elo <= $5::integer AND r.max_elo >= $5::integer)
			  )
			  AND NOT EXISTS (
			      SELECT 1 FROM room_players existing
			      WHERE existing.room_id = r.id AND existing.user_id = $3
			  )
			ORDER BY r.created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.bot_difficulty, r.practice_mode, r.min_elo, r.max_elo, r.status, r.created_by, r.created_at,
		       (SELECT COUNT(*) FROM room_players rp WHERE rp.room_id = r.id) AS player_count
		FROM rooms r
		JOIN candidate ON candidate.id = r.id
	`, QuickPlayTurnTimerSeconds, QuickPlayBotDifficulty, opts.UserID, opts.Ranked, ratingArg).Scan(
		&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, scanIntPtr(&room.MinElo), scanIntPtr(&room.MaxElo), &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount,
	)
	if err != nil && err != sql.ErrNoRows {
		return room, false, fmt.Errorf("find quick-play room: %w", err)
	}

	if err == sql.ErrNoRows {
		inviteCode, err := GenerateInviteCode()
		if err != nil {
			return room, false, err
		}
		room = RoomWithPlayerCount{Room: Room{ID: uuid.New(), InviteCode: inviteCode, Visibility: "public", TurnTimerSeconds: QuickPlayTurnTimerSeconds, BotDifficulty: QuickPlayBotDifficulty, PracticeMode: false, MinElo: minElo, MaxElo: maxElo, Status: "waiting", CreatedBy: opts.UserID, CreatedAt: time.Now()}}
		if err := tx.QueryRow(`
			INSERT INTO rooms (id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, min_elo, max_elo, status, created_by, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, invite_code, visibility, turn_timer_seconds, bot_difficulty, practice_mode, min_elo, max_elo, status, created_by, created_at
		`, room.ID, room.InviteCode, room.Visibility, room.TurnTimerSeconds, room.BotDifficulty, room.PracticeMode, nullableInt(room.MinElo), nullableInt(room.MaxElo), room.Status, room.CreatedBy, room.CreatedAt).
			Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, scanIntPtr(&room.MinElo), scanIntPtr(&room.MaxElo), &room.Status, &room.CreatedBy, &room.CreatedAt); err != nil {
			return room, false, fmt.Errorf("create quick-play room: %w", err)
		}
		created = true
	}

	if _, err := tx.Exec(`INSERT INTO room_players (id, room_id, user_id, display_name, joined_at) VALUES ($1, $2, $3, $4, $5)`, uuid.New(), room.ID, opts.UserID, opts.DisplayName, time.Now()); err != nil {
		return room, created, fmt.Errorf("join quick-play room: %w", err)
	}
	room.PlayerCount++

	if err := tx.Commit(); err != nil {
		return room, created, fmt.Errorf("commit: %w", err)
	}
	return room, created, nil
}

func GetPublicWaitingRooms(db *sql.DB) ([]RoomWithPlayerCount, error) {
	rows, err := db.Query(`
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.bot_difficulty, r.practice_mode, r.min_elo, r.max_elo, r.status, r.created_by, r.created_at,
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
		if err := rows.Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, scanIntPtr(&room.MinElo), scanIntPtr(&room.MaxElo), &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount); err != nil {
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
		SELECT r.id, r.invite_code, r.visibility, r.turn_timer_seconds, r.bot_difficulty, r.practice_mode, r.min_elo, r.max_elo, r.status, r.created_by, r.created_at,
		       COUNT(rp.id) AS player_count
		FROM rooms r
		LEFT JOIN room_players rp ON rp.room_id = r.id
		`+where+`
		GROUP BY r.id
	`, arg).Scan(&room.ID, &room.InviteCode, &room.Visibility, &room.TurnTimerSeconds, &room.BotDifficulty, &room.PracticeMode, scanIntPtr(&room.MinElo), scanIntPtr(&room.MaxElo), &room.Status, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return &room, nil
}

func AddPlayerToRoom(db *sql.DB, roomID uuid.UUID, player JoinRoomPlayer) (playerCount int, retErr error) {
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
	var minElo, maxElo sql.NullInt64
	err = tx.QueryRow(`SELECT status, min_elo, max_elo, (SELECT COUNT(*) FROM room_players WHERE room_id = $1) FROM rooms WHERE id = $1 FOR UPDATE`, roomID).
		Scan(&status, &minElo, &maxElo, &currentPlayerCount)
	if err != nil {
		return 0, fmt.Errorf("check room: %w", err)
	}
	if status != "waiting" {
		return 0, ErrRoomNotAcceptingPlayers
	}
	if currentPlayerCount >= 4 {
		return 0, ErrRoomFull
	}
	if minElo.Valid || maxElo.Valid {
		if player.Rating == nil || !minElo.Valid || !maxElo.Valid || *player.Rating < int(minElo.Int64) || *player.Rating > int(maxElo.Int64) {
			return 0, ErrRoomRatingRestricted
		}
	}

	var existing int
	err = tx.QueryRow(`SELECT COUNT(*) FROM room_players WHERE room_id = $1 AND user_id = $2`, roomID, player.UserID).Scan(&existing)
	if err != nil {
		return 0, fmt.Errorf("check existing player: %w", err)
	}
	if existing > 0 {
		return 0, ErrPlayerAlreadyInRoom
	}

	_, err = tx.Exec(`INSERT INTO room_players (id, room_id, user_id, display_name, joined_at) VALUES ($1, $2, $3, $4, $5)`, uuid.New(), roomID, player.UserID, player.DisplayName, time.Now())
	if err != nil {
		return 0, fmt.Errorf("add player: %w", err)
	}

	newCount := currentPlayerCount + 1
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newCount, nil
}

func GetUserRating(db *sql.DB, userID uuid.UUID) (int, error) {
	var rating int
	err := db.QueryRow(`SELECT rating FROM user_stats WHERE user_id = $1`, userID).Scan(&rating)
	if err == sql.ErrNoRows {
		return DefaultRating, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get user rating: %w", err)
	}
	return rating, nil
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func scanIntPtr(target **int) any {
	return &nullableIntScanner{target: target}
}

type nullableIntScanner struct {
	target **int
}

func (s *nullableIntScanner) Scan(value any) error {
	if value == nil {
		*s.target = nil
		return nil
	}
	var out int
	switch v := value.(type) {
	case int64:
		out = int(v)
	case int32:
		out = int(v)
	case int:
		out = v
	default:
		return fmt.Errorf("scan nullable int: unsupported %T", value)
	}
	*s.target = &out
	return nil
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
//	finished -> in_progress (a unanimous rematch starts a new game in the same room)
//	finished -> waiting (a partial rematch drops the voters back to the waiting room)
//
// Other transitions are rejected so the public lobby list cannot regress.
func UpdateRoomStatus(db *sql.DB, roomID uuid.UUID, newStatus string) error {
	if newStatus != "in_progress" && newStatus != "finished" && newStatus != "waiting" {
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
	case "finished":
		// A rematch reuses the same room: unanimous votes start a new game
		// (in_progress), while a partial vote drops the voters back to the
		// waiting room (waiting).
		allowed = newStatus == "in_progress" || newStatus == "waiting"
	}
	if !allowed {
		return fmt.Errorf("cannot transition room status from %s to %s", current, newStatus)
	}
	if _, err := tx.Exec(`UPDATE rooms SET status = $1 WHERE id = $2`, newStatus, roomID); err != nil {
		return fmt.Errorf("update room status: %w", err)
	}
	return tx.Commit()
}
