package room

import (
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/internal/transport"

	"github.com/faytranevozter/7spade/services/ws/game"
)

// handleSpectator attaches a spectator to a locally owned or single-replica
// room. Spectating requires an existing in-progress or finished room; it never
// creates a seat. The room sends the initial redacted view, then supplies
// callbacks to the session loop for disconnects and cosmetic emotes. Admission
// routes a remotely owned room to cluster.Edge instead.
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
		// Spectator admission requires the playing phase, including finished games.
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

// runSpectatorSession supplies spectator callbacks to the session runtime. The
// transport loop handles reads, liveness, flood limiting, and shutdown; room
// accepts only cosmetic emotes and removes the spectator on disconnect.
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
