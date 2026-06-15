package relay

import (
	"testing"
)

// fakeConn records the payloads delivered to it.
type fakeConn struct {
	got []map[string]any
}

func (f *fakeConn) Send(payload map[string]any) { f.got = append(f.got, payload) }

func env(target Target, marker string) Envelope {
	return Envelope{Target: target, Payload: map[string]any{"m": marker}}
}

// A sub-targeted envelope reaches only the matching player, never other players
// or spectators — the core no-hidden-info-leak guarantee. Per-seat state is
// routed this way (by sub), so a player only ever sees their own hand.
func TestRegistrySubTargetingIsolatesPlayers(t *testing.T) {
	r := NewRegistry()
	p0, p1 := &fakeConn{}, &fakeConn{}
	spec := &fakeConn{}
	r.AddPlayer("room1", "sub-0", p0)
	r.AddPlayer("room1", "sub-1", p1)
	r.AddSpectator("room1", "spec-1", spec)

	r.Deliver("room1", env(Target{Kind: TargetSub, Sub: "sub-1"}, "for-sub-1"))

	if len(p0.got) != 0 {
		t.Fatalf("sub-0 received %v, want nothing", p0.got)
	}
	if len(spec.got) != 0 {
		t.Fatalf("spectator received %v, want nothing", spec.got)
	}
	if len(p1.got) != 1 || p1.got[0]["m"] != "for-sub-1" {
		t.Fatalf("sub-1 got %v, want the sub-1 payload", p1.got)
	}
}

// TargetAll reaches every player but no spectator; TargetSpectators is the
// inverse.
func TestRegistryAllAndSpectatorTargeting(t *testing.T) {
	r := NewRegistry()
	p0, p1, spec := &fakeConn{}, &fakeConn{}, &fakeConn{}
	r.AddPlayer("room1", "sub-0", p0)
	r.AddPlayer("room1", "sub-1", p1)
	r.AddSpectator("room1", "spec-1", spec)

	r.Deliver("room1", env(Target{Kind: TargetAll}, "all"))
	if len(p0.got) != 1 || len(p1.got) != 1 {
		t.Fatalf("TargetAll missed a player: p0=%v p1=%v", p0.got, p1.got)
	}
	if len(spec.got) != 0 {
		t.Fatalf("TargetAll leaked to spectator: %v", spec.got)
	}

	r.Deliver("room1", env(Target{Kind: TargetSpectators}, "spec"))
	if len(spec.got) != 1 || spec.got[0]["m"] != "spec" {
		t.Fatalf("spectator got %v, want the spectators payload", spec.got)
	}
	if len(p0.got) != 1 || len(p1.got) != 1 {
		t.Fatalf("TargetSpectators leaked to a player")
	}
}

// TargetSpectator reaches exactly one spectator by id — used to send a freshly
// joined remote spectator its initial snapshot without spamming every viewer.
func TestRegistrySingleSpectatorTargeting(t *testing.T) {
	r := NewRegistry()
	specA, specB, player := &fakeConn{}, &fakeConn{}, &fakeConn{}
	r.AddSpectator("room1", "spec-A", specA)
	r.AddSpectator("room1", "spec-B", specB)
	r.AddPlayer("room1", "sub-0", player)

	r.Deliver("room1", env(Target{Kind: TargetSpectator, Sub: "spec-A"}, "snapshot"))

	if len(specA.got) != 1 || specA.got[0]["m"] != "snapshot" {
		t.Fatalf("spec-A got %v, want the snapshot payload", specA.got)
	}
	if len(specB.got) != 0 {
		t.Fatalf("spec-B received %v, want nothing", specB.got)
	}
	if len(player.got) != 0 {
		t.Fatalf("player received %v, want nothing", player.got)
	}
}

// Re-adding the same (room, sub) replaces the stale socket so only the newest
// connection receives frames (reconnect path).
func TestRegistryReconnectReplacesSocket(t *testing.T) {
	r := NewRegistry()
	old, fresh := &fakeConn{}, &fakeConn{}
	r.AddPlayer("room1", "sub-0", old)
	r.AddPlayer("room1", "sub-0", fresh)

	r.Deliver("room1", env(Target{Kind: TargetSub, Sub: "sub-0"}, "x"))
	if len(old.got) != 0 {
		t.Fatalf("stale socket still received frames: %v", old.got)
	}
	if len(fresh.got) != 1 {
		t.Fatalf("fresh socket got %v, want 1 frame", fresh.got)
	}
}

// Removal drops a socket and cleans up the empty room entry.
func TestRegistryRemoval(t *testing.T) {
	r := NewRegistry()
	c := &fakeConn{}
	r.AddPlayer("room1", "sub-0", c)
	if !r.HasRoom("room1") {
		t.Fatal("expected room present after add")
	}
	r.RemovePlayer("room1", "sub-0")
	if r.HasRoom("room1") {
		t.Fatal("expected room gone after removing its only socket")
	}
	r.Deliver("room1", env(Target{Kind: TargetAll}, "x"))
	if len(c.got) != 0 {
		t.Fatalf("removed socket received frames: %v", c.got)
	}
}

// matches is exhaustive over kinds for both player and spectator entries.
func TestMatchesTable(t *testing.T) {
	player := &localConn{sub: "s"}
	spectator := &localConn{sub: "spec", spectator: true}
	cases := []struct {
		name          string
		target        Target
		wantPlayer    bool
		wantSpectator bool
	}{
		{"sub hit", Target{Kind: TargetSub, Sub: "s"}, true, false},
		{"sub miss", Target{Kind: TargetSub, Sub: "other"}, false, false},
		{"all", Target{Kind: TargetAll}, true, false},
		{"spectators", Target{Kind: TargetSpectators}, false, true},
		{"spectator hit", Target{Kind: TargetSpectator, Sub: "spec"}, false, true},
		{"spectator miss", Target{Kind: TargetSpectator, Sub: "other-spec"}, false, false},
		{"unknown", Target{Kind: "bogus"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matches(tc.target, player); got != tc.wantPlayer {
				t.Fatalf("player match = %v, want %v", got, tc.wantPlayer)
			}
			if got := matches(tc.target, spectator); got != tc.wantSpectator {
				t.Fatalf("spectator match = %v, want %v", got, tc.wantSpectator)
			}
		})
	}
}
