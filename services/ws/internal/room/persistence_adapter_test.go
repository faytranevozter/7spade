package room

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/store"
)

// redisStateStore persists room snapshots to Redis via the store package.
// Writes are performed asynchronously so Redis I/O never blocks the move
// hot-path (each room serialises its own mutations under room.mu, so
// last-write-wins ordering per room is safe). Loads are synchronous but only
// happen on the join path.
type redisStateStore struct {
	store   *store.Store
	timeout time.Duration
}

func newRedisStateStore(s *store.Store) *redisStateStore {
	return &redisStateStore{store: s, timeout: 5 * time.Second}
}

func (r *redisStateStore) SaveRoom(roomID string, snap roomSnapshot) {
	// Fire-and-forget so Redis I/O never blocks the move/lobby hot-path.
	// store.SaveRoom serialises per-room versions, so a delayed write can't
	// resurrect a deleted room. Callers that need durability before a
	// subsequent LoadRoom (e.g. tests) must wait for the key to appear.
	persisted := toStoreSnapshot(snap)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.SaveRoom(ctx, roomID, persisted); err != nil {
			log.Printf("persist room %s: %v", roomID, err)
		}
	}()
}

func (r *redisStateStore) LoadRoom(roomID string) (roomSnapshot, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	persisted, err := r.store.LoadRoom(ctx, roomID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("load room %s: %v", roomID, err)
		}
		return roomSnapshot{}, false
	}
	return fromStoreSnapshot(persisted), true
}

func (r *redisStateStore) DeleteRoom(roomID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.Delete(ctx, roomID); err != nil {
			log.Printf("delete room %s: %v", roomID, err)
		}
	}()
}
