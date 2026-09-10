package room

import (
	"time"
)

// handleEmote validates an emote against the allowlist and a per-player
// cooldown, then echoes it to everyone in the room (including the sender, so
// every client renders the bubble from the same broadcast).
func (room *room) handleEmote(player *player, emote string) {
	if !room.controlEnabled(controlEmotes) {
		player.sendError("emotes are temporarily unavailable")
		return
	}
	if !allowedEmotes[emote] {
		player.sendError("unknown emote")
		return
	}

	room.mu.Lock()
	now := time.Now()
	if !player.lastEmoteAt.IsZero() && now.Sub(player.lastEmoteAt) < emoteCooldown {
		// Too soon after the last emote: silently drop to avoid spamming the
		// room (and a flood of error toasts on the sender).
		room.mu.Unlock()
		return
	}
	player.lastEmoteAt = now
	displayName := player.displayName
	room.mu.Unlock()

	room.broadcastEmote(displayName, emote)
}

// broadcastEmote fans an emote out to every connected human in the room,
// including the sender, so all clients render the bubble identically.
// Spectators receive it too so their view stays live.
func (room *room) broadcastEmote(displayName string, emote string) {
	room.mu.Lock()
	message := map[string]any{"type": messageTypeEmote, "display_name": displayName, "emote": emote}
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}

// broadcastSpectatorEmote fans a spectator's emote out to every connected human
// in the room — all seated players and all spectators (including the sender).
// It is tagged with a distinct message type and the spectator's id so clients
// can render spectator reactions separately from player emotes (different
// placement/colour, and players aggregate/throttle them). Spectator emotes are
// purely cosmetic and never touch game state, so this is never persisted.
func (room *room) broadcastSpectatorEmote(spectatorID string, emote string) {
	room.mu.Lock()
	message := map[string]any{"type": messageTypeSpectatorEmote, "spectator_id": spectatorID, "emote": emote}
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}
