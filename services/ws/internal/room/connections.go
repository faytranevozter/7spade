package room

import (
	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/internal/transport"
)

func (room *room) runPlayerSession(player *player, sessionID session.ID) {
	if room.sessions == nil {
		return
	}
	room.sessions.Run(sessionID, session.Loop{
		PingEvery: room.wsPingEvery, PongWait: room.wsPongWait,
		Access: room.accessChecker, UserID: player.sub, Guest: player.isGuest,
		Closed:  func() { room.handleDisconnect(player, sessionID) },
		Message: func(payload []byte) { room.handleCommand(playerCommand(player, payload)) },
		Inbound: &transport.InboundLimiter{},
		Rejected: func(decision transport.InboundDecision) {
			if decision == transport.InboundClose {
				player.sendError("connection closed: too many messages")
				return
			}
			player.sendError("too many messages, slow down")
		},
	})
}
