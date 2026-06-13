package relay

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (redis.UniversalClient, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

// Acquire grants the lease to the first caller, bumps the fencing token, and
// denies a second replica until release.
func TestLeaseAcquireAndFence(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()
	a := NewLeaseManager(client, "replica-A", time.Second)
	b := NewLeaseManager(client, "replica-B", time.Second)

	ok, token, owner, err := a.Acquire(ctx, "room1")
	if err != nil {
		t.Fatalf("acquire A: %v", err)
	}
	if !ok || owner != "replica-A" || token != 1 {
		t.Fatalf("A acquire = ok:%v owner:%q token:%d, want ok owner replica-A token 1", ok, owner, token)
	}

	ok2, _, owner2, err := b.Acquire(ctx, "room1")
	if err != nil {
		t.Fatalf("acquire B: %v", err)
	}
	if ok2 || owner2 != "replica-A" {
		t.Fatalf("B acquire = ok:%v owner:%q, want denied with owner replica-A", ok2, owner2)
	}

	// After A releases, B can claim and the fence token increments.
	if err := a.Release(ctx, "room1"); err != nil {
		t.Fatalf("release A: %v", err)
	}
	ok3, token3, owner3, err := b.Acquire(ctx, "room1")
	if err != nil {
		t.Fatalf("acquire B again: %v", err)
	}
	if !ok3 || owner3 != "replica-B" || token3 != 2 {
		t.Fatalf("B re-acquire = ok:%v owner:%q token:%d, want ok replica-B token 2", ok3, owner3, token3)
	}
}

// Renew succeeds for the holder and fails (ErrNotOwner) for a non-holder.
func TestLeaseRenew(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()
	a := NewLeaseManager(client, "replica-A", time.Second)
	b := NewLeaseManager(client, "replica-B", time.Second)

	if _, _, _, err := a.Acquire(ctx, "room1"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := a.Renew(ctx, "room1"); err != nil {
		t.Fatalf("renew by owner: %v", err)
	}
	if err := b.Renew(ctx, "room1"); err != ErrNotOwner {
		t.Fatalf("renew by non-owner = %v, want ErrNotOwner", err)
	}
}

// A lease expires after its TTL, letting another replica take over.
func TestLeaseExpiry(t *testing.T) {
	client, mr := newTestClient(t)
	ctx := context.Background()
	a := NewLeaseManager(client, "replica-A", time.Second)
	b := NewLeaseManager(client, "replica-B", time.Second)

	if _, _, _, err := a.Acquire(ctx, "room1"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	mr.FastForward(2 * time.Second) // expire the lease

	ok, token, owner, err := b.Acquire(ctx, "room1")
	if err != nil {
		t.Fatalf("acquire after expiry: %v", err)
	}
	if !ok || owner != "replica-B" || token != 2 {
		t.Fatalf("B acquire after expiry = ok:%v owner:%q token:%d, want ok replica-B token 2", ok, owner, token)
	}
}

// Outbound envelopes published by the owner reach a subscribed edge.
func TestBrokerOutboundRoundTrip(t *testing.T) {
	client, _ := newTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker := NewBroker(client)

	var (
		mu   sync.Mutex
		got  []Envelope
		recv = make(chan struct{}, 1)
	)
	sub := broker.SubscribeOutbound(ctx, "room1", func(env Envelope) {
		mu.Lock()
		got = append(got, env)
		mu.Unlock()
		select {
		case recv <- struct{}{}:
		default:
		}
	})
	defer sub.Close()

	// Give the subscription a moment to register before publishing.
	waitForSubscribers(t, client, outChannel("room1"))

	env := Envelope{Seq: 7, Target: Target{Kind: TargetSub, Sub: "sub-2"}, Payload: map[string]any{"type": "state"}}
	if err := broker.PublishOutbound(ctx, "room1", env); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-recv:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for outbound envelope")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].Seq != 7 || got[0].Target.Kind != TargetSub || got[0].Target.Sub != "sub-2" {
		t.Fatalf("got %+v, want seq 7 sub sub-2", got)
	}
}

// Inbound messages forwarded by an edge reach the owner's subscriber.
func TestBrokerInboundRoundTrip(t *testing.T) {
	client, _ := newTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker := NewBroker(client)

	recv := make(chan Inbound, 1)
	sub := broker.SubscribeInbound(ctx, "room1", func(in Inbound) { recv <- in })
	defer sub.Close()

	waitForSubscribers(t, client, inChannel("room1"))

	if err := broker.PublishInbound(ctx, "room1", Inbound{Kind: InboundData, Sub: "user-1", Payload: []byte(`{"type":"play_card"}`)}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	select {
	case in := <-recv:
		if in.Sub != "user-1" || in.Kind != InboundData {
			t.Fatalf("got %+v, want sub user-1 kind data", in)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for inbound message")
	}
}

// waitForSubscribers blocks until at least one subscriber is registered on the
// channel, so a publish isn't dropped before the subscription is live.
func waitForSubscribers(t *testing.T, client redis.UniversalClient, channel string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := client.PubSubNumSub(context.Background(), channel).Result()
		if err == nil && counts[channel] >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no subscriber registered on %s", channel)
}
