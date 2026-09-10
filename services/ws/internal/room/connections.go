package room

import (
	"log"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

func (room *room) readLoop(player *player, conn *session.Connection, sessionID session.ID) {
	stopHeartbeat := conn.StartHeartbeat(room.wsPingEvery, room.wsPongWait)
	stopAccessCheck := startAccessCheck(room.accessChecker, player.sub, player.isGuest, conn)
	defer func() {
		stopAccessCheck()
		stopHeartbeat()
		room.handleDisconnect(player, sessionID)
		room.delivery.Remove(sessionID)
		if err := conn.Close(); err != nil {
			log.Printf("close websocket read loop: %v", err)
		}
	}()
	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		ok, closeConn := player.allowInbound()
		if closeConn {
			player.sendError("connection closed: too many messages")
			return
		}
		if !ok {
			player.sendError("too many messages, slow down")
			continue
		}
		room.handleMessage(player, message)
	}
}
