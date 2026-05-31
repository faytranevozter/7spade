package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestPresence(t *testing.T) (*Presence, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewPresence(client, time.Minute), mr, client
}

func TestPresenceOnlineSetsKeyWithTTL(t *testing.T) {
	p, mr, _ := newTestPresence(t)
	ctx := context.Background()

	if err := p.Online(ctx, "user-1", "room-9"); err != nil {
		t.Fatalf("Online: %v", err)
	}

	got, err := mr.Get(PresenceKey("user-1"))
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if got != "room-9" {
		t.Fatalf("value = %q, want room-9", got)
	}
	if ttl := mr.TTL(PresenceKey("user-1")); ttl <= 0 {
		t.Fatalf("expected a positive TTL, got %v", ttl)
	}
}

func TestPresenceOnlineEmptyRoom(t *testing.T) {
	p, mr, _ := newTestPresence(t)
	if err := p.Online(context.Background(), "user-2", ""); err != nil {
		t.Fatalf("Online: %v", err)
	}
	got, err := mr.Get(PresenceKey("user-2"))
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if got != "" {
		t.Fatalf("value = %q, want empty", got)
	}
}

func TestPresenceOfflineClearsKey(t *testing.T) {
	p, mr, _ := newTestPresence(t)
	ctx := context.Background()
	if err := p.Online(ctx, "user-3", "room-1"); err != nil {
		t.Fatalf("Online: %v", err)
	}
	if err := p.Offline(ctx, "user-3"); err != nil {
		t.Fatalf("Offline: %v", err)
	}
	if mr.Exists(PresenceKey("user-3")) {
		t.Fatal("expected key to be cleared")
	}
}

func TestPresenceTTLFallback(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Non-positive ttl falls back to PresenceTTL.
	p := NewPresence(client, 0)
	if err := p.Online(context.Background(), "u", ""); err != nil {
		t.Fatalf("Online: %v", err)
	}
	ttl := mr.TTL(PresenceKey("u"))
	if ttl <= 0 || ttl > PresenceTTL {
		t.Fatalf("ttl = %v, want (0, %v]", ttl, PresenceTTL)
	}
}
