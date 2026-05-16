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

func connectPlayers(t *testing.T, baseURL, secret, roomID string, names []string) []*websocket.Conn {
	t.Helper()
	clients := make([]*websocket.Conn, 0, len(names))
	for _, name := range names {
		token := signTestToken(t, secret, name)
		conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
		if err != nil {
			closeClients(clients)
			t.Fatalf("dial %s: %v", name, err)
		}
		clients = append(clients, conn)
	}
	return clients
}

func signTestToken(t *testing.T, secret, displayName string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          displayName + "-id",
		"display_name": displayName,
		"is_guest":     true,
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

func testDependencyChecks() map[string]dependencyCheck {
	return map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	}
}
