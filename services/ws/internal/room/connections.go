package room

import (
	"log"
)

func (room *room) readLoop(player *player) {
	conn := player.conn
	stopHeartbeat := conn.StartHeartbeat(room.wsPingEvery, room.wsPongWait)
	stopAccessCheck := startAccessCheck(room.accessChecker, player.sub, player.isGuest, conn)
	defer func() {
		stopAccessCheck()
		stopHeartbeat()
		room.handleDisconnect(player, conn)
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
