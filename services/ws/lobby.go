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

	// defaultRematchWindow is how long the rematch countdown runs once the
	// first player votes. If every connected human votes before it expires the
	// rematch deals immediately; otherwise, on expiry, the voters drop back to
	// the waiting room (same room) and the non-voters are removed.
	defaultRematchWindow = 30 * time.Second

	messageTypeLobbyState = "lobby_state"
	messageTypeSetReady   = "set_ready"
	messageTypeStartGame  = "start_game"
	messageTypeLeave      = "leave"
	messageTypeKick       = "kick"
	messageTypeSetTeam    = "set_team"
)

// roomStatusUpdater notifies the API service of room status changes.
type roomStatusUpdater interface {
	UpdateRoomStatus(roomID, status string) error
}

// roomMemberRemover notifies the API service that a player has left a room,
// so the API can drop the membership row (and delete the room when empty).
// KickRoomPlayer additionally records the kick so the player can't rejoin.
type roomMemberRemover interface {
	RemoveRoomPlayer(roomID, userID string) error
	KickRoomPlayer(roomID, userID string) error
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

// KickRoomPlayer removes a player and records the kick so they can't rejoin.
func (r *apiRoomMemberRemover) KickRoomPlayer(roomID, userID string) error {
	url := fmt.Sprintf("%s/internal/rooms/%s/kick/%s", r.url, roomID, userID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
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
		return fmt.Errorf("kick room player returned status %d", resp.StatusCode)
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
	// A player the host kicked can't rejoin this room (until a WS restart clears
	// the in-memory set). They're free to join other rooms.
	if claims.Sub != "" && room.kickedSubs[claims.Sub] {
		return nil, false, fmt.Errorf("you were removed from this room by the host")
	}
	if len(room.players) >= room.maxPlayers() {
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
		room:        room,
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
	room.removeAndNotifyLobbyLeaveLocked(target, false)
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
	room.removeAndNotifyLobbyLeaveLocked(target, false)
}

// handleKick lets the host (seat 0) remove another human from the waiting room.
// The target is identified by their current seat index (slot), which is the
// server's canonical identity and is exposed in lobby_state. The kicked
// player's sub is remembered so they cannot immediately rejoin the same room.
func (room *room) handleKick(initiator *player, targetSlot int) {
	room.mu.Lock()
	if room.phase != phaseLobby {
		room.mu.Unlock()
		initiator.sendError("can only remove players before the game starts")
		return
	}
	if initiator.index != 0 {
		room.mu.Unlock()
		initiator.sendError("only the host can remove players")
		return
	}
	if targetSlot == 0 {
		// The host can't kick themselves.
		room.mu.Unlock()
		initiator.sendError("the host cannot be removed")
		return
	}
	var target *player
	for _, p := range room.players {
		if p.index == targetSlot && !p.isBot {
			target = p
			break
		}
	}
	if target == nil {
		room.mu.Unlock()
		initiator.sendError("player not found")
		return
	}
	// Remember the kicked identity so addLobbyPlayerLocked rejects a reconnect
	// into this room (in-memory only; cleared on a WS restart).
	if target.sub != "" {
		if room.kickedSubs == nil {
			room.kickedSubs = map[string]bool{}
		}
		room.kickedSubs[target.sub] = true
	}
	// Cancel any pending grace timer so a stale finalize can't double-remove.
	if target.leaveTimer != nil {
		target.leaveTimer.Stop()
		target.leaveTimer = nil
	}
	target.leaveToken++
	target.disconnected = true
	// Tell the kicked player to leave before we drop their seat. send() is
	// safe to call while holding room.mu (it locks the player, not the room).
	target.send(map[string]any{"type": messageTypeRoomClosed, "reason": "kicked"})
	// removeAndNotifyLobbyLeaveLocked drops the seat + DB row, rebroadcasts the
	// roster to the remaining players, and releases room.mu. kick=true records
	// the kick server-side so the player can't rejoin via the HTTP join path.
	room.removeAndNotifyLobbyLeaveLocked(target, true)
}

// the membership row. Caller must hold room.mu; this function releases it.
func (room *room) removeAndNotifyLobbyLeaveLocked(target *player, kick bool) {
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
			var err error
			if kick {
				err = remover.KickRoomPlayer(roomID, leaverSub)
			} else {
				err = remover.RemoveRoomPlayer(roomID, leaverSub)
			}
			if err != nil {
				log.Printf("remove room player on lobby leave (kick=%v): %v", kick, err)
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
			"slot":         p.index,
			"is_host":      p.index == 0,
			"ready":        p.ready,
			"disconnected": true,
			"team":         p.team,
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
			"slot":         p.index,
			"is_host":      p.index == 0,
			"ready":        p.ready,
			"disconnected": false,
			"team":         p.team,
		})
	}
	canStart := allReady && connectedCount >= room.startThresholdLocked()
	return map[string]any{
		"type":              messageTypeLobbyState,
		"host_display_name": hostName,
		"min_to_start":      room.startThresholdLocked(),
		"max_players":       room.maxPlayers(),
		"can_start":         canStart,
		"practice_mode":     room.practiceMode,
		"team_mode":         string(room.gameConfig.TeamMode),
		"players":           playerPayloads,
	}
}

// startThresholdLocked is the minimum number of connected, ready human players
// required before the host can start. Practice rooms are solo vs bots, so a
// single host is enough; normal rooms need the usual minimum. Caller must hold
// room.mu.
func (room *room) startThresholdLocked() int {
	if room.practiceMode {
		return 1
	}
	return minPlayersToStart
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
	room.deliverToPlayers(targets, message)
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

func (room *room) handleSetTeam(p *player, team int) {
	room.mu.Lock()
	if room.phase != phaseLobby {
		room.mu.Unlock()
		p.sendError("game has already started")
		return
	}
	if p.ready {
		room.mu.Unlock()
		p.sendError("unready first to change team")
		return
	}
	if room.gameConfig.TeamMode != game.Team2v2 {
		room.mu.Unlock()
		p.sendError("team selection is only available in team mode")
		return
	}
	numTeams := room.maxPlayers() / 2
	if team < 0 || team >= numTeams {
		room.mu.Unlock()
		p.sendError(fmt.Sprintf("team must be 0 to %d", numTeams-1))
		return
	}
	teamCount := 0
	for _, pl := range room.players {
		if pl == p {
			continue
		}
		if pl.team == team {
			teamCount++
		}
	}
	teamCap := 2
	if teamCount >= teamCap {
		room.mu.Unlock()
		p.sendError("that team is already full")
		return
	}
	p.team = team
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
	if connectedCount < room.startThresholdLocked() {
		room.mu.Unlock()
		initiator.sendError(fmt.Sprintf("need at least %d players to start", room.startThresholdLocked()))
		return
	}

	if room.gameConfig.TeamMode == game.Team2v2 {
		numTeams := room.maxPlayers() / 2
		teamCounts := make([]int, numTeams)
		for _, p := range room.players {
			if p.disconnected {
				continue
			}
			if p.team >= 0 && p.team < numTeams {
				teamCounts[p.team]++
			}
		}
		for i, count := range teamCounts {
			if count == 0 {
				room.mu.Unlock()
				initiator.sendError(fmt.Sprintf("team %d has no players", i+1))
				return
			}
		}
	}

	// Commit to starting: drop disconnected players (the initiator is connected,
	// so the host seat survives) and bot-fill the freed seats. Their DB rows are
	// removed off-lock below so the membership doesn't orphan.
	droppedSubs := room.dropDisconnectedLobbyPlayersLocked()

	botNumber := 1
	for len(room.players) < room.maxPlayers() {
		botTeam := 0
		if room.gameConfig.TeamMode == game.Team2v2 {
			numTeams := room.maxPlayers() / 2
			teamCounts := make([]int, numTeams)
			for _, p := range room.players {
				if p.team >= 0 && p.team < numTeams {
					teamCounts[p.team]++
				}
			}
			minCount := teamCounts[0]
			for i := 1; i < numTeams; i++ {
				if teamCounts[i] < minCount {
					minCount = teamCounts[i]
					botTeam = i
				}
			}
		}
		room.players = append(room.players, &player{
			displayName: fmt.Sprintf("Bot %d", botNumber),
			isBot:       true,
			ready:       true,
			index:       len(room.players),
			team:        botTeam,
		})
		botNumber++
	}

	if room.gameConfig.TeamMode == game.Team2v2 {
		numTeams := room.maxPlayers() / 2
		teams := make([][]int, numTeams)
		for _, p := range room.players {
			teams[p.team] = append(teams[p.team], p.index)
		}
		room.gameConfig.Teams = teams
	}

	state, starter := game.DealWithConfig(time.Now().UnixNano(), room.gameConfig)
	room.state = state
	room.state.CurrentPlayer = starter
	room.initialHands = make([][]game.Card, len(room.state.Hands))
	for i := range room.state.Hands {
		room.initialHands[i] = append([]game.Card(nil), room.state.Hands[i]...)
	}
	room.moves = nil
	room.savedGameID = ""
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
	// Bot auto-play is an authoritative action: only the owning replica drives
	// it (no-op for a demoted owner so a failed-over replica doesn't double-play).
	if !room.isOwnerOrSolo() {
		return
	}
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
	move, ok := game.PickMoveWithDifficulty(room.state, botIdx, room.botDifficulty)
	if !ok {
		room.mu.Unlock()
		return
	}
	state, rec, err := applyBotMove(room.state, botIdx, move)
	if err != nil {
		log.Printf("bot move failed: %v", err)
		room.mu.Unlock()
		return
	}
	room.state = state
	room.moves = append(room.moves, rec)
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
