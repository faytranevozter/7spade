package relay

import (
	"context"
	"sort"
	"testing"
	"time"
)

// PublishActiveRooms + ActiveRooms expose the union of replicas' owned rooms,
// and entries past their freshness deadline are pruned (a dead replica's rooms
// fall out).
func TestCoordinatorActiveRoomsUnionAndExpiry(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()
	a := NewCoordinator(client, "replica-A")
	b := NewCoordinator(client, "replica-B")

	if err := a.PublishActiveRooms(ctx, []string{"room1", "room2"}, time.Minute); err != nil {
		t.Fatalf("publish A: %v", err)
	}
	if err := b.PublishActiveRooms(ctx, []string{"room3"}, time.Minute); err != nil {
		t.Fatalf("publish B: %v", err)
	}

	got, err := a.ActiveRooms(ctx)
	if err != nil {
		t.Fatalf("active rooms: %v", err)
	}
	sort.Strings(got)
	want := []string{"room1", "room2", "room3"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("union = %v, want %v", got, want)
	}

	// Freshness is score-based on real wall-clock (ZSET scores are unix-nano
	// deadlines), which the in-memory Redis fake can't fast-forward. Use a tiny
	// TTL and a real sleep so the deadlines genuinely lapse, then confirm both
	// the read filter and the prune-on-publish drop the stale entries.
	if err := a.PublishActiveRooms(ctx, []string{"room1", "room2"}, 20*time.Millisecond); err != nil {
		t.Fatalf("re-publish A short ttl: %v", err)
	}
	if err := b.PublishActiveRooms(ctx, []string{"room3"}, 20*time.Millisecond); err != nil {
		t.Fatalf("re-publish B short ttl: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	got, err = a.ActiveRooms(ctx)
	if err != nil {
		t.Fatalf("active rooms after expiry: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no active rooms after expiry, got %v", got)
	}
}

// Leadership is exclusive: one replica holds it, another is denied until expiry.
func TestCoordinatorLeadership(t *testing.T) {
	client, mr := newTestClient(t)
	ctx := context.Background()
	a := NewCoordinator(client, "replica-A")
	b := NewCoordinator(client, "replica-B")

	leadA, err := a.AcquireLeadership(ctx, time.Minute)
	if err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	if !leadA {
		t.Fatal("A should be leader")
	}
	leadB, err := b.AcquireLeadership(ctx, time.Minute)
	if err != nil {
		t.Fatalf("B acquire: %v", err)
	}
	if leadB {
		t.Fatal("B should not be leader while A holds it")
	}

	// A renews and keeps leadership.
	leadA, err = a.AcquireLeadership(ctx, time.Minute)
	if err != nil {
		t.Fatalf("A renew: %v", err)
	}
	if !leadA {
		t.Fatal("A should retain leadership on renew")
	}

	// After A's lease lapses, B can take over.
	mr.FastForward(2 * time.Minute)
	leadB, err = b.AcquireLeadership(ctx, time.Minute)
	if err != nil {
		t.Fatalf("B acquire after expiry: %v", err)
	}
	if !leadB {
		t.Fatal("B should take leadership after A's lease expires")
	}
}
