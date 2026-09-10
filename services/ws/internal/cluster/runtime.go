package cluster

import (
	"context"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/relay"
)

// Runtime owns process-wide relay lifecycle. Game runtimes use its semantic
// operations rather than carrying Redis relay primitives themselves.
type Runtime struct {
	id          string
	broker      *relay.Broker
	leases      *relay.LeaseManager
	registry    *relay.Registry
	coordinator *relay.Coordinator
	ctx         context.Context
	cancel      context.CancelFunc
}

func (r *Runtime) NewEdge(sessions session.Connections, ping, pong time.Duration, access session.AccessChecker, presence func(*session.Claims, string) func()) *Edge {
	return NewEdge(r.Context(), r.ID(), r.broker, r.Registry(), sessions, ping, pong, access, presence)
}

func NewRuntime(id string, broker *relay.Broker, leases *relay.LeaseManager, coordinator *relay.Coordinator) *Runtime {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runtime{id: id, broker: broker, leases: leases, registry: relay.NewRegistry(), coordinator: coordinator, ctx: ctx, cancel: cancel}
}

func (r *Runtime) Enabled() bool { return r != nil && r.broker != nil && r.leases != nil }
func (r *Runtime) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}
func (r *Runtime) Context() context.Context {
	if r == nil {
		return context.Background()
	}
	return r.ctx
}
func (r *Runtime) Stop() {
	if r != nil && r.cancel != nil {
		r.cancel()
	}
}
func (r *Runtime) NewOwnership(roomID string) *Ownership {
	if !r.Enabled() {
		return nil
	}
	return NewOwnership(roomID, r.broker, r.leases)
}
func (r *Runtime) Registry() *relay.Registry {
	if r == nil {
		return nil
	}
	return r.registry
}

func (r *Runtime) Acquire(roomID string) (bool, int64, bool) {
	if !r.Enabled() {
		return true, 0, false
	}
	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()
	acquired, token, owner, err := r.leases.Acquire(ctx, roomID)
	if err != nil {
		return false, 0, false
	}
	if acquired {
		return true, token, true
	}
	return owner == r.id, 0, false
}

func (r *Runtime) IsLocalOwner(roomID string) bool {
	if !r.Enabled() {
		return true
	}
	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()
	owner, err := r.leases.Owner(ctx, roomID)
	return err == nil && owner == r.id
}

func (r *Runtime) Inspect(ctx context.Context, roomID string) (string, int64, error) {
	if r == nil || r.leases == nil {
		return r.ID(), 0, nil
	}
	return r.leases.Inspect(ctx, roomID)
}

func (r *Runtime) Coordinator() *relay.Coordinator {
	if r == nil {
		return nil
	}
	return r.coordinator
}

func (r *Runtime) Broker() *relay.Broker {
	if r == nil {
		return nil
	}
	return r.broker
}
func (r *Runtime) Leases() *relay.LeaseManager {
	if r == nil {
		return nil
	}
	return r.leases
}
