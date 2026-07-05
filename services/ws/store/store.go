// Package store persists live Seven Spade game state to Redis.
//
// The Game State Store is the bridge between the WebSocket game server and the
// running room state. A whole-room snapshot (game state plus lobby/roster
// metadata) is JSON-encoded and stored under a room-scoped key with a TTL that
// is refreshed on every write, so that abandoned rooms eventually expire from
// Redis. On restart the WS server rehydrates a room from its snapshot the first
// time a player (re)connects.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/game"
)

// ErrNotFound is returned by LoadRoom when no snapshot exists for the room.
var ErrNotFound = errors.New("room snapshot not found")

// DefaultTTL is the default expiry applied to a saved snapshot when the caller
// does not specify one.
const DefaultTTL = time.Hour

// PersistedPlayer is the durable subset of a room player. Runtime-only fields
// (the live WebSocket connection and leave timers) are deliberately omitted: a
// rehydrated player has no socket until one re-attaches, so it is treated as
// disconnected until then.
type PersistedPlayer struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
	IsGuest     bool   `json:"is_guest"`
	IsBot       bool   `json:"is_bot"`
	Ready       bool   `json:"ready"`
	Index       int    `json:"index"`
}

// RoomSnapshot is the complete durable state of a room — enough to rebuild it
// after a WS process restart.
type RoomSnapshot struct {
	State            game.GameState    `json:"state"`
	Players          []PersistedPlayer `json:"players"`
	Phase            int               `json:"phase"`
	Started          bool              `json:"started"`
	StartedAt        time.Time         `json:"started_at"`
	TurnExpiresAt    time.Time         `json:"turn_expires_at"`
	TurnTimerSeconds int               `json:"turn_timer_seconds"`
	BotDifficulty    string            `json:"bot_difficulty,omitempty"`
	PracticeMode     bool              `json:"practice_mode,omitempty"`
	TurnTimerToken   int               `json:"turn_timer_token"`
	RematchVotes     []int             `json:"rematch_votes"`
	InitialHands     [][]game.Card     `json:"initial_hands,omitempty"`
	Moves            []PersistedMove   `json:"moves,omitempty"`
	GameConfig       *game.GameConfig  `json:"game_config,omitempty"`
}

// PersistedMove is the durable form of a single game move for replay recording.
type PersistedMove struct {
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit"`
	Rank         int    `json:"rank"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

// Store reads and writes [RoomSnapshot] values to Redis.
type Store struct {
	client redis.Cmdable
	ttl    time.Duration
}

// New constructs a Store that uses the given Redis client and TTL for every
// write. A non-positive ttl falls back to [DefaultTTL].
func New(client redis.Cmdable, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Store{client: client, ttl: ttl}
}

// StateKey returns the Redis key used to store a room's snapshot.
func StateKey(roomID string) string {
	return "room:" + roomID + ":state"
}

// SaveRoom serialises the snapshot as JSON and writes it under StateKey(roomID).
// The TTL is set on every write, so an active room continually refreshes its
// expiry while abandoned rooms fall out automatically.
func (s *Store) SaveRoom(ctx context.Context, roomID string, snap RoomSnapshot) error {
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("store: marshal room snapshot: %w", err)
	}
	if err := s.client.Set(ctx, StateKey(roomID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("store: save room snapshot: %w", err)
	}
	return nil
}

// LoadRoom returns the snapshot for the room. If none exists, LoadRoom returns
// [ErrNotFound] wrapped in a descriptive error.
func (s *Store) LoadRoom(ctx context.Context, roomID string) (RoomSnapshot, error) {
	payload, err := s.client.Get(ctx, StateKey(roomID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return RoomSnapshot{}, fmt.Errorf("store: room %q: %w", roomID, ErrNotFound)
		}
		return RoomSnapshot{}, fmt.Errorf("store: load room snapshot: %w", err)
	}

	snap := RoomSnapshot{State: game.NewGameState()}
	if err := json.Unmarshal(payload, &snap); err != nil {
		return RoomSnapshot{}, fmt.Errorf("store: unmarshal room snapshot: %w", err)
	}
	// Maps may be nil after unmarshal if the JSON contained null/missing keys;
	// restore the empty-map invariant established by NewGameState so callers
	// can index without nil checks.
	if snap.State.Board == nil {
		snap.State.Board = map[game.Suit]game.SuitSequence{}
	}
	if snap.State.Closed == nil {
		snap.State.Closed = map[game.Suit]bool{}
	}
	return snap, nil
}

// Delete removes the snapshot for the room. It is not an error to delete a
// missing room, but a subsequent LoadRoom will return [ErrNotFound].
func (s *Store) Delete(ctx context.Context, roomID string) error {
	if err := s.client.Del(ctx, StateKey(roomID)).Err(); err != nil {
		return fmt.Errorf("store: delete room snapshot: %w", err)
	}
	return nil
}
