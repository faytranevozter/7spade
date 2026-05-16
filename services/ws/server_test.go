package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

type memoryGameHistoryStore struct {
	results []savedGameResult
}

func (store *memoryGameHistoryStore) SaveGame(result savedGameResult) error {
	store.results = append(store.results, result)
	return nil
}

func TestWebSocketRoomStartsGameWhenFourthPlayerJoins(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-start", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		if got := len(message["your_hand"].([]any)); got != 13 {
			t.Fatalf("client %d got %d cards, want 13", index, got)
		}
		if got := len(message["opponents"].([]any)); got != 3 {
			t.Fatalf("client %d got %d opponents, want 3", index, got)
		}
	}
}

func TestWebSocketPlayCardRejectsOutOfTurnAndBroadcastsLegalMove(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-play", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	notStarter := (starter + 1) % len(clients)

	if err := clients[notStarter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write out-of-turn move: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[notStarter], "error")
	if errorMessage["message"] != "not your turn" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}

	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write legal move: %v", err)
	}
	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		board := message["board"].(map[string]any)
		spades := board["spades"].(map[string]any)
		if spades["low"].(float64) != 7 || spades["high"].(float64) != 7 {
			t.Fatalf("client %d unexpected spades board: %+v", index, spades)
		}
	}
}

func TestWebSocketPlayCardPersistsUpdatedRoomState(t *testing.T) {
	store := newMemoryStateStore()
	server := NewGameServerWithStateStore("test-secret", store)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-persist", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write legal move: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "state_update")
	}

	saved, ok := store.Load("room-persist")
	if !ok {
		t.Fatal("expected room state to be persisted")
	}
	spades := saved.Board["spades"]
	if spades.Low != 7 || spades.High != 7 {
		t.Fatalf("unexpected persisted spades board: %+v", spades)
	}
	if hasGameCard(saved.Hands[starter], "spades", "7") {
		t.Fatal("persisted hand still contains played seven of spades")
	}
}

func TestWebSocketTurnTimerExpiryAutoPlaysAndBroadcastsStateUpdate(t *testing.T) {
	server := NewGameServerWithOptions("test-secret", newMemoryStateStore(), 20*time.Millisecond)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-autoplay", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	message := readTypedMessage(t, clients[starter], "state_update")
	if hasCard(message, "spades", "7") {
		t.Fatal("timer expiry did not auto-play the seven of spades")
	}
	board := message["board"].(map[string]any)
	spades := board["spades"].(map[string]any)
	if spades["low"].(float64) != 7 || spades["high"].(float64) != 7 {
		t.Fatalf("unexpected spades board after auto-play: %+v", spades)
	}
}

func TestWebSocketDisconnectActivatesBotAndBroadcastsDisconnect(t *testing.T) {
	server := NewGameServerWithOptions("test-secret", newMemoryStateStore(), 20*time.Millisecond)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-disconnect", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	if err := clients[starter].Close(); err != nil {
		t.Fatalf("close starter connection: %v", err)
	}

	observer := clients[(starter+1)%len(clients)]
	disconnect := readTypedMessage(t, observer, "player_disconnected")
	if disconnect["display_name"] == "" {
		t.Fatalf("disconnect event missing display name: %+v", disconnect)
	}

	update := readTypedMessage(t, observer, "state_update")
	if update["current_turn"] == disconnect["display_name"] {
		t.Fatalf("disconnected starter did not auto-play on their timer: %+v", update)
	}
	opponents := update["opponents"].([]any)
	if !opponentDisconnected(opponents, disconnect["display_name"].(string)) {
		t.Fatalf("state update did not mark disconnected opponent: %+v", opponents)
	}
}

func TestWebSocketReconnectDeactivatesBotAndRestoresPlayerControl(t *testing.T) {
	server := NewGameServerWithOptions("test-secret", newMemoryStateStore(), time.Hour)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	names := []string{"Alice", "Bob", "Carol", "Dave"}
	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-reconnect", names)
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	if err := clients[starter].Close(); err != nil {
		t.Fatalf("close starter connection: %v", err)
	}
	observer := clients[(starter+1)%len(clients)]
	readTypedMessage(t, observer, "player_disconnected")

	reconnected := connectPlayer(t, httpServer.URL, "test-secret", "room-reconnect", names[starter])
	defer reconnected.Close()

	reconnect := readTypedMessage(t, observer, "player_reconnected")
	if reconnect["display_name"] == "" {
		t.Fatalf("reconnect event missing display name: %+v", reconnect)
	}
	update := readTypedMessage(t, reconnected, "state_update")
	if !hasCard(update, "spades", "7") {
		t.Fatalf("reconnected active player did not regain manual control: %+v", update)
	}
}

func TestWebSocketPlaceFaceDownRejectsWhenValidMoveExists(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-facedown", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "place_facedown", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write face-down move: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[starter], "error")
	if errorMessage["message"] != "cannot place face-down while a legal play is available" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}
}

func TestRoomResultsIncludeRevealedFaceDownCardsWithPointValues(t *testing.T) {
	room := &room{
		players: []*player{
			{displayName: "Alice", index: 0},
			{displayName: "Bob", index: 1},
			{displayName: "Carol", index: 2},
			{displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.CloseMethod = game.CloseLow
	room.state.FaceDown[0] = []game.Card{{Suit: game.Hearts, Rank: game.Ace}, {Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Nine}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Clubs, Rank: game.Ten}}

	results := room.results()
	alice := results[0]
	if alice["penalty_points"] != 6 {
		t.Fatalf("expected Alice score 6, got %+v", alice)
	}
	cards := alice["facedown_cards"].([]map[string]any)
	if len(cards) != 2 {
		t.Fatalf("expected two revealed cards, got %+v", cards)
	}
	if cards[0]["rank"] != "A" || cards[0]["suit"] != "hearts" || cards[0]["points"] != 1 {
		t.Fatalf("unexpected revealed ace payload: %+v", cards[0])
	}
	if cards[1]["rank"] != "5" || cards[1]["suit"] != "clubs" || cards[1]["points"] != 5 {
		t.Fatalf("unexpected revealed five payload: %+v", cards[1])
	}
	if alice["rank"] != 1 || alice["is_winner"] != true || results[1]["rank"] != 1 || results[1]["is_winner"] != true {
		t.Fatalf("expected Alice and Bob to share rank 1: %+v", results)
	}
}

func TestRoomResultsRanksPlayersByPenaltyTotalWithSkippedTieRanks(t *testing.T) {
	room := &room{
		players: []*player{
			{displayName: "Alice", index: 0},
			{displayName: "Bob", index: 1},
			{displayName: "Carol", index: 2},
			{displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Nine}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}

	results := room.results()

	if results[0]["rank"] != 1 || results[1]["rank"] != 1 || results[2]["rank"] != 3 || results[3]["rank"] != 4 {
		t.Fatalf("expected competition ranks 1, 1, 3, 4, got %+v", results)
	}
}

func TestWebSocketBroadcastsGameOverAfterFinalMove(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-game-over", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	room := server.rooms["room-game-over"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Seven, High: game.Seven}
	room.state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "6"}); err != nil {
		t.Fatalf("write final move: %v", err)
	}

	for index, client := range clients {
		message := readTypedMessage(t, client, "game_over")
		results := message["results"].([]any)
		if len(results) != 4 {
			t.Fatalf("client %d expected four results, got %+v", index, message)
		}
		alice := results[0].(map[string]any)
		if alice["display_name"] != "Alice" || alice["penalty_points"] != float64(5) || alice["rank"] != float64(1) || alice["is_winner"] != true {
			t.Fatalf("client %d unexpected Alice result: %+v", index, alice)
		}
		bob := results[1].(map[string]any)
		if bob["rank"] != float64(1) || bob["is_winner"] != true {
			t.Fatalf("client %d expected Bob to share winner rank: %+v", index, bob)
		}
		cards := alice["facedown_cards"].([]any)
		if len(cards) != 1 {
			t.Fatalf("client %d expected Alice revealed card, got %+v", index, alice)
		}
		card := cards[0].(map[string]any)
		if card["rank"] != "5" || card["suit"] != "clubs" || card["points"] != float64(5) {
			t.Fatalf("client %d unexpected revealed card: %+v", index, card)
		}
	}
}

func TestWebSocketRematchVotesStartNewGameInSameRoom(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	forceGameOverRoom(t, server.rooms["room-rematch"])
	server.rooms["room-rematch"].broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	for voter, client := range clients {
		if err := client.WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
			t.Fatalf("write rematch vote %d: %v", voter, err)
		}
		if voter < len(clients)-1 {
			for observer, observerClient := range clients {
				message := readTypedMessage(t, observerClient, "rematch_status")
				if message["votes"] != float64(voter+1) || message["total"] != float64(game.PlayerCount) {
					t.Fatalf("observer %d unexpected rematch status after vote %d: %+v", observer, voter, message)
				}
				if !rematchStatusIncludesVote(message, "Alice") {
					t.Fatalf("observer %d status missing per-player vote details: %+v", observer, message)
				}
			}
		}
	}

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected rematch state update, got %+v", index, message)
		}
		if got := len(message["your_hand"].([]any)); got != 13 {
			t.Fatalf("client %d got %d rematch cards, want 13", index, got)
		}
	}
}

func TestWebSocketDisconnectCancelsPendingRematch(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch-cancel", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	forceGameOverRoom(t, server.rooms["room-rematch-cancel"])
	server.rooms["room-rematch-cancel"].broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	if err := clients[0].WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatalf("write first rematch vote: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "rematch_status")
	}

	if err := clients[1].Close(); err != nil {
		t.Fatalf("close rematch voter: %v", err)
	}

	for index, client := range clients[2:] {
		message := readTypedMessage(t, client, "rematch_cancelled")
		if message["type"] != "rematch_cancelled" {
			t.Fatalf("observer %d expected rematch cancellation, got %+v", index, message)
		}
	}
}

func TestWebSocketSavesGameResultAfterFinalMove(t *testing.T) {
	history := &memoryGameHistoryStore{}
	server := NewGameServerWithOptions("test-secret", newMemoryStateStore(), time.Hour)
	server.gameHistory = history
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-save-game", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	room := server.rooms["room-save-game"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Seven, High: game.Seven}
	room.state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Four}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Nine}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Ten}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "6"}); err != nil {
		t.Fatalf("write final move: %v", err)
	}
	readTypedMessage(t, clients[0], "game_over")

	if len(history.results) != 1 {
		t.Fatalf("expected one saved game result, got %+v", history.results)
	}
	result := history.results[0]
	if result.RoomID != "room-save-game" || result.StartedAt.IsZero() || result.FinishedAt.IsZero() {
		t.Fatalf("saved result missing game metadata: %+v", result)
	}
	if len(result.Players) != 4 {
		t.Fatalf("expected four saved players, got %+v", result.Players)
	}
	if result.Players[0].UserID != "Alice-id" || result.Players[0].DisplayName != "Alice" || result.Players[0].PenaltyPoints != 4 || result.Players[0].Rank != 1 || !result.Players[0].IsWinner {
		t.Fatalf("unexpected saved Alice result: %+v", result.Players[0])
	}
}

func TestSavedGameResultOmitsGuestUserIDs(t *testing.T) {
	room := &room{
		id:        "room-guests",
		startedAt: time.Now().UTC().Add(-15 * time.Minute),
		players: []*player{
			{sub: "Alice-id", displayName: "Alice", index: 0},
			{sub: "Guest-id", displayName: "Guest", isGuest: true, index: 1},
			{sub: "Carol-id", displayName: "Carol", index: 2},
			{sub: "Dave-id", displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Four}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Nine}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Ten}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}

	result := room.savedResultLocked(time.Now().UTC())

	if result.Players[0].UserID != "Alice-id" {
		t.Fatalf("expected authenticated player user id to be saved, got %+v", result.Players[0])
	}
	if result.Players[1].UserID != "" || result.Players[1].DisplayName != "Guest" {
		t.Fatalf("expected guest player display name without user id, got %+v", result.Players[1])
	}
}

func TestWebSocketUnknownMessageTypeReturnsTypeError(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-unknown", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "dance"}); err != nil {
		t.Fatalf("write unknown message: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[starter], "error")
	if errorMessage["message"] != "unknown message type: dance" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}
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
	return clients
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

func signTestToken(t *testing.T, secret, displayName string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          displayName + "-id",
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

func readTypedMessage(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
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
	if message["type"] != wantType {
		t.Fatalf("message type = %v, want %s: %+v", message["type"], wantType, message)
	}
	return message
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
