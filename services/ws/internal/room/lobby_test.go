package room

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/gorilla/websocket"
)

func TestWebSocketEmoteWorksInLobby(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	alice := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-emote", "Alice")
	bob := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-emote", "Bob")
	defer closeClients([]*websocket.Conn{alice, bob})

	// Ensure both players are registered server-side (Bob's lobby view lists
	// both) before emoting, so the broadcast can reach Bob.
	waitForLobbyPlayerCount(t, bob, 2)

	if err := alice.WriteJSON(map[string]any{"type": "emote", "emote": "gg"}); err != nil {
		t.Fatalf("write emote: %v", err)
	}
	msg := readTypedMessage(t, bob, "emote")
	if msg["display_name"] != "Alice" || msg["emote"] != "gg" {
		t.Fatalf("unexpected lobby emote: %+v", msg)
	}
}

func TestWebSocketLobbyIncludesAvatar(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	// Alice's token carries an avatar claim; Bob's does not.
	aliceToken := signTestTokenWithAvatar(t, "test-secret", "Alice", "https://cdn/alice.png")
	alice := dialPlayer(t, httpServer.URL, "room-avatar", aliceToken)
	bob := connectPlayer(t, httpServer.URL, "test-secret", "room-avatar", "Bob")
	defer closeClients([]*websocket.Conn{alice, bob})

	// Read lobby_state messages on Bob until both players are present, and assert
	// Alice's entry carries her avatar while Bob's is empty.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := bob.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := bob.ReadMessage()
		if err != nil {
			t.Fatalf("read lobby_state: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if msg["type"] != "lobby_state" {
			continue
		}
		players, _ := msg["players"].([]any)
		if len(players) != 2 {
			continue
		}
		avatars := map[string]string{}
		for _, raw := range players {
			p := raw.(map[string]any)
			name, _ := p["display_name"].(string)
			avatar, _ := p["avatar_url"].(string)
			avatars[name] = avatar
		}
		if avatars["Alice"] != "https://cdn/alice.png" {
			t.Fatalf("Alice avatar = %q, want https://cdn/alice.png", avatars["Alice"])
		}
		if avatars["Bob"] != "" {
			t.Fatalf("Bob avatar = %q, want empty", avatars["Bob"])
		}
		return
	}
	t.Fatal("timed out waiting for two-player lobby_state with avatars")
}

func TestWebSocketLobbyBroadcastsStateOnJoin(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-join", "Alice")
	defer host.Close()

	first := readTypedMessage(t, host, "lobby_state")
	if first["host_display_name"] != "Alice" {
		t.Fatalf("expected Alice as host, got %+v", first)
	}
	if first["your_slot"] != float64(0) {
		t.Fatalf("host expected your_slot=0, got %+v", first)
	}
	if first["can_start"] != false {
		t.Fatalf("expected can_start=false with single player: %+v", first)
	}
	if got := len(first["players"].([]any)); got != 1 {
		t.Fatalf("expected one player in lobby state, got %d", got)
	}

	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-join", "Bob")
	defer second.Close()

	hostUpdate := readTypedMessage(t, host, "lobby_state")
	if got := len(hostUpdate["players"].([]any)); got != 2 {
		t.Fatalf("host expected two-player lobby state, got %+v", hostUpdate)
	}
	if hostUpdate["your_slot"] != float64(0) {
		t.Fatalf("host update expected your_slot=0, got %+v", hostUpdate)
	}
	bobUpdate := readTypedMessage(t, second, "lobby_state")
	if bobUpdate["host_display_name"] != "Alice" {
		t.Fatalf("Bob expected Alice as host, got %+v", bobUpdate)
	}
	if bobUpdate["your_slot"] != float64(1) {
		t.Fatalf("Bob expected your_slot=1, got %+v", bobUpdate)
	}
}

func TestWebSocketLobbyBroadcastsPerClientSlotForDuplicateNames(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayerWithSub(t, httpServer.URL, "test-secret", "room-lobby-duplicate-names", "alex-1", "Alex")
	defer host.Close()
	readTypedMessage(t, host, "lobby_state")

	second := connectPlayerWithSub(t, httpServer.URL, "test-secret", "room-lobby-duplicate-names", "alex-2", "Alex")
	defer second.Close()

	hostUpdate := readTypedMessage(t, host, "lobby_state")
	secondUpdate := readTypedMessage(t, second, "lobby_state")
	if hostUpdate["your_slot"] != float64(0) {
		t.Fatalf("host expected your_slot=0 despite duplicate display name, got %+v", hostUpdate)
	}
	if secondUpdate["your_slot"] != float64(1) {
		t.Fatalf("second player expected your_slot=1 despite duplicate display name, got %+v", secondUpdate)
	}
	if hostUpdate["host_display_name"] != "Alex" || secondUpdate["host_display_name"] != "Alex" {
		t.Fatalf("both duplicate-name clients should still see host name Alex: host=%+v second=%+v", hostUpdate, secondUpdate)
	}
}

func TestWebSocketLobbyHostStartsGameWithBotsFillingSeats(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bots", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bots", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	hostState := readTypedMessage(t, host, "state_update")
	bobState := readTypedMessage(t, second, "state_update")
	for _, msg := range []map[string]any{hostState, bobState} {
		opponents, ok := msg["opponents"].([]any)
		if !ok {
			t.Fatalf("expected opponents in state_update: %+v", msg)
		}
		if len(opponents) != 3 {
			t.Fatalf("expected 3 opponents (1 human + 2 bots), got %d", len(opponents))
		}
		var botCount int
		for _, raw := range opponents {
			opp := raw.(map[string]any)
			if name, _ := opp["display_name"].(string); name == "Bot 1" || name == "Bot 2" {
				if opp["is_bot"] != true {
					t.Fatalf("expected bot opponent to include is_bot=true, got %+v", opp)
				}
				botCount++
			}
		}
		if botCount != 2 {
			t.Fatalf("expected 2 bot opponents, got %d in %+v", botCount, opponents)
		}
	}
}

func TestWebSocketLobbyRejectsStartFromNonHost(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-host", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-host", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game from non-host: %v", err)
	}
	errMsg := readTypedMessage(t, second, "error")
	if errMsg["message"] != "only the host can start the game" {
		t.Fatalf("unexpected error from non-host start: %+v", errMsg)
	}
}

func TestWebSocketLobbyRejectsStartBelowMinimumPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-min", "Alice")
	defer host.Close()
	readTypedMessage(t, host, "lobby_state")

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game with one player: %v", err)
	}
	errMsg := readTypedMessage(t, host, "error")
	if errMsg["message"] != "need at least 2 players to start" {
		t.Fatalf("unexpected error with one player: %+v", errMsg)
	}
}

func TestWebSocketLobbyHostPromotionWhenHostLeaves(t *testing.T) {
	server := NewGameServer("test-secret")
	// Short grace so the leave finalizes (and Bob is promoted) within the test
	// deadline rather than after the production 10s window.
	server.lobbyLeaveGrace = 50 * time.Millisecond
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-promote", "Alice")
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-promote", "Bob")
	defer second.Close()

	// Drain Bob's initial lobby_state messages (join broadcast).
	readTypedMessage(t, second, "lobby_state")

	if err := host.Close(); err != nil {
		t.Fatalf("close host: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := second.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := second.ReadMessage()
		if err != nil {
			t.Fatalf("read after host leave: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode lobby state: %v", err)
		}
		if message["type"] != "lobby_state" {
			continue
		}
		if message["host_display_name"] == "Bob" {
			players := message["players"].([]any)
			if len(players) != 1 {
				t.Fatalf("expected 1 player after host leave, got %+v", players)
			}
			bob := players[0].(map[string]any)
			if bob["is_host"] != true || bob["ready"] != true {
				t.Fatalf("expected promoted Bob to be host+ready, got %+v", bob)
			}
			return
		}
	}
	t.Fatal("timed out waiting for host promotion")
}

func TestWebSocketLobbyLeaveNotifiesAPIToRemovePlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	// Short grace so the leave finalizes within the test deadline.
	server.lobbyLeaveGrace = 50 * time.Millisecond
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	// Two players so the room isn't empty after one leaves; the remaining
	// player keeps the room alive and receives the updated lobby state.
	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-leave", "Bob")

	// Drain Bob's join broadcast so we know both players are seated.
	readTypedMessage(t, host, "lobby_state")

	if err := second.Close(); err != nil {
		t.Fatalf("close second player: %v", err)
	}

	select {
	case call := <-remover.calls:
		if call.roomID != "room-lobby-leave" {
			t.Fatalf("remove call room = %q, want room-lobby-leave", call.roomID)
		}
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer to be called on lobby leave")
	}
}

func TestWebSocketLobbyExplicitLeaveRemovesImmediately(t *testing.T) {
	server := NewGameServer("test-secret")
	// Long grace: an explicit leave must NOT wait for it.
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-explicit-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-explicit-leave", "Bob")
	defer second.Close()
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast

	if err := second.WriteJSON(map[string]any{"type": "leave"}); err != nil {
		t.Fatalf("write leave: %v", err)
	}

	// Bob's removal should be reported to the API immediately (well within the
	// 1h grace), proving the explicit leave bypasses the reconnect grace.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for immediate RemoveRoomPlayer on explicit leave")
	}

	// Host should see a lobby_state with only Alice remaining.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, host, "lobby_state")
		players := state["players"].([]any)
		if len(players) == 1 {
			only := players[0].(map[string]any)
			if only["display_name"] == "Alice" {
				return
			}
		}
	}
	t.Fatal("timed out waiting for Bob to disappear from the lobby roster")
}

func TestWebSocketLobbyDuplicateLeaveIsIdempotent(t *testing.T) {
	server := NewGameServer("test-secret")
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-dup-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-dup-leave", "Bob")
	defer second.Close()
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast

	// Send leave twice; only the first should produce a removal.
	for i := 0; i < 2; i++ {
		if err := second.WriteJSON(map[string]any{"type": "leave"}); err != nil {
			t.Fatalf("write leave %d: %v", i, err)
		}
	}

	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer on explicit leave")
	}

	// A second removal must NOT fire for the duplicate leave.
	select {
	case call := <-remover.calls:
		t.Fatalf("unexpected second RemoveRoomPlayer on duplicate leave: %+v", call)
	case <-time.After(300 * time.Millisecond):
		// No duplicate — expected.
	}
}

func TestWebSocketLobbyDisconnectDisablesCanStart(t *testing.T) {
	server := NewGameServer("test-secret")
	// Long grace so Bob stays listed-but-disconnected during the assertion.
	server.lobbyLeaveGrace = time.Hour
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-disc-canstart", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-disc-canstart", "Bob")

	// Bob marks ready -> host should see can_start=true.
	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)

	// Bob drops (no explicit leave) -> within the grace he's still listed but
	// must no longer count toward can_start.
	if err := second.Close(); err != nil {
		t.Fatalf("close second: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, host, "lobby_state")
		if state["can_start"] == false {
			// Bob should still be listed as disconnected during the grace.
			players := state["players"].([]any)
			for _, raw := range players {
				p := raw.(map[string]any)
				if p["display_name"] == "Bob" && p["disconnected"] != true {
					t.Fatalf("expected Bob listed as disconnected, got %+v", p)
				}
			}
			return
		}
	}
	t.Fatal("timed out waiting for can_start to flip false after disconnect")
}

func TestWebSocketLobbyStartDropsDisconnectedPlayers(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Bob")
	third := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Carol")
	defer third.Close()

	for _, c := range []*websocket.Conn{second, third} {
		if err := c.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready: %v", err)
		}
	}
	waitForLobbyCanStart(t, host)

	// Bob drops within the grace (still seated server-side); Carol stays.
	if err := second.Close(); err != nil {
		t.Fatalf("close Bob: %v", err)
	}
	// Wait until the host observes can_start again (Alice + Carol, both ready).
	waitForLobbyCanStart(t, host)

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	// The dealt game must contain Alice + Carol + 2 bots, never the dropped Bob.
	state := readTypedMessage(t, host, "state_update")
	opponents := state["opponents"].([]any)
	for _, raw := range opponents {
		opp := raw.(map[string]any)
		if opp["display_name"] == "Bob" {
			t.Fatalf("dropped player Bob was dealt into the game: %+v", opponents)
		}
	}
	var botCount int
	for _, raw := range opponents {
		opp := raw.(map[string]any)
		if name, _ := opp["display_name"].(string); name == "Bot 1" || name == "Bot 2" {
			botCount++
		}
	}
	if botCount != 2 {
		t.Fatalf("expected 2 bots backfilling after dropping Bob, got %d in %+v", botCount, opponents)
	}

	// Bob's DB membership row must be removed when he's dropped at start, so
	// player_count doesn't describe a participant who isn't in the game.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer for the dropped player on start")
	}
}

func TestWebSocketLobbyReconnectWithinGraceKeepsPlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	// Generous grace so the reconnect comfortably lands inside the window.
	server.lobbyLeaveGrace = time.Second
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-reconnect", "Alice")
	defer host.Close()
	// Drain the host's own join broadcast.
	readTypedMessage(t, host, "lobby_state")

	// Alice drops her socket (e.g. a page refresh) then reconnects with the
	// same identity before the grace period elapses.
	if err := host.Close(); err != nil {
		t.Fatalf("close host socket: %v", err)
	}
	reconnected := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-reconnect", "Alice")
	defer reconnected.Close()

	// The reconnect resumes the same single-player lobby with Alice still host.
	state := readTypedMessage(t, reconnected, "lobby_state")
	if state["host_display_name"] != "Alice" {
		t.Fatalf("expected Alice still host after reconnect, got %+v", state)
	}
	players, ok := state["players"].([]any)
	if !ok || len(players) != 1 {
		t.Fatalf("expected single seated player after reconnect, got %+v", state["players"])
	}
	alice := players[0].(map[string]any)
	if alice["is_host"] != true {
		t.Fatalf("expected reconnected Alice to remain host, got %+v", alice)
	}
	if alice["disconnected"] != false {
		t.Fatalf("expected reconnected Alice to be marked connected, got %+v", alice)
	}

	// No removal should fire: the grace timer was cancelled by the reconnect.
	select {
	case call := <-remover.calls:
		t.Fatalf("unexpected RemoveRoomPlayer after reconnect within grace: %+v", call)
	case <-time.After(1500 * time.Millisecond):
		// Past the grace window with no removal — the seat was preserved.
	}
}

func TestWebSocketLobbyBotAutoPlaysOnItsTurn(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bot-play", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bot-play", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	// Read initial state_update for both clients.
	first := readTypedMessage(t, host, "state_update")
	readTypedMessage(t, second, "state_update")

	// If a bot is the starter, both clients will receive a follow-up state_update
	// after the bot plays without anyone touching the turn timer.
	if firstName, _ := first["current_turn"].(string); firstName == "Bot 1" || firstName == "Bot 2" {
		next := readTypedMessage(t, host, "state_update")
		if next["current_turn"] == firstName {
			t.Fatalf("expected bot to advance turn after auto-play: %+v", next)
		}
	}
}

func TestWebSocketRehydratesLobbyAfterRestart(t *testing.T) {
	sharedStore := newMemoryStateStore()

	// First process: two players in a lobby (Bob ready), then both disconnect.
	server1 := NewGameServerWithStateStore("test-secret", sharedStore)
	server1.lobbyLeaveGrace = time.Hour // hold seats so the snapshot keeps both players
	http1 := httptest.NewServer(server1.routes(testDependencyChecks()))
	host := connectPlayer(t, http1.URL, "test-secret", "room-lobby-restart", "Alice")
	second := connectPlayer(t, http1.URL, "test-secret", "room-lobby-restart", "Bob")
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast
	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	_ = host.Close()
	_ = second.Close()
	http1.Close()

	// Second process: Alice reconnects and should see the restored roster
	// (both seats), with Bob shown disconnected until he returns.
	server2 := NewGameServerWithStateStore("test-secret", sharedStore)
	server2.lobbyLeaveGrace = time.Hour
	http2 := httptest.NewServer(server2.routes(testDependencyChecks()))
	defer http2.Close()

	alice := connectPlayer(t, http2.URL, "test-secret", "room-lobby-restart", "Alice")
	defer alice.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, alice, "lobby_state")
		players, _ := state["players"].([]any)
		if len(players) != 2 {
			continue
		}
		var bob map[string]any
		for _, raw := range players {
			p := raw.(map[string]any)
			if p["display_name"] == "Bob" {
				bob = p
			}
		}
		if bob == nil {
			t.Fatalf("expected Bob in restored roster, got %+v", players)
		}
		if bob["disconnected"] != true {
			t.Fatalf("expected Bob restored as disconnected, got %+v", bob)
		}
		return
	}
	t.Fatal("timed out waiting for restored lobby roster")
}

func TestPracticeLobbyStartsSoloWithThreeBots(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{PracticeMode: true}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-practice", "Alice")
	defer host.Close()

	// The host alone can start a practice room: min_to_start is 1, can_start is
	// already true on the first lobby_state, and practice_mode is reported.
	lobby := readTypedMessage(t, host, "lobby_state")
	if lobby["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in lobby_state, got %+v", lobby)
	}
	if lobby["min_to_start"] != float64(1) {
		t.Fatalf("expected min_to_start 1 for practice, got %+v", lobby["min_to_start"])
	}
	if lobby["can_start"] != true {
		t.Fatalf("expected can_start true for solo practice host, got %+v", lobby)
	}

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	state := readTypedMessage(t, host, "state_update")
	if state["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in state_update, got %+v", state)
	}
	if got := len(state["opponents"].([]any)); got != 3 {
		t.Fatalf("expected 3 bot opponents, got %d in %+v", got, state)
	}
	for _, raw := range state["opponents"].([]any) {
		if raw.(map[string]any)["is_bot"] != true {
			t.Fatalf("expected every opponent to be a bot, got %+v", raw)
		}
	}
}

func TestPracticeGameSkipsHistorySave(t *testing.T) {
	server := NewGameServer("test-secret")
	history := &memoryGameHistoryStore{}
	server.gameHistory = history
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{PracticeMode: true}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-practice-save", "Alice")
	defer host.Close()
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	readTypedMessage(t, host, "state_update")

	room := server.rooms["room-practice-save"]
	forceGameOverRoom(t, room)
	room.saveGameResult()

	if len(history.results) != 0 {
		t.Fatalf("expected no history save for practice game, got %+v", history.results)
	}

	room.broadcastGameOver()
	gameOver := readTypedMessage(t, host, "game_over")
	if gameOver["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in game_over, got %+v", gameOver)
	}
}

func TestPracticeModePersistsInSnapshot(t *testing.T) {
	original := roomSnapshot{
		practiceMode:     true,
		botDifficulty:    game.BotHard,
		turnTimerSeconds: 60,
	}
	roundTrip := fromStoreSnapshot(toStoreSnapshot(original))
	if !roundTrip.practiceMode {
		t.Fatalf("expected practiceMode to survive snapshot round-trip, got %+v", roundTrip)
	}
}

func TestNonPracticeRoomStillRequiresTwoPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-solo-blocked", "Alice")
	defer host.Close()

	lobby := readTypedMessage(t, host, "lobby_state")
	if lobby["practice_mode"] != false {
		t.Fatalf("expected practice_mode false for normal room, got %+v", lobby)
	}
	if lobby["can_start"] != false {
		t.Fatalf("expected can_start false with a single non-practice player, got %+v", lobby)
	}
	if lobby["min_to_start"] != float64(2) {
		t.Fatalf("expected min_to_start 2 for normal room, got %+v", lobby["min_to_start"])
	}
}

func TestWebSocketHostKicksPlayerFromLobby(t *testing.T) {
	server := NewGameServer("test-secret")
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-kick", "Alice")
	defer host.Close()
	guest := connectPlayer(t, httpServer.URL, "test-secret", "room-kick", "Bob")
	defer guest.Close()

	// Wait until the host's lobby_state shows both players, then read Bob's slot.
	var bobSlot int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readTypedMessage(t, host, "lobby_state")
		players := msg["players"].([]any)
		if len(players) < 2 {
			continue
		}
		for _, raw := range players {
			p := raw.(map[string]any)
			if p["display_name"] == "Bob" {
				bobSlot = int(p["slot"].(float64))
			}
		}
		break
	}
	if bobSlot == 0 {
		t.Fatalf("expected Bob to have a non-host slot")
	}

	if err := host.WriteJSON(map[string]any{"type": "kick", "target": bobSlot}); err != nil {
		t.Fatalf("write kick: %v", err)
	}

	// The kicked player is told to leave.
	readTypedMessage(t, guest, "room_closed")

	// The host sees an updated roster without Bob.
	roster := readTypedMessage(t, host, "lobby_state")
	for _, raw := range roster["players"].([]any) {
		if raw.(map[string]any)["display_name"] == "Bob" {
			t.Fatalf("expected Bob removed from roster, got %+v", roster["players"])
		}
	}

	// Bob's DB membership row is dropped and the kick is recorded.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" || !call.kick {
			t.Fatalf("expected Bob-id kicked, got %+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kicked player removal")
	}

	// A kicked player cannot rejoin the same room.
	token := signTestToken(t, "test-secret", "Bob")
	rejoin, _, err := websocket.DefaultDialer.Dial("ws"+httpServer.URL[len("http"):]+"/ws?room_id=room-kick&token="+token, nil)
	if err != nil {
		t.Fatalf("dial rejoin: %v", err)
	}
	defer rejoin.Close()
	errMsg := readTypedMessage(t, rejoin, "error")
	if msg, _ := errMsg["message"].(string); msg == "" {
		t.Fatalf("expected a join error for the kicked player, got %+v", errMsg)
	}
}

func TestWebSocketNonHostCannotKick(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-kick-deny", "Alice")
	defer host.Close()
	guest := connectPlayer(t, httpServer.URL, "test-secret", "room-kick-deny", "Bob")
	defer guest.Close()

	// Wait for Bob to see both players seated.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readTypedMessage(t, guest, "lobby_state")
		if len(msg["players"].([]any)) >= 2 {
			break
		}
	}

	// Bob (non-host) tries to kick the host at slot 0.
	if err := guest.WriteJSON(map[string]any{"type": "kick", "target": 0}); err != nil {
		t.Fatalf("write kick: %v", err)
	}
	errMsg := readTypedMessage(t, guest, "error")
	if errMsg["message"] != "only the host can remove players" {
		t.Fatalf("expected host-only error, got %+v", errMsg)
	}
}
