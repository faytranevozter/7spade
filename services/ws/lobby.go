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
	botMoveDelay      = 1500 * time.Millisecond

	// defaultLobbyLeaveGrace is how long a lobby player's seat (and DB
	// membership row) is held after their socket drops, so a refresh or a
	// brief network blip reconnects to the same slot instead of being treated
	// as a permanent leave. Only after this elapses without a reconnect is the
	// player removed and the API told to drop the membership row.
	defaultLobbyLeaveGrace = 10 * time.Second

	messageTypeLobbyState = "lobby_state"
	messageTypeSetReady   = "set_ready"
	messageTypeStartGame  = "start_game"
	messageTypeLeave      = "leave"
)

// roomStatusUpdater notifies the API service of room status changes.
type roomStatusUpdater interface {
	UpdateRoomStatus(roomID, status string) error
}

// roomMemberRemover notifies the API service that a player has left a room,
// so the API can drop the membership row (and delete the room when empty).
type roomMemberRemover interface {
	RemoveRoomPlayer(roomID, userID string) error
}

type apiRoomStatusUpdater struct {
	url    string
	client *http.Client
	secret string
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
	setInternalSecret(req, u.secret)
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

type apiRoomMemberRemover struct {
	url    string
	client *http.Client
	secret string
}

func (r *apiRoomMemberRemover) RemoveRoomPlayer(roomID, userID string) error {
	url := fmt.Sprintf("%s/internal/rooms/%s/players/%s", r.url, roomID, userID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, r.secret)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("remove room player returned status %d", resp.StatusCode)
	}
	return nil
}

// roomReconciler reports the set of room IDs the WS service is actively
// tracking so the API can delete 'waiting' rooms that have no live presence
// (orphaned lobbies whose member never connected over WebSocket).
type roomReconciler interface {
	ReconcileRooms(activeRoomIDs []string) error
}

type apiRoomReconciler struct {
	url    string
	client *http.Client
	secret string
}

func (r *apiRoomReconciler) ReconcileRooms(activeRoomIDs []string) error {
	payload, err := json.Marshal(map[string][]string{"active_room_ids": activeRoomIDs})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/internal/rooms/reconcile", r.url)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, r.secret)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("reconcile rooms returned status %d", resp.StatusCode)
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
			existing.avatar = claims.AvatarURL // refresh from the (possibly newer) token
			wasDisconnected := existing.disconnected
			existing.disconnected = false
			// Reconnected within the grace window: cancel the pending leave so
			// the seat and DB membership row are preserved.
			room.cancelLobbyLeaveLocked(existing)
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
		avatar:      claims.AvatarURL,
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

// dropDisconnectedLobbyPlayersLocked removes every player currently flagged as
// disconnected (e.g. left or dropped within the grace window) and recompacts
// indices. Pending grace timers are cancelled so a stale finalize can't act on
// an already-removed player. Returns the subs of dropped human players so the
// caller can drop their DB membership rows (off-lock). Caller must hold room.mu.
func (room *room) dropDisconnectedLobbyPlayersLocked() []string {
	droppedSubs := make([]string, 0)
	kept := make([]*player, 0, len(room.players))
	for _, p := range room.players {
		if p.disconnected {
			if p.leaveTimer != nil {
				p.leaveTimer.Stop()
				p.leaveTimer = nil
			}
			p.leaveToken++
			if !p.isBot && p.sub != "" {
				droppedSubs = append(droppedSubs, p.sub)
			}
			continue
		}
		kept = append(kept, p)
	}
	room.players = kept
	for i, p := range room.players {
		p.index = i
		if i == 0 {
			p.ready = true
		}
	}
	return droppedSubs
}

// scheduleLobbyLeaveLocked starts (or restarts) the grace timer that finalizes
// a lobby player's departure if they don't reconnect in time. Each schedule
// bumps leaveToken so a stale timer that fires after a reconnect is ignored.
// Caller must hold room.mu.
func (room *room) scheduleLobbyLeaveLocked(target *player) {
	if target.leaveTimer != nil {
		target.leaveTimer.Stop()
	}
	target.leaveToken++
	token := target.leaveToken
	grace := room.lobbyLeaveGrace
	if grace <= 0 {
		// No grace configured: finalize immediately on the next tick so the
		// behaviour matches the historical "remove on disconnect" semantics.
		grace = time.Nanosecond
	}
	target.leaveTimer = time.AfterFunc(grace, func() {
		room.finalizeLobbyLeave(target, token)
	})
}

// cancelLobbyLeaveLocked stops a pending grace timer and invalidates it so a
// concurrently-firing timer becomes a no-op. Caller must hold room.mu.
func (room *room) cancelLobbyLeaveLocked(target *player) {
	if target.leaveTimer != nil {
		target.leaveTimer.Stop()
		target.leaveTimer = nil
	}
	target.leaveToken++
}

// finalizeLobbyLeave removes a player whose grace period elapsed without a
// reconnect, then notifies the API to drop the membership row (and delete the
// room when the last human leaves). A reconnect bumps leaveToken, so a stale
// timer firing here is ignored.
func (room *room) finalizeLobbyLeave(target *player, token int) {
	room.mu.Lock()
	// Phase moved on (game started) or a reconnect superseded this timer.
	if room.phase != phaseLobby || token != target.leaveToken || !target.disconnected {
		room.mu.Unlock()
		return
	}
	room.removeAndNotifyLobbyLeaveLocked(target)
}

// handleLobbyLeave removes a player who explicitly left the waiting room (vs. a
// transient disconnect). Removal is immediate — no reconnect grace — so other
// players see the seat free up in realtime.
func (room *room) handleLobbyLeave(target *player) {
	room.mu.Lock()
	if room.phase != phaseLobby {
		room.mu.Unlock()
		return
	}
	// Ignore a duplicate leave (e.g. the client sends it twice): if the player
	// is no longer seated, there's nothing to remove or notify about.
	seated := false
	for _, p := range room.players {
		if p == target {
			seated = true
			break
		}
	}
	if !seated {
		room.mu.Unlock()
		return
	}
	// Cancel any pending grace timer and invalidate stale tokens so a
	// concurrently-firing finalize becomes a no-op.
	if target.leaveTimer != nil {
		target.leaveTimer.Stop()
		target.leaveTimer = nil
	}
	target.leaveToken++
	target.disconnected = true
	room.removeAndNotifyLobbyLeaveLocked(target)
}

// removeAndNotifyLobbyLeaveLocked drops the player from the lobby, then (with
// the lock released) broadcasts the updated state and tells the API to remove
// the membership row. Caller must hold room.mu; this function releases it.
func (room *room) removeAndNotifyLobbyLeaveLocked(target *player) {
	room.removeLobbyPlayerLocked(target)
	target.leaveTimer = nil
	hasPlayers := len(room.players) > 0
	// Capture leave details before releasing the lock so the API can drop the
	// membership row. Bots have no DB row; real users (including guests) do.
	remover := room.memberRemover
	roomID := room.id
	leaverSub := target.sub
	notifyLeave := remover != nil && leaverSub != "" && !target.isBot
	room.mu.Unlock()

	if notifyLeave {
		go func() {
			if err := remover.RemoveRoomPlayer(roomID, leaverSub); err != nil {
				log.Printf("remove room player on lobby leave: %v", err)
			}
		}()
	}
	if hasPlayers {
		room.broadcastLobbyState()
	} else if room.store != nil {
		// Last player left an unstarted room: drop its durable snapshot so it
		// isn't resurrected on a later connect (the API also deletes the room).
		room.store.DeleteRoom(roomID)
	}
}

func (room *room) lobbyStateMessageLocked() map[string]any {
	hostName := ""
	if len(room.players) > 0 {
		hostName = room.players[0].displayName
	}
	allReady := true
	connectedCount := 0
	playerPayloads := make([]map[string]any, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		// A disconnected player (within the reconnect grace window) is still
		// listed so a refresh can resume their seat, but they don't count
		// toward "everyone is ready" or the startable player count — so the
		// host can't start a game with a phantom who has already left/dropped.
		if p.disconnected {
			playerPayloads = append(playerPayloads, map[string]any{
				"display_name": p.displayName,
				"avatar_url":   p.avatar,
				"is_host":      p.index == 0,
				"ready":        p.ready,
				"disconnected": true,
			})
			continue
		}
		connectedCount++
		if !p.ready {
			allReady = false
		}
		playerPayloads = append(playerPayloads, map[string]any{
			"display_name": p.displayName,
			"avatar_url":   p.avatar,
			"is_host":      p.index == 0,
			"ready":        p.ready,
			"disconnected": false,
		})
	}
	canStart := allReady && connectedCount >= minPlayersToStart
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
	// Persist the lobby roster on every change so a restart mid-lobby can
	// rehydrate the waiting room. broadcastLobbyState is the single funnel for
	// every lobby-state change (join, ready, leave, disconnect, host promotion).
	room.persistLocked()
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

	// Validate against connected players only — a player mid-disconnect (left
	// or dropped within the grace window) must neither count toward the minimum
	// nor block start by being "not ready". They're removed below before the
	// deal so they're never dealt in as phantom humans.
	connectedCount := 0
	for _, p := range room.players {
		if p.disconnected {
			continue
		}
		connectedCount++
		if !p.ready {
			room.mu.Unlock()
			initiator.sendError(fmt.Sprintf("waiting for %s to ready up", p.displayName))
			return
		}
	}
	if connectedCount < minPlayersToStart {
		room.mu.Unlock()
		initiator.sendError(fmt.Sprintf("need at least %d players to start", minPlayersToStart))
		return
	}

	// Commit to starting: drop disconnected players (the initiator is connected,
	// so the host seat survives) and bot-fill the freed seats. Their DB rows are
	// removed off-lock below so the membership doesn't orphan.
	droppedSubs := room.dropDisconnectedLobbyPlayersLocked()

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
	room.persistLocked()
	room.startTurnTimerLocked()
	updater := room.statusUpdater
	remover := room.memberRemover
	roomID := room.id
	room.mu.Unlock()

	// Drop the DB membership rows of players removed at start so player_count
	// doesn't describe participants who aren't in the game.
	if remover != nil {
		for _, sub := range droppedSubs {
			sub := sub
			go func() {
				if err := remover.RemoveRoomPlayer(roomID, sub); err != nil {
					log.Printf("remove dropped player on game start: %v", err)
				}
			}()
		}
	}

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
	state, err := applyBotMove(room.state, botIdx, move)
	if err != nil {
		log.Printf("bot move failed: %v", err)
		room.mu.Unlock()
		return
	}
	room.state = state
	room.persistLocked()
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
