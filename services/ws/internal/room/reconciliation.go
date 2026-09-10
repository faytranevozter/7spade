package room

import (
	"context"
	"log"
	"time"
)

// reconcileInterval is how often the WS service reports its live room set to
// the API so orphaned 'waiting' rooms (no live presence) get cleaned up.
const reconcileInterval = 60 * time.Second

// presenceHeartbeat refreshes a connected user's presence TTL. It must be
// shorter than store.PresenceTTL so a still-connected user never lapses offline.
const presenceHeartbeat = 25 * time.Second

// StartRoomReconciler periodically reports the set of rooms this server is
// tracking to the API, which deletes stale 'waiting' rooms that have no live
// WS presence. No-op when no API reconciler is configured (e.g. tests or when
// API_URL is unset). Runs until ctx is cancelled.
func (server *Manager) StartRoomReconciler(ctx context.Context) {
	if server.reconciler == nil {
		return
	}
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			server.reconcileOnce(ctx)
		}
	}
}

// reconcileOnce performs one reconcile pass. Single-process: report the local
// room set as before. Multi-replica: every replica publishes its owned rooms to
// the shared active-room set, but only the elected leader reports the union to
// the API — so reconciliation runs once cluster-wide and never treats another
// replica's rooms as orphaned.
func (server *Manager) reconcileOnce(ctx context.Context) {
	if server.coordinator == nil {
		if err := server.reconciler.ReconcileRooms(server.activeRoomIDs()); err != nil {
			log.Printf("reconcile rooms: %v", err)
		}
		return
	}

	// Freshness window: a couple of reconcile intervals so a brief blip doesn't
	// drop a still-owned room from the union.
	pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	if err := server.coordinator.PublishActiveRooms(pubCtx, server.ownedRoomIDs(), 3*reconcileInterval); err != nil {
		log.Printf("publish active rooms: %v", err)
	}
	cancel()

	leadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	leader, err := server.coordinator.AcquireLeadership(leadCtx, 3*reconcileInterval)
	cancel()
	if err != nil {
		log.Printf("reconciler leadership: %v", err)
		return
	}
	if !leader {
		return
	}

	unionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	rooms, err := server.coordinator.ActiveRooms(unionCtx)
	cancel()
	if err != nil {
		log.Printf("read active rooms union: %v", err)
		return
	}
	if err := server.reconciler.ReconcileRooms(rooms); err != nil {
		log.Printf("reconcile rooms: %v", err)
	}
}

// activeRoomIDs snapshots the IDs of every room currently held in memory.
func (server *Manager) activeRoomIDs() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	ids := make([]string, 0, len(server.rooms))
	for id := range server.rooms {
		ids = append(ids, id)
	}
	return ids
}

// ownedRoomIDs snapshots the IDs of rooms this replica currently owns (relay
// active). Used to publish into the shared active-room set.
func (server *Manager) ownedRoomIDs() []string {
	server.mu.Lock()
	rooms := make([]*room, 0, len(server.rooms))
	for _, r := range server.rooms {
		rooms = append(rooms, r)
	}
	server.mu.Unlock()
	ids := make([]string, 0, len(rooms))
	for _, r := range rooms {
		if r.isOwnerOrSolo() {
			ids = append(ids, r.id)
		}
	}
	return ids
}
