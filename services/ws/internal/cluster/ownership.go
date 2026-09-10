// Package cluster owns replica coordination and edge forwarding. It has no
// dependency on room state: inbound messages cross a callback into the runtime.
package cluster

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/relay"
)

type Ownership struct {
	roomID  string
	broker  *relay.Broker
	leases  *relay.LeaseManager
	mu      sync.Mutex
	owner   bool
	token   int64
	seq     int64
	started bool
	stopped bool
	sub     *relay.Subscription
}

func NewOwnership(roomID string, broker *relay.Broker, leases *relay.LeaseManager) *Ownership {
	return &Ownership{roomID: roomID, broker: broker, leases: leases}
}

// Promote activates this owner once. A nil broker/lease pair supports an
// isolated ownership state without starting distributed background work.
func (o *Ownership) Promote(ctx context.Context, token int64, newly bool, inbound func(relay.Inbound)) {
	o.mu.Lock()
	o.owner = true
	if newly {
		o.token = token
	}
	start := !o.started
	o.started = true
	o.mu.Unlock()
	if !start || o.broker == nil || o.leases == nil {
		return
	}
	sub := o.broker.SubscribeInbound(ctx, o.roomID, inbound)
	o.mu.Lock()
	if !o.owner {
		o.mu.Unlock()
		_ = sub.Close()
		return
	}
	o.sub = sub
	o.mu.Unlock()
	interval := o.leases.TTL() / 3
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				o.mu.Lock()
				active := o.owner && !o.stopped
				o.mu.Unlock()
				if !active {
					return
				}
				renewCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := o.leases.Renew(renewCtx, o.roomID)
				cancel()
				if err != nil {
					o.Demote()
					log.Printf("relay lost ownership of room %s: %v", o.roomID, err)
					return
				}
			}
		}
	}()
}

func (o *Ownership) IsOwner() bool {
	if o == nil {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.owner
}

func (o *Ownership) Matches(token int64) bool {
	if o == nil {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.owner && o.token == token
}

func (o *Ownership) Demote() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.owner = false
	sub := o.sub
	o.sub = nil
	o.started = false
	o.mu.Unlock()
	if sub != nil {
		_ = sub.Close()
	}
}

// Stop disables lease renewal and outbound delivery for this room handle.
func (o *Ownership) Stop() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.stopped = true
	o.mu.Unlock()
	o.Demote()
}

func (o *Ownership) Publish(target relay.Target, payload map[string]any) {
	if o == nil {
		return
	}
	o.mu.Lock()
	if !o.owner {
		o.mu.Unlock()
		return
	}
	o.seq++
	envelope := relay.Envelope{Seq: o.seq, Target: target, Payload: payload}
	o.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := o.broker.PublishOutbound(ctx, o.roomID, envelope); err != nil {
		log.Printf("relay publish outbound room %s: %v", o.roomID, err)
	}
}

func (o *Ownership) PublishToPlayer(sub string, payload map[string]any) {
	o.Publish(relay.Target{Kind: relay.TargetSub, Sub: sub}, payload)
}

func (o *Ownership) PublishToSpectator(id string, payload map[string]any) {
	o.Publish(relay.Target{Kind: relay.TargetSpectator, Sub: id}, payload)
}

func (o *Ownership) PublishToSpectators(payload map[string]any) {
	o.Publish(relay.Target{Kind: relay.TargetSpectators}, payload)
}
