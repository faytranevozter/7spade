package room

import (
	"net/http/httptest"
	"testing"
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
