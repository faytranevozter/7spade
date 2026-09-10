package session

import (
	"context"
	"testing"
)

type testRoomInventory struct{ active, owned []string }

func (r testRoomInventory) ActiveRoomIDs() []string { return r.active }
func (r testRoomInventory) OwnedRoomIDs() []string  { return r.owned }

type testReconciler struct{ received []string }

func (r *testReconciler) ReconcileRooms(ids []string) error {
	r.received = append([]string(nil), ids...)
	return nil
}

func TestReconcileOnceReportsLocalActiveRooms(t *testing.T) {
	reconciler := &testReconciler{}
	ReconcileOnce(context.Background(), testRoomInventory{active: []string{"room-live"}}, reconciler, nil)
	if len(reconciler.received) != 1 || reconciler.received[0] != "room-live" {
		t.Fatalf("received = %v, want [room-live]", reconciler.received)
	}
}

func TestStartReconcilerReturnsWithoutReconciler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartReconciler(ctx, testRoomInventory{}, nil, nil)
}
