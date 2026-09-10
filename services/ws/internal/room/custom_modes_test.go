package room

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketCustomGame6PlayersStartsAndDeals(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       6,
		DeckCount:        1,
		ScoringMode:      "rank_value",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-6p", []string{"Alice", "Bob", "Carol", "Dave", "Eve", "Frank"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		hand := message["your_hand"].([]any)
		if len(hand) == 0 {
			t.Fatalf("client %d got 0 cards", index)
		}
		opponents := message["opponents"].([]any)
		if len(opponents) != 5 {
			t.Fatalf("client %d got %d opponents, want 5", index, len(opponents))
		}
	}
}

func TestWebSocketCustomGameDoubleDeck(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        2,
		ScoringMode:      "rank_value",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-2deck", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		hand := message["your_hand"].([]any)
		if len(hand) != 26 {
			t.Fatalf("client %d got %d cards, want 26", index, len(hand))
		}
	}
}

func TestWebSocketCustomGameFlatScoring(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        1,
		ScoringMode:      "flat",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-flat", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
	}
}

func TestWebSocketCustomGameTeamMode(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        1,
		ScoringMode:      "rank_value",
		TeamMode:         "2v2",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := make([]*websocket.Conn, 0, 4)
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	for _, name := range names {
		clients = append(clients, connectPlayer(t, httpServer.URL, "test-secret", "room-team", name))
	}
	for i := 2; i < 4; i++ {
		if err := clients[i].WriteJSON(map[string]any{"type": "set_team", "team": 1}); err != nil {
			t.Fatalf("write set_team %d: %v", i, err)
		}
	}
	for i, client := range clients {
		if i == 0 {
			continue
		}
		if err := client.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready %d: %v", i, err)
		}
	}
	waitForLobbyCanStart(t, clients[0])
	if err := clients[0].WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
	}
}
