package room

import (
	"encoding/json"
)

// command is the room's single inbound boundary after transport has admitted a
// frame. It decodes and dispatches client messages.
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
		room.handleMessage(command.player, message)
		return
	}

	if command.spectator == nil {
		return
	}
	var message clientMessage
	if err := json.Unmarshal(command.payload, &message); err != nil || message.Type != messageTypeEmote {
		return
	}
	room.handleSpectatorEmote(command.spectator, message.Emote)
}
