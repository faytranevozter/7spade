package cluster

import (
	"testing"

	"github.com/faytranevozter/7spade/services/ws/relay"
)

func TestDispatcherRoutesEveryInboundKind(t *testing.T) {
	var got []relay.InboundKind
	dispatcher := NewDispatcher(func() bool { return true }, InboundHandlers{
		Join: func(in Inbound) { got = append(got, in.Kind) }, Leave: func(in Inbound) { got = append(got, in.Kind) },
		Data: func(in Inbound) { got = append(got, in.Kind) }, SpectatorJoin: func(in Inbound) { got = append(got, in.Kind) },
		SpectatorLeave: func(in Inbound) { got = append(got, in.Kind) }, SpectatorData: func(in Inbound) { got = append(got, in.Kind) },
	})
	want := []relay.InboundKind{relay.InboundJoin, relay.InboundLeave, relay.InboundData, relay.InboundSpectatorJoin, relay.InboundSpectatorLeave, relay.InboundSpectatorData}
	for _, kind := range want {
		dispatcher.Handle(Inbound{Kind: kind})
	}
	if len(got) != len(want) {
		t.Fatalf("routed %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDispatcherRejectsCommandsAfterOwnershipLoss(t *testing.T) {
	called := false
	dispatcher := NewDispatcher(func() bool { return false }, InboundHandlers{Data: func(Inbound) { called = true }})
	dispatcher.Handle(Inbound{Kind: relay.InboundData})
	if called {
		t.Fatal("demoted owner accepted an inbound command")
	}
}
