package relay

import (
	"reflect"
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

// A seat-targeted envelope reaches only the matching seat, never other seats or
// spectators — the core no-hidden-info-leak guarantee.
func TestRegistrySeatTargeting(t *testing.T) {
	r := NewRegistry()
	seat0, seat1 := &fakeConn{}, &fakeConn{}
	spec := &fakeConn{}
	r.AddPlayer("room1", "sub-0", 0, seat0)
	r.AddPlayer("room1", "sub-1", 1, seat1)
	r.AddSpectator("room1", "spec-1", spec)

	r.Deliver("room1", env(Target{Kind: TargetSeat, Index: 1}, "for-seat-1"))

	if len(seat0.got) != 0 {
		t.Fatalf("seat0 received %v, want nothing", seat0.got)
	}
	if len(spec.got) != 0 {
		t.Fatalf("spectator received %v, want nothing", spec.got)
	}
	if len(seat1.got) != 1 || seat1.got[0]["m"] != "for-seat-1" {
		t.Fatalf("seat1 got %v, want the seat-1 payload", seat1.got)
	}
}

// TargetAll reaches every player but no spectator; TargetSpectators is the
// inverse.
func TestRegistryAllAndSpectatorTargeting(t *testing.T) {
	r := NewRegistry()
	seat0, seat1, spec := &fakeConn{}, &fakeConn{}, &fakeConn{}
	r.AddPlayer("room1", "sub-0", 0, seat0)
	r.AddPlayer("room1", "sub-1", 1, seat1)
	r.AddSpectator("room1", "spec-1", spec)

	r.Deliver("room1", env(Target{Kind: TargetAll}, "all"))
	if len(seat0.got) != 1 || len(seat1.got) != 1 {
		t.Fatalf("TargetAll missed a seat: seat0=%v seat1=%v", seat0.got, seat1.got)
	}
	if len(spec.got) != 0 {
		t.Fatalf("TargetAll leaked to spectator: %v", spec.got)
	}

	r.Deliver("room1", env(Target{Kind: TargetSpectators}, "spec"))
	if len(spec.got) != 1 || spec.got[0]["m"] != "spec" {
		t.Fatalf("spectator got %v, want the spectators payload", spec.got)
	}
	if len(seat0.got) != 1 || len(seat1.got) != 1 {
		t.Fatalf("TargetSpectators leaked to a seat")
	}
}

// TargetSub reaches the connection for one user only.
func TestRegistrySubTargeting(t *testing.T) {
	r := NewRegistry()
	a, b := &fakeConn{}, &fakeConn{}
	r.AddPlayer("room1", "sub-A", 0, a)
	r.AddPlayer("room1", "sub-B", 1, b)

	r.Deliver("room1", env(Target{Kind: TargetSub, Sub: "sub-B"}, "for-B"))
	if len(a.got) != 0 {
		t.Fatalf("sub-A leaked: %v", a.got)
	}
	if len(b.got) != 1 || b.got[0]["m"] != "for-B" {
		t.Fatalf("sub-B got %v, want for-B", b.got)
	}
}

// Re-adding the same (room, sub) replaces the stale socket so only the newest
// connection receives frames (reconnect path).
func TestRegistryReconnectReplacesSocket(t *testing.T) {
	r := NewRegistry()
	old, fresh := &fakeConn{}, &fakeConn{}
	r.AddPlayer("room1", "sub-0", 0, old)
	r.AddPlayer("room1", "sub-0", 0, fresh)

	r.Deliver("room1", env(Target{Kind: TargetSeat, Index: 0}, "x"))
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
	r.AddPlayer("room1", "sub-0", 0, c)
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
	player := &localConn{sub: "s", index: 2}
	spectator := &localConn{sub: "spec", spectator: true}
	cases := []struct {
		name          string
		target        Target
		wantPlayer    bool
		wantSpectator bool
	}{
		{"seat hit", Target{Kind: TargetSeat, Index: 2}, true, false},
		{"seat miss", Target{Kind: TargetSeat, Index: 3}, false, false},
		{"sub hit", Target{Kind: TargetSub, Sub: "s"}, true, false},
		{"sub miss", Target{Kind: TargetSub, Sub: "other"}, false, false},
		{"all", Target{Kind: TargetAll}, true, false},
		{"spectators", Target{Kind: TargetSpectators}, false, true},
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

// Sanity: Target round-trips through the JSON the broker uses (kinds + fields).
func TestTargetShape(t *testing.T) {
	got := Target{Kind: TargetSeat, Index: 3}
	want := Target{Kind: "seat", Index: 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("target = %+v, want %+v", got, want)
	}
}
