package room

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/cluster"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/relay"
)

// newRoomLocked constructs a room with the server's shared dependencies and,
// when the relay is enabled, attaches a roomRelay so the room participates in
// cross-replica coordination. Caller holds server.mu.
func (server *Manager) newRoomLocked(roomID string, botDifficulty game.BotDifficulty, practiceMode bool, turnTimerDuration time.Duration, gameConfig game.GameConfig) *room {
	r := &room{
		id:                  roomID,
		botDifficulty:       botDifficulty,
		practiceMode:        practiceMode,
		gameConfig:          gameConfig,
		store:               server.store,
		gameHistory:         server.gameHistory,
		statusUpdater:       server.statusUpdater,
		memberRemover:       server.memberRemover,
		turnTimerDuration:   turnTimerDuration,
		lobbyLeaveGrace:     server.lobbyLeaveGrace,
		rematchWindow:       server.rematchWindow,
		wsPingEvery:         server.wsPingEvery,
		wsPongWait:          server.wsPongWait,
		accessChecker:       server.accessChecker,
		delivery:            server.delivery,
		applicationControls: server.applicationControls,
		rematchVotes:        map[int]bool{},
		phase:               phaseLobby,
	}
	// teardown drops the room from the server's in-memory map and releases
	// its relay lease (when owned). Single-process mode fully tears the
	// room down once it is empty; relay mode skips this (teardown there is a
	// separate planned concern) so it doesn't race the lease/fencing.
	r.teardown = func() {
		if server.relayEnabled() {
			return
		}
		server.mu.Lock()
		if server.rooms[roomID] == r {
			delete(server.rooms, roomID)
		}
		server.mu.Unlock()
	}
	if server.relayEnabled() {
		r.relay = cluster.NewOwnership(roomID, server.broker, server.leases)
	}
	return r
}

// acquireOwnership decides this replica's role for a room. It returns owned=true
// when this replica is (or just became) the owner, with newly=true only on a
// fresh acquisition (so the caller promotes/starts the consumer exactly once).
// owned=false means another replica owns the room and the caller must take the
// edge path. A no-op owner (single-process) when the relay is disabled.
func (server *Manager) acquireOwnership(roomID string) (owned bool, token int64, newly bool) {
	if !server.relayEnabled() {
		return true, 0, false
	}
	ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
	defer cancel()
	acquired, token, owner, err := server.leases.Acquire(ctx, roomID)
	if err != nil {
		log.Printf("relay acquire ownership room %s: %v", roomID, err)
		return false, 0, false
	}
	if acquired {
		return true, token, true
	}
	// Lease already held: ours (another local socket already owns it) or remote.
	if owner == server.replicaID {
		return true, 0, false
	}
	return false, 0, false
}

// ownsOrCanServeSpectator reports whether this replica should serve a spectator
// locally rather than proxying to a remote owner. True only when this replica
// already owns the room (so it holds the authoritative live state). A non-empty
// owner that isn't us, OR an as-yet-unowned room, must be served as an
// edge so the spectator proxies to the eventual owner's live envelopes.
//
// Serving an unowned room locally would rehydrate it from the snapshot as a
// fresh solo room (relay == nil), making isOwnerOrSolo() true here while
// another replica may concurrently acquire the lease and also drive the room —
// a split-brain. Routing unowned-room spectators to the edge (which waits for
// the owner's envelopes) avoids that.
func (server *Manager) ownsOrCanServeSpectator(roomID string) bool {
	if !server.relayEnabled() {
		return true
	}
	ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
	defer cancel()
	owner, err := server.leases.Owner(ctx, roomID)
	if err != nil {
		// On a lookup error, refuse local serving and fall back to the edge
		// path rather than risk rehydrating an unowned room as a solo owner.
		log.Printf("relay spectator owner lookup room %s: %v", roomID, err)
		return false
	}
	return owner == server.replicaID
}

func (server *Manager) promoteToOwner(gameRoom *room, token int64, newly bool) {
	if gameRoom == nil || gameRoom.relay == nil {
		return
	}
	gameRoom.relay.Promote(server.relayCtx, token, newly, func(in relay.Inbound) { server.handleInbound(gameRoom, in) })
}

// handleInbound applies one edge-forwarded message on the owner replica. Join
// and leave are control messages; data is a gameplay client frame attributed to
// the originating player's seat. Gated on current ownership: a Redis pub/sub
// message fans out to every subscriber, so a demoted owner whose subscription
// hasn't been torn down yet must not apply anything (defense-in-depth alongside
// demoteOwner closing the subscription).
func (server *Manager) handleInbound(gameRoom *room, in relay.Inbound) {
	if !gameRoom.isOwnerOrSolo() {
		return
	}
	switch in.Kind {
	case relay.InboundJoin:
		server.handleRemoteJoin(gameRoom, in)
	case relay.InboundLeave:
		server.handleRemoteLeave(gameRoom, in)
	case relay.InboundData:
		server.handleRemoteData(gameRoom, in)
	case relay.InboundSpectatorJoin:
		server.handleRemoteSpectatorJoin(gameRoom, in)
	case relay.InboundSpectatorLeave:
		server.handleRemoteSpectatorLeave(gameRoom, in)
	case relay.InboundSpectatorData:
		server.handleRemoteSpectatorData(gameRoom, in)
	}
}

// handleRemoteJoin seats (or reconnects) a player whose socket lives on an edge
// replica. The owner adds a "remote" player (conn == nil) to its roster; all
// sends to that seat are published back to the edge via the relay.
func (server *Manager) handleRemoteJoin(gameRoom *room, in relay.Inbound) {
	var claims tokenClaims
	if len(in.Payload) > 0 {
		if err := json.Unmarshal(in.Payload, &claims); err != nil {
			log.Printf("relay remote join: bad claims: %v", err)
			return
		}
	}
	if claims.Sub == "" {
		return
	}
	_, player, result, err := gameRoom.joinRemote(&claims)
	if err != nil {
		// Reply with a fatal error envelope the edge will forward to the socket,
		// so a rejected remote join (kicked, full, started) routes the user away.
		gameRoom.publishEnvelope(relay.Target{Kind: relay.TargetSub, Sub: claims.Sub}, fatalErrorMessage(err.Error()))
		return
	}
	server.afterJoin(gameRoom, player, result)
}

// handleRemoteLeave marks a remote player's socket as dropped on the owner.
func (server *Manager) handleRemoteLeave(gameRoom *room, in relay.Inbound) {
	gameRoom.mu.Lock()
	var target *player
	for _, p := range gameRoom.players {
		if p.sub == in.Sub {
			target = p
			break
		}
	}
	gameRoom.mu.Unlock()
	if target == nil {
		return
	}
	// A sub can have several live edge sockets (e.g. two tabs). Only mark
	// the seat disconnected when no other live player socket for that sub
	// remains on this replica — otherwise one tab's edge leave would
	// wrongly show the player disconnected to everyone else.
	if server.registry.CountPlayers(gameRoom.id, in.Sub) > 0 {
		return
	}
	gameRoom.handleDisconnect(target, "")
}

// handleRemoteData applies a gameplay frame from an edge-held player.
func (server *Manager) handleRemoteData(gameRoom *room, in relay.Inbound) {
	var msg clientMessage
	if err := json.Unmarshal(in.Payload, &msg); err != nil {
		log.Printf("relay remote data: bad frame: %v", err)
		return
	}
	gameRoom.mu.Lock()
	var target *player
	for _, p := range gameRoom.players {
		if p.sub == in.Sub {
			target = p
			break
		}
	}
	gameRoom.mu.Unlock()
	if target == nil {
		return
	}
	ok, closeConn := target.allowInbound()
	if closeConn {
		target.sendError("connection closed: too many messages")
		return
	}
	if !ok {
		target.sendError("too many messages, slow down")
		return
	}
	gameRoom.handleMessage(target, msg)
}

// handleRemoteSpectatorJoin registers a spectator whose socket lives on an edge
// replica. The owner adds a "remote" spectator (conn == nil) so it counts toward
// the spectator total and gets per-viewer emote rate-limiting; delivery happens
// via the relay (the registry routes TargetSpectators / TargetSpectator
// envelopes to the edge socket). It replies to that one viewer with the initial
// redacted snapshot. Mirrors handleSpectator's local seating, minus the socket.
func (server *Manager) handleRemoteSpectatorJoin(gameRoom *room, in relay.Inbound) {
	if in.SpectatorID == "" {
		return
	}
	if !server.controlEnabled(controlSpectatorAccess) {
		gameRoom.publishEnvelope(relay.Target{Kind: relay.TargetSpectator, Sub: in.SpectatorID}, fatalErrorMessage("spectator access is temporarily unavailable"))
		return
	}
	s := &spectator{sub: in.Sub, id: in.SpectatorID}

	gameRoom.mu.Lock()
	if gameRoom.phase != phasePlaying {
		gameRoom.mu.Unlock()
		gameRoom.publishEnvelope(relay.Target{Kind: relay.TargetSpectator, Sub: in.SpectatorID}, fatalErrorMessage("game has not started"))
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

	// Send the initial snapshot only to the joining spectator.
	gameRoom.publishEnvelope(relay.Target{Kind: relay.TargetSpectator, Sub: in.SpectatorID}, snapshot)
	// Refresh the seated players' spectator count.
	if !gameOver {
		gameRoom.broadcastState()
	}
}

// handleRemoteSpectatorLeave drops a remote spectator from the owner's roster
// and refreshes the seated players' spectator count.
func (server *Manager) handleRemoteSpectatorLeave(gameRoom *room, in relay.Inbound) {
	if in.SpectatorID == "" {
		return
	}
	gameRoom.mu.Lock()
	var target *spectator
	for _, s := range gameRoom.spectators {
		if s.id == in.SpectatorID {
			target = s
			break
		}
	}
	gameRoom.mu.Unlock()
	if target == nil {
		return
	}
	gameRoom.removeSpectator(target)
}

// handleRemoteSpectatorData applies a spectator frame (an emote) from an edge.
func (server *Manager) handleRemoteSpectatorData(gameRoom *room, in relay.Inbound) {
	var msg clientMessage
	if err := json.Unmarshal(in.Payload, &msg); err != nil {
		log.Printf("relay remote spectator data: bad frame: %v", err)
		return
	}
	if msg.Type != messageTypeEmote {
		return
	}
	gameRoom.mu.Lock()
	var target *spectator
	for _, s := range gameRoom.spectators {
		if s.id == in.SpectatorID {
			target = s
			break
		}
	}
	gameRoom.mu.Unlock()
	if target == nil {
		return
	}
	gameRoom.handleSpectatorEmote(target, msg.Emote)
}

// joinRemote seats a player whose socket lives on an edge replica (conn == nil).
// Same seating semantics as a local join; the owner reaches the player via the
// relay rather than a local socket.
func (room *room) joinRemote(claims *tokenClaims) (*room, *player, joinResult, error) {
	room.mu.Lock()
	defer room.mu.Unlock()
	return room.seatLocked(claims, "")
}

// afterJoin emits the post-join message for a freshly seated player and any
// roster broadcast, mirroring handleWebSocket's switch but usable for both local
// and remote (relay) joins.
func (server *Manager) afterJoin(gameRoom *room, player *player, result joinResult) {
	switch result {
	case joinResultLobbyJoined, joinResultLobbyReconnected:
		gameRoom.broadcastLobbyState()
	case joinResultGameReconnected:
		player.send(gameRoom.stateMessage(player.index))
	case joinResultGameOver:
		player.send(gameRoom.gameOverMessage())
	}
}

// startPresenceForUser marks a registered user online from an edge replica that
// holds no authoritative room object. The room id is reported empty (same as
// lobby presence) since the edge can't observe the owner's phase. Returns a stop
// func that ends the heartbeat.
func (server *Manager) startPresenceForUser(claims *tokenClaims, roomID string) func() {
	if server.presence == nil || claims.IsGuest || claims.Sub == "" {
		return func() {}
	}
	mark := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.presence.Online(ctx, claims.Sub, ""); err != nil {
			log.Printf("presence: mark online (edge): %v", err)
		}
	}
	mark()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(presenceHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mark()
			}
		}
	}()
	return func() { close(done) }
}
