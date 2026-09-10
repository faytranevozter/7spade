package room

import (
	"context"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

// Cluster and Relay are the room runtime's complete distributed coordination
// contract. Relay and Redis protocol types stay behind the cluster boundary.
type Cluster interface {
	Enabled() bool
	ID() string
	Context() context.Context
	Stop()
	Acquire(roomID string) (owned bool, token int64, newly bool)
	IsLocalOwner(roomID string) bool
	Inspect(ctx context.Context, roomID string) (owner string, token int64, err error)
	NewRelay(roomID string) Relay
	Promote(relay Relay, token int64, newly bool, handle func(Inbound))
	PlayerConnections(roomID, sub string) int
	Coordinator() Coordinator
}

type Relay interface {
	IsOwner() bool
	Matches(token int64) bool
	Stop()
	PublishToPlayer(sub string, payload map[string]any)
	PublishToSpectator(id string, payload map[string]any)
	PublishToSpectators(payload map[string]any)
}

type Edge interface {
	ServePlayer(roomID string, claims *session.Claims, sessionID session.ID, token string)
	ServeSpectator(roomID string, claims *session.Claims, sessionID session.ID, spectatorID string)
}

type Coordinator interface {
	PublishActiveRooms(ctx context.Context, rooms []string, ttl time.Duration) error
	AcquireLeadership(ctx context.Context, ttl time.Duration) (bool, error)
	ActiveRooms(ctx context.Context) ([]string, error)
}

type InboundKind string

const (
	InboundJoin           InboundKind = "join"
	InboundLeave          InboundKind = "leave"
	InboundData           InboundKind = "data"
	InboundSpectatorJoin  InboundKind = "spectator_join"
	InboundSpectatorLeave InboundKind = "spectator_leave"
	InboundSpectatorData  InboundKind = "spectator_data"
)

// Inbound is the room-level command emitted by an edge replica.
type Inbound struct {
	Kind        InboundKind
	Sub         string
	SpectatorID string
	Payload     []byte
}
