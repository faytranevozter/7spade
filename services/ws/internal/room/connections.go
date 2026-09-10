package room

import "github.com/faytranevozter/7spade/services/ws/internal/session"

func (room *room) runPlayerSession(player *player, sessionID session.ID) {
	if room.sessions == nil {
		return
	}
	room.sessions.Run(sessionID, session.Loop{
		PingEvery: room.wsPingEvery, PongWait: room.wsPongWait,
		Access: room.accessChecker, UserID: player.sub, Guest: player.isGuest,
		Closed:  func() { room.handleDisconnect(player, sessionID) },
		Message: func(payload []byte) { room.handleCommand(playerCommand(player, payload)) },
	})
}
