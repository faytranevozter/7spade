package room

import "sync"

type memoryStateStore struct {
	mu        sync.Mutex
	snapshots map[string]roomSnapshot
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{snapshots: map[string]roomSnapshot{}}
}

func (store *memoryStateStore) SaveRoom(roomID string, snap roomSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.snapshots[roomID] = cloneRoomSnapshot(snap)
}

func (store *memoryStateStore) LoadRoom(roomID string) (roomSnapshot, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	snap, ok := store.snapshots[roomID]
	if !ok {
		return roomSnapshot{}, false
	}
	return cloneRoomSnapshot(snap), true
}

func (store *memoryStateStore) DeleteRoom(roomID string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.snapshots, roomID)
}

// cloneRoomSnapshot deep-copies a snapshot so the in-memory store doesn't alias
// the caller's live state (mirrors the JSON copy the Redis adapter performs).
func cloneRoomSnapshot(snap roomSnapshot) roomSnapshot {
	clone := snap
	clone.state = cloneGameState(snap.state)
	clone.players = append([]persistedPlayer(nil), snap.players...)
	clone.rematchVotes = append([]int(nil), snap.rematchVotes...)
	return clone
}

type Snapshot = roomSnapshot
