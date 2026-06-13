package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// TargetKind selects which sockets of a room an outbound envelope is delivered
// to. Edges match this against their locally-held sockets; no game logic lives
// at the edge.
type TargetKind string

const (
	// TargetSub delivers to the connection(s) for a specific user sub. This is
	// how per-seat messages are routed across replicas: the edge registers its
	// socket by sub (known at connect time), so the owner never needs the seat
	// index — which avoids any dependence on seat assignment ordering.
	TargetSub TargetKind = "sub"
	// TargetSpectators delivers to every spectator of the room.
	TargetSpectators TargetKind = "spectators"
	// TargetAll delivers to every connected player seat of the room.
	TargetAll TargetKind = "all"
)

// Target selects the recipients of an Envelope.
type Target struct {
	Kind TargetKind `json:"kind"`
	Sub  string     `json:"sub,omitempty"`
}

// Envelope is one outbound message published by a room's owner. Payload is the
// already-rendered client message (per-seat state, lobby state, game over, etc).
type Envelope struct {
	Seq     int64          `json:"seq"`
	Target  Target         `json:"target"`
	Payload map[string]any `json:"payload"`
}

// InboundKind distinguishes the control messages an edge sends to a room owner
// over the inbound channel from ordinary gameplay data.
type InboundKind string

const (
	// InboundJoin asks the owner to seat (or reconnect) a player whose socket
	// lives on the sending edge. The owner replies with state via outbound.
	InboundJoin InboundKind = "join"
	// InboundLeave tells the owner a remote player's socket has dropped.
	InboundLeave InboundKind = "leave"
	// InboundData forwards a gameplay client message to the owner.
	InboundData InboundKind = "data"
)

// Inbound is one message forwarded from an edge replica to a room's owner. For
// InboundData, Payload holds the raw client frame. For InboundJoin it holds the
// joining player's token claims (so the owner can seat them); EdgeID identifies
// the originating replica so the owner can route the join result back.
type Inbound struct {
	Kind    InboundKind     `json:"kind"`
	Sub     string          `json:"sub"`
	EdgeID  string          `json:"edge_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// inChannel / outChannel are the per-room pub/sub channels. Inbound flows
// edge -> owner; outbound flows owner -> all edges.
func inChannel(roomID string) string  { return "room:" + roomID + ":in" }
func outChannel(roomID string) string { return "room:" + roomID + ":out" }

// Broker is a thin JSON pub/sub wrapper over Redis for room relay traffic.
type Broker struct {
	client redis.UniversalClient
}

// NewBroker builds a Broker over the given Redis client. A UniversalClient is
// required because pub/sub needs a live connection (Cmdable is insufficient).
func NewBroker(client redis.UniversalClient) *Broker {
	return &Broker{client: client}
}

// PublishOutbound sends an owner-rendered envelope to every edge of the room.
func (b *Broker) PublishOutbound(ctx context.Context, roomID string, env Envelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("relay: marshal outbound: %w", err)
	}
	if err := b.client.Publish(ctx, outChannel(roomID), payload).Err(); err != nil {
		return fmt.Errorf("relay: publish outbound: %w", err)
	}
	return nil
}

// PublishInbound forwards a client message from an edge to the room's owner.
func (b *Broker) PublishInbound(ctx context.Context, roomID string, msg Inbound) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("relay: marshal inbound: %w", err)
	}
	if err := b.client.Publish(ctx, inChannel(roomID), payload).Err(); err != nil {
		return fmt.Errorf("relay: publish inbound: %w", err)
	}
	return nil
}

// Subscription is an active pub/sub subscription to one room channel. Cancel it
// with Close when the replica no longer needs the room's traffic.
type Subscription struct {
	pubsub *redis.PubSub
	once   sync.Once
}

// Close tears down the subscription. Safe to call multiple times.
func (s *Subscription) Close() error {
	var err error
	s.once.Do(func() {
		err = s.pubsub.Close()
	})
	return err
}

// SubscribeOutbound subscribes to a room's outbound channel and invokes handle
// for every envelope until the returned Subscription is closed or ctx is done.
func (b *Broker) SubscribeOutbound(ctx context.Context, roomID string, handle func(Envelope)) *Subscription {
	pubsub := b.client.Subscribe(ctx, outChannel(roomID))
	sub := &Subscription{pubsub: pubsub}
	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var env Envelope
				if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
					continue
				}
				handle(env)
			}
		}
	}()
	return sub
}

// SubscribeInbound subscribes to a room's inbound channel (owner side) and
// invokes handle for every client message until closed or ctx is done.
func (b *Broker) SubscribeInbound(ctx context.Context, roomID string, handle func(Inbound)) *Subscription {
	pubsub := b.client.Subscribe(ctx, inChannel(roomID))
	sub := &Subscription{pubsub: pubsub}
	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var in Inbound
				if err := json.Unmarshal([]byte(msg.Payload), &in); err != nil {
					continue
				}
				handle(in)
			}
		}
	}()
	return sub
}
