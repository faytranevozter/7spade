// Package roombridge adapts cluster's relay runtime to room's coordination API.
package roombridge

import (
	"context"

	"github.com/faytranevozter/7spade/services/ws/internal/cluster"
	"github.com/faytranevozter/7spade/services/ws/internal/room"
)

type Runtime struct{ runtime *cluster.Runtime }

func New(runtime *cluster.Runtime) *Runtime { return &Runtime{runtime: runtime} }

func (c *Runtime) Enabled() bool                         { return c.runtime.Enabled() }
func (c *Runtime) ID() string                            { return c.runtime.ID() }
func (c *Runtime) Context() context.Context              { return c.runtime.Context() }
func (c *Runtime) Stop()                                 { c.runtime.Stop() }
func (c *Runtime) Acquire(id string) (bool, int64, bool) { return c.runtime.Acquire(id) }
func (c *Runtime) IsLocalOwner(id string) bool           { return c.runtime.IsLocalOwner(id) }
func (c *Runtime) Inspect(ctx context.Context, id string) (string, int64, error) {
	return c.runtime.Inspect(ctx, id)
}
func (c *Runtime) NewRelay(id string) room.Relay { return c.runtime.NewOwnership(id) }
func (c *Runtime) PlayerConnections(roomID, sub string) int {
	return c.runtime.Registry().CountPlayers(roomID, sub)
}
func (c *Runtime) Coordinator() room.Coordinator { return c.runtime.Coordinator() }

// Promote is the only place relay inbound envelopes cross into room commands.
func (c *Runtime) Promote(handle room.Relay, token int64, newly bool, callback func(room.Inbound)) {
	ownership, ok := handle.(*cluster.Ownership)
	if !ok {
		return
	}
	dispatcher := cluster.NewDispatcher(ownership.IsOwner, cluster.InboundHandlers{
		Join: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundJoin, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		Leave: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundLeave, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		Data: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundData, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorJoin: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundSpectatorJoin, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorLeave: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundSpectatorLeave, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorData: func(in cluster.Inbound) {
			callback(room.Inbound{Kind: room.InboundSpectatorData, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
	})
	ownership.Promote(c.runtime.Context(), token, newly, dispatcher.Handle)
}
