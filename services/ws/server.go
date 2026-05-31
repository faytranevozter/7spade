package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

type GameServer struct {
	jwtSecret         string
	rooms             map[string]*room
	store             stateStore
	gameHistory       gameHistoryStore
	statusUpdater     roomStatusUpdater
	memberRemover     roomMemberRemover
	reconciler        roomReconciler
	turnTimerDuration time.Duration
	lobbyLeaveGrace   time.Duration
	mu                sync.Mutex
	upgrader          websocket.Upgrader
}

type room struct {
	id                string
	players           []*player
	state             game.GameState
	store             stateStore
	gameHistory       gameHistoryStore
	statusUpdater     roomStatusUpdater
	memberRemover     roomMemberRemover
	phase             roomPhase
	started           bool
	startedAt         time.Time
	turnTimerDuration time.Duration
	lobbyLeaveGrace   time.Duration
	turnExpiresAt     time.Time
	turnTimer         *time.Timer
	turnTimerToken    int
	rematchVotes      map[int]bool
	mu                sync.Mutex
}

type stateStore interface {
	// SaveRoom persists a room snapshot. Implementations are fire-and-forget
	// from the caller's perspective (the Redis adapter writes asynchronously),
	// so this returns no error.
	SaveRoom(roomID string, snap roomSnapshot)
	// LoadRoom returns a persisted snapshot for the room, or ok=false when none
	// exists. Called only on the join path, never on the move hot-path.
	LoadRoom(roomID string) (roomSnapshot, bool)
	// DeleteRoom drops a room's persisted snapshot when the room is torn down.
	DeleteRoom(roomID string)
}

// persistedPlayer is the durable subset of a room player.
type persistedPlayer struct {
	sub         string
	displayName string
	isGuest     bool
	isBot       bool
	ready       bool
	index       int
}

// roomSnapshot is the complete durable state of a room, used to rebuild it
// after a WS process restart.
type roomSnapshot struct {
	state          game.GameState
	players        []persistedPlayer
	phase          roomPhase
	started        bool
	startedAt      time.Time
	turnExpiresAt  time.Time
	turnTimerToken int
	rematchVotes   []int
}

type gameHistoryStore interface {
	SaveGame(result savedGameResult) error
}

type savedGameResult struct {
	RoomID     string            `json:"room_id"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
	Players    []savedGamePlayer `json:"players"`
}

type savedGamePlayer struct {
	UserID        string `json:"user_id,omitempty"`
	DisplayName   string `json:"display_name"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
}

type apiGameHistoryStore struct {
	url    string
	client *http.Client
	secret string
}

type memoryStateStore struct {
	mu        sync.Mutex
	snapshots map[string]roomSnapshot
}

type player struct {
	sub          string
	displayName  string
	isGuest      bool
	isBot        bool
	ready        bool
	index        int
	conn         *websocket.Conn
	disconnected bool
	leaveTimer   *time.Timer
	leaveToken   int
	lastEmoteAt  time.Time
	mu           sync.Mutex
}

var orderedSuits = []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs}

type tokenClaims struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	IsGuest     bool   `json:"is_guest"`
	jwt.RegisteredClaims
}

type clientMessage struct {
	Type   string `json:"type"`
	Suit   string `json:"suit"`
	Rank   string `json:"rank"`
	Method string `json:"method"`
	Ready  bool   `json:"ready"`
	Emote  string `json:"emote"`
}

const (
	messageTypeError              = "error"
	messageTypeGameOver           = "game_over"
	messageTypePlaceFaceDown      = "place_facedown"
	messageTypePlayerDisconnected = "player_disconnected"
	messageTypePlayerReconnected  = "player_reconnected"
	messageTypeRematchCancelled   = "rematch_cancelled"
	messageTypeRematchStatus      = "rematch_status"
	messageTypeRematchVote        = "rematch_vote"
	messageTypePlayCard           = "play_card"
	messageTypeStateUpdate        = "state_update"
	messageTypeEmote              = "emote"
)

// allowedEmotes is the server-side allowlist of emote IDs. Emotes outside this
// set are rejected so the broadcast channel can't be abused to relay arbitrary
// payloads. The frontend catalog (web/src/game/emotes.ts) must stay in sync.
var allowedEmotes = map[string]bool{
	"thumbs_up": true,
	"laugh":     true,
	"wow":       true,
	"think":     true,
	"celebrate": true,
	"sad":       true,
	"gg":        true,
	"nice":      true,
	"oops":      true,
}

// emoteCooldown is the minimum gap between emotes from a single player. Faster
// emotes are silently dropped to prevent spamming the room.
const emoteCooldown = time.Second

func NewGameServerFromConfig(cfg Config, store stateStore) *GameServer {
	return NewGameServerWithOptions(cfg, store, 60*time.Second)
}

func NewGameServer(jwtSecret string) *GameServer {
	return NewGameServerWithStateStore(jwtSecret, newMemoryStateStore())
}

func NewGameServerWithStateStore(jwtSecret string, store stateStore) *GameServer {
	return NewGameServerWithOptions(Config{JWTSecret: jwtSecret}, store, 60*time.Second)
}

func NewGameServerWithOptions(cfg Config, store stateStore, turnTimerDuration time.Duration) *GameServer {
	historyStore := gameHistoryStore(nil)
	var statusUpdater roomStatusUpdater
	var memberRemover roomMemberRemover
	var reconciler roomReconciler
	if apiURL := strings.TrimRight(cfg.APIURL, "/"); apiURL != "" {
		historyStore = &apiGameHistoryStore{url: apiURL + "/internal/games", client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		statusUpdater = &apiRoomStatusUpdater{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		memberRemover = &apiRoomMemberRemover{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		reconciler = &apiRoomReconciler{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
	}
	return &GameServer{
		jwtSecret:         cfg.JWTSecret,
		rooms:             map[string]*room{},
		store:             store,
		gameHistory:       historyStore,
		statusUpdater:     statusUpdater,
		memberRemover:     memberRemover,
		reconciler:        reconciler,
		turnTimerDuration: turnTimerDuration,
		lobbyLeaveGrace:   defaultLobbyLeaveGrace,
		upgrader:          websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// reconcileInterval is how often the WS service reports its live room set to
// the API so orphaned 'waiting' rooms (no live presence) get cleaned up.
const reconcileInterval = 60 * time.Second

// StartRoomReconciler periodically reports the set of rooms this server is
// tracking to the API, which deletes stale 'waiting' rooms that have no live
// WS presence. No-op when no API reconciler is configured (e.g. tests or when
// API_URL is unset). Runs until ctx is cancelled.
func (server *GameServer) StartRoomReconciler(ctx context.Context) {
	if server.reconciler == nil {
		return
	}
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := server.reconciler.ReconcileRooms(server.activeRoomIDs()); err != nil {
				log.Printf("reconcile rooms: %v", err)
			}
		}
	}
}

// activeRoomIDs snapshots the IDs of every room currently held in memory.
func (server *GameServer) activeRoomIDs() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	ids := make([]string, 0, len(server.rooms))
	for id := range server.rooms {
		ids = append(ids, id)
	}
	return ids
}

func (store *apiGameHistoryStore) SaveGame(result savedGameResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, store.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, store.secret)
	resp, err := store.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("save game returned status %d", resp.StatusCode)
	}
	return nil
}

// setInternalSecret attaches the shared internal-API secret header when one is
// configured, so the API's /internal guard accepts the request.
func setInternalSecret(req *http.Request, secret string) {
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{snapshots: map[string]roomSnapshot{}}
}

func (store *memoryStateStore) SaveRoom(roomID string, snap roomSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.snapshots[roomID] = cloneRoomSnapshot(snap)
}

func (store *memoryStateStore) LoadRoom(roomID string) (roomSnapshot, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	snap, ok := store.snapshots[roomID]
	if !ok {
		return roomSnapshot{}, false
	}
	return cloneRoomSnapshot(snap), true
}

func (store *memoryStateStore) DeleteRoom(roomID string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.snapshots, roomID)
}

// cloneRoomSnapshot deep-copies a snapshot so the in-memory store doesn't alias
// the caller's live state (mirrors the JSON copy the Redis adapter performs).
func cloneRoomSnapshot(snap roomSnapshot) roomSnapshot {
	clone := snap
	clone.state = cloneGameState(snap.state)
	clone.players = append([]persistedPlayer(nil), snap.players...)
	clone.rematchVotes = append([]int(nil), snap.rematchVotes...)
	return clone
}

// redisStateStore persists room snapshots to Redis via the store package.
// Writes are performed asynchronously so Redis I/O never blocks the move
// hot-path (each room serialises its own mutations under room.mu, so
// last-write-wins ordering per room is safe). Loads are synchronous but only
// happen on the join path.
type redisStateStore struct {
	store   *store.Store
	timeout time.Duration
}

func newRedisStateStore(s *store.Store) *redisStateStore {
	return &redisStateStore{store: s, timeout: 5 * time.Second}
}

func (r *redisStateStore) SaveRoom(roomID string, snap roomSnapshot) {
	// Each save runs in its own goroutine so Redis I/O never blocks the move
	// hot-path. Writes for the same room can in principle land out of order,
	// but the snapshot is only read on a cold rehydrate (after a full restart);
	// a momentarily stale snapshot self-corrects on the next persisted change.
	persisted := toStoreSnapshot(snap)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.SaveRoom(ctx, roomID, persisted); err != nil {
			log.Printf("persist room %s: %v", roomID, err)
		}
	}()
}

func (r *redisStateStore) LoadRoom(roomID string) (roomSnapshot, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	persisted, err := r.store.LoadRoom(ctx, roomID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("load room %s: %v", roomID, err)
		}
		return roomSnapshot{}, false
	}
	return fromStoreSnapshot(persisted), true
}

func (r *redisStateStore) DeleteRoom(roomID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		if err := r.store.Delete(ctx, roomID); err != nil {
			log.Printf("delete room %s: %v", roomID, err)
		}
	}()
}

func toStoreSnapshot(snap roomSnapshot) store.RoomSnapshot {
	players := make([]store.PersistedPlayer, 0, len(snap.players))
	for _, p := range snap.players {
		players = append(players, store.PersistedPlayer{
			Sub:         p.sub,
			DisplayName: p.displayName,
			IsGuest:     p.isGuest,
			IsBot:       p.isBot,
			Ready:       p.ready,
			Index:       p.index,
		})
	}
	return store.RoomSnapshot{
		State:          snap.state,
		Players:        players,
		Phase:          int(snap.phase),
		Started:        snap.started,
		StartedAt:      snap.startedAt,
		TurnExpiresAt:  snap.turnExpiresAt,
		TurnTimerToken: snap.turnTimerToken,
		RematchVotes:   append([]int(nil), snap.rematchVotes...),
	}
}

func fromStoreSnapshot(snap store.RoomSnapshot) roomSnapshot {
	players := make([]persistedPlayer, 0, len(snap.Players))
	for _, p := range snap.Players {
		players = append(players, persistedPlayer{
			sub:         p.Sub,
			displayName: p.DisplayName,
			isGuest:     p.IsGuest,
			isBot:       p.IsBot,
			ready:       p.Ready,
			index:       p.Index,
		})
	}
	return roomSnapshot{
		state:          snap.State,
		players:        players,
		phase:          roomPhase(snap.Phase),
		started:        snap.Started,
		startedAt:      snap.StartedAt,
		turnExpiresAt:  snap.TurnExpiresAt,
		turnTimerToken: snap.TurnTimerToken,
		rematchVotes:   append([]int(nil), snap.RematchVotes...),
	}
}

func cloneGameState(state game.GameState) game.GameState {
	clone := game.GameState{
		Board:         make(map[game.Suit]game.SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[game.Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
	}
	for player := range state.Hands {
		clone.Hands[player] = append([]game.Card(nil), state.Hands[player]...)
		clone.FaceDown[player] = append([]game.Card(nil), state.FaceDown[player]...)
	}
	for suit, sequence := range state.Board {
		clone.Board[suit] = sequence
	}
	for suit, closed := range state.Closed {
		clone.Closed[suit] = closed
	}
	return clone
}

// snapshotLocked builds a durable snapshot of the room. Caller must hold room.mu.
func (room *room) snapshotLocked() roomSnapshot {
	players := make([]persistedPlayer, 0, len(room.players))
	for _, p := range room.players {
		players = append(players, persistedPlayer{
			sub:         p.sub,
			displayName: p.displayName,
			isGuest:     p.isGuest,
			isBot:       p.isBot,
			ready:       p.ready,
			index:       p.index,
		})
	}
	votes := make([]int, 0, len(room.rematchVotes))
	for idx := range room.rematchVotes {
		votes = append(votes, idx)
	}
	return roomSnapshot{
		state:          cloneGameState(room.state),
		players:        players,
		phase:          room.phase,
		started:        room.started,
		startedAt:      room.startedAt,
		turnExpiresAt:  room.turnExpiresAt,
		turnTimerToken: room.turnTimerToken,
		rematchVotes:   votes,
	}
}

// persistLocked saves the current room snapshot to the state store. Caller must
// hold room.mu. The Redis adapter writes asynchronously, so this does not block
// the move hot-path.
func (room *room) persistLocked() {
	if room.store == nil {
		return
	}
	room.store.SaveRoom(room.id, room.snapshotLocked())
}

// restoreFromSnapshotLocked rebuilds a freshly-created room from a persisted
// snapshot. Players are restored as disconnected (no live socket yet); a
// reconnecting client re-attaches via the normal join path. Called during
// joinRoom on a brand-new room that has not yet been published to
// server.rooms, so no room-level synchronisation is needed (the caller holds
// server.mu and no other goroutine can observe this room).
func (room *room) restoreFromSnapshotLocked(snap roomSnapshot) {
	room.state = cloneGameState(snap.state)
	room.phase = snap.phase
	room.started = snap.started
	room.startedAt = snap.startedAt
	room.turnExpiresAt = snap.turnExpiresAt
	room.turnTimerToken = snap.turnTimerToken
	room.rematchVotes = map[int]bool{}
	for _, idx := range snap.rematchVotes {
		room.rematchVotes[idx] = true
	}
	room.players = make([]*player, 0, len(snap.players))
	for _, p := range snap.players {
		room.players = append(room.players, &player{
			sub:          p.sub,
			displayName:  p.displayName,
			isGuest:      p.isGuest,
			isBot:        p.isBot,
			ready:        p.ready,
			index:        p.index,
			disconnected: !p.isBot, // humans have no socket yet; bots are always "present"
		})
	}
	// Resume the turn timer for an in-progress game so play continues even if no
	// human reconnects (auto-play keeps bots and absent players moving).
	if room.phase == phasePlaying && room.started && !game.IsGameOver(room.state) {
		room.startTurnTimerLocked()
	}
}

func (server *GameServer) routes(checks map[string]dependencyCheck) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("ws", checks))
	mux.HandleFunc("GET /ws", server.handleWebSocket)
	return mux
}

func (server *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}
	claims, err := parseToken(r.URL.Query().Get("token"), server.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	room, player, joinResult, err := server.joinRoom(roomID, claims, conn)
	if err != nil {
		if writeErr := conn.WriteJSON(errorMessage(err.Error())); writeErr != nil {
			log.Printf("write websocket join error: %v", writeErr)
		}
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("close websocket after join error: %v", closeErr)
		}
		return
	}

	switch joinResult {
	case joinResultLobbyJoined:
		room.broadcastLobbyState()
	case joinResultLobbyReconnected:
		room.broadcastLobbyState()
	case joinResultGameReconnected:
		player.send(room.stateMessageFor(player.index))
	case joinResultGameOver:
		player.send(room.gameOverMessage())
	}
	go room.readLoop(player)
}

type joinResult int

const (
	joinResultLobbyJoined joinResult = iota
	joinResultLobbyReconnected
	joinResultGameReconnected
	joinResultGameOver
)

func (server *GameServer) joinRoom(roomID string, claims *tokenClaims, conn *websocket.Conn) (*room, *player, joinResult, error) {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	if gameRoom == nil {
		gameRoom = &room{
			id:                roomID,
			store:             server.store,
			gameHistory:       server.gameHistory,
			statusUpdater:     server.statusUpdater,
			memberRemover:     server.memberRemover,
			turnTimerDuration: server.turnTimerDuration,
			lobbyLeaveGrace:   server.lobbyLeaveGrace,
			rematchVotes:      map[int]bool{},
			phase:             phaseLobby,
		}
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

	if gameRoom.phase == phasePlaying {
		for _, existing := range gameRoom.players {
			if existing.sub == claims.Sub {
				existing.conn = conn
				wasDisconnected := existing.disconnected
				existing.disconnected = false
				// If the game already finished, the player is reconnecting to a
				// completed room — send them the results, not a live board.
				if game.IsGameOver(gameRoom.state) {
					return gameRoom, existing, joinResultGameOver, nil
				}
				if wasDisconnected {
					go gameRoom.broadcastPlayerConnection(messageTypePlayerReconnected, existing.displayName, existing.index)
				}
				return gameRoom, existing, joinResultGameReconnected, nil
			}
		}
		return nil, nil, 0, fmt.Errorf("game already started")
	}

	joined, wasDisconnected, err := gameRoom.addLobbyPlayerLocked(claims, conn)
	if err != nil {
		return nil, nil, 0, err
	}
	if wasDisconnected {
		return gameRoom, joined, joinResultLobbyReconnected, nil
	}
	return gameRoom, joined, joinResultLobbyJoined, nil
}

func (room *room) readLoop(player *player) {
	conn := player.conn
	defer func() {
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
		room.handleMessage(player, message)
	}
}

func (room *room) handleDisconnect(player *player, conn *websocket.Conn) {
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
	cancelRematch := game.IsGameOver(room.state) && len(room.rematchVotes) > 0
	if cancelRematch {
		room.rematchVotes = map[int]bool{}
	}
	room.mu.Unlock()

	if cancelRematch {
		room.broadcastRematchCancelled()
	}
	room.broadcastPlayerConnection(messageTypePlayerDisconnected, player.displayName, player.index)
}

func (room *room) handleMessage(player *player, message clientMessage) {
	room.mu.Lock()

	// Lobby-phase messages are handled before the started/turn checks.
	if room.phase == phaseLobby {
		room.mu.Unlock()
		switch message.Type {
		case messageTypeSetReady:
			room.handleSetReady(player, message.Ready)
		case messageTypeStartGame:
			room.handleStartGame(player)
		case messageTypeLeave:
			room.handleLobbyLeave(player)
		case messageTypeEmote:
			room.handleEmote(player, message.Emote)
		default:
			player.sendError("game has not started")
		}
		return
	}

	if !room.started {
		room.mu.Unlock()
		player.sendError("game has not started")
		return
	}
	if player.disconnected {
		room.mu.Unlock()
		player.sendError("player is disconnected")
		return
	}
	if message.Type == messageTypeRematchVote {
		room.handleRematchVoteLocked(player)
		return
	}
	// Emotes are social, not gameplay: they're allowed on any player's turn, so
	// handle them before the turn-ownership check. handleEmote manages its own
	// locking, so release the lock first.
	if message.Type == messageTypeEmote {
		room.mu.Unlock()
		room.handleEmote(player, message.Emote)
		return
	}
	if room.state.CurrentPlayer != player.index {
		room.mu.Unlock()
		player.sendError("not your turn")
		return
	}

	state, err := applyClientMessage(room.state, player.index, message)
	if err != nil {
		room.mu.Unlock()
		player.sendError(err.Error())
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

func (room *room) handleRematchVoteLocked(player *player) {
	if !game.IsGameOver(room.state) {
		room.mu.Unlock()
		player.sendError("rematch is only available after game over")
		return
	}
	if room.rematchVotes == nil {
		room.rematchVotes = map[int]bool{}
	}
	room.rematchVotes[player.index] = true
	if len(room.rematchVotes) < game.PlayerCount {
		room.mu.Unlock()
		room.broadcastRematchStatus()
		return
	}

	state, starter := game.Deal(time.Now().UnixNano())
	room.state = state
	room.state.CurrentPlayer = starter
	room.rematchVotes = map[int]bool{}
	room.persistLocked()
	room.startTurnTimerLocked()
	room.mu.Unlock()
	room.broadcastState()
	room.playBotIfNeeded()
}

func (room *room) startTurnTimerLocked() {
	if room.turnTimer != nil {
		room.turnTimer.Stop()
	}
	room.turnExpiresAt = time.Now().Add(room.turnTimerDuration).UTC()
	room.turnTimerToken++
	token := room.turnTimerToken
	room.turnTimer = time.AfterFunc(room.turnTimerDuration, func() {
		room.handleTurnTimerExpired(token)
	})
}

func (room *room) handleTurnTimerExpired(token int) {
	room.mu.Lock()
	if !room.started || token != room.turnTimerToken || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	playerIndex := room.state.CurrentPlayer
	move, ok := game.PickMove(room.state, room.state.Hands[playerIndex])
	if !ok {
		room.mu.Unlock()
		return
	}
	state, err := applyBotMove(room.state, playerIndex, move)
	if err != nil {
		log.Printf("auto-play move failed: %v", err)
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

func applyClientMessage(state game.GameState, playerIndex int, message clientMessage) (game.GameState, error) {
	switch message.Type {
	case messageTypePlayCard:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, err
		}
		// Aces never extend a sequence; a play_card on an Ace always means
		// "close this suit". Resolve which end to close from the explicit
		// method, the locked global method, or the single available end.
		if card.Rank == game.Ace {
			method, err := resolveAceCloseMethod(state, playerIndex, card.Suit, message.Method)
			if err != nil {
				return game.GameState{}, err
			}
			return game.ApplyAceClose(state, playerIndex, card.Suit, method)
		}
		return game.ApplyMove(state, playerIndex, card, false)
	case messageTypePlaceFaceDown:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, err
		}
		return game.ApplyMove(state, playerIndex, card, true)
	default:
		return game.GameState{}, fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// applyBotMove applies a bot/auto-play move, routing Ace closes through
// ApplyAceClose and everything else through ApplyMove.
func applyBotMove(state game.GameState, playerIndex int, move game.BotMove) (game.GameState, error) {
	if move.Close {
		return game.ApplyAceClose(state, playerIndex, move.Card.Suit, move.Method)
	}
	return game.ApplyMove(state, playerIndex, move.Card, move.FaceDown)
}

// resolveAceCloseMethod decides which end an Ace close targets. An explicit
// client method is used as-is (ApplyAceClose validates it). Otherwise the
// locked global method wins; if none is locked the method is inferred when
// exactly one end is legal, and is reported ambiguous when both are.
func resolveAceCloseMethod(state game.GameState, playerIndex int, suit game.Suit, requested string) (game.CloseMethod, error) {
	if requested != "" {
		return game.CloseMethod(requested), nil
	}
	if state.CloseMethod != "" {
		return state.CloseMethod, nil
	}
	var option *game.AceCloseOption
	for _, candidate := range game.AceCloseOptions(state, state.Hands[playerIndex]) {
		if candidate.Suit == suit {
			opt := candidate
			option = &opt
			break
		}
	}
	if option == nil {
		return "", fmt.Errorf("cannot close %s: no Ace close available", suit)
	}
	switch {
	case option.CanLow && !option.CanHigh:
		return game.CloseLow, nil
	case option.CanHigh && !option.CanLow:
		return game.CloseHigh, nil
	default:
		return "", fmt.Errorf("ambiguous close for %s: specify low or high", suit)
	}
}

func (room *room) broadcastState() {
	room.mu.Lock()
	type stateSnapshot struct {
		player  *player
		message map[string]any
	}
	snapshots := make([]stateSnapshot, 0, len(room.players))
	for _, player := range room.players {
		if player.disconnected || player.isBot || player.conn == nil {
			continue
		}
		snapshots = append(snapshots, stateSnapshot{player: player, message: room.stateMessageFor(player.index)})
	}
	room.mu.Unlock()
	for _, snapshot := range snapshots {
		snapshot.player.send(snapshot.message)
	}
}

func (room *room) stateMessageFor(playerIndex int) map[string]any {
	moves := game.ValidMoves(room.state, room.state.Hands[playerIndex])
	validCards := map[game.Card]bool{}
	for _, card := range moves.Cards {
		validCards[card] = true
	}
	// A closable Ace is a legal play (via close), so mark it valid in the hand
	// and surface which ends are available so the client can prompt low/high.
	aceCloseOptions := make([]map[string]any, 0, len(moves.AceCloses))
	for _, option := range moves.AceCloses {
		validCards[game.Card{Suit: option.Suit, Rank: game.Ace}] = true
		aceCloseOptions = append(aceCloseOptions, map[string]any{
			"suit":     string(option.Suit),
			"can_low":  option.CanLow,
			"can_high": option.CanHigh,
		})
	}

	yourHand := make([]map[string]any, 0, len(room.state.Hands[playerIndex]))
	for _, card := range room.state.Hands[playerIndex] {
		yourHand = append(yourHand, cardPayload(card, validCards[card]))
	}

	opponents := make([]map[string]any, 0, game.PlayerCount-1)
	for i := 1; i < len(room.players); i++ {
		idx := (playerIndex + i) % len(room.players)
		player := room.players[idx]
		opponents = append(opponents, map[string]any{
			"display_name":   player.displayName,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
			"disconnected":   player.disconnected,
		})
	}

	return map[string]any{
		"type":              messageTypeStateUpdate,
		"status":            "in_progress",
		"board":             boardPayload(room.state),
		"closed_suits":      closedSuits(room.state),
		"ace_close_method":  room.state.CloseMethod,
		"ace_close_options": aceCloseOptions,
		"your_hand":         yourHand,
		"opponents":         opponents,
		"current_turn":      room.players[room.state.CurrentPlayer].displayName,
		"turn_ends_at":      room.turnExpiresAt.Format(time.RFC3339),
	}
}

// handleEmote validates an emote against the allowlist and a per-player
// cooldown, then echoes it to everyone in the room (including the sender, so
// every client renders the bubble from the same broadcast).
func (room *room) handleEmote(player *player, emote string) {
	if !allowedEmotes[emote] {
		player.sendError("unknown emote")
		return
	}

	room.mu.Lock()
	now := time.Now()
	if !player.lastEmoteAt.IsZero() && now.Sub(player.lastEmoteAt) < emoteCooldown {
		// Too soon after the last emote: silently drop to avoid spamming the
		// room (and a flood of error toasts on the sender).
		room.mu.Unlock()
		return
	}
	player.lastEmoteAt = now
	displayName := player.displayName
	room.mu.Unlock()

	room.broadcastEmote(displayName, emote)
}

// broadcastEmote fans an emote out to every connected human in the room,
// including the sender, so all clients render the bubble identically.
func (room *room) broadcastEmote(displayName string, emote string) {
	room.mu.Lock()
	message := map[string]any{"type": messageTypeEmote, "display_name": displayName, "emote": emote}
	players := make([]*player, 0, len(room.players))
	for _, player := range room.players {
		if !player.disconnected && !player.isBot && player.conn != nil {
			players = append(players, player)
		}
	}
	room.mu.Unlock()

	for _, player := range players {
		player.send(message)
	}
}

func (room *room) broadcastPlayerConnection(messageType string, displayName string, playerIndex int) {
	room.mu.Lock()
	message := map[string]any{"type": messageType, "display_name": displayName}
	players := make([]*player, 0, len(room.players))
	for _, player := range room.players {
		if player.index != playerIndex && !player.disconnected && !player.isBot && player.conn != nil {
			players = append(players, player)
		}
	}
	room.mu.Unlock()

	for _, player := range players {
		player.send(message)
	}
}

func (room *room) broadcastGameOver() {
	room.mu.Lock()
	room.rematchVotes = map[int]bool{}
	message := room.gameOverMessageLocked()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	for _, player := range players {
		player.send(message)
	}
}

// gameOverMessage builds the game_over payload for a single recipient (e.g. a
// player reconnecting to an already-finished room).
func (room *room) gameOverMessage() map[string]any {
	room.mu.Lock()
	defer room.mu.Unlock()
	return room.gameOverMessageLocked()
}

// gameOverMessageLocked builds the game_over payload, including the final board
// so reconnecting clients (with no prior state_update this session) can still
// render the completed board alongside the results. Caller must hold room.mu.
func (room *room) gameOverMessageLocked() map[string]any {
	return map[string]any{
		"type":             messageTypeGameOver,
		"results":          room.results(),
		"board":            boardPayload(room.state),
		"closed_suits":     closedSuits(room.state),
		"ace_close_method": room.state.CloseMethod,
	}
}
func (room *room) broadcastRematchStatus() {
	room.mu.Lock()
	message := room.rematchStatusMessageLocked()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	for _, player := range players {
		player.send(message)
	}
}

func (room *room) broadcastRematchCancelled() {
	room.mu.Lock()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	for _, player := range players {
		player.send(map[string]any{"type": messageTypeRematchCancelled})
	}
}

func (room *room) rematchStatusMessageLocked() map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		if player.isBot {
			continue
		}
		players = append(players, map[string]any{
			"display_name": player.displayName,
			"voted":        room.rematchVotes[player.index],
		})
	}
	return map[string]any{
		"type":    messageTypeRematchStatus,
		"votes":   len(room.rematchVotes),
		"total":   game.PlayerCount,
		"players": players,
	}
}

func connectedPlayersLocked(players []*player) []*player {
	connected := make([]*player, 0, len(players))
	for _, player := range players {
		if player.disconnected || player.isBot || player.conn == nil {
			continue
		}
		connected = append(connected, player)
	}
	return connected
}

func (room *room) saveGameResult() {
	if room == nil {
		return
	}
	room.mu.Lock()
	result := room.savedResultLocked(time.Now().UTC())
	historyStore := room.gameHistory
	statusUpdater := room.statusUpdater
	roomID := room.id
	room.mu.Unlock()
	if historyStore != nil {
		if err := historyStore.SaveGame(result); err != nil {
			log.Printf("save game result: %v", err)
		}
	}
	if statusUpdater != nil {
		if err := statusUpdater.UpdateRoomStatus(roomID, "finished"); err != nil {
			log.Printf("update room status to finished: %v", err)
		}
	}
}

func (room *room) savedResultLocked(finishedAt time.Time) savedGameResult {
	scoredPlayers := room.scoredPlayersLocked()
	players := make([]savedGamePlayer, 0, len(room.players))
	for _, scoredPlayer := range scoredPlayers {
		player := scoredPlayer.player
		userID := player.sub
		if player.isGuest || player.isBot {
			userID = ""
		}
		players = append(players, savedGamePlayer{
			UserID:        userID,
			DisplayName:   player.displayName,
			PenaltyPoints: scoredPlayer.score,
			Rank:          scoredPlayer.rank,
			IsWinner:      scoredPlayer.isWinner,
		})
	}
	startedAt := room.startedAt
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	return savedGameResult{RoomID: room.id, StartedAt: startedAt, FinishedAt: finishedAt, Players: players}
}

func (room *room) results() []map[string]any {
	scoredPlayers := room.scoredPlayersLocked()
	results := make([]map[string]any, 0, len(scoredPlayers))
	for _, scoredPlayer := range scoredPlayers {
		player := scoredPlayer.player
		results = append(results, map[string]any{
			"display_name":   player.displayName,
			"facedown_cards": revealedFaceDownCards(room.state, player.index),
			"penalty_points": scoredPlayer.score,
			"rank":           scoredPlayer.rank,
			"is_winner":      scoredPlayer.isWinner,
		})
	}
	return results
}

type scoredPlayer struct {
	player   *player
	score    int
	rank     int
	isWinner bool
}

func (room *room) scoredPlayersLocked() []scoredPlayer {
	scores := game.CalculateScores(room.state)
	sortedScores := append([]int(nil), scores[:]...)
	sort.Ints(sortedScores)
	ranksByScore := competitionRanks(sortedScores)
	lowest := sortedScores[0]
	scoredPlayers := make([]scoredPlayer, 0, len(room.players))
	for _, player := range room.players {
		score := scores[player.index]
		scoredPlayers = append(scoredPlayers, scoredPlayer{player: player, score: score, rank: ranksByScore[score], isWinner: score == lowest})
	}
	return scoredPlayers
}

func competitionRanks(sortedScores []int) map[int]int {
	ranks := make(map[int]int, len(sortedScores))
	for index, score := range sortedScores {
		if _, exists := ranks[score]; exists {
			continue
		}
		ranks[score] = index + 1
	}
	return ranks
}

func revealedFaceDownCards(state game.GameState, playerIndex int) []map[string]any {
	cards := make([]map[string]any, 0, len(state.FaceDown[playerIndex]))
	for _, card := range state.FaceDown[playerIndex] {
		cards = append(cards, map[string]any{
			"suit":   string(card.Suit),
			"rank":   rankString(card.Rank),
			"points": scoringValue(card, state.CloseMethod),
		})
	}
	return cards
}

func scoringValue(card game.Card, method game.CloseMethod) int {
	if card.Rank == game.Ace && method == game.CloseLow {
		return 1
	}
	return card.PointValue()
}

func (player *player) send(message map[string]any) {
	if player == nil || player.conn == nil {
		return
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	if err := player.conn.WriteJSON(message); err != nil {
		log.Printf("write websocket message: %v", err)
	}
}

func (player *player) sendError(message string) {
	player.send(errorMessage(message))
}

func errorMessage(message string) map[string]any {
	return map[string]any{"type": messageTypeError, "message": message}
}

func cardPayload(card game.Card, valid bool) map[string]any {
	return map[string]any{"suit": card.Suit, "rank": rankString(card.Rank), "valid": valid}
}

func boardPayload(state game.GameState) map[string]any {
	board := map[string]any{}
	for _, suit := range orderedSuits {
		sequence, ok := state.Board[suit]
		if !ok {
			board[string(suit)] = nil
			continue
		}
		board[string(suit)] = map[string]any{"low": sequence.Low, "high": sequence.High}
	}
	return board
}

func closedSuits(state game.GameState) []string {
	closed := []string{}
	for _, suit := range orderedSuits {
		if state.Closed[suit] {
			closed = append(closed, string(suit))
		}
	}
	return closed
}

func parseToken(tokenString string, secret string) (*tokenClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid || claims.Sub == "" || claims.DisplayName == "" {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func parseCard(suitValue string, rankValue string) (game.Card, error) {
	suit := game.Suit(strings.ToLower(suitValue))
	if suit != game.Spades && suit != game.Hearts && suit != game.Diamonds && suit != game.Clubs {
		return game.Card{}, fmt.Errorf("unknown suit: %s", suitValue)
	}
	rank, err := parseRank(rankValue)
	if err != nil {
		return game.Card{}, err
	}
	return game.Card{Suit: suit, Rank: rank}, nil
}

func parseRank(value string) (game.Rank, error) {
	switch strings.ToUpper(value) {
	case "J":
		return game.Jack, nil
	case "Q":
		return game.Queen, nil
	case "K":
		return game.King, nil
	case "A":
		return game.Ace, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < int(game.Two) || number > int(game.Ace) {
		return 0, fmt.Errorf("unknown rank: %s", value)
	}
	return game.Rank(number), nil
}

func rankString(rank game.Rank) string {
	switch rank {
	case game.Jack:
		return "J"
	case game.Queen:
		return "Q"
	case game.King:
		return "K"
	case game.Ace:
		return "A"
	default:
		return strconv.Itoa(int(rank))
	}
}
