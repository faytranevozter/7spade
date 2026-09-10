package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/room"
	"github.com/faytranevozter/7spade/services/ws/store"
)

// Redis adapts room snapshots to the store package. Writes run asynchronously
// so Redis I/O does not block room mutations; store.Store uses snapshot versions
// to reject delayed stale writes. Loads are synchronous on the join path.
type Redis struct {
	store   *store.Store
	timeout time.Duration
}

func NewRedis(s *store.Store) *Redis {
	return &Redis{store: s, timeout: 5 * time.Second}
}

func (r *Redis) SaveRoom(roomID string, snap room.Snapshot) {
	// Fire-and-forget so Redis I/O never blocks the move/lobby hot-path.
	// store.SaveRoom serialises per-room versions, so a delayed write can't
	// resurrect a deleted room. Callers that need durability before a
	// subsequent LoadRoom (e.g. tests) must wait for the key to appear.
	persisted, err := toStoreSnapshot(room.DurableSnapshotFromLive(snap))
	if err != nil {
		log.Printf("encode room %s: %v", roomID, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.SaveRoom(ctx, roomID, persisted); err != nil {
			log.Printf("persist room %s: %v", roomID, err)
		}
	}()
}

func (r *Redis) LoadRoom(roomID string) (room.Snapshot, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	persisted, err := r.store.LoadRoom(ctx, roomID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("load room %s: %v", roomID, err)
		}
		return room.Snapshot{}, false
	}
	durable, err := fromStoreSnapshot(persisted)
	if err != nil {
		log.Printf("decode room %s: %v", roomID, err)
		return room.Snapshot{}, false
	}
	return room.LiveSnapshotFromDurable(durable), true
}

// JSON translation keeps the existing Redis schema authoritative while the room
// exposes a storage-neutral durable DTO. It also accepts legacy payloads that
// omit fields added after their original write.
func toStoreSnapshot(snapshot room.DurableSnapshot) (store.RoomSnapshot, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return store.RoomSnapshot{}, err
	}
	var persisted store.RoomSnapshot
	return persisted, json.Unmarshal(payload, &persisted)
}

func fromStoreSnapshot(snapshot store.RoomSnapshot) (room.DurableSnapshot, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return room.DurableSnapshot{}, err
	}
	var durable room.DurableSnapshot
	return durable, json.Unmarshal(payload, &durable)
}

func (r *Redis) DeleteRoom(roomID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.Delete(ctx, roomID); err != nil {
			log.Printf("delete room %s: %v", roomID, err)
		}
	}()
}
