package room

import (
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/internal/transport"

	"github.com/faytranevozter/7spade/services/ws/game"
)

// handleSpectator attaches a spectator to a room this replica can serve
// authoritatively: either it owns the room, or the room is unowned and this
// replica rehydrates it from the durable store. Spectating requires an existing,
// in-progress (or finished) room — a spectator never creates a room or takes a
// seat. It immediately sends a redacted snapshot (or the game_over results if the
// game is already done), then reads the socket to detect disconnect and to honour
// the spectator's cosmetic emotes (gameplay frames are still ignored).
//
// Multi-replica: when another replica owns the room, the spectator is instead
// served as an edge (handleEdgeSpectator) so it receives the owner's live
// envelopes — state updates and spectator emotes — rather than a stale local
// snapshot. handleWebSocket routes to whichever path applies.
func (server *Manager) handleSpectator(roomID string, claims *tokenClaims, sessionID session.ID) {
	if !server.controlEnabled(controlSpectatorAccess) {
		_ = server.sessions.Send(sessionID, fatalErrorMessage("spectator access is temporarily unavailable"))
		server.sessions.Close(sessionID)
		return
	}
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	if gameRoom == nil && server.store != nil {
		// Rehydrate a finished/in-progress room from the durable store so a
		// spectator can attach after a WS restart.
		if snap, ok := server.store.LoadRoom(roomID); ok {
			gameRoom = server.newRoomLocked(roomID, game.BotMedium, false, server.turnTimerDuration, game.DefaultConfig())
			gameRoom.restoreFromSnapshotLocked(snap)
			server.rooms[roomID] = gameRoom
		}
	}
	server.mu.Unlock()

	if gameRoom == nil {
		if err := server.sessions.Send(sessionID, errorMessage("room not found")); err != nil {
			log.Printf("write spectator room-not-found: %v", err)
		}
		server.sessions.Close(sessionID)
		return
	}

	s := &spectator{sub: claims.Sub, id: server.nextSpectatorID(), sessionID: sessionID, sessions: server.sessions}

	gameRoom.mu.Lock()
	if gameRoom.phase != phasePlaying {
		// v1 only spectates an in-progress or finished game, not the lobby.
		gameRoom.mu.Unlock()
		if err := server.sessions.Send(sessionID, errorMessage("game has not started")); err != nil {
			log.Printf("write spectator not-started: %v", err)
		}
		server.sessions.Close(sessionID)
		return
	}
	gameRoom.spectators = append(gameRoom.spectators, s)
	gameOver := game.IsGameOver(gameRoom.state)
	var snapshot map[string]any
	if gameOver {
		snapshot = gameRoom.gameOverMessageLocked("")
	} else {
		snapshot = gameRoom.spectatorStateMessageLocked()
	}
	gameRoom.mu.Unlock()

	s.send(snapshot)
	// Let seated players see the updated spectator count.
	if !gameOver {
		gameRoom.broadcastState()
	}

	// A spectator is also "online" for presence — they're watching, not seated.
	stop := server.startPresence(claims, gameRoom)
	defer stop()
	gameRoom.runSpectatorSession(s)
}

// spectatorReadLoop reads the spectator socket until it closes (to detect
// disconnect) and dispatches the few inbound messages a spectator is allowed to
// send. Spectators remain read-only with respect to game state: the only
// inbound type honoured is "emote" (a purely cosmetic reaction); every other
// frame is ignored. On exit it removes the spectator and refreshes the seated
// players' spectator count.
func (room *room) runSpectatorSession(s *spectator) {
	if room.sessions == nil {
		return
	}
	room.sessions.Run(s.sessionID, session.Loop{
		PingEvery: room.wsPingEvery, PongWait: room.wsPongWait,
		Closed:  func() { room.removeSpectator(s) },
		Message: func(payload []byte) { room.handleCommand(spectatorCommand(s, payload)) },
		Inbound: &transport.InboundLimiter{},
	})
}

// handleSpectatorEmote validates and rate-limits a spectator's emote, then
// broadcasts it to everyone in the room. It mirrors handleEmote but uses the
// per-spectator cooldown and attributes the emote to the spectator's id rather
// than a seat. Emotes never touch game state.
func (room *room) handleSpectatorEmote(s *spectator, emote string) {
	if !room.controlEnabled(controlEmotes) {
		s.send(errorMessage("emotes are temporarily unavailable"))
		return
	}
	if !allowedEmotes[emote] {
		// Spectators have no error toast surface today, but reply so a
		// misbehaving client still learns the id was rejected.
		s.send(errorMessage("unknown emote"))
		return
	}

	room.mu.Lock()
	now := time.Now()
	if !s.lastEmoteAt.IsZero() && now.Sub(s.lastEmoteAt) < spectatorEmoteCooldown {
		// Too soon after the last emote: silently drop to avoid spamming the
		// room with spectator reactions.
		room.mu.Unlock()
		return
	}
	s.lastEmoteAt = now
	spectatorID := s.id
	room.mu.Unlock()

	room.broadcastSpectatorEmote(spectatorID, emote)
}

// removeSpectator drops a spectator from the room and tells seated players the
// new count (unless the game is already over).
func (room *room) removeSpectator(s *spectator) {
	room.mu.Lock()
	found := false
	filtered := room.spectators[:0]
	for _, existing := range room.spectators {
		if existing == s {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	room.spectators = filtered
	gameOver := game.IsGameOver(room.state)
	room.mu.Unlock()
	if found && !gameOver {
		room.broadcastState()
	}
}
