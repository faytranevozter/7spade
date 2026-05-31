package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PresenceTTL is how long a presence key lives without a refresh. The heartbeat
// must run more often than this so a still-connected user never lapses.
const PresenceTTL = 60 * time.Second

// Presence writes per-user online markers to Redis with a TTL, so the API can
// read who is currently connected. The key holds the user's current room_id (or
// "" when connected but not in a room). It shares the same Redis client as the
// snapshot store.
type Presence struct {
	client redis.Cmdable
	ttl    time.Duration
}

// NewPresence builds a Presence writer. A non-positive ttl falls back to
// PresenceTTL.
func NewPresence(client redis.Cmdable, ttl time.Duration) *Presence {
	if ttl <= 0 {
		ttl = PresenceTTL
	}
	return &Presence{client: client, ttl: ttl}
}

// PresenceKey is the agreed key format read by the API (cache.presenceKey).
func PresenceKey(userID string) string { return "presence:user:" + userID }

// Online marks a user online with their current room_id (may be ""), resetting
// the TTL.
func (p *Presence) Online(ctx context.Context, userID, roomID string) error {
	if err := p.client.Set(ctx, PresenceKey(userID), roomID, p.ttl).Err(); err != nil {
		return fmt.Errorf("store: set presence: %w", err)
	}
	return nil
}

// Offline clears a user's presence immediately.
func (p *Presence) Offline(ctx context.Context, userID string) error {
	if err := p.client.Del(ctx, PresenceKey(userID)).Err(); err != nil {
		return fmt.Errorf("store: del presence: %w", err)
	}
	return nil
}
