package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/relay"
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
	roomSettings      roomSettingsStore
	presence          presenceWriter
	turnTimerDuration time.Duration
	lobbyLeaveGrace   time.Duration
	rematchWindow     time.Duration
	mu                sync.Mutex
	upgrader          websocket.Upgrader

	// Cross-replica relay (Phase 1+). nil when running without a relay (the
	// default in tests and single-process setups), in which case the server
	// behaves exactly as the original in-memory single-replica implementation.
	replicaID   string
	broker      *relay.Broker
	leases      *relay.LeaseManager
	registry    *relay.Registry
	coordinator *relay.Coordinator
	relayCtx    context.Context
	relayCancel context.CancelFunc

	// edgeSubs holds the ref-counted per-room outbound subscriptions this
	// replica maintains while it acts as an edge for those rooms.
	edgeMu   sync.Mutex
	edgeSubs map[string]*edgeSub
}

// edgeSub is one ref-counted outbound subscription on an edge replica.
type edgeSub struct {
	sub    *relay.Subscription
	cancel context.CancelFunc
	refs   int
}

// attachRelay wires the cross-replica relay primitives onto the server. Called
// once at startup from main; safe to skip entirely (single-replica mode).
func (server *GameServer) attachRelay(replicaID string, broker *relay.Broker, leases *relay.LeaseManager, coordinator *relay.Coordinator) {
	server.replicaID = replicaID
	server.broker = broker
	server.leases = leases
	server.coordinator = coordinator
	server.registry = relay.NewRegistry()
	server.relayCtx, server.relayCancel = context.WithCancel(context.Background())
}

// shutdownRelay cancels every relay background goroutine (lease heartbeats,
// inbound consumers, edge subscriptions, join retries) for this server. Used on
// process shutdown and by tests to avoid leaking goroutines across cases.
func (server *GameServer) shutdownRelay() {
	if server.relayCancel != nil {
		server.relayCancel()
	}
}

// relayEnabled reports whether cross-replica coordination is active.
func (server *GameServer) relayEnabled() bool {
	return server.broker != nil && server.leases != nil
}

// presenceWriter marks users online/offline in a shared store (Redis) so the
// API can report who is currently connected. nil disables presence (tests /
// no-Redis); all calls are guarded by a nil check.
type presenceWriter interface {
	Online(ctx context.Context, userID, roomID string) error
	Offline(ctx context.Context, userID string) error
}

type room struct {
	id                string
	players           []*player
	state             game.GameState
	botDifficulty     game.BotDifficulty
	practiceMode      bool
	gameConfig        game.GameConfig
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
	rematchWindow     time.Duration
	rematchExpiresAt  time.Time
	rematchTimer      *time.Timer
	rematchTimerToken int
	kickedSubs        map[string]bool
	spectators        []*spectator
	gameDeltas        map[string]playerDelta
	savedGameID       string

	// Replay recording: the hands dealt at the start of the current game and
	// the ordered log of moves applied since the deal. Reset on every deal and
	// persisted in the room snapshot so an in-progress replay survives a WS
	// restart. Shipped to the API on game over.
	initialHands [][]game.Card
	moves        []recordedMove

	mu sync.Mutex

	// roomRelay is set when the server is running with cross-replica relay
	// enabled. nil means single-process mode, in which every send writes
	// directly to the local socket exactly as the original implementation did.
	relay *roomRelay
}

// roomRelay carries the per-room cross-replica coordination handle: the shared
// broker/registry plus this room's current ownership facts. It is attached when
// a room is created on a relay-enabled server. All sends funnel through
// room.deliver, which writes to local sockets via the registry and, when this
// replica owns the room, also publishes the envelope so other replicas' edges
// deliver to their local sockets.
type roomRelay struct {
	broker   *relay.Broker
	registry *relay.Registry
	leases   *relay.LeaseManager

	mu              sync.Mutex
	owner           bool
	token           int64 // fencing token from the last successful lease acquisition
	seq             int64 // monotonically increasing per-room outbound sequence
	consumerStarted bool
	stopped         bool
	inboundSub      *relay.Subscription
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
	avatar      string
	isGuest     bool
	isBot       bool
	ready       bool
	index       int
	team        int
}

// roomSnapshot is the complete durable state of a room, used to rebuild it
// after a WS process restart.
type roomSnapshot struct {
	state            game.GameState
	players          []persistedPlayer
	phase            roomPhase
	started          bool
	startedAt        time.Time
	turnExpiresAt    time.Time
	turnTimerSeconds int
	botDifficulty    game.BotDifficulty
	practiceMode     bool
	turnTimerToken   int
	rematchVotes     []int
	initialHands     [][]game.Card
	moves            []recordedMove
}

// roomSettingsStore supplies persisted room configuration from the API. The WS
// service owns live gameplay, but room creation options are stored by the API.
type roomSettingsStore interface {
	GetRoomSettings(roomID, token string) (roomSettings, error)
}

type roomSettings struct {
	TurnTimerSeconds int            `json:"turn_timer_seconds"`
	BotDifficulty    string         `json:"bot_difficulty"`
	PracticeMode     bool           `json:"practice_mode"`
	GameMode         string         `json:"game_mode"`
	MaxPlayers       int            `json:"max_players"`
	DeckCount        int            `json:"deck_count"`
	ScoringMode      string         `json:"scoring_mode"`
	CustomScores     map[game.Rank]int `json:"custom_scores,omitempty"`
	TeamMode         string         `json:"team_mode"`
}

func normalizeBotDifficulty(value string) game.BotDifficulty {
	switch game.BotDifficulty(strings.ToLower(strings.TrimSpace(value))) {
	case game.BotEasy:
		return game.BotEasy
	case game.BotMedium:
		return game.BotMedium
	case game.BotHard:
		return game.BotHard
	default:
		return game.BotMedium
	}
}

func gameConfigFromSettings(settings roomSettings) game.GameConfig {
	cfg := game.DefaultConfig()
	if settings.MaxPlayers > 0 {
		cfg.PlayerCount = settings.MaxPlayers
	}
	if settings.DeckCount > 0 {
		cfg.DeckCount = settings.DeckCount
	}
	if settings.ScoringMode != "" {
		cfg.ScoringMode = game.ScoringMode(settings.ScoringMode)
	}
	if settings.CustomScores != nil {
		cfg.CustomScores = settings.CustomScores
	}
	if settings.TeamMode != "" {
		cfg.TeamMode = game.TeamMode(settings.TeamMode)
	}
	return cfg
}

type gameHistoryStore interface {
	SaveGame(result savedGameResult) (string, []playerDelta, error)
}

type playerDelta struct {
	UserID      string `json:"user_id"`
	RatingDelta int    `json:"rating_delta"`
	RatingAfter int    `json:"rating_after"`
	XPDelta     int    `json:"xp_delta"`
	XPAfter     int64  `json:"xp_after"`
	Level       int    `json:"level"`
}

type savedGameResult struct {
	RoomID       string            `json:"room_id"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at"`
	Players      []savedGamePlayer `json:"players"`
	InitialHands [][]savedCard     `json:"initial_hands,omitempty"`
	Moves        []savedReplayMove `json:"moves,omitempty"`
}

type savedGamePlayer struct {
	UserID        string `json:"user_id,omitempty"`
	DisplayName   string `json:"display_name"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
	IsBot         bool   `json:"is_bot"`
}

// recordedMove captures a single applied move for replay. PlayerIndex is the
// seat 0..3, Suit/Rank identify the card, Type is one of "play", "face_down",
// or "ace_close", and AceDirection is "low" or "high" only for ace_close moves.
type recordedMove struct {
	PlayerIndex  int
	Suit         game.Suit
	Rank         game.Rank
	Type         string
	AceDirection game.CloseMethod
}

// Move type constants used by the replay system to describe how a card was
// played. Match the values stored in the API's game_moves.move_type column.
const (
	moveTypePlay     = "play"
	moveTypeFaceDown = "face_down"
	moveTypeAceClose = "ace_close"
)

// savedCard is the wire form of a card used in replay payloads. Suit is the
// engine string (spades/hearts/diamonds/clubs); Rank is the engine int (2..14).
type savedCard struct {
	Suit string `json:"suit"`
	Rank int    `json:"rank"`
}

func toSavedCards(cards []game.Card) []savedCard {
	out := make([]savedCard, len(cards))
	for i, c := range cards {
		out[i] = savedCard{Suit: string(c.Suit), Rank: int(c.Rank)}
	}
	return out
}

// savedReplayMove is the wire form of a recorded move. Index is its 0-based
// position in the game's move sequence.
type savedReplayMove struct {
	Index        int    `json:"index"`
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit"`
	Rank         int    `json:"rank"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

func toSavedMoves(moves []recordedMove) []savedReplayMove {
	out := make([]savedReplayMove, len(moves))
	for i, m := range moves {
		out[i] = savedReplayMove{
			Index:        i,
			PlayerIndex:  m.PlayerIndex,
			Suit:         string(m.Suit),
			Rank:         int(m.Rank),
			Type:         m.Type,
			AceDirection: string(m.AceDirection),
		}
	}
	return out
}

type apiGameHistoryStore struct {
	url    string
	client *http.Client
	secret string
}

type apiRoomSettingsStore struct {
	url    string
	client *http.Client
}

type memoryStateStore struct {
	mu        sync.Mutex
	snapshots map[string]roomSnapshot
}

type player struct {
	sub          string
	displayName  string
	avatar       string
	isGuest      bool
	isBot        bool
	ready        bool
	index        int
	team         int
	conn         *websocket.Conn
	disconnected bool
	leaveTimer   *time.Timer
	leaveToken   int
	lastEmoteAt  time.Time
	mu           sync.Mutex

	room *room
}

// spectator is a read-only viewer attached to a room. It holds a connection but
// no seat: spectators never enter room.players, never affect can_start / turn
// order / bot backfill / results / rematch, and are never persisted to the
// room snapshot. Their identity is kept only for logging/debugging.
//
// Spectators may emote (a purely cosmetic social action that never touches game
// state); id uniquely identifies the spectator connection in emote broadcasts
// (so the same sub can spectate from several tabs, and guests don't collide),
// and lastEmoteAt rate-limits those emotes per the spectatorEmoteCooldown.
type spectator struct {
	sub         string
	id          string
	conn        *websocket.Conn
	lastEmoteAt time.Time
	mu          sync.Mutex
}

// send writes a message to the spectator's socket, guarded by its own mutex so
// concurrent broadcasts don't interleave frames.
func (s *spectator) send(message map[string]any) {
	if s == nil || s.conn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.WriteJSON(message); err != nil {
		log.Printf("write spectator message: %v", err)
	}
}

// Send satisfies relay.Conn so the edge registry can fan an owner-published
// envelope out to this spectator's local socket.
func (s *spectator) Send(message map[string]any) { s.send(message) }

// deliverToSeat sends a per-seat payload to one player. player.send routes it:
// a local socket is written directly, a remote (edge-held) player is reached via
// a sub-targeted relay publish.
func (room *room) deliverToSeat(p *player, payload map[string]any) {
	if p == nil {
		return
	}
	p.send(payload)
}

// deliverToSpectators sends a payload to every spectator: local sockets write
// directly, and (owner + relay) a spectators-targeted envelope is published for
// remote edges.
func (room *room) deliverToSpectators(spectators []*spectator, payload map[string]any) {
	for _, s := range spectators {
		s.send(payload)
	}
	room.publishEnvelope(relay.Target{Kind: relay.TargetSpectators}, payload)
}

// publishEnvelope publishes an outbound envelope for remote edges when this
// replica owns the room under an active relay. A no-op in single-process mode.
func (room *room) publishEnvelope(target relay.Target, payload map[string]any) {
	rr := room.relay
	if rr == nil {
		return
	}
	rr.mu.Lock()
	if !rr.owner {
		rr.mu.Unlock()
		return
	}
	rr.seq++
	env := relay.Envelope{Seq: rr.seq, Target: target, Payload: payload}
	rr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rr.broker.PublishOutbound(ctx, room.id, env); err != nil {
		log.Printf("relay publish outbound room %s: %v", room.id, err)
	}
}

// isOwnerOrSolo reports whether this replica is responsible for authoritative
// side effects on the room: true in single-process mode (no relay), and true on
// the owning replica when the relay is active. A demoted owner (lost lease)
// returns false so it stops running timers / bot auto-play / result saves —
// fencing those effects to the live owner.
func (room *room) isOwnerOrSolo() bool {
	rr := room.relay
	if rr == nil {
		return true
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.owner
}

var orderedSuits = []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs}

type tokenClaims struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	IsGuest     bool   `json:"is_guest"`
	AvatarURL   string `json:"avatar_url"`
	jwt.RegisteredClaims
}

type clientMessage struct {
	Type   string `json:"type"`
	Suit   string `json:"suit"`
	Rank   string `json:"rank"`
	Method string `json:"method"`
	Ready  bool   `json:"ready"`
	Emote  string `json:"emote"`
	Target int    `json:"target"`
	Team   int    `json:"team"`
}

const (
	messageTypeError              = "error"
	messageTypeGameOver           = "game_over"
	messageTypePlaceFaceDown      = "place_facedown"
	messageTypePlayerDisconnected = "player_disconnected"
	messageTypePlayerReconnected  = "player_reconnected"
	messageTypeRematchCancelled   = "rematch_cancelled"
	messageTypeRematchStatus      = "rematch_status"
	messageTypeRematchCountdown   = "rematch_countdown"
	messageTypeRematchVote        = "rematch_vote"
	messageTypeGoToWaitingRoom    = "go_to_waiting_room"
	messageTypeRoomClosed         = "room_closed"
	messageTypePlayCard           = "play_card"
	messageTypeStateUpdate        = "state_update"
	messageTypeSpectatorState     = "spectator_state"
	messageTypeEmote              = "emote"
	messageTypeSpectatorEmote     = "spectator_emote"
)

// spectatorRole is the query-param value that opens a read-only spectator
// connection (ws://host/ws?room_id=X&token=JWT&role=spectator) instead of
// taking a seat.
const spectatorRole = "spectator"

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

// spectatorEmoteCooldown is the minimum gap between emotes from a single
// spectator. Spectators are an open, potentially large audience, so they get a
// stricter limit than seated players; faster emotes are silently dropped.
const spectatorEmoteCooldown = 2 * time.Second

// spectatorIDCounter assigns each spectator connection a process-unique id used
// to attribute emote broadcasts. The id only needs to disambiguate concurrent
// spectators of a room (so two anonymous/guest viewers don't collide); it is
// never persisted and is paired with the replica id to stay unique across the
// cluster.
var spectatorIDCounter atomic.Uint64

func (server *GameServer) nextSpectatorID() string {
	n := spectatorIDCounter.Add(1)
	if server.replicaID != "" {
		return server.replicaID + "-spec-" + strconv.FormatUint(n, 10)
	}
	return "spec-" + strconv.FormatUint(n, 10)
}

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
	var roomSettings roomSettingsStore
	if apiURL := strings.TrimRight(cfg.APIURL, "/"); apiURL != "" {
		historyStore = &apiGameHistoryStore{url: apiURL + "/internal/games", client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		statusUpdater = &apiRoomStatusUpdater{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		memberRemover = &apiRoomMemberRemover{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		reconciler = &apiRoomReconciler{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}, secret: cfg.InternalSecret}
		roomSettings = &apiRoomSettingsStore{url: apiURL, client: &http.Client{Timeout: 5 * time.Second}}
	}
	return &GameServer{
		jwtSecret:         cfg.JWTSecret,
		rooms:             map[string]*room{},
		store:             store,
		gameHistory:       historyStore,
		statusUpdater:     statusUpdater,
		memberRemover:     memberRemover,
		reconciler:        reconciler,
		roomSettings:      roomSettings,
		turnTimerDuration: turnTimerDuration,
		lobbyLeaveGrace:   defaultLobbyLeaveGrace,
		rematchWindow:     defaultRematchWindow,
		upgrader:          websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// reconcileInterval is how often the WS service reports its live room set to
// the API so orphaned 'waiting' rooms (no live presence) get cleaned up.
const reconcileInterval = 60 * time.Second

// presenceHeartbeat refreshes a connected user's presence TTL. It must be
// shorter than store.PresenceTTL so a still-connected user never lapses offline.
const presenceHeartbeat = 25 * time.Second

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
			server.reconcileOnce(ctx)
		}
	}
}

// reconcileOnce performs one reconcile pass. Single-process: report the local
// room set as before. Multi-replica: every replica publishes its owned rooms to
// the shared active-room set, but only the elected leader reports the union to
// the API — so reconciliation runs once cluster-wide and never treats another
// replica's rooms as orphaned.
func (server *GameServer) reconcileOnce(ctx context.Context) {
	if server.coordinator == nil {
		if err := server.reconciler.ReconcileRooms(server.activeRoomIDs()); err != nil {
			log.Printf("reconcile rooms: %v", err)
		}
		return
	}

	// Freshness window: a couple of reconcile intervals so a brief blip doesn't
	// drop a still-owned room from the union.
	pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	if err := server.coordinator.PublishActiveRooms(pubCtx, server.ownedRoomIDs(), 3*reconcileInterval); err != nil {
		log.Printf("publish active rooms: %v", err)
	}
	cancel()

	leadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	leader, err := server.coordinator.AcquireLeadership(leadCtx, 3*reconcileInterval)
	cancel()
	if err != nil {
		log.Printf("reconciler leadership: %v", err)
		return
	}
	if !leader {
		return
	}

	unionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	rooms, err := server.coordinator.ActiveRooms(unionCtx)
	cancel()
	if err != nil {
		log.Printf("read active rooms union: %v", err)
		return
	}
	if err := server.reconciler.ReconcileRooms(rooms); err != nil {
		log.Printf("reconcile rooms: %v", err)
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

// ownedRoomIDs snapshots the IDs of rooms this replica currently owns (relay
// active). Used to publish into the shared active-room set.
func (server *GameServer) ownedRoomIDs() []string {
	server.mu.Lock()
	rooms := make([]*room, 0, len(server.rooms))
	for _, r := range server.rooms {
		rooms = append(rooms, r)
	}
	server.mu.Unlock()
	ids := make([]string, 0, len(rooms))
	for _, r := range rooms {
		if r.isOwnerOrSolo() {
			ids = append(ids, r.id)
		}
	}
	return ids
}

func (store *apiRoomSettingsStore) GetRoomSettings(roomID, token string) (roomSettings, error) {
	req, err := http.NewRequest(http.MethodGet, store.url+"/rooms/"+roomID, nil)
	if err != nil {
		return roomSettings{}, err
	}
	// Reuse the player's JWT because the existing API room endpoint is protected
	// by normal user auth, not the internal-service secret.
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := store.client.Do(req)
	if err != nil {
		return roomSettings{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return roomSettings{}, fmt.Errorf("get room settings returned status %d", resp.StatusCode)
	}
	var settings roomSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return roomSettings{}, err
	}
	return settings, nil
}

func (store *apiGameHistoryStore) SaveGame(result savedGameResult) (string, []playerDelta, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequest(http.MethodPost, store.url, bytes.NewReader(payload))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setInternalSecret(req, store.secret)
	resp, err := store.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", nil, fmt.Errorf("save game returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var saveResp struct {
		GameID string        `json:"game_id"`
		Deltas []playerDelta `json:"deltas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&saveResp); err != nil {
		return "", nil, nil
	}
	return saveResp.GameID, saveResp.Deltas, nil
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
			Avatar:      p.avatar,
			IsGuest:     p.isGuest,
			IsBot:       p.isBot,
			Ready:       p.ready,
			Index:       p.index,
			Team:        p.team,
		})
	}
	initialHands := make([][]game.Card, len(snap.initialHands))
	for i := range snap.initialHands {
		if len(snap.initialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), snap.initialHands[i]...)
		}
	}
	var moves []store.PersistedMove
	if len(snap.moves) > 0 {
		moves = make([]store.PersistedMove, len(snap.moves))
		for i, m := range snap.moves {
			moves[i] = store.PersistedMove{
				PlayerIndex:  m.PlayerIndex,
				Suit:         string(m.Suit),
				Rank:         int(m.Rank),
				Type:         m.Type,
				AceDirection: string(m.AceDirection),
			}
		}
	}
	return store.RoomSnapshot{
		State:            snap.state,
		Players:          players,
		Phase:            int(snap.phase),
		Started:          snap.started,
		StartedAt:        snap.startedAt,
		TurnExpiresAt:    snap.turnExpiresAt,
		TurnTimerSeconds: snap.turnTimerSeconds,
		BotDifficulty:    string(snap.botDifficulty),
		PracticeMode:     snap.practiceMode,
		TurnTimerToken:   snap.turnTimerToken,
		RematchVotes:     append([]int(nil), snap.rematchVotes...),
		InitialHands:     initialHands,
		Moves:            moves,
	}
}

func fromStoreSnapshot(snap store.RoomSnapshot) roomSnapshot {
	players := make([]persistedPlayer, 0, len(snap.Players))
	for _, p := range snap.Players {
		players = append(players, persistedPlayer{
			sub:         p.Sub,
			displayName: p.DisplayName,
			avatar:      p.Avatar,
			isGuest:     p.IsGuest,
			isBot:       p.IsBot,
			ready:       p.Ready,
			index:       p.Index,
			team:        p.Team,
		})
	}
	initialHands := make([][]game.Card, len(snap.InitialHands))
	for i := range snap.InitialHands {
		if len(snap.InitialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), snap.InitialHands[i]...)
		}
	}
	var moves []recordedMove
	if len(snap.Moves) > 0 {
		moves = make([]recordedMove, len(snap.Moves))
		for i, m := range snap.Moves {
			moves[i] = recordedMove{
				PlayerIndex:  m.PlayerIndex,
				Suit:         game.Suit(m.Suit),
				Rank:         game.Rank(m.Rank),
				Type:         m.Type,
				AceDirection: game.CloseMethod(m.AceDirection),
			}
		}
	}
	return roomSnapshot{
		state:            snap.State,
		players:          players,
		phase:            roomPhase(snap.Phase),
		started:          snap.Started,
		startedAt:        snap.StartedAt,
		turnExpiresAt:    snap.TurnExpiresAt,
		turnTimerSeconds: snap.TurnTimerSeconds,
		botDifficulty:    normalizeBotDifficulty(snap.BotDifficulty),
		practiceMode:     snap.PracticeMode,
		turnTimerToken:   snap.TurnTimerToken,
		rematchVotes:     append([]int(nil), snap.RematchVotes...),
		initialHands:     initialHands,
		moves:            moves,
	}
}

func cloneGameState(state game.GameState) game.GameState {
	clone := game.GameState{
		Hands:         make([][]game.Card, len(state.Hands)),
		FaceDown:      make([][]game.Card, len(state.FaceDown)),
		Board:         make(map[game.Suit]game.SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[game.Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
		Config:        state.Config,
	}
	for player := range state.Hands {
		clone.Hands[player] = append([]game.Card(nil), state.Hands[player]...)
		clone.FaceDown[player] = append([]game.Card(nil), state.FaceDown[player]...)
	}
	for suit, sequence := range state.Board {
		cloned := game.SuitSequence{Low: sequence.Low, High: sequence.High}
		if len(sequence.Stacks) > 0 {
			cloned.Stacks = make(map[game.Rank]int, len(sequence.Stacks))
			for rank, count := range sequence.Stacks {
				cloned.Stacks[rank] = count
			}
		}
		clone.Board[suit] = cloned
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
			avatar:      p.avatar,
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
	initialHands := make([][]game.Card, len(room.initialHands))
	for i := range room.initialHands {
		if len(room.initialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), room.initialHands[i]...)
		}
	}
	return roomSnapshot{
		state:            cloneGameState(room.state),
		players:          players,
		phase:            room.phase,
		started:          room.started,
		startedAt:        room.startedAt,
		turnExpiresAt:    room.turnExpiresAt,
		turnTimerSeconds: int(room.turnTimerDuration / time.Second),
		botDifficulty:    room.botDifficulty,
		practiceMode:     room.practiceMode,
		turnTimerToken:   room.turnTimerToken,
		rematchVotes:     votes,
		initialHands:     initialHands,
		moves:            append([]recordedMove(nil), room.moves...),
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
	room.botDifficulty = snap.botDifficulty
	if room.botDifficulty == "" {
		room.botDifficulty = game.BotMedium
	}
	room.practiceMode = snap.practiceMode
	if snap.turnTimerSeconds > 0 {
		room.turnTimerDuration = time.Duration(snap.turnTimerSeconds) * time.Second
	}
	room.turnTimerToken = snap.turnTimerToken
	room.rematchVotes = map[int]bool{}
	for _, idx := range snap.rematchVotes {
		room.rematchVotes[idx] = true
	}
	room.initialHands = make([][]game.Card, len(snap.initialHands))
	for i := range snap.initialHands {
		if len(snap.initialHands[i]) > 0 {
			room.initialHands[i] = append([]game.Card(nil), snap.initialHands[i]...)
		} else {
			room.initialHands[i] = nil
		}
	}
	room.moves = append([]recordedMove(nil), snap.moves...)
	room.players = make([]*player, 0, len(snap.players))
	for _, p := range snap.players {
		room.players = append(room.players, &player{
			sub:          p.sub,
			displayName:  p.displayName,
			avatar:       p.avatar,
			isGuest:      p.isGuest,
			isBot:        p.isBot,
			ready:        p.ready,
			index:        p.index,
			team:         p.team,
			disconnected: !p.isBot,
		})
	}
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
	token := r.URL.Query().Get("token")
	claims, err := parseToken(token, server.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	if r.URL.Query().Get("role") == spectatorRole {
		// With the relay enabled, a spectator must be served as an edge unless
		// this replica owns the room, so it receives the owner's live envelopes
		// (state updates + spectator emotes) rather than a stale local snapshot.
		if server.relayEnabled() && !server.ownsOrCanServeSpectator(roomID) {
			server.handleEdgeSpectator(roomID, claims, conn)
			return
		}
		server.handleSpectator(roomID, claims, conn)
		return
	}

	// With the relay enabled, decide this replica's role for the room. If
	// another replica owns it, take the edge path: hold the socket locally and
	// proxy to the owner. Otherwise this replica owns the room and seats the
	// player authoritatively.
	var ownerToken int64
	var ownerNewly bool
	if server.relayEnabled() {
		owned, leaseToken, newly := server.acquireOwnership(roomID)
		if !owned {
			server.handleEdgePlayer(roomID, claims, conn, token)
			return
		}
		ownerToken = leaseToken
		ownerNewly = newly
	}

	room, player, joinResult, err := server.joinRoom(roomID, claims, conn, token)
	if err != nil {
		// A join rejection (kicked, room full, already started) is fatal for this
		// socket: flag it so the client routes the user back to the lobby with the
		// reason, instead of stranding them on an empty waiting room.
		if writeErr := conn.WriteJSON(fatalErrorMessage(err.Error())); writeErr != nil {
			log.Printf("write websocket join error: %v", writeErr)
		}
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("close websocket after join error: %v", closeErr)
		}
		return
	}
	if server.relayEnabled() {
		server.promoteToOwner(room, ownerToken, ownerNewly)
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
	// Mark the (registered) user online for the friends-presence feature. Guests
	// have no durable identity, so they're skipped. Presence lapses via TTL on
	// disconnect (a heartbeat refreshes it while connected), which avoids
	// flapping offline on a transient reconnect.
	stop := server.startPresence(claims, room)
	defer stop()
	room.readLoop(player)
}

// startPresence marks a registered user online and starts a heartbeat that
// refreshes the TTL until the returned stop func is called. A no-op (returning
// an empty stop) for guests or when presence is disabled. The presence value is
// the user's room id only once the game is actually in progress; in the lobby
// it's reported as "online but not in a game" (empty room id) so friends aren't
// shown a "Watch" link for a game that hasn't started.
func (server *GameServer) startPresence(claims *tokenClaims, room *room) func() {
	if server.presence == nil || claims.IsGuest || claims.Sub == "" {
		return func() {}
	}
	mark := func() {
		room.mu.Lock()
		roomID := ""
		if room.phase == phasePlaying {
			roomID = room.id
		}
		room.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.presence.Online(ctx, claims.Sub, roomID); err != nil {
			log.Printf("presence: mark online: %v", err)
		}
	}
	mark()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(presenceHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mark()
			}
		}
	}()
	return func() { close(done) }
}

type joinResult int

const (
	joinResultLobbyJoined joinResult = iota
	joinResultLobbyReconnected
	joinResultGameReconnected
	joinResultGameOver
)

func (server *GameServer) joinRoom(roomID string, claims *tokenClaims, conn *websocket.Conn, token string) (*room, *player, joinResult, error) {
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
func (room *room) seatLocked(claims *tokenClaims, conn *websocket.Conn) (*room, *player, joinResult, error) {
	if room.phase == phasePlaying {
		for _, existing := range room.players {
			if existing.sub == claims.Sub {
				existing.conn = conn
				existing.room = room
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

// handleSpectator attaches a spectator to a room this replica can serve
// authoritatively: either it owns the room, or the room is unowned and this
// replica rehydrates it from the durable store. Spectating requires an existing,
// in-progress (or finished) room — a spectator never creates a room or takes a
// seat. It immediately sends a redacted snapshot (or the game_over results if the
// game is already done), then reads the socket to detect disconnect and to honour
// the spectator's cosmetic emotes (gameplay frames are still ignored).
//
// Multi-replica: when another replica owns the room, the spectator is instead
// served as an edge (handleEdgeSpectator) so it receives the owner's live
// envelopes — state updates and spectator emotes — rather than a stale local
// snapshot. handleWebSocket routes to whichever path applies.
func (server *GameServer) handleSpectator(roomID string, claims *tokenClaims, conn *websocket.Conn) {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	if gameRoom == nil && server.store != nil {
		// Rehydrate a finished/in-progress room from the durable store so a
		// spectator can attach after a WS restart.
		if snap, ok := server.store.LoadRoom(roomID); ok {
			gameRoom = &room{
				id:                roomID,
				store:             server.store,
				gameHistory:       server.gameHistory,
				statusUpdater:     server.statusUpdater,
				memberRemover:     server.memberRemover,
				turnTimerDuration: server.turnTimerDuration,
				lobbyLeaveGrace:   server.lobbyLeaveGrace,
				rematchWindow:     server.rematchWindow,
				rematchVotes:      map[int]bool{},
				phase:             phaseLobby,
			}
			gameRoom.restoreFromSnapshotLocked(snap)
			server.rooms[roomID] = gameRoom
		}
	}
	server.mu.Unlock()

	if gameRoom == nil {
		if err := conn.WriteJSON(errorMessage("room not found")); err != nil {
			log.Printf("write spectator room-not-found: %v", err)
		}
		if err := conn.Close(); err != nil {
			log.Printf("close spectator after room-not-found: %v", err)
		}
		return
	}

	s := &spectator{sub: claims.Sub, id: server.nextSpectatorID(), conn: conn}

	gameRoom.mu.Lock()
	if gameRoom.phase != phasePlaying {
		// v1 only spectates an in-progress or finished game, not the lobby.
		gameRoom.mu.Unlock()
		if err := conn.WriteJSON(errorMessage("game has not started")); err != nil {
			log.Printf("write spectator not-started: %v", err)
		}
		if err := conn.Close(); err != nil {
			log.Printf("close spectator after not-started: %v", err)
		}
		return
	}
	gameRoom.spectators = append(gameRoom.spectators, s)
	gameOver := game.IsGameOver(gameRoom.state)
	var snapshot map[string]any
	if gameOver {
		snapshot = gameRoom.gameOverMessageLocked()
	} else {
		snapshot = gameRoom.spectatorStateMessageLocked()
	}
	gameRoom.mu.Unlock()

	s.send(snapshot)
	// Let seated players see the updated spectator count.
	if !gameOver {
		gameRoom.broadcastState()
	}

	// A spectator is also "online" for presence — they're watching, not seated.
	stop := server.startPresence(claims, gameRoom)
	defer stop()
	gameRoom.spectatorReadLoop(s)
}

// spectatorReadLoop reads the spectator socket until it closes (to detect
// disconnect) and dispatches the few inbound messages a spectator is allowed to
// send. Spectators remain read-only with respect to game state: the only
// inbound type honoured is "emote" (a purely cosmetic reaction); every other
// frame is ignored. On exit it removes the spectator and refreshes the seated
// players' spectator count.
func (room *room) spectatorReadLoop(s *spectator) {
	conn := s.conn
	defer func() {
		room.removeSpectator(s)
		if err := conn.Close(); err != nil {
			log.Printf("close spectator read loop: %v", err)
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			// Malformed frame: ignore it. Spectators can't affect the game, so
			// there's nothing actionable to report back.
			continue
		}
		if msg.Type == messageTypeEmote {
			room.handleSpectatorEmote(s, msg.Emote)
		}
		// Any other payload from a spectator is ignored; they cannot affect the
		// game.
	}
}

// handleSpectatorEmote validates and rate-limits a spectator's emote, then
// broadcasts it to everyone in the room. It mirrors handleEmote but uses the
// per-spectator cooldown and attributes the emote to the spectator's id rather
// than a seat. Emotes never touch game state.
func (room *room) handleSpectatorEmote(s *spectator, emote string) {
	if !allowedEmotes[emote] {
		// Spectators have no error toast surface today, but reply so a
		// misbehaving client still learns the id was rejected.
		s.send(errorMessage("unknown emote"))
		return
	}

	room.mu.Lock()
	now := time.Now()
	if !s.lastEmoteAt.IsZero() && now.Sub(s.lastEmoteAt) < spectatorEmoteCooldown {
		// Too soon after the last emote: silently drop to avoid spamming the
		// room with spectator reactions.
		room.mu.Unlock()
		return
	}
	s.lastEmoteAt = now
	spectatorID := s.id
	room.mu.Unlock()

	room.broadcastSpectatorEmote(spectatorID, emote)
}

// removeSpectator drops a spectator from the room and tells seated players the
// new count (unless the game is already over).
func (room *room) removeSpectator(s *spectator) {
	room.mu.Lock()
	found := false
	filtered := room.spectators[:0]
	for _, existing := range room.spectators {
		if existing == s {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	room.spectators = filtered
	gameOver := game.IsGameOver(room.state)
	room.mu.Unlock()
	if found && !gameOver {
		room.broadcastState()
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
		case messageTypeKick:
			room.handleKick(player, message.Target)
		case messageTypeLeave:
			room.handleLobbyLeave(player)
		case messageTypeSetTeam:
			room.handleSetTeam(player, message.Team)
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
	if message.Type == messageTypeGoToWaitingRoom {
		room.handleGoToWaitingRoomLocked(player)
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

	state, move, err := applyClientMessage(room.state, player.index, message)
	if err != nil {
		room.mu.Unlock()
		player.sendError(err.Error())
		return
	}
	room.state = state
	room.moves = append(room.moves, move)
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
	firstVote := len(room.rematchVotes) == 0
	room.rematchVotes[player.index] = true

	// The first vote opens a fixed countdown window. If every connected human
	// votes before it expires the rematch deals immediately; otherwise, on
	// expiry, the voters fall back to the waiting room (see handleRematchTimeout).
	if firstVote {
		room.startRematchTimerLocked()
	}

	// Bots never submit rematch votes, so the target is connected humans only.
	// Disconnected humans are also excluded because they cannot currently respond.
	if len(room.rematchVotes) < connectedHumanPlayerCountLocked(room.players) {
		countdown := room.rematchCountdownMessageLocked()
		room.mu.Unlock()
		if firstVote {
			room.broadcastRematchCountdown(countdown)
		}
		room.broadcastRematchStatus()
		return
	}

	// Everyone voted: cancel the countdown and start the new game right away.
	room.startRematchGameLocked()
	room.mu.Unlock()
	room.broadcastState()
	room.playBotIfNeeded()
}

// startRematchGameLocked deals a fresh game in the same room, refreshes the
// room's started timestamp, and flips the API room status back to in_progress
// so a mid-game refresh of game 2+ reconnects instead of being treated as a
// finished room. Caller holds room.mu; it does not release it.
func (room *room) startRematchGameLocked() {
	room.stopRematchTimerLocked()
	state, starter := game.DealWithConfig(time.Now().UnixNano(), room.gameConfig)
	room.state = state
	room.state.CurrentPlayer = starter
	room.initialHands = make([][]game.Card, len(room.state.Hands))
	for i := range room.state.Hands {
		room.initialHands[i] = append([]game.Card(nil), room.state.Hands[i]...)
	}
	room.moves = nil
	room.savedGameID = ""
	room.rematchVotes = map[int]bool{}
	room.startedAt = time.Now().UTC()
	room.persistLocked()
	room.startTurnTimerLocked()

	updater := room.statusUpdater
	roomID := room.id
	if updater != nil {
		go func() {
			if err := updater.UpdateRoomStatus(roomID, "in_progress"); err != nil {
				log.Printf("update room status to in_progress on rematch: %v", err)
			}
		}()
	}
}

// startRematchTimerLocked opens the rematch countdown window. A token guards
// against a stale timer firing after the window was cancelled or restarted.
// Caller holds room.mu.
func (room *room) startRematchTimerLocked() {
	if room.rematchTimer != nil {
		room.rematchTimer.Stop()
	}
	window := room.rematchWindow
	if window <= 0 {
		window = defaultRematchWindow
	}
	room.rematchExpiresAt = time.Now().Add(window).UTC()
	room.rematchTimerToken++
	token := room.rematchTimerToken
	room.rematchTimer = time.AfterFunc(window, func() {
		room.handleRematchTimeout(token)
	})
}

// stopRematchTimerLocked cancels any pending countdown and invalidates its
// token so a concurrently-firing timer becomes a no-op. Caller holds room.mu.
func (room *room) stopRematchTimerLocked() {
	if room.rematchTimer != nil {
		room.rematchTimer.Stop()
		room.rematchTimer = nil
	}
	room.rematchTimerToken++
	room.rematchExpiresAt = time.Time{}
}

// handleRematchTimeout fires when the countdown expires without a unanimous
// vote. Voters drop back to the waiting room (same room, lobby phase) and the
// non-voters are removed. If nobody voted the room is torn down.
func (room *room) handleRematchTimeout(token int) {
	room.mu.Lock()
	if token != room.rematchTimerToken {
		// Superseded by a unanimous vote or a restart; ignore.
		room.mu.Unlock()
		return
	}
	room.rematchTimer = nil
	if !game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}

	// Partition connected humans into voters and non-voters. Bots are dropped
	// regardless: a fresh waiting room re-fills bot seats only when the host
	// starts the next game.
	voters := make([]*player, 0, len(room.players))
	nonVoters := make([]*player, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		if p.disconnected {
			// A held-but-disconnected seat is treated as a non-voter, but it has
			// no live socket to notify; just let its grace timer/route handle it.
			continue
		}
		if room.rematchVotes[p.index] {
			voters = append(voters, p)
		} else {
			nonVoters = append(nonVoters, p)
		}
	}

	if len(voters) == 0 {
		// Nobody wants a rematch: tear the room down and send everyone home.
		players := connectedPlayersLocked(room.players)
		room.players = nil
		room.rematchVotes = map[int]bool{}
		roomID := room.id
		store := room.store
		room.mu.Unlock()
		room.deliverToPlayers(players, map[string]any{"type": messageTypeRoomClosed})
		if store != nil {
			store.DeleteRoom(roomID)
		}
		return
	}

	// Rebuild the roster from the voters and reset it to a fresh lobby (see
	// returnToWaitingRoomLocked). Non-voters are removed.
	room.returnToWaitingRoomLocked(voters, nonVoters)
}

// returnToWaitingRoomLocked resets the room to a fresh pre-game lobby containing
// only `keep`, removing `remove` (their seats + DB membership rows + a
// room_closed notice). Bots and any players not in `keep` are dropped from the
// roster. Index 0 becomes the host (implicitly ready); everyone else must ready
// up again. Caller holds room.mu; this function releases it and performs the
// off-lock notifications and broadcast.
func (room *room) returnToWaitingRoomLocked(keep, remove []*player) {
	room.stopRematchTimerLocked()

	remover := room.memberRemover
	roomID := room.id
	droppedSubs := make([]string, 0, len(remove))
	for _, p := range remove {
		if p.sub != "" && !p.isBot {
			droppedSubs = append(droppedSubs, p.sub)
		}
	}

	room.players = keep
	for i, p := range room.players {
		p.index = i
		p.ready = i == 0
	}
	room.phase = phaseLobby
	room.started = false
	room.state = game.GameState{}
	room.rematchVotes = map[int]bool{}
	room.rematchExpiresAt = time.Time{}
	room.persistLocked()

	updater := room.statusUpdater
	room.mu.Unlock()

	// Send the removed players away and drop their DB membership rows.
	for _, p := range remove {
		if !p.isBot {
			p.send(map[string]any{"type": messageTypeRoomClosed})
		}
	}
	if remover != nil {
		for _, sub := range droppedSubs {
			sub := sub
			go func() {
				if err := remover.RemoveRoomPlayer(roomID, sub); err != nil {
					log.Printf("remove player on waiting-room return: %v", err)
				}
			}()
		}
	}
	// Re-list the room as joinable again (it was 'finished' after game over).
	if updater != nil {
		go func() {
			if err := updater.UpdateRoomStatus(roomID, "waiting"); err != nil {
				log.Printf("update room status to waiting on waiting-room return: %v", err)
			}
		}()
	}
	room.broadcastLobbyState()
}

// handleGoToWaitingRoomLocked is invoked when a player chooses to drop back to
// the waiting room after a human left during the results screen (a full rematch
// is no longer possible). All currently-connected humans move to the fresh
// lobby together; bots and any humans who already left are dropped. Caller holds
// room.mu; this function releases it.
func (room *room) handleGoToWaitingRoomLocked(requester *player) {
	if !game.IsGameOver(room.state) {
		room.mu.Unlock()
		requester.sendError("only available after game over")
		return
	}
	keep := make([]*player, 0, len(room.players))
	remove := make([]*player, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		if p.disconnected {
			// A human who already left: drop their seat + DB membership row.
			remove = append(remove, p)
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		// No connected humans remain to seat a lobby; nothing to do.
		room.mu.Unlock()
		return
	}
	room.returnToWaitingRoomLocked(keep, remove)
}

func (room *room) broadcastRematchCountdown(message map[string]any) {
	room.mu.Lock()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, message)
}

func (room *room) rematchCountdownMessageLocked() map[string]any {
	expires := ""
	if !room.rematchExpiresAt.IsZero() {
		expires = room.rematchExpiresAt.Format(time.RFC3339Nano)
	}
	return map[string]any{
		"type":       messageTypeRematchCountdown,
		"expires_at": expires,
	}
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
	// Auto-play on timeout is authoritative; only the owner fires it.
	if !room.isOwnerOrSolo() {
		return
	}
	room.mu.Lock()
	if !room.started || token != room.turnTimerToken || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	playerIndex := room.state.CurrentPlayer
	botMove, ok := game.PickMoveWithDifficulty(room.state, playerIndex, room.botDifficulty)
	if !ok {
		room.mu.Unlock()
		return
	}
	state, rec, err := applyBotMove(room.state, playerIndex, botMove)
	if err != nil {
		log.Printf("auto-play move failed: %v", err)
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

func applyClientMessage(state game.GameState, playerIndex int, message clientMessage) (game.GameState, recordedMove, error) {
	switch message.Type {
	case messageTypePlayCard:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		if card.Rank == game.Ace {
			method, err := resolveAceCloseMethod(state, playerIndex, card.Suit, message.Method)
			if err != nil {
				return game.GameState{}, recordedMove{}, err
			}
			newState, err := game.ApplyAceClose(state, playerIndex, card.Suit, method)
			if err != nil {
				return game.GameState{}, recordedMove{}, err
			}
			return newState, recordedMove{
				PlayerIndex:  playerIndex,
				Suit:         card.Suit,
				Rank:         card.Rank,
				Type:         moveTypeAceClose,
				AceDirection: method,
			}, nil
		}
		newState, err := game.ApplyMove(state, playerIndex, card, false)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex: playerIndex,
			Suit:        card.Suit,
			Rank:        card.Rank,
			Type:        moveTypePlay,
		}, nil
	case messageTypePlaceFaceDown:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		newState, err := game.ApplyMove(state, playerIndex, card, true)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex: playerIndex,
			Suit:        card.Suit,
			Rank:        card.Rank,
			Type:        moveTypeFaceDown,
		}, nil
	default:
		return game.GameState{}, recordedMove{}, fmt.Errorf("unknown message type: %s", message.Type)
	}
}

func applyBotMove(state game.GameState, playerIndex int, move game.BotMove) (game.GameState, recordedMove, error) {
	if move.Close {
		newState, err := game.ApplyAceClose(state, playerIndex, move.Card.Suit, move.Method)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex:  playerIndex,
			Suit:         move.Card.Suit,
			Rank:         move.Card.Rank,
			Type:         moveTypeAceClose,
			AceDirection: move.Method,
		}, nil
	}
	moveType := moveTypePlay
	if move.FaceDown {
		moveType = moveTypeFaceDown
	}
	newState, err := game.ApplyMove(state, playerIndex, move.Card, move.FaceDown)
	if err != nil {
		return game.GameState{}, recordedMove{}, err
	}
	return newState, recordedMove{
		PlayerIndex: playerIndex,
		Suit:        move.Card.Suit,
		Rank:        move.Card.Rank,
		Type:        moveType,
	}, nil
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
		// Skip bots (never receive) and disconnected players. A connected
		// player whose socket lives on another replica has conn == nil here but
		// is not disconnected; deliverToSeat reaches them via the relay publish.
		if player.isBot || player.disconnected {
			continue
		}
		snapshots = append(snapshots, stateSnapshot{player: player, message: room.stateMessageFor(player.index)})
	}
	spectatorMsg := room.spectatorStateMessageLocked()
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()
	for _, snapshot := range snapshots {
		room.deliverToSeat(snapshot.player, snapshot.message)
	}
	room.deliverToSpectators(spectators, spectatorMsg)
}

// broadcastToSpectators fans a single message out to all current spectators,
// snapshotting the slice under the lock and sending off-lock (mirrors
// broadcastState's pattern). Used for game_over, emotes, and connection events.
func (room *room) broadcastToSpectators(message map[string]any) {
	room.mu.Lock()
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()
	room.deliverToSpectators(spectators, message)
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

	yourFaceDown := make([]map[string]any, 0, len(room.state.FaceDown[playerIndex]))
	for _, card := range room.state.FaceDown[playerIndex] {
		yourFaceDown = append(yourFaceDown, cardPayload(card, false))
	}

	opponents := make([]map[string]any, 0, len(room.players)-1)
	for i := 1; i < len(room.players); i++ {
		idx := (playerIndex + i) % len(room.players)
		player := room.players[idx]
		opponentPayload := map[string]any{
			"display_name":   player.displayName,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
			"disconnected":   player.disconnected,
			"team":           player.team,
		}
		if room.gameConfig.TeamMode == game.Team2v2 && player.team == room.players[playerIndex].team {
			opponentPayload["is_teammate"] = true
			teammateHand := make([]map[string]any, 0, len(room.state.Hands[player.index]))
			for _, card := range room.state.Hands[player.index] {
				teammateHand = append(teammateHand, map[string]any{
					"suit": string(card.Suit),
					"rank": rankString(card.Rank),
				})
			}
			opponentPayload["hand"] = teammateHand
		}
		opponents = append(opponents, opponentPayload)
	}

	var teamInfo map[string]any
	if room.gameConfig.TeamMode == game.Team2v2 {
		myTeam := room.players[playerIndex].team
		teamPenalty := 0
		for _, p := range room.players {
			if p.team == myTeam {
				for _, card := range room.state.FaceDown[p.index] {
					teamPenalty += game.ScoreCard(card, room.state)
				}
			}
		}
		teammates := make([]string, 0)
		for _, p := range room.players {
			if p.team == myTeam && p.index != playerIndex {
				teammates = append(teammates, p.displayName)
			}
		}
		teamInfo = map[string]any{
			"team":          myTeam,
			"team_penalty":  teamPenalty,
			"teammates":     teammates,
		}
	}

	payload := map[string]any{
		"type":                messageTypeStateUpdate,
		"status":              "in_progress",
		"board":               boardPayload(room.state),
		"closed_suits":        closedSuits(room.state),
		"ace_close_method":    room.state.CloseMethod,
		"ace_close_options":   aceCloseOptions,
		"your_hand":           yourHand,
		"your_facedown":       yourFaceDown,
		"your_facedown_count": len(yourFaceDown),
		"opponents":           opponents,
		"current_turn":        room.players[room.state.CurrentPlayer].displayName,
		"turn_ends_at":        room.turnExpiresAt.Format(time.RFC3339),
		"turn_timer_seconds":  int(room.turnTimerDuration / time.Second),
		"bot_difficulty":      string(room.botDifficulty),
		"practice_mode":       room.practiceMode,
		"spectator_count":     len(room.spectators),
	}
	if teamInfo != nil {
		payload["team_info"] = teamInfo
	}
	return payload
}

// spectatorStateMessageLocked builds the redacted live-state payload for
// spectators: the public board plus every player's public info (name, avatar,
// hand COUNT, face-down COUNT, disconnected) — but never any hand cards or
// hand-derived ace-close options. This is the core no-hidden-info-leak property.
// Caller must hold room.mu.
func (room *room) spectatorStateMessageLocked() map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		players = append(players, map[string]any{
			"display_name":   player.displayName,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
			"disconnected":   player.disconnected,
		})
	}
	currentTurn := ""
	if len(room.players) > 0 {
		currentTurn = room.players[room.state.CurrentPlayer].displayName
	}
	return map[string]any{
		"type":               messageTypeSpectatorState,
		"status":             "in_progress",
		"board":              boardPayload(room.state),
		"closed_suits":       closedSuits(room.state),
		"ace_close_method":   room.state.CloseMethod,
		"players":            players,
		"current_turn":       currentTurn,
		"turn_ends_at":       room.turnExpiresAt.Format(time.RFC3339),
		"turn_timer_seconds": int(room.turnTimerDuration / time.Second),
		"bot_difficulty":     string(room.botDifficulty),
		"practice_mode":      room.practiceMode,
		"spectator_count":    len(room.spectators),
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

// deliverToPlayers sends the same payload to an explicit set of players. Each
// is routed via deliverToSeat, so a local socket is written directly and a
// remote edge's player is reached by a seat-targeted publish. Using per-seat
// targeting (rather than a single TargetAll) keeps inclusion/exclusion correct
// for callers that broadcast to a subset (e.g. excluding the reconnecting
// player); a room has at most game.PlayerCount seats so the publish count is
// bounded.
func (room *room) deliverToPlayers(players []*player, payload map[string]any) {
	for _, p := range players {
		room.deliverToSeat(p, payload)
	}
}

// broadcastEmote fans an emote out to every connected human in the room,
// including the sender, so all clients render the bubble identically.
// Spectators receive it too so their view stays live.
func (room *room) broadcastEmote(displayName string, emote string) {
	room.mu.Lock()
	message := map[string]any{"type": messageTypeEmote, "display_name": displayName, "emote": emote}
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}

// broadcastSpectatorEmote fans a spectator's emote out to every connected human
// in the room — all seated players and all spectators (including the sender).
// It is tagged with a distinct message type and the spectator's id so clients
// can render spectator reactions separately from player emotes (different
// placement/colour, and players aggregate/throttle them). Spectator emotes are
// purely cosmetic and never touch game state, so this is never persisted.
func (room *room) broadcastSpectatorEmote(spectatorID string, emote string) {
	room.mu.Lock()
	message := map[string]any{"type": messageTypeSpectatorEmote, "spectator_id": spectatorID, "emote": emote}
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}

func (room *room) broadcastPlayerConnection(messageType string, displayName string, playerIndex int) {
	room.mu.Lock()
	message := map[string]any{"type": messageType, "display_name": displayName}
	players := make([]*player, 0, len(room.players))
	for _, player := range room.players {
		if player.index != playerIndex && !player.disconnected && !player.isBot {
			players = append(players, player)
		}
	}
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}

func (room *room) broadcastGameOver() {
	room.mu.Lock()
	room.rematchVotes = map[int]bool{}
	message := room.gameOverMessageLocked()
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()
	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
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
		"practice_mode":    room.practiceMode,
		"team_mode":        string(room.gameConfig.TeamMode),
		"spectator_count":  len(room.spectators),
		"game_id":          room.savedGameID,
	}
}
func (room *room) broadcastRematchStatus() {
	room.mu.Lock()
	message := room.rematchStatusMessageLocked()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, message)
}

func (room *room) broadcastRematchCancelled() {
	room.mu.Lock()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, map[string]any{"type": messageTypeRematchCancelled})
}

func (room *room) rematchStatusMessageLocked() map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		// The rematch panel is a human decision list. Bots can't vote, so they're
		// hidden entirely. A human who dropped during voting is kept but flagged
		// as "left" so the others can see they're no longer deciding.
		if player.isBot {
			continue
		}
		players = append(players, map[string]any{
			"display_name": player.displayName,
			"voted":        room.rematchVotes[player.index],
			"left":         player.disconnected,
		})
	}
	return map[string]any{
		"type":    messageTypeRematchStatus,
		"votes":   len(room.rematchVotes),
		"total":   humanPlayerCountLocked(room.players),
		"players": players,
	}
}

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

func (room *room) saveGameResult() {
	if room == nil {
		return
	}
	// Persisting the result is authoritative: only the owner does it, so a
	// failed-over replica can't double-save the same finished game.
	if !room.isOwnerOrSolo() {
		return
	}
	room.mu.Lock()
	result := room.savedResultLocked(time.Now().UTC())
	historyStore := room.gameHistory
	statusUpdater := room.statusUpdater
	practiceMode := room.practiceMode
	roomID := room.id
	room.mu.Unlock()
	// Practice games are solo vs bots: never recorded to history or stats, so a
	// practice round can't pollute the leaderboard. Status is still flipped to
	// 'finished' so the room can be reconciled/cleaned up like any other.
	if historyStore != nil && !practiceMode {
		gameID, deltas, err := historyStore.SaveGame(result)
		if err != nil {
			log.Printf("save game result: %v", err)
		} else {
			room.mu.Lock()
			room.savedGameID = gameID
			if len(deltas) > 0 {
				deltaMap := make(map[string]playerDelta, len(deltas))
				for _, d := range deltas {
					deltaMap[d.UserID] = d
				}
				room.gameDeltas = deltaMap
			}
			room.mu.Unlock()
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
			IsBot:         player.isBot,
		})
	}
	startedAt := room.startedAt
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	var initialHands [][]savedCard
	if len(room.moves) > 0 {
		initialHands = make([][]savedCard, len(room.initialHands))
		for i := range room.initialHands {
			initialHands[i] = toSavedCards(room.initialHands[i])
		}
	}
	return savedGameResult{
		RoomID:       room.id,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		Players:      players,
		InitialHands: initialHands,
		Moves:        toSavedMoves(room.moves),
	}
}

func (room *room) results() []map[string]any {
	scoredPlayers := room.scoredPlayersLocked()
	results := make([]map[string]any, 0, len(scoredPlayers))
	for _, scoredPlayer := range scoredPlayers {
		player := scoredPlayer.player
		entry := map[string]any{
			"display_name":   player.displayName,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"facedown_cards": revealedFaceDownCards(room.state, player.index),
			"penalty_points": scoredPlayer.score,
			"rank":           scoredPlayer.rank,
			"is_winner":      scoredPlayer.isWinner,
		}
		if room.gameConfig.TeamMode == game.Team2v2 {
			entry["team"] = player.team
		}
		if !player.isBot && !player.isGuest {
			if d, ok := room.gameDeltas[player.sub]; ok {
				entry["rating_delta"] = d.RatingDelta
				entry["rating_after"] = d.RatingAfter
				entry["xp_delta"] = d.XPDelta
				entry["xp_after"] = d.XPAfter
				entry["level"] = d.Level
			}
		}
		results = append(results, entry)
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
	sortedScores := append([]int(nil), scores...)
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
			"points": game.ScoreCard(card, state),
		})
	}
	return cards
}

func (player *player) send(message map[string]any) {
	if player == nil {
		return
	}
	// Remote player: socket lives on an edge replica. Route via the relay so the
	// edge delivers it locally. (Only reached when this replica owns the room
	// under an active relay; in single-process mode conn is always non-nil for a
	// connected player.)
	if player.conn == nil {
		if player.room != nil {
			player.room.publishEnvelope(relay.Target{Kind: relay.TargetSub, Sub: player.sub}, message)
		}
		return
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	if err := player.conn.WriteJSON(message); err != nil {
		log.Printf("write websocket message: %v", err)
	}
}

// Send satisfies relay.Conn so the edge registry can fan an owner-published
// envelope out to this player's local socket. It is the same write path as
// send; the distinct name keeps the relay interface explicit.
func (player *player) Send(message map[string]any) { player.send(message) }

func (player *player) sendError(message string) {
	player.send(errorMessage(message))
}

func errorMessage(message string) map[string]any {
	return map[string]any{"type": messageTypeError, "message": message}
}

// fatalErrorMessage marks an error that ends the connection (a rejected join),
// so the client can route the user away with the reason rather than treating it
// as a transient in-game error toast.
func fatalErrorMessage(message string) map[string]any {
	return map[string]any{"type": messageTypeError, "message": message, "fatal": true}
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
		payload := map[string]any{"low": sequence.Low, "high": sequence.High}
		if state.Config.DeckCount > 1 && len(sequence.Stacks) > 0 {
			stacks := map[string]int{}
			for rank, count := range sequence.Stacks {
				if count > 1 {
					stacks[rankString(rank)] = count
				}
			}
			if len(stacks) > 0 {
				payload["stacks"] = stacks
			}
		}
		board[string(suit)] = payload
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
