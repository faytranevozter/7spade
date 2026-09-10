package room

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/internal/protocol"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var ErrAccessDenied = protocol.ErrAccessDenied

var requiredWSControls = []string{controlNewGameStarts, controlSpectatorAccess, controlEmotes}

type staticApplicationControls map[string]bool

func (c staticApplicationControls) Enabled(key string) bool { return c[key] }

type memoryGameHistoryStore struct {
	results []savedGameResult
}

func (store *memoryGameHistoryStore) SaveGame(result savedGameResult) (string, []playerDelta, error) {
	store.results = append(store.results, result)
	return "", nil, nil
}

type rejectingAccessChecker struct{}

func (rejectingAccessChecker) CheckAccess(string) error { return context.Canceled }

type delayedRejectingAccessChecker struct {
	mu    sync.Mutex
	calls int
}

func (checker *delayedRejectingAccessChecker) CheckAccess(string) error {
	checker.mu.Lock()
	defer checker.mu.Unlock()
	checker.calls++
	if checker.calls > 2 {
		return ErrAccessDenied
	}
	return nil
}

func (checker *delayedRejectingAccessChecker) callCount() int {
	checker.mu.Lock()
	defer checker.mu.Unlock()
	return checker.calls
}

type staticRoomSettingsStore struct {
	settings roomSettings
}

func (store staticRoomSettingsStore) GetRoomSettings(string, string) (roomSettings, error) {
	return store.settings, nil
}

type removeCall struct {
	roomID string
	userID string
	kick   bool
}

type capturingMemberRemover struct {
	calls chan removeCall
}

func (r *capturingMemberRemover) RemoveRoomPlayer(roomID, userID string) error {
	r.calls <- removeCall{roomID: roomID, userID: userID}
	return nil
}

func (r *capturingMemberRemover) KickRoomPlayer(roomID, userID string) error {
	r.calls <- removeCall{roomID: roomID, userID: userID, kick: true}
	return nil
}

type statusCall struct {
	roomID string
	status string
}

type capturingStatusUpdater struct {
	calls chan statusCall
}

func (u *capturingStatusUpdater) UpdateRoomStatus(roomID, status string) error {
	select {
	case u.calls <- statusCall{roomID: roomID, status: status}:
	default:
	}
	return nil
}

// readEmoteOptional reads the next message within the window, returning ok=false
// when nothing arrives (a read deadline timeout). Used to assert an emote was
// dropped without a flaky fixed sleep.
func readEmoteOptional(t *testing.T, conn *websocket.Conn, within time.Duration) (map[string]any, bool) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(within)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return nil, false
	}
	var message map[string]any
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("decode message %s: %v", payload, err)
	}
	return message, true
}

// waitForLobbyPlayerCount reads lobby_state messages until one lists exactly
// want players, confirming all of them are registered in the room.
func waitForLobbyPlayerCount(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read while waiting for lobby player count: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		if message["type"] == "lobby_state" {
			if players, ok := message["players"].([]any); ok && len(players) == want {
				_ = conn.SetReadDeadline(time.Time{})
				return
			}
		}
	}
}

// dialSpectator opens a read-only spectator connection to a room.
func dialSpectator(t *testing.T, baseURL, secret, roomID, name string) *websocket.Conn {
	t.Helper()
	token := signTestToken(t, secret, name)
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token+"&role=spectator", nil)
	if err != nil {
		t.Fatalf("dial spectator: %v", err)
	}
	return conn
}

// readSpectatorUntil reads frames until one of wantType arrives, skipping
// intervening spectator_state updates (bot auto-plays can emit several).
func readSpectatorUntil(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode %s: %v", payload, err)
		}
		if message["type"] == wantType {
			return message
		}
		if message["type"] == "spectator_state" {
			continue
		}
		t.Fatalf("unexpected message type %v, want %s", message["type"], wantType)
	}
	t.Fatalf("timed out waiting for %s", wantType)
	return nil
}

// readUntilTypeOptional reads frames (skipping state_update / lobby_state
// heartbeats) until one of wantType arrives or the deadline passes. Used by the
// spectator-emote tests where a spectator join triggers a state_update refresh
// to seated players just before the emote broadcast.
func readUntilTypeOptional(t *testing.T, conn *websocket.Conn, wantType string, within time.Duration) (map[string]any, bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return nil, false
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		switch message["type"] {
		case wantType:
			return message, true
		case "state_update", "spectator_state", "lobby_state", "player_connected", "player_reconnected":
			continue
		default:
			continue
		}
	}
	return nil, false
}

func aceCloseTestState() game.GameState {
	state := game.NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Ace}}
	state.Board[game.Spades] = game.SuitSequence{Low: game.Two, High: game.King}
	return state
}

func readInitialUpdatesAndFindStarter(t *testing.T, clients []*websocket.Conn) int {
	t.Helper()

	starter := -1
	for index, client := range clients {
		update := readTypedMessage(t, client, "state_update")
		if hasCard(update, "spades", "7") {
			starter = index
		}
	}
	if starter == -1 {
		t.Fatal("no player received seven of spades")
	}
	return starter
}

func forceGameOverRoom(t *testing.T, room *room) {
	t.Helper()
	room.mu.Lock()
	defer room.mu.Unlock()
	room.state = game.NewGameState()
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
}

func rematchStatusIncludesVote(message map[string]any, displayName string) bool {
	players, ok := message["players"].([]any)
	if !ok {
		return false
	}
	for _, rawPlayer := range players {
		player := rawPlayer.(map[string]any)
		if player["display_name"] == displayName {
			return player["voted"] == true
		}
	}
	return false
}

func connectPlayers(t *testing.T, baseURL, secret, roomID string, names []string) []*websocket.Conn {
	t.Helper()
	clients := make([]*websocket.Conn, 0, len(names))
	for _, name := range names {
		clients = append(clients, connectPlayer(t, baseURL, secret, roomID, name))
	}
	startGameAndDrainLobby(t, clients)
	return clients
}

// startGameAndDrainLobby reads any pending lobby_state messages emitted as
// players join, marks every non-host player ready, waits until the host sees
// can_start=true, then asks the host to start the game. After this returns,
// each client's next message will be the initial state_update.
func startGameAndDrainLobby(t *testing.T, clients []*websocket.Conn) {
	t.Helper()
	if len(clients) == 0 {
		return
	}
	host := clients[0]
	// Give each seat's readLoop a moment to start before ready-up.
	time.Sleep(50 * time.Millisecond)
	for index, client := range clients {
		if index == 0 {
			continue
		}
		if err := client.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready %d: %v", index, err)
		}
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
}

func waitForLobbyCanStart(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	// Single deadline for the whole wait. Do NOT re-enter ReadMessage after a
	// timeout — gorilla/websocket panics on "repeated read on failed connection".
	if err := conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read while waiting for can_start: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		if message["type"] == "lobby_state" && message["can_start"] == true {
			_ = conn.SetReadDeadline(time.Time{})
			return
		}
	}
}

func connectPlayer(t *testing.T, baseURL, secret, roomID string, name string) *websocket.Conn {
	t.Helper()
	token := signTestToken(t, secret, name)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	return conn
}

func connectPlayerWithSub(t *testing.T, baseURL, secret, roomID string, sub string, name string) *websocket.Conn {
	t.Helper()
	token := signTestTokenWithSub(t, secret, sub, name)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial %s/%s: %v", sub, name, err)
	}
	return conn
}

func signTestToken(t *testing.T, secret, displayName string) string {
	t.Helper()
	return signTestTokenWithSub(t, secret, displayName+"-id", displayName)
}

func signTestTokenWithSub(t *testing.T, secret, sub, displayName string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          sub,
		"display_name": displayName,
		"is_guest":     false,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func signTestTokenWithAvatar(t *testing.T, secret, displayName, avatarURL string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          displayName + "-id",
		"display_name": displayName,
		"is_guest":     false,
		"avatar_url":   avatarURL,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func assertTurnEndsNear(t *testing.T, message map[string]any, want time.Duration) {
	t.Helper()
	raw, ok := message["turn_ends_at"].(string)
	if !ok || raw == "" {
		t.Fatalf("state update missing turn_ends_at: %+v", message)
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse turn_ends_at %q: %v", raw, err)
	}
	remaining := time.Until(expiresAt)
	if remaining < want-5*time.Second || remaining > want+5*time.Second {
		t.Fatalf("turn timer remaining %v, want near %v in %+v", remaining, want, message)
	}
}

// dialPlayer opens a WebSocket with a caller-supplied token (e.g. one carrying
// an avatar claim), unlike connectPlayer which signs a plain token by name.
func dialPlayer(t *testing.T, baseURL, roomID, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func readTypedMessage(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket message: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		// Skip lobby_state heartbeats so existing tests don't need to be aware
		// of the lobby phase that precedes the first state_update.
		if message["type"] == "lobby_state" && wantType != "lobby_state" {
			continue
		}
		// Skip the rematch countdown banner that now precedes the first
		// rematch_status so vote-flow tests don't each need to consume it.
		if message["type"] == "rematch_countdown" && wantType != "rematch_countdown" {
			continue
		}
		// Late set_ready after start_game can produce an error frame; ignore
		// when the caller is waiting for a different event.
		if message["type"] == "error" && wantType != "error" {
			continue
		}
		if message["type"] != wantType {
			t.Fatalf("message type = %v, want %s: %+v", message["type"], wantType, message)
		}
		return message
	}
	t.Fatalf("timed out waiting for %s", wantType)
	return nil
}

func hasCard(update map[string]any, suit string, rank string) bool {
	for _, rawCard := range update["your_hand"].([]any) {
		card := rawCard.(map[string]any)
		if card["suit"] == suit && card["rank"] == rank {
			return true
		}
	}
	return false
}

func hasGameCard(cards []game.Card, suit string, rank string) bool {
	card, err := parseCard(suit, rank)
	if err != nil {
		return false
	}
	for _, candidate := range cards {
		if candidate == card {
			return true
		}
	}
	return false
}

func closeClients(clients []*websocket.Conn) {
	for _, client := range clients {
		_ = client.Close()
	}
}

func opponentDisconnected(opponents []any, displayName string) bool {
	for _, rawOpponent := range opponents {
		opponent := rawOpponent.(map[string]any)
		if opponent["display_name"] == displayName {
			return opponent["disconnected"] == true
		}
	}
	return false
}

func testDependencyChecks() map[string]dependencyCheck {
	return map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	}
}

// --- Lobby phase tests ---------------------------------------------------

type capturingReconciler struct {
	calls chan []string
}

func (r *capturingReconciler) ReconcileRooms(activeRoomIDs []string) error {
	r.calls <- append([]string(nil), activeRoomIDs...)
	return nil
}
