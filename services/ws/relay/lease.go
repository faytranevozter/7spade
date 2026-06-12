// Package relay provides the cross-replica coordination primitives that let
// multiple ws replicas serve a single room: per-room owner leases (with a
// monotonic fencing token to prevent split-brain writes) and a JSON pub/sub
// broker for forwarding client messages to a room's owner and fanning the
// owner's outbound messages back out to every replica holding that room's
// sockets.
//
// All state lives in the dedicated WS Redis; nothing is held only in process
// memory, so a replica can crash and another can take over a room from the
// last persisted snapshot.
package relay

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultLeaseTTL is how long an owner lease lives without a renewal. The owner
// heartbeats well within this window; if it dies, the lease expires and another
// replica can claim the room.
const DefaultLeaseTTL = 10 * time.Second

// ErrNotOwner is returned when a renew/release targets a lease this replica no
// longer holds (it expired or was claimed by another replica).
var ErrNotOwner = errors.New("relay: lease not held by this replica")

// leaseKey / fenceKey are the Redis keys backing a room's ownership.
func leaseKey(roomID string) string { return "roomlease:" + roomID }
func fenceKey(roomID string) string { return "roomfence:" + roomID }

// LeaseManager acquires and maintains per-room owner leases in Redis. Each
// replica constructs one with a stable replica id.
type LeaseManager struct {
	client    redis.Cmdable
	replicaID string
	ttl       time.Duration
}

// NewLeaseManager builds a LeaseManager. A non-positive ttl falls back to
// DefaultLeaseTTL.
func NewLeaseManager(client redis.Cmdable, replicaID string, ttl time.Duration) *LeaseManager {
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	return &LeaseManager{client: client, replicaID: replicaID, ttl: ttl}
}

// ReplicaID returns this manager's replica identifier.
func (m *LeaseManager) ReplicaID() string { return m.replicaID }

// TTL returns the configured lease TTL.
func (m *LeaseManager) TTL() time.Duration { return m.ttl }

// Acquire attempts to become the owner of a room. On success it returns
// acquired=true and the room's current fencing token (a monotonically
// increasing counter bumped on every successful acquisition). When another
// replica already owns the room it returns acquired=false and that owner's id.
//
// The fencing token lets the rest of the system reject writes from a replica
// that believes it is still the owner but has actually been superseded: every
// persist/publish carries the token, and a higher token always wins.
func (m *LeaseManager) Acquire(ctx context.Context, roomID string) (acquired bool, token int64, owner string, err error) {
	ok, err := m.client.SetNX(ctx, leaseKey(roomID), m.replicaID, m.ttl).Result()
	if err != nil {
		return false, 0, "", fmt.Errorf("relay: acquire lease: %w", err)
	}
	if !ok {
		current, getErr := m.client.Get(ctx, leaseKey(roomID)).Result()
		if getErr != nil && !errors.Is(getErr, redis.Nil) {
			return false, 0, "", fmt.Errorf("relay: read lease owner: %w", getErr)
		}
		return false, 0, current, nil
	}
	// Newly acquired: bump the fencing token so any prior owner's token is stale.
	token, err = m.client.Incr(ctx, fenceKey(roomID)).Result()
	if err != nil {
		return false, 0, "", fmt.Errorf("relay: bump fence token: %w", err)
	}
	return true, token, m.replicaID, nil
}

// Renew extends this replica's lease on a room. It returns ErrNotOwner if the
// lease no longer belongs to this replica (expired or stolen), which the caller
// must treat as losing ownership.
func (m *LeaseManager) Renew(ctx context.Context, roomID string) error {
	// Atomic check-and-extend: only refresh the TTL if we still hold the key.
	result, err := renewScript.Run(ctx, m.client, []string{leaseKey(roomID)}, m.replicaID, m.ttl.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("relay: renew lease: %w", err)
	}
	if result == 0 {
		return ErrNotOwner
	}
	return nil
}

// Release relinquishes ownership of a room if (and only if) this replica still
// holds the lease. Releasing a lease this replica no longer owns is a no-op.
func (m *LeaseManager) Release(ctx context.Context, roomID string) error {
	if err := releaseScript.Run(ctx, m.client, []string{leaseKey(roomID)}, m.replicaID).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("relay: release lease: %w", err)
	}
	return nil
}

// CurrentToken reads a room's current fencing token without changing ownership.
// Returns 0 when no token exists yet.
func (m *LeaseManager) CurrentToken(ctx context.Context, roomID string) (int64, error) {
	token, err := m.client.Get(ctx, fenceKey(roomID)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("relay: read fence token: %w", err)
	}
	return token, nil
}

// renewScript extends the TTL only if the key still holds our replica id.
// Returns 1 on success, 0 if we no longer own the lease.
var renewScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("PEXPIRE", KEYS[1], ARGV[2])
	end
	return 0
`)

// releaseScript deletes the key only if it still holds our replica id, so we
// never delete a lease another replica has since claimed.
var releaseScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
`)
