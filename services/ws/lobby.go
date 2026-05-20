package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/gorilla/websocket"
)

type roomPhase int

const (
	phaseLobby roomPhase = iota
	phasePlaying
)

const (
	minPlayersToStart = 2
	botMoveDelay      = 600 * time.Millisecond

	messageTypeLobbyState = "lobby_state"
	messageTypeSetReady   = "set_ready"
	messageTypeStartGame  = "start_game"
)

// roomStatusUpdater notifies the API service of room status changes.
type roomStatusUpdater interface {
	UpdateRoomStatus(roomID, status string) error
}

type apiRoomStatusUpdater struct {
	url    string
	client *http.Client
}

func (u *apiRoomStatusUpdater) UpdateRoomStatus(roomID, status string) error {
	payload, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/internal/rooms/%s/status", u.url, roomID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("update room status returned status %d", resp.StatusCode)
	}
	return nil
}

// addLobbyPlayer appends a new human player to the lobby slot order, or
// resumes an existing slot when the same identity reconnects mid-lobby.
// Caller must hold room.mu.
func (room *room) addLobbyPlayerLocked(claims *tokenClaims, conn *websocket.Conn) (*player, bool, error) {
	for _, existing := range room.players {
		if existing.sub == claims.Sub {
			existing.conn = conn
			wasDisconnected := existing.disconnected
			existing.disconnected = false
			return existing, wasDisconnected, nil
		}
	}
	if len(room.players) >= game.PlayerCount {
		return nil, false, fmt.Errorf("room is full")
	}
	isHost := len(room.players) == 0
	p := &player{
		sub:         claims.Sub,
		displayName: claims.DisplayName,
		isGuest:     claims.IsGuest,
		ready:       isHost, // host is implicitly ready
		index:       len(room.players),
		conn:        conn,
	}
	room.players = append(room.players, p)
	return p, false, nil
}

// removeLobbyPlayerLocked drops the player from room.players and recompacts
// indices so the next-in-line player becomes host (index 0).
func (room *room) removeLobbyPlayerLocked(target *player) {
	filtered := room.players[:0]
	for _, p := range room.players {
		if p == target {
			continue
		}
		filtered = append(filtered, p)
	}
	room.players = filtered
	for i, p := range room.players {
		p.index = i
		if i == 0 {
			// new host is implicitly ready
			p.ready = true
		}
	}
}

func (room *room) lobbyStateMessageLocked() map[string]any {
	hostName := ""
	if len(room.players) > 0 {
		hostName = room.players[0].displayName
	}
	allReady := true
	playerPayloads := make([]map[string]any, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		if !p.ready {
			allReady = false
		}
		playerPayloads = append(playerPayloads, map[string]any{
			"display_name": p.displayName,
			"is_host":      p.index == 0,
			"ready":        p.ready,
			"disconnected": p.disconnected,
		})
	}
	canStart := allReady && len(playerPayloads) >= minPlayersToStart
	return map[string]any{
		"type":              messageTypeLobbyState,
		"host_display_name": hostName,
		"min_to_start":      minPlayersToStart,
		"max_players":       game.PlayerCount,
		"can_start":         canStart,
		"players":           playerPayloads,
	}
}

func (room *room) broadcastLobbyState() {
	room.mu.Lock()
	message := room.lobbyStateMessageLocked()
	targets := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	for _, p := range targets {
		p.send(message)
	}
}

func (room *room) handleSetReady(p *player, ready bool) {
	room.mu.Lock()
	if room.phase != phaseLobby {
		room.mu.Unlock()
		p.sendError("game has already started")
		return
	}
	// Host stays implicitly ready; ignore explicit toggles for them.
	if p.index != 0 {
		p.ready = ready
	}
	room.mu.Unlock()
	room.broadcastLobbyState()
}

func (room *room) handleStartGame(initiator *player) {
	room.mu.Lock()
	if room.phase != phaseLobby {
		room.mu.Unlock()
		initiator.sendError("game has already started")
		return
	}
	if initiator.index != 0 {
		room.mu.Unlock()
		initiator.sendError("only the host can start the game")
		return
	}
	if len(room.players) < minPlayersToStart {
		room.mu.Unlock()
		initiator.sendError(fmt.Sprintf("need at least %d players to start", minPlayersToStart))
		return
	}
	for _, p := range room.players {
		if !p.ready {
			room.mu.Unlock()
			initiator.sendError(fmt.Sprintf("waiting for %s to ready up", p.displayName))
			return
		}
	}

	// Fill remaining seats with bots so the engine always has 4 hands.
	botNumber := 1
	for len(room.players) < game.PlayerCount {
		room.players = append(room.players, &player{
			displayName: fmt.Sprintf("Bot %d", botNumber),
			isBot:       true,
			ready:       true,
			index:       len(room.players),
		})
		botNumber++
	}

	state, starter := game.Deal(time.Now().UnixNano())
	room.state = state
	room.state.CurrentPlayer = starter
	room.phase = phasePlaying
	room.started = true
	room.startedAt = time.Now().UTC()
	room.store.Save(room.id, room.state)
	room.startTurnTimerLocked()
	updater := room.statusUpdater
	roomID := room.id
	room.mu.Unlock()

	if updater != nil {
		go func() {
			if err := updater.UpdateRoomStatus(roomID, "in_progress"); err != nil {
				log.Printf("update room status to in_progress: %v", err)
			}
		}()
	}

	room.broadcastState()
	room.playBotIfNeeded()
}

// playBotIfNeeded schedules an auto-move when it is currently a bot's turn.
// Caller must NOT hold room.mu.
func (room *room) playBotIfNeeded() {
	room.mu.Lock()
	if !room.started || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	idx := room.state.CurrentPlayer
	if idx < 0 || idx >= len(room.players) || !room.players[idx].isBot {
		room.mu.Unlock()
		return
	}
	room.mu.Unlock()

	time.AfterFunc(botMoveDelay, func() {
		room.executeBotMove(idx)
	})
}

func (room *room) executeBotMove(botIdx int) {
	room.mu.Lock()
	if !room.started || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	if room.state.CurrentPlayer != botIdx {
		room.mu.Unlock()
		return
	}
	if botIdx < 0 || botIdx >= len(room.players) || !room.players[botIdx].isBot {
		room.mu.Unlock()
		return
	}
	move, ok := game.PickMove(room.state, room.state.Hands[botIdx])
	if !ok {
		room.mu.Unlock()
		return
	}
	state, err := game.ApplyMove(room.state, botIdx, move.Card, move.FaceDown)
	if err != nil {
		log.Printf("bot move failed: %v", err)
		room.mu.Unlock()
		return
	}
	room.state = state
	room.store.Save(room.id, room.state)
	gameOver := game.IsGameOver(room.state)
	if !gameOver {
		room.startTurnTimerLocked()
	}
	room.mu.Unlock()

	if gameOver {
		room.saveGameResult()
		room.broadcastGameOver()
		return
	}
	room.broadcastState()
	room.playBotIfNeeded()
}
