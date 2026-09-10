package room

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func TestSpectatorFirstRestoreRetainsPlayerReconnectAccessChecks(t *testing.T) {
	store := newMemoryStateStore()
	state := game.NewGameState()
	state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Seven}}
	store.SaveRoom("spectator-first", roomSnapshot{
		state:   state,
		phase:   phasePlaying,
		started: false,
		players: []persistedPlayer{{sub: "player-1", displayName: "Alice", index: 0}},
	})
	checker := &delayedRejectingAccessChecker{}
	server := NewGameServerWithStateStore("test-secret", store)
	server.accessChecker = checker
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	spectator := dialSpectator(t, httpServer.URL, "test-secret", "spectator-first", "Watcher")
	defer func() { _ = spectator.Close() }()
	readTypedMessage(t, spectator, "spectator_state")

	player := connectPlayerWithSub(t, httpServer.URL, "test-secret", "spectator-first", "player-1", "Alice")
	defer func() { _ = player.Close() }()
	readTypedMessage(t, player, "state_update")
	if err := player.SetReadDeadline(time.Now().Add(accessCheckEvery + 2*time.Second)); err != nil {
		t.Fatalf("set reconnect read deadline: %v", err)
	}
	if _, _, err := player.ReadMessage(); err == nil {
		t.Fatal("reconnected player remained connected after access was revoked")
	}
	if calls := checker.callCount(); calls < 3 {
		t.Fatalf("access checker called %d times, want reconnect periodic check", calls)
	}
}

// A spectator receives a redacted snapshot with no hand and per-player counts,
// and is never leaked any hidden information.
func TestWebSocketSpectatorReceivesRedactedState(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate", "Watcher")
	defer func() { _ = spec.Close() }()

	msg := readTypedMessage(t, spec, "spectator_state")
	if _, hasHand := msg["your_hand"]; hasHand {
		t.Fatalf("spectator payload leaked your_hand: %+v", msg)
	}
	if _, hasAce := msg["ace_close_options"]; hasAce {
		t.Fatalf("spectator payload leaked ace_close_options: %+v", msg)
	}
	players, ok := msg["players"].([]any)
	if !ok || len(players) != 4 {
		t.Fatalf("expected 4 players in spectator state, got %+v", msg["players"])
	}
	first := players[0].(map[string]any)
	if _, hasCount := first["hand_count"]; !hasCount {
		t.Fatalf("expected hand_count in spectator player info: %+v", first)
	}
	// No player object should carry hand card identities.
	for _, raw := range players {
		p := raw.(map[string]any)
		if _, leaked := p["hand"]; leaked {
			t.Fatalf("spectator player leaked hand cards: %+v", p)
		}
	}
}

// Spectating does not add the viewer to the lobby/player set, and players see
// the spectator_count.
func TestWebSocketSpectatorDoesNotJoinPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate-count", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate-count", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	room := server.rooms["room-spectate-count"]
	room.mu.Lock()
	playerCount := len(room.players)
	spectatorCount := len(room.spectators)
	room.mu.Unlock()
	if playerCount != 4 {
		t.Fatalf("spectator changed player count: got %d, want 4", playerCount)
	}
	if spectatorCount != 1 {
		t.Fatalf("expected 1 spectator, got %d", spectatorCount)
	}

	// A seated player gets a refreshed state_update carrying spectator_count=1.
	update := readTypedMessage(t, clients[0], "state_update")
	if int(update["spectator_count"].(float64)) != 1 {
		t.Fatalf("expected spectator_count=1 in player state, got %v", update["spectator_count"])
	}
}

// A spectator receives board updates after a move and the final game_over.
func TestWebSocketSpectatorReceivesUpdatesAndGameOver(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate-go", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	starter := readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate-go", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	// Starter plays 7 of spades; spectator should see a fresh spectator_state.
	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write play: %v", err)
	}
	readTypedMessage(t, spec, "spectator_state")

	// Force game over and broadcast; spectator should receive game_over results.
	// Bot auto-plays may emit extra spectator_state frames first, so drain until
	// game_over arrives.
	room := server.rooms["room-spectate-go"]
	forceGameOverRoom(t, room)
	room.broadcastGameOver()
	msg := readSpectatorUntil(t, spec, "game_over")
	if _, ok := msg["results"].([]any); !ok {
		t.Fatalf("spectator game_over missing results: %+v", msg)
	}
}

// Spectating a room that does not exist is rejected.
func TestWebSocketSpectatorUnknownRoomRejected(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	spec := dialSpectator(t, httpServer.URL, "test-secret", "no-such-room", "Watcher")
	defer func() { _ = spec.Close() }()
	msg := readTypedMessage(t, spec, "error")
	if msg["message"] != "room not found" {
		t.Fatalf("unexpected error: %+v", msg)
	}
}

// A spectator emote reaches every seated player and the spectator itself, tagged
// with the distinct spectator_emote type and the spectator's id.
func TestWebSocketSpectatorEmoteBroadcastsToPlayersAndSpectator(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-emote", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-emote", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "celebrate"}); err != nil {
		t.Fatalf("write spectator emote: %v", err)
	}

	// The sending spectator sees its own emote.
	specMsg, ok := readUntilTypeOptional(t, spec, "spectator_emote", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive its own spectator_emote")
	}
	if specMsg["emote"] != "celebrate" {
		t.Fatalf("spectator emote = %v, want celebrate", specMsg["emote"])
	}
	specID, _ := specMsg["spectator_id"].(string)
	if specID == "" {
		t.Fatalf("spectator_emote missing spectator_id: %+v", specMsg)
	}

	// Every seated player receives the same spectator_emote.
	for i, client := range clients {
		msg, ok := readUntilTypeOptional(t, client, "spectator_emote", 2*time.Second)
		if !ok {
			t.Fatalf("player %d did not receive spectator_emote", i)
		}
		if msg["emote"] != "celebrate" {
			t.Fatalf("player %d: emote = %v, want celebrate", i, msg["emote"])
		}
		if msg["spectator_id"] != specID {
			t.Fatalf("player %d: spectator_id = %v, want %s", i, msg["spectator_id"], specID)
		}
	}
}

// An unknown spectator emote id is rejected with an error frame to the sender.
func TestWebSocketSpectatorUnknownEmoteReturnsError(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-bad-emote", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-bad-emote", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "definitely_not_real"}); err != nil {
		t.Fatalf("write spectator emote: %v", err)
	}
	msg, ok := readUntilTypeOptional(t, spec, "error", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive an error for an unknown emote")
	}
	if msg["message"] != "unknown emote" {
		t.Fatalf("unexpected error: %+v", msg)
	}
}

// A spectator's second emote inside the cooldown window is silently dropped.
func TestWebSocketSpectatorEmoteRateLimited(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-emote-rate", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-emote-rate", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "thumbs_up"}); err != nil {
		t.Fatalf("write first spectator emote: %v", err)
	}
	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "laugh"}); err != nil {
		t.Fatalf("write second spectator emote: %v", err)
	}

	first, ok := readUntilTypeOptional(t, spec, "spectator_emote", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive its first emote")
	}
	if first["emote"] != "thumbs_up" {
		t.Fatalf("first spectator emote = %v, want thumbs_up", first["emote"])
	}
	if msg, ok := readUntilTypeOptional(t, spec, "spectator_emote", 400*time.Millisecond); ok {
		t.Fatalf("expected the second spectator emote to be dropped, but received: %+v", msg)
	}
}
