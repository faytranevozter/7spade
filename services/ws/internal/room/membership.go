package room

import (
	"fmt"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"

	"github.com/faytranevozter/7spade/services/ws/game"
)

type joinResult int

const (
	joinResultLobbyJoined joinResult = iota
	joinResultLobbyReconnected
	joinResultGameReconnected
	joinResultGameOver
)

func (server *Manager) joinRoom(roomID string, claims *tokenClaims, conn *session.Connection, token string) (*room, *player, joinResult, error) {
	turnTimerDuration := server.turnTimerDuration
	botDifficulty := game.BotMedium
	practiceMode := false
	gameConfig := game.DefaultConfig()
	// Check whether this is the first in-memory join without holding server.mu
	// across the API call below. A slow API must not block unrelated room joins.
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	needsCreate := gameRoom == nil
	server.mu.Unlock()

	// Only the first in-memory room creation needs persisted settings. Existing
	// live rooms and reconnects already carry their configured timer in memory.
	if needsCreate && server.roomSettings != nil {
		settings, err := server.roomSettings.GetRoomSettings(roomID, token)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("load room settings: %w", err)
		}
		if settings.TurnTimerSeconds > 0 {
			turnTimerDuration = time.Duration(settings.TurnTimerSeconds) * time.Second
		}
		botDifficulty = normalizeBotDifficulty(settings.BotDifficulty)
		practiceMode = settings.PracticeMode
		gameConfig = gameConfigFromSettings(settings)
	}

	server.mu.Lock()
	gameRoom = server.rooms[roomID]
	// Another join may have created the room while this goroutine was fetching
	// settings, so re-check under the lock before publishing a new room.
	if gameRoom == nil {
		gameRoom = server.newRoomLocked(roomID, botDifficulty, practiceMode, turnTimerDuration, gameConfig)
		// Rehydrate from the durable store if this room existed before a
		// restart. Restored players start disconnected; the join flow below
		// re-attaches the reconnecting socket to its existing seat.
		if server.store != nil {
			if snap, ok := server.store.LoadRoom(roomID); ok {
				gameRoom.restoreFromSnapshotLocked(snap)
			}
		}
		server.rooms[roomID] = gameRoom
	}
	server.mu.Unlock()

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	return gameRoom.seatLocked(claims, conn)
}

// seatLocked attaches a (re)connecting player to the room: reconnecting to an
// in-progress/finished game, or joining/resuming a lobby seat. conn may be nil
// for a remote player whose socket lives on an edge replica (the owner reaches
// them via the relay). Caller holds room.mu.
func (room *room) seatLocked(claims *tokenClaims, conn *session.Connection) (*room, *player, joinResult, error) {
	if room.phase == phasePlaying {
		for _, existing := range room.players {
			if existing.sub == claims.Sub {
				// Synchronize with player.send / the heartbeat, which read conn
				// and room under existing.mu (see the player struct contract).
				existing.mu.Lock()
				existing.conn = conn
				existing.room = room
				existing.mu.Unlock()
				wasDisconnected := existing.disconnected
				existing.disconnected = false
				// If the game already finished, the player is reconnecting to a
				// completed room — send them the results, not a live board.
				if game.IsGameOver(room.state) {
					return room, existing, joinResultGameOver, nil
				}
				if wasDisconnected {
					go room.broadcastPlayerConnection(messageTypePlayerReconnected, existing.displayName, existing.index)
				}
				return room, existing, joinResultGameReconnected, nil
			}
		}
		return nil, nil, 0, fmt.Errorf("game already started")
	}

	joined, wasDisconnected, err := room.addLobbyPlayerLocked(claims, conn)
	if err != nil {
		return nil, nil, 0, err
	}
	if wasDisconnected {
		return room, joined, joinResultLobbyReconnected, nil
	}
	return room, joined, joinResultLobbyJoined, nil
}

func (room *room) handleDisconnect(player *player, conn *session.Connection) {
	room.mu.Lock()
	if player.conn != conn {
		room.mu.Unlock()
		return
	}

	if room.phase == phaseLobby {
		// A lobby socket dropping is usually transient (page refresh, brief
		// network blip, dev-mode StrictMode remount). Don't tear down the seat
		// or the DB membership immediately — hold it for a grace period so a
		// reconnect with the same identity resumes the same slot. Only if no
		// reconnect arrives do we finalize the leave (see finalizeLobbyLeave).
		if player.disconnected {
			// Already pending removal from an earlier drop; nothing to do.
			room.mu.Unlock()
			return
		}
		player.disconnected = true
		room.scheduleLobbyLeaveLocked(player)
		room.mu.Unlock()
		// Other connected players see the seat as held-but-disconnected.
		room.broadcastLobbyState()
		return
	}

	if !room.started || player.disconnected {
		room.mu.Unlock()
		return
	}
	player.disconnected = true

	// A disconnect during the rematch countdown means a full-table rematch is no
	// longer possible (a rematch needs every human, not bots). Drop the leaver's
	// vote and stop the countdown; the remaining humans are offered a move back
	// to the waiting room instead (client swaps the button on rematch_status).
	rematchActive := game.IsGameOver(room.state) && room.rematchTimer != nil
	if rematchActive {
		delete(room.rematchVotes, player.index)
		room.stopRematchTimerLocked()
	}
	room.mu.Unlock()

	if rematchActive {
		// Clear the client countdown, then refresh the vote panel so it shows the
		// "Left" badge and the Go-to-waiting-room button.
		room.broadcastRematchCountdown(map[string]any{"type": messageTypeRematchCountdown, "expires_at": ""})
		room.broadcastRematchStatus()
	}
	room.broadcastPlayerConnection(messageTypePlayerDisconnected, player.displayName, player.index)
}
