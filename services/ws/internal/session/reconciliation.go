package session

import (
	"context"
	"log"
	"time"
)

// ReconcileInterval is how often live room ownership is reported to the API.
const ReconcileInterval = 60 * time.Second

// RoomInventory exposes only the live-room view reconciliation needs.
type RoomInventory interface {
	ActiveRoomIDs() []string
	OwnedRoomIDs() []string
}

type RoomReconciler interface {
	ReconcileRooms([]string) error
}

type RoomCoordinator interface {
	PublishActiveRooms(context.Context, []string, time.Duration) error
	AcquireLeadership(context.Context, time.Duration) (bool, error)
	ActiveRooms(context.Context) ([]string, error)
}

// StartReconciler periodically reports active rooms until ctx is cancelled.
// A coordinator makes reconciliation cluster-wide; without one, local rooms are
// reported directly.
func StartReconciler(ctx context.Context, rooms RoomInventory, reconciler RoomReconciler, coordinator RoomCoordinator) {
	if reconciler == nil {
		return
	}
	ticker := time.NewTicker(ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ReconcileOnce(ctx, rooms, reconciler, coordinator)
		}
	}
}

// ReconcileOnce performs one reconciliation pass. Only the elected cluster
// leader reports the union, preventing another replica's rooms being removed.
func ReconcileOnce(ctx context.Context, rooms RoomInventory, reconciler RoomReconciler, coordinator RoomCoordinator) {
	if coordinator == nil {
		if err := reconciler.ReconcileRooms(rooms.ActiveRoomIDs()); err != nil {
			log.Printf("reconcile rooms: %v", err)
		}
		return
	}

	ttl := 3 * ReconcileInterval
	pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	if err := coordinator.PublishActiveRooms(pubCtx, rooms.OwnedRoomIDs(), ttl); err != nil {
		log.Printf("publish active rooms: %v", err)
	}
	cancel()

	leadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	leader, err := coordinator.AcquireLeadership(leadCtx, ttl)
	cancel()
	if err != nil {
		log.Printf("reconciler leadership: %v", err)
		return
	}
	if !leader {
		return
	}

	unionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	active, err := coordinator.ActiveRooms(unionCtx)
	cancel()
	if err != nil {
		log.Printf("read active rooms union: %v", err)
		return
	}
	if err := reconciler.ReconcileRooms(active); err != nil {
		log.Printf("reconcile rooms: %v", err)
	}
}
