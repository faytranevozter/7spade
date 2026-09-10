package room

import (
	"github.com/faytranevozter/7spade/services/ws/game"
)

// humanPlayerCountLocked counts all human seats (bots excluded), including ones
// currently disconnected. The rematch panel shows every human (leavers carry a
// "Left" badge), so the total denominator must include them too.
func humanPlayerCountLocked(players []*player) int {
	count := 0
	for _, player := range players {
		if player.isBot {
			continue
		}
		count++
	}
	return count
}

func connectedHumanPlayerCountLocked(players []*player) int {
	count := 0
	for _, player := range players {
		if player.disconnected || player.isBot {
			continue
		}
		count++
	}
	return count
}

func (room *room) maxPlayers() int {
	if room.gameConfig.PlayerCount > 0 {
		return room.gameConfig.PlayerCount
	}
	return game.PlayerCount
}

func connectedPlayersLocked(players []*player) []*player {
	connected := make([]*player, 0, len(players))
	for _, player := range players {
		if player.disconnected || player.isBot {
			continue
		}
		connected = append(connected, player)
	}
	return connected
}
