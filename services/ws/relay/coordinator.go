package relay

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// activeRoomsKey is the shared sorted set of live rooms across all replicas.
// Members are room ids; scores are unix-nano expiry deadlines. Each owner
// refreshes its rooms' deadlines every reconcile cycle; entries whose deadline
// has passed (e.g. a replica that died) are pruned and excluded from the union,
// so the API never sees a dead replica's rooms as "still active".
const activeRoomsKey = "ws:active_rooms"

// leaderKey is the lease that elects the single replica which reports the union
// of active rooms to the API, so reconciliation runs once cluster-wide.
const leaderKey = "ws:reconciler:leader"

// Coordinator maintains the cross-replica active-room set and reconciler leader
// election in the shared WS Redis.
type Coordinator struct {
	client    redis.Cmdable
	replicaID string
}

// NewCoordinator builds a Coordinator over the shared WS Redis client.
func NewCoordinator(client redis.Cmdable, replicaID string) *Coordinator {
	return &Coordinator{client: client, replicaID: replicaID}
}

// PublishActiveRooms records this replica's currently-owned room ids in the
// shared set with a freshness deadline of now+ttl, and prunes any entries whose
// deadline has already passed. Call once per reconcile cycle on every replica.
func (c *Coordinator) PublishActiveRooms(ctx context.Context, roomIDs []string, ttl time.Duration) error {
	now := time.Now()
	// Drop expired members first so the set doesn't grow unbounded with rooms
	// from replicas that have gone away.
	if err := c.client.ZRemRangeByScore(ctx, activeRoomsKey, "-inf", strconv.FormatInt(now.UnixNano(), 10)).Err(); err != nil {
		return fmt.Errorf("relay: prune active rooms: %w", err)
	}
	if len(roomIDs) == 0 {
		return nil
	}
	deadline := float64(now.Add(ttl).UnixNano())
	members := make([]redis.Z, 0, len(roomIDs))
	for _, id := range roomIDs {
		members = append(members, redis.Z{Score: deadline, Member: id})
	}
	if err := c.client.ZAdd(ctx, activeRoomsKey, members...).Err(); err != nil {
		return fmt.Errorf("relay: publish active rooms: %w", err)
	}
	return nil
}

// ActiveRooms returns the union of all replicas' non-expired active room ids.
func (c *Coordinator) ActiveRooms(ctx context.Context) ([]string, error) {
	now := strconv.FormatInt(time.Now().UnixNano(), 10)
	ids, err := c.client.ZRangeByScore(ctx, activeRoomsKey, &redis.ZRangeBy{Min: now, Max: "+inf"}).Result()
	if err != nil {
		return nil, fmt.Errorf("relay: read active rooms: %w", err)
	}
	return ids, nil
}

// AcquireLeadership tries to become (or remain) the reconciler leader for the
// given lease duration. Returns true while this replica holds leadership. The
// lease auto-expires, so a dead leader is replaced within ttl.
func (c *Coordinator) AcquireLeadership(ctx context.Context, ttl time.Duration) (bool, error) {
	// Renew if we already hold it; otherwise try to take it.
	ok, err := c.client.SetNX(ctx, leaderKey, c.replicaID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("relay: acquire leadership: %w", err)
	}
	if ok {
		return true, nil
	}
	held, err := leaderRenewScript.Run(ctx, c.client, []string{leaderKey}, c.replicaID, ttl.Milliseconds()).Int()
	if err != nil {
		return false, fmt.Errorf("relay: renew leadership: %w", err)
	}
	return held == 1, nil
}

// leaderRenewScript extends the leader lease only if we still hold it.
var leaderRenewScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("PEXPIRE", KEYS[1], ARGV[2])
	end
	return 0
`)
