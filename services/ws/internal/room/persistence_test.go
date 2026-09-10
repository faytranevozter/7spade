package room

import (
	"net/http/httptest"
	"testing"
)

func TestActiveRoomIDsSnapshotsInMemoryRooms(t *testing.T) {
	server := NewGameServer("test-secret")
	server.rooms["room-a"] = &room{id: "room-a"}
	server.rooms["room-b"] = &room{id: "room-b"}

	ids := server.activeRoomIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 active room ids, got %+v", ids)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen["room-a"] || !seen["room-b"] {
		t.Fatalf("expected room-a and room-b in active set, got %+v", ids)
	}
}

func TestWebSocketRehydratesInProgressGameAfterRestart(t *testing.T) {
	// A shared store stands in for Redis surviving a process restart.
	sharedStore := newMemoryStateStore()

	// First "process": four players start a game, then everyone disconnects.
	server1 := NewGameServerWithStateStore("test-secret", sharedStore)
	http1 := httptest.NewServer(server1.routes(testDependencyChecks()))
	clients := connectPlayers(t, http1.URL, "test-secret", "room-restart", []string{"Alice", "Bob", "Carol", "Dave"})
	readInitialUpdatesAndFindStarter(t, clients)
	closeClients(clients)
	http1.Close()

	// Second "process": a brand-new server backed by the same store. Alice
	// reconnects and must land back in the in-progress game (state_update),
	// not a fresh lobby.
	server2 := NewGameServerWithStateStore("test-secret", sharedStore)
	http2 := httptest.NewServer(server2.routes(testDependencyChecks()))
	defer http2.Close()

	alice := connectPlayer(t, http2.URL, "test-secret", "room-restart", "Alice")
	defer alice.Close()

	msg := readTypedMessage(t, alice, "state_update")
	if msg["status"] != "in_progress" {
		t.Fatalf("expected in_progress after restart, got %+v", msg)
	}
	hand, ok := msg["your_hand"].([]any)
	if !ok || len(hand) == 0 {
		t.Fatalf("expected Alice's hand restored after restart, got %+v", msg["your_hand"])
	}
	opponents, ok := msg["opponents"].([]any)
	if !ok || len(opponents) != 3 {
		t.Fatalf("expected 3 opponents after restart, got %+v", msg["opponents"])
	}
}
