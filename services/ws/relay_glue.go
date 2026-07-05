package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/relay"
)

// newRoomLocked constructs a room with the server's shared dependencies and,
// when the relay is enabled, attaches a roomRelay so the room participates in
// cross-replica coordination. Caller holds server.mu.
func (server *GameServer) newRoomLocked(roomID string, botDifficulty game.BotDifficulty, practiceMode bool, turnTimerDuration time.Duration, gameConfig game.GameConfig) *room {
	r := &room{
		id:                roomID,
		botDifficulty:     botDifficulty,
		practiceMode:      practiceMode,
		gameConfig:        gameConfig,
		store:             server.store,
		gameHistory:       server.gameHistory,
		statusUpdater:     server.statusUpdater,
		memberRemover:     server.memberRemover,
		turnTimerDuration: turnTimerDuration,
		lobbyLeaveGrace:   server.lobbyLeaveGrace,
		rematchWindow:     server.rematchWindow,
		rematchVotes:      map[int]bool{},
		phase:             phaseLobby,
	}
	if server.relayEnabled() {
		r.relay = &roomRelay{
			broker:   server.broker,
			registry: server.registry,
			leases:   server.leases,
		}
	}
	return r
}

// acquireOwnership decides this replica's role for a room. It returns owned=true
// when this replica is (or just became) the owner, with newly=true only on a
// fresh acquisition (so the caller promotes/starts the consumer exactly once).
// owned=false means another replica owns the room and the caller must take the
// edge path. A no-op owner (single-process) when the relay is disabled.
func (server *GameServer) acquireOwnership(roomID string) (owned bool, token int64, newly bool) {
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
// locally rather than proxying to a remote owner. True when this replica already
// owns the room (so it has the authoritative live state), or when the room is
// currently unowned (no replica has claimed it — the local handler will rehydrate
// from the snapshot, matching the pre-relay behaviour). A non-empty owner that
// isn't us means the spectator must be an edge so it sees the owner's live
// envelopes.
func (server *GameServer) ownsOrCanServeSpectator(roomID string) bool {
	if !server.relayEnabled() {
		return true
	}
	ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
	defer cancel()
	owner, err := server.leases.Owner(ctx, roomID)
	if err != nil {
		// On a lookup error, fall back to the local handler: it can still serve
		// the last snapshot, which is strictly better than refusing the viewer.
		log.Printf("relay spectator owner lookup room %s: %v", roomID, err)
		return true
	}
	return owner == "" || owner == server.replicaID
}

// promoteToOwner marks a freshly-created/owned room as this replica's, recording
// the fencing token and starting the owner inbound consumer + lease heartbeat
// exactly once. Safe to call on every owner join; only the first starts the
// background goroutines.
func (server *GameServer) promoteToOwner(gameRoom *room, token int64, newly bool) {
	if gameRoom == nil || gameRoom.relay == nil {
		return
	}
	rr := gameRoom.relay
	rr.mu.Lock()
	rr.owner = true
	if newly {
		rr.token = token
	}
	start := !rr.consumerStarted
	rr.consumerStarted = true
	rr.mu.Unlock()
	if start {
		server.startOwnerConsumer(gameRoom)
		server.startLeaseHeartbeat(gameRoom)
	}
}

// startOwnerConsumer subscribes to the room's inbound channel and applies every
// edge-forwarded message on the owner. All inbound funnels through this single
// subscriber so the engine still sees one total order of moves.
func (server *GameServer) startOwnerConsumer(gameRoom *room) {
	rr := gameRoom.relay
	sub := server.broker.SubscribeInbound(server.relayCtx, gameRoom.id, func(in relay.Inbound) {
		server.handleInbound(gameRoom, in)
	})
	rr.mu.Lock()
	rr.inboundSub = sub
	rr.mu.Unlock()
}

// startLeaseHeartbeat renews the owner lease until the room is torn down or this
// replica loses ownership. Losing the lease (ErrNotOwner) demotes the room so
// it stops publishing/persisting (fencing).
//
// KNOWN LIMITATION (issue #63 follow-up): there is no per-room teardown path
// yet. Rooms are never removed from server.rooms (pre-existing), so an owner's
// heartbeat goroutine + inbound subscription persist for the process lifetime
// and the Redis lease keeps being renewed even after a room empties. The relay
// context (server.relayCtx) bounds all of these to the process lifetime — they
// are cancelled on shutdown — but a long-running replica that churns through
// many rooms accumulates them. A room-teardown path (Release lease, close
// inboundSub, stop heartbeat, drop from server.rooms) is the planned fix.
func (server *GameServer) startLeaseHeartbeat(gameRoom *room) {
	rr := gameRoom.relay
	interval := server.leases.TTL() / 3
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-server.relayCtx.Done():
				return
			case <-ticker.C:
				rr.mu.Lock()
				owner := rr.owner
				stopped := rr.stopped
				rr.mu.Unlock()
				if stopped {
					return
				}
				if !owner {
					return
				}
				ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
				err := server.leases.Renew(ctx, gameRoom.id)
				cancel()
				if err != nil {
					// Lost the lease: demote and stop consuming inbound so a
					// demoted/partitioned replica can't keep applying moves (and
					// clobbering the shared snapshot) after another replica has
					// taken over. The new owner is authoritative from here.
					server.demoteOwner(gameRoom)
					log.Printf("relay lost ownership of room %s: %v", gameRoom.id, err)
					return
				}
			}
		}
	}()
}

// demoteOwner relinquishes this replica's owner role for a room: it clears the
// owner flag and tears down the inbound subscriber so no further edge-forwarded
// frames are applied here. Idempotent.
func (server *GameServer) demoteOwner(gameRoom *room) {
	rr := gameRoom.relay
	if rr == nil {
		return
	}
	rr.mu.Lock()
	rr.owner = false
	sub := rr.inboundSub
	rr.inboundSub = nil
	rr.consumerStarted = false
	rr.mu.Unlock()
	if sub != nil {
		_ = sub.Close()
	}
}

// handleInbound applies one edge-forwarded message on the owner replica. Join
// and leave are control messages; data is a gameplay client frame attributed to
// the originating player's seat. Gated on current ownership: a Redis pub/sub
// message fans out to every subscriber, so a demoted owner whose subscription
// hasn't been torn down yet must not apply anything (defense-in-depth alongside
// demoteOwner closing the subscription).
func (server *GameServer) handleInbound(gameRoom *room, in relay.Inbound) {
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
func (server *GameServer) handleRemoteJoin(gameRoom *room, in relay.Inbound) {
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
func (server *GameServer) handleRemoteLeave(gameRoom *room, in relay.Inbound) {
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
	gameRoom.handleDisconnect(target, nil)
}

// handleRemoteData applies a gameplay frame from an edge-held player.
func (server *GameServer) handleRemoteData(gameRoom *room, in relay.Inbound) {
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
	gameRoom.handleMessage(target, msg)
}

// handleRemoteSpectatorJoin registers a spectator whose socket lives on an edge
// replica. The owner adds a "remote" spectator (conn == nil) so it counts toward
// the spectator total and gets per-viewer emote rate-limiting; delivery happens
// via the relay (the registry routes TargetSpectators / TargetSpectator
// envelopes to the edge socket). It replies to that one viewer with the initial
// redacted snapshot. Mirrors handleSpectator's local seating, minus the socket.
func (server *GameServer) handleRemoteSpectatorJoin(gameRoom *room, in relay.Inbound) {
	if in.SpectatorID == "" {
		return
	}
	s := &spectator{sub: in.Sub, id: in.SpectatorID}

	gameRoom.mu.Lock()
	if gameRoom.phase != phasePlaying {
		gameRoom.mu.Unlock()
		gameRoom.publishEnvelope(relay.Target{Kind: relay.TargetSpectator, Sub: in.SpectatorID}, errorMessage("game has not started"))
		return
	}
	gameRoom.spectators = append(gameRoom.spectators, s)
	gameOver := game.IsGameOver(gameRoom.state)
	var snapshot map[string]any
	if gameOver {
		snapshot = gameRoom.gameOverMessageLocked()
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
func (server *GameServer) handleRemoteSpectatorLeave(gameRoom *room, in relay.Inbound) {
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
func (server *GameServer) handleRemoteSpectatorData(gameRoom *room, in relay.Inbound) {
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
	return room.seatLocked(claims, nil)
}

// afterJoin emits the post-join message for a freshly seated player and any
// roster broadcast, mirroring handleWebSocket's switch but usable for both local
// and remote (relay) joins.
func (server *GameServer) afterJoin(gameRoom *room, player *player, result joinResult) {
	switch result {
	case joinResultLobbyJoined, joinResultLobbyReconnected:
		gameRoom.broadcastLobbyState()
	case joinResultGameReconnected:
		player.send(gameRoom.stateMessageFor(player.index))
	case joinResultGameOver:
		player.send(gameRoom.gameOverMessage())
	}
}

// ownsRoomForTest reports whether this replica currently owns the room. Used by
// integration tests to assert single-ownership.
func (server *GameServer) ownsRoomForTest(roomID string) bool {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil || gameRoom.relay == nil {
		return false
	}
	gameRoom.relay.mu.Lock()
	defer gameRoom.relay.mu.Unlock()
	return gameRoom.relay.owner
}

// demoteRoomForTest simulates this replica losing ownership of a room (e.g. a
// crash or lease loss) without tearing down the process, so a failover test can
// let another replica claim the room. Test-only.
func (server *GameServer) demoteRoomForTest(roomID string) {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil || gameRoom.relay == nil {
		return
	}
	gameRoom.relay.mu.Lock()
	gameRoom.relay.owner = false
	gameRoom.relay.stopped = true
	gameRoom.relay.mu.Unlock()
}

// startPresenceForUser marks a registered user online from an edge replica that
// holds no authoritative room object. The room id is reported empty (same as
// lobby presence) since the edge can't observe the owner's phase. Returns a stop
// func that ends the heartbeat.
func (server *GameServer) startPresenceForUser(claims *tokenClaims, roomID string) func() {
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

// edgePlayerConn adapts a player's websocket to relay.Conn for the edge
// registry. The edge holds the live socket; the owner publishes envelopes that
// the registry delivers here. acked flips true on the first delivered payload,
// which the join loop uses to know the owner has seated the player.
type edgePlayerConn struct {
	server *GameServer
	conn   *websocket.Conn
	mu     *sync.Mutex
	acked  *atomicBool
}

func (e edgePlayerConn) Send(payload map[string]any) {
	if e.acked != nil {
		e.acked.Store(true)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.conn.WriteJSON(payload); err != nil {
		log.Printf("edge write: %v", err)
	}
}

// atomicBool is a tiny lock-free flag (avoids pulling sync/atomic.Bool naming
// churn across the file).
type atomicBool struct{ v int32 }

func (a *atomicBool) Store(b bool) {
	var n int32
	if b {
		n = 1
	}
	atomic.StoreInt32(&a.v, n)
}
func (a *atomicBool) Load() bool { return atomic.LoadInt32(&a.v) == 1 }

// handleEdgePlayer serves a player socket whose room is owned by another
// replica. The edge registers the socket so owner-published envelopes reach it,
// forwards a join control message to the owner, then proxies every client frame
// to the owner over the inbound channel until the socket closes.
func (server *GameServer) handleEdgePlayer(roomID string, claims *tokenClaims, conn *websocket.Conn, token string) {
	var writeMu sync.Mutex
	acked := &atomicBool{}
	server.registry.AddPlayer(roomID, claims.Sub, edgePlayerConn{server: server, conn: conn, mu: &writeMu, acked: acked})

	// One outbound subscription per room per replica (ref-counted), shared by
	// every edge socket of that room here. Subscribing per-socket would deliver
	// each envelope through the shared registry once per socket — duplicating
	// every message. The registry's target matching still routes each envelope
	// to the right local socket(s). Subscribe BEFORE publishing the join so the
	// owner's join reply can't be missed once the join is processed.
	server.subscribeRoomOutbound(roomID)
	defer server.unsubscribeRoomOutbound(roomID)

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		log.Printf("edge marshal claims: %v", err)
		_ = conn.Close()
		server.registry.RemovePlayer(roomID, claims.Sub)
		return
	}
	publishJoin := func() {
		ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		defer cancel()
		if err := server.broker.PublishInbound(ctx, roomID, relay.Inbound{Kind: relay.InboundJoin, Sub: claims.Sub, EdgeID: server.replicaID, Payload: claimsJSON}); err != nil {
			log.Printf("edge publish join: %v", err)
		}
	}
	publishJoin()

	// Retry the join until the owner replies (acked) or we give up. The owner's
	// inbound subscriber may not be live yet on a brand-new room (it starts after
	// the owner finishes joinRoom), and pub/sub has no buffering, so a single
	// join publish can be dropped. Re-publishing until the first delivered
	// payload closes that race without requiring an explicit ack protocol.
	joinDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(10 * time.Second)
		for {
			if acked.Load() {
				return
			}
			select {
			case <-joinDone:
				return
			case <-server.relayCtx.Done():
				return
			case <-deadline:
				return
			case <-ticker.C:
				if !acked.Load() {
					publishJoin()
				}
			}
		}
	}()

	// Presence on the edge: the user is connected to this replica.
	stop := server.startPresenceForUser(claims, roomID)
	defer stop()

	defer func() {
		close(joinDone)
		server.registry.RemovePlayer(roomID, claims.Sub)
		lctx, lcancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		if err := server.broker.PublishInbound(lctx, roomID, relay.Inbound{Kind: relay.InboundLeave, Sub: claims.Sub, EdgeID: server.replicaID}); err != nil {
			log.Printf("edge publish leave: %v", err)
		}
		lcancel()
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		fctx, fcancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		if err := server.broker.PublishInbound(fctx, roomID, relay.Inbound{Kind: relay.InboundData, Sub: claims.Sub, EdgeID: server.replicaID, Payload: payload}); err != nil {
			log.Printf("edge forward data: %v", err)
		}
		fcancel()
	}
}

// subscribeRoomOutbound ensures exactly one outbound subscription exists for the
// room on this replica, ref-counted across all edge sockets of that room. The
// single subscriber delivers each envelope through the shared registry, which
// routes it to the matching local sockets.
func (server *GameServer) subscribeRoomOutbound(roomID string) {
	server.edgeMu.Lock()
	defer server.edgeMu.Unlock()
	if server.edgeSubs == nil {
		server.edgeSubs = map[string]*edgeSub{}
	}
	if es := server.edgeSubs[roomID]; es != nil {
		es.refs++
		return
	}
	ctx, cancel := context.WithCancel(server.relayCtx)
	sub := server.broker.SubscribeOutbound(ctx, roomID, func(env relay.Envelope) {
		server.registry.Deliver(roomID, env)
	})
	server.edgeSubs[roomID] = &edgeSub{sub: sub, cancel: cancel, refs: 1}
}

// unsubscribeRoomOutbound drops one ref on the room's outbound subscription and
// tears it down when the last edge socket of the room on this replica leaves.
func (server *GameServer) unsubscribeRoomOutbound(roomID string) {
	server.edgeMu.Lock()
	defer server.edgeMu.Unlock()
	es := server.edgeSubs[roomID]
	if es == nil {
		return
	}
	es.refs--
	if es.refs > 0 {
		return
	}
	es.cancel()
	_ = es.sub.Close()
	delete(server.edgeSubs, roomID)
}

// edgeSpectatorConn adapts a spectator's websocket to relay.Conn so the edge
// registry can fan owner-published envelopes (state updates, spectator emotes,
// the initial snapshot) out to this local socket.
type edgeSpectatorConn struct {
	conn *websocket.Conn
	mu   *sync.Mutex
}

func (e edgeSpectatorConn) Send(payload map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.conn.WriteJSON(payload); err != nil {
		log.Printf("edge spectator write: %v", err)
	}
}

// handleEdgeSpectator serves a spectator socket whose room is owned by another
// replica. The edge registers the socket (by a process-unique spectator id) so
// the owner's TargetSpectators / TargetSpectator envelopes reach it, asks the
// owner to register the viewer (which replies with the initial snapshot), then
// proxies the spectator's emote frames to the owner until the socket closes.
// Mirrors handleEdgePlayer, but spectators are read-only with respect to the
// game so there is no join-ack retry: the snapshot is best-effort and live
// envelopes follow.
func (server *GameServer) handleEdgeSpectator(roomID string, claims *tokenClaims, conn *websocket.Conn) {
	spectatorID := server.nextSpectatorID()
	var writeMu sync.Mutex
	server.registry.AddSpectator(roomID, spectatorID, edgeSpectatorConn{conn: conn, mu: &writeMu})

	// Subscribe BEFORE publishing the join so the owner's snapshot reply can't
	// be missed. Ref-counted and shared with any edge players of the room here.
	server.subscribeRoomOutbound(roomID)
	defer server.unsubscribeRoomOutbound(roomID)

	publishJoin := func() {
		ctx, cancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		defer cancel()
		if err := server.broker.PublishInbound(ctx, roomID, relay.Inbound{
			Kind:        relay.InboundSpectatorJoin,
			Sub:         claims.Sub,
			SpectatorID: spectatorID,
			EdgeID:      server.replicaID,
		}); err != nil {
			log.Printf("edge spectator publish join: %v", err)
		}
	}
	// Publish the join request once. The room already has an active owner (it was
	// started by seated players), so the owner's inbound subscriber is already
	// running — unlike handleEdgePlayer's race on a brand-new room where the
	// subscriber may not exist yet. A single publish is sufficient here; the
	// snapshot is best-effort and the spectator immediately receives any live
	// envelopes that follow. Each publish is idempotent on the owner
	// (re-registering the same spectator id is a no-op append guarded by removal
	// on leave).
	publishJoin()

	// Presence on the edge: the user is watching from this replica.
	stop := server.startPresenceForUser(claims, roomID)
	defer stop()

	defer func() {
		server.registry.RemoveSpectator(roomID, spectatorID)
		lctx, lcancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		if err := server.broker.PublishInbound(lctx, roomID, relay.Inbound{
			Kind:        relay.InboundSpectatorLeave,
			Sub:         claims.Sub,
			SpectatorID: spectatorID,
			EdgeID:      server.replicaID,
		}); err != nil {
			log.Printf("edge spectator publish leave: %v", err)
		}
		lcancel()
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		fctx, fcancel := context.WithTimeout(server.relayCtx, 2*time.Second)
		if err := server.broker.PublishInbound(fctx, roomID, relay.Inbound{
			Kind:        relay.InboundSpectatorData,
			Sub:         claims.Sub,
			SpectatorID: spectatorID,
			EdgeID:      server.replicaID,
			Payload:     payload,
		}); err != nil {
			log.Printf("edge spectator forward data: %v", err)
		}
		fcancel()
	}
}
