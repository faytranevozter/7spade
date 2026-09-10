package room

import (
	"context"
	"testing"
	"time"
)

func TestRoomReconcilerReportsActiveRoomIDs(t *testing.T) {
	server := NewGameServer("test-secret")
	reconciler := &capturingReconciler{calls: make(chan []string, 1)}
	server.reconciler = reconciler
	server.rooms["room-live"] = &room{id: "room-live"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Drive one reconcile tick directly rather than waiting the production
	// interval; this exercises the same snapshot + report path.
	go func() {
		_ = reconciler.ReconcileRooms(server.activeRoomIDs())
		<-ctx.Done()
	}()

	select {
	case ids := <-reconciler.calls:
		if len(ids) != 1 || ids[0] != "room-live" {
			t.Fatalf("expected [room-live], got %+v", ids)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reconcile call")
	}
}

func TestStartRoomReconcilerNoopWithoutReconciler(t *testing.T) {
	server := NewGameServer("test-secret") // no API URL -> reconciler is nil
	if server.reconciler != nil {
		t.Fatal("expected nil reconciler when no API URL is configured")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should return promptly without panicking when reconciler is nil.
	server.StartRoomReconciler(ctx)
}
