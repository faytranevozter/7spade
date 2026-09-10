package cluster

import "github.com/faytranevozter/7spade/services/ws/relay"

// Inbound is the cluster command envelope delivered to an owning replica from
// an edge. Runtime code receives the command, not the Redis relay dependency.
type Inbound = relay.Inbound

type InboundHandlers struct {
	Join           func(Inbound)
	Leave          func(Inbound)
	Data           func(Inbound)
	SpectatorJoin  func(Inbound)
	SpectatorLeave func(Inbound)
	SpectatorData  func(Inbound)
}

// Dispatcher owns relay command routing and checks authority immediately before
// each runtime callback. This keeps relay protocol branching out of room code.
type Dispatcher struct {
	owned func() bool
	h     InboundHandlers
}

func NewDispatcher(owned func() bool, handlers InboundHandlers) Dispatcher {
	return Dispatcher{owned: owned, h: handlers}
}

func (d Dispatcher) Handle(in Inbound) {
	if d.owned != nil && !d.owned() {
		return
	}
	switch in.Kind {
	case relay.InboundJoin:
		if d.h.Join != nil {
			d.h.Join(in)
		}
	case relay.InboundLeave:
		if d.h.Leave != nil {
			d.h.Leave(in)
		}
	case relay.InboundData:
		if d.h.Data != nil {
			d.h.Data(in)
		}
	case relay.InboundSpectatorJoin:
		if d.h.SpectatorJoin != nil {
			d.h.SpectatorJoin(in)
		}
	case relay.InboundSpectatorLeave:
		if d.h.SpectatorLeave != nil {
			d.h.SpectatorLeave(in)
		}
	case relay.InboundSpectatorData:
		if d.h.SpectatorData != nil {
			d.h.SpectatorData(in)
		}
	}
}
