package cluster

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/internal/transport"
	"github.com/faytranevozter/7spade/services/ws/relay"
)

type Edge struct {
	replicaID               string
	broker                  *relay.Broker
	registry                *relay.Registry
	sessions                session.Connections
	relayCtx                context.Context
	wsPingEvery, wsPongWait time.Duration
	accessChecker           session.AccessChecker
	startPresenceForUser    func(*session.Claims, string) func()
	edgeMu                  sync.Mutex
	edgeSubs                map[string]*edgeSub
}
type edgeSub struct {
	sub    *relay.Subscription
	cancel context.CancelFunc
	refs   int
}

func NewEdge(ctx context.Context, id string, broker *relay.Broker, registry *relay.Registry, sessions session.Connections, ping, pong time.Duration, access session.AccessChecker, presence func(*session.Claims, string) func()) *Edge {
	return &Edge{replicaID: id, broker: broker, registry: registry, sessions: sessions, relayCtx: ctx, wsPingEvery: ping, wsPongWait: pong, accessChecker: access, startPresenceForUser: presence}
}

// edgePlayerConn adapts a player's websocket to relay.Conn for the edge
// registry. The edge holds the live socket; the owner publishes envelopes that
// the registry delivers here. acked flips true on the first delivered payload,
// which the join loop uses to know the owner has seated the player.
type edgePlayerConn struct {
	conn *session.Connection

	acked *atomicBool
}

func (e edgePlayerConn) Send(payload map[string]any) {
	if e.acked != nil {
		e.acked.Store(true)
	}
	if err := e.conn.Send(payload); err != nil {
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

// ServePlayer serves a player socket whose room is owned by another
// replica. The edge registers the socket so owner-published envelopes reach it,
// forwards a join control message to the owner, then proxies every client frame
// to the owner over the inbound channel until the socket closes.
func (server *Edge) ServePlayer(roomID string, claims *session.Claims, sessionID session.ID, token string) {
	conn := server.sessions.Connection(sessionID)
	if conn == nil {
		return
	}
	stopHeartbeat := conn.StartHeartbeat(server.wsPingEvery, server.wsPongWait)
	stopAccessCheck := session.StartAccessCheck(server.accessChecker, claims.Sub, claims.IsGuest, conn)
	inbound := &transport.InboundLimiter{}
	acked := &atomicBool{}
	server.registry.AddPlayer(roomID, claims.Sub, edgePlayerConn{conn: conn, acked: acked})

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
		stopHeartbeat()
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
		stopAccessCheck()
		stopHeartbeat()
		close(joinDone)
		server.registry.RemovePlayer(roomID, claims.Sub)
		// Multi-tab: only tell the owner the seat left when this was the last
		// live socket for the sub on this edge. The owner has no local registry
		// entry for edge-held players, so CountPlayers there would always be 0.
		if server.registry.CountPlayers(roomID, claims.Sub) == 0 {
			lctx, lcancel := context.WithTimeout(server.relayCtx, 2*time.Second)
			if err := server.broker.PublishInbound(lctx, roomID, relay.Inbound{Kind: relay.InboundLeave, Sub: claims.Sub, EdgeID: server.replicaID}); err != nil {
				log.Printf("edge publish leave: %v", err)
			}
			lcancel()
		}
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch inbound.Allow() {
		case transport.InboundClose:
			if err := conn.Send(map[string]any{"type": "error", "message": "connection closed: too many messages"}); err != nil {
				log.Printf("edge flood close error: %v", err)
			}
			continue
		case transport.InboundSlowDown:
			if err := conn.Send(map[string]any{"type": "error", "message": "too many messages, slow down"}); err != nil {
				log.Printf("edge flood slow-down error: %v", err)
			}
			continue
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
func (server *Edge) subscribeRoomOutbound(roomID string) {
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
func (server *Edge) unsubscribeRoomOutbound(roomID string) {
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
	conn *session.Connection
}

func (e edgeSpectatorConn) Send(payload map[string]any) {
	if err := e.conn.Send(payload); err != nil {
		log.Printf("edge spectator write: %v", err)
	}
	if fatal, _ := payload["fatal"].(bool); fatal {
		_ = e.conn.Close()
	}
}

// ServeSpectator serves a spectator socket whose room is owned by another
// replica. The edge registers the socket (by a process-unique spectator id) so
// the owner's TargetSpectators / TargetSpectator envelopes reach it, asks the
// owner to register the viewer (which replies with the initial snapshot), then
// proxies the spectator's emote frames to the owner until the socket closes.
// Mirrors ServePlayer, but spectators are read-only with respect to the
// game so there is no join-ack retry. The targeted initial snapshot admits the
// socket to live broadcasts; without that reply it remains pending.
func (server *Edge) ServeSpectator(roomID string, claims *session.Claims, sessionID session.ID, spectatorID string) {
	conn := server.sessions.Connection(sessionID)
	if conn == nil {
		return
	}
	stopHeartbeat := conn.StartHeartbeat(server.wsPingEvery, server.wsPongWait)
	inbound := &transport.InboundLimiter{}
	server.registry.AddSpectator(roomID, spectatorID, edgeSpectatorConn{conn: conn})

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
	// running — unlike ServePlayer's race on a brand-new room where the
	// subscriber may not exist yet. A single publish is sufficient here; the
	// spectator remains pending until the owner's targeted snapshot arrives.
	publishJoin()

	// Presence on the edge: the user is watching from this replica.
	stop := server.startPresenceForUser(claims, roomID)
	defer stop()

	defer func() {
		stopHeartbeat()
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
		if inbound.Allow() != transport.InboundAllowed {
			continue
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
