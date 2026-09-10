package room

import (
	"sync"
	"testing"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

type capturedDelivery struct {
	mu      sync.Mutex
	sent    []session.ID
	closed  []session.ID
	removed []session.ID
}

func (d *capturedDelivery) Send(id session.ID, _ map[string]any) error {
	d.mu.Lock()
	d.sent = append(d.sent, id)
	d.mu.Unlock()
	return nil
}

func (d *capturedDelivery) Close(id session.ID) {
	d.mu.Lock()
	d.closed = append(d.closed, id)
	d.mu.Unlock()
}

func (d *capturedDelivery) Remove(id session.ID) {
	d.mu.Lock()
	d.removed = append(d.removed, id)
	d.mu.Unlock()
}

func (*capturedDelivery) Run(session.ID, session.Loop) {}

func TestPlayerDeliveryUsesReplacementSessionID(t *testing.T) {
	delivery := &capturedDelivery{}
	r := &room{sessions: delivery}
	p := &player{room: r, sessionID: "original"}

	p.mu.Lock()
	p.sessionID = "replacement"
	p.mu.Unlock()
	p.send(map[string]any{"type": "state_update"})

	if len(delivery.sent) != 1 || delivery.sent[0] != "replacement" {
		t.Fatalf("delivered sessions = %v, want [replacement]", delivery.sent)
	}
}

func TestStaleDisconnectCannotDisconnectReplacementSession(t *testing.T) {
	r := &room{phase: phaseLobby}
	p := &player{room: r, sessionID: "replacement"}
	r.players = []*player{p}

	r.handleDisconnect(p, "original")

	if p.disconnected {
		t.Fatal("stale session disconnected the replacement session")
	}
}
