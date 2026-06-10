package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client, err := New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func TestPasswordResetTokenConsumeOnce(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	if err := client.StorePasswordResetToken(ctx, "hash-1", "user-1", time.Minute); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := client.ConsumePasswordResetToken(ctx, "hash-1")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if got != "user-1" {
		t.Fatalf("user id = %q, want user-1", got)
	}

	// Single-use: a second consume must fail (token deleted).
	if _, err := client.ConsumePasswordResetToken(ctx, "hash-1"); err == nil {
		t.Fatal("expected error on second consume, got nil")
	}
}

func TestPasswordResetTokenExpires(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	if err := client.StorePasswordResetToken(ctx, "hash-exp", "user-2", time.Minute); err != nil {
		t.Fatalf("store: %v", err)
	}
	mr.FastForward(2 * time.Minute)

	if _, err := client.ConsumePasswordResetToken(ctx, "hash-exp"); err == nil {
		t.Fatal("expected expired token to be unusable")
	}
}

func TestEmailVerifyTokenConsumeOnce(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	if err := client.StoreEmailVerifyToken(ctx, "vhash", "user-3", time.Hour); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := client.ConsumeEmailVerifyToken(ctx, "vhash")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if got != "user-3" {
		t.Fatalf("user id = %q, want user-3", got)
	}
	if _, err := client.ConsumeEmailVerifyToken(ctx, "vhash"); err == nil {
		t.Fatal("expected error on second consume")
	}
}

func TestConsumeUnknownTokenFails(t *testing.T) {
	client, _ := newTestRedis(t)
	if _, err := client.ConsumePasswordResetToken(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestAllowEmailRateEnforcesLimit(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	// limit=3: first three allowed, fourth denied.
	for i := 1; i <= 3; i++ {
		ok, err := client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour)
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}
	ok, err := client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour)
	if err != nil {
		t.Fatalf("4th attempt: %v", err)
	}
	if ok {
		t.Fatal("4th attempt should be denied")
	}
}

func TestAllowEmailRateIsPerEmailAndScope(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	// Exhaust one email's reset budget.
	for i := 0; i < 3; i++ {
		if _, err := client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// A different email is unaffected.
	if ok, _ := client.AllowEmailRate(ctx, "pwreset", "other@b.com", 3, time.Hour); !ok {
		t.Fatal("different email should have its own budget")
	}
	// A different scope for the same email is unaffected.
	if ok, _ := client.AllowEmailRate(ctx, "verify", "a@b.com", 5, time.Hour); !ok {
		t.Fatal("different scope should have its own budget")
	}
}

func TestAllowEmailRateWindowResets(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour)
	}
	if ok, _ := client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour); ok {
		t.Fatal("should be limited before the window resets")
	}
	mr.FastForward(2 * time.Hour)
	if ok, _ := client.AllowEmailRate(ctx, "pwreset", "a@b.com", 3, time.Hour); !ok {
		t.Fatal("should be allowed again after the window resets")
	}
}

func TestAllowOnceEnforcesCooldown(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	ok, err := client.AllowOnce(ctx, "quick_play", "user-1", 3*time.Second)
	if err != nil {
		t.Fatalf("first AllowOnce: %v", err)
	}
	if !ok {
		t.Fatal("first attempt should be allowed")
	}

	ok, err = client.AllowOnce(ctx, "quick_play", "user-1", 3*time.Second)
	if err != nil {
		t.Fatalf("second AllowOnce: %v", err)
	}
	if ok {
		t.Fatal("second attempt inside cooldown should be denied")
	}

	if ok, _ := client.AllowOnce(ctx, "quick_play", "user-2", 3*time.Second); !ok {
		t.Fatal("different subject should have its own cooldown")
	}

	mr.FastForward(4 * time.Second)
	if ok, _ := client.AllowOnce(ctx, "quick_play", "user-1", 3*time.Second); !ok {
		t.Fatal("same subject should be allowed after cooldown")
	}
}
