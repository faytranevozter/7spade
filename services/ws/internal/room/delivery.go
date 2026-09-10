package room

// deliverToSeat sends a per-seat payload to one player. player.send uses
// session.Runtime for local delivery and a sub-targeted Relay publication for an
// edge-held player.
func (room *room) deliverToSeat(p *player, payload map[string]any) {
	if p == nil {
		return
	}
	p.send(payload)
}

// deliverToSpectators sends through session.Runtime for local spectators and
// publishes a spectators-targeted Relay envelope for remote edges.
func (room *room) deliverToSpectators(spectators []*spectator, payload map[string]any) {
	for _, s := range spectators {
		s.send(payload)
	}
	room.publishToSpectators(payload)
}

// publishToPlayer publishes a sub-targeted outbound payload through the Relay
// capability. It is a no-op in single-replica mode.
func (room *room) publishToPlayer(sub string, payload map[string]any) {
	if room.relay != nil {
		room.relay.PublishToPlayer(sub, payload)
	}
}

func (room *room) publishToSpectator(id string, payload map[string]any) {
	if room.relay != nil {
		room.relay.PublishToSpectator(id, payload)
	}
}

func (room *room) publishToSpectators(payload map[string]any) {
	if room.relay != nil {
		room.relay.PublishToSpectators(payload)
	}
}

// isOwnerOrSolo reports whether this replica is responsible for authoritative
// side effects on the room: true in single-process mode (no relay), and true on
// the owning replica when the relay is active. A demoted owner (lost lease)
// returns false so it stops running timers / bot auto-play / result saves —
// fencing those effects to the live owner.
func (room *room) isOwnerOrSolo() bool { return room.relay == nil || room.relay.IsOwner() }
