// Package store persists live Seven Spade game state to Redis.
//
// The Game State Store is the bridge between the WebSocket game server and
// the Game Engine's pure data structures. State is JSON-encoded and stored
// under a room-scoped key with a TTL that is refreshed on every write so
// that abandoned rooms eventually expire from Redis.
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

// ErrNotFound is returned by Load when no state exists for the room.
var ErrNotFound = errors.New("game state not found")

// DefaultTTL is the default expiry applied to a saved game state when the
// caller does not specify one.
const DefaultTTL = time.Hour

// Store reads and writes [game.GameState] to Redis.
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

// StateKey returns the Redis key used to store live game state for the room.
func StateKey(roomID string) string {
	return "room:" + roomID + ":state"
}

// Save serialises state as JSON and writes it to Redis under StateKey(roomID).
// The TTL is set on every write, so an active room continually refreshes its
// expiry while abandoned rooms fall out automatically.
func (s *Store) Save(ctx context.Context, roomID string, state game.GameState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("store: marshal game state: %w", err)
	}
	if err := s.client.Set(ctx, StateKey(roomID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("store: save game state: %w", err)
	}
	return nil
}

// Load returns the game state for the room. If no state exists for the
// roomID, Load returns [ErrNotFound] wrapped in a descriptive error.
func (s *Store) Load(ctx context.Context, roomID string) (game.GameState, error) {
	payload, err := s.client.Get(ctx, StateKey(roomID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return game.GameState{}, fmt.Errorf("store: room %q: %w", roomID, ErrNotFound)
		}
		return game.GameState{}, fmt.Errorf("store: load game state: %w", err)
	}

	state := game.NewGameState()
	if err := json.Unmarshal(payload, &state); err != nil {
		return game.GameState{}, fmt.Errorf("store: unmarshal game state: %w", err)
	}
	// Maps may be nil after unmarshal if the JSON contained null/missing
	// keys; restore the empty-map invariant established by NewGameState so
	// callers can index without nil checks.
	if state.Board == nil {
		state.Board = map[game.Suit]game.SuitSequence{}
	}
	if state.Closed == nil {
		state.Closed = map[game.Suit]bool{}
	}
	return state, nil
}

// Delete removes the state for the room. It is not an error to delete a
// missing room, but a subsequent Load will return [ErrNotFound].
func (s *Store) Delete(ctx context.Context, roomID string) error {
	if err := s.client.Del(ctx, StateKey(roomID)).Err(); err != nil {
		return fmt.Errorf("store: delete game state: %w", err)
	}
	return nil
}
