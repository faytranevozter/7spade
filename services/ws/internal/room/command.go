package room

import (
	"encoding/json"
)

// command is the room's single inbound boundary. Local socket loops and relay
// consumers both submit raw frames here, so decoding and flood protection stay
// identical regardless of which replica holds the connection.
type command struct {
	player    *player
	spectator *spectator
	payload   []byte
}

func playerCommand(player *player, payload []byte) command {
	return command{player: player, payload: payload}
}

func spectatorCommand(spectator *spectator, payload []byte) command {
	return command{spectator: spectator, payload: payload}
}

func (room *room) handleCommand(command command) {
	if command.player != nil {
		var message clientMessage
		if err := json.Unmarshal(command.payload, &message); err != nil {
			return
		}
		ok, closeConn := command.player.allowInbound()
		if closeConn {
			command.player.sendError("connection closed: too many messages")
			return
		}
		if !ok {
			command.player.sendError("too many messages, slow down")
			return
		}
		room.handleMessage(command.player, message)
		return
	}

	if command.spectator == nil {
		return
	}
	ok, closeConn := command.spectator.allowInbound()
	if closeConn || !ok {
		return
	}
	var message clientMessage
	if err := json.Unmarshal(command.payload, &message); err != nil || message.Type != messageTypeEmote {
		return
	}
	room.handleSpectatorEmote(command.spectator, message.Emote)
}
