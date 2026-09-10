package room

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/gorilla/websocket"
)

func TestWebSocketRematchTimeoutPartialVoteReturnsVotersToWaitingRoom(t *testing.T) {
	server := NewGameServer("test-secret")
	server.rematchWindow = 150 * time.Millisecond
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	updater := &capturingStatusUpdater{calls: make(chan statusCall, 8)}
	server.statusUpdater = updater
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch-timeout", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	forceGameOverRoom(t, server.rooms["room-rematch-timeout"])
	server.rooms["room-rematch-timeout"].broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	// Only Alice and Bob vote. The window expires with a partial vote.
	for _, voter := range []int{0, 1} {
		if err := clients[voter].WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
			t.Fatalf("write rematch vote %d: %v", voter, err)
		}
		for _, client := range clients {
			readTypedMessage(t, client, "rematch_status")
		}
	}

	// Voters (Alice, Bob) land back in the waiting room: they receive lobby_state.
	for _, voter := range []int{0, 1} {
		message := readTypedMessage(t, clients[voter], "lobby_state")
		players := message["players"].([]any)
		if len(players) != 2 {
			t.Fatalf("voter %d expected 2 players in waiting room, got %+v", voter, players)
		}
	}

	// Non-voters (Carol, Dave) are removed: they receive room_closed.
	for _, nonVoter := range []int{2, 3} {
		readTypedMessage(t, clients[nonVoter], "room_closed")
	}

	// Their DB membership rows are dropped.
	dropped := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case call := <-remover.calls:
			dropped[call.userID] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for non-voter removal; got %+v", dropped)
		}
	}
	if !dropped["Carol-id"] || !dropped["Dave-id"] {
		t.Fatalf("expected Carol and Dave removed, got %+v", dropped)
	}

	// The room is re-listed as joinable.
	sawWaiting := false
	deadline := time.After(2 * time.Second)
	for !sawWaiting {
		select {
		case call := <-updater.calls:
			if call.status == "waiting" {
				sawWaiting = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for room status -> waiting")
		}
	}

	room := server.rooms["room-rematch-timeout"]
	room.mu.Lock()
	phase := room.phase
	started := room.started
	playerCount := len(room.players)
	room.mu.Unlock()
	if phase != phaseLobby || started {
		t.Fatalf("expected room back in lobby phase, got phase=%d started=%v", phase, started)
	}
	if playerCount != 2 {
		t.Fatalf("expected 2 players after partial rematch, got %d", playerCount)
	}
}

func TestWebSocketRematchTimeoutNoVotesClosesRoom(t *testing.T) {
	server := NewGameServer("test-secret")
	server.rematchWindow = 120 * time.Millisecond
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch-empty", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	room := server.rooms["room-rematch-empty"]
	forceGameOverRoom(t, room)
	room.broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	// A single vote opens the countdown, then that voter changes their mind by
	// not being counted — but to exercise the zero-voter teardown we open the
	// window via a vote and immediately remove the vote by disconnecting the
	// only voter, leaving nobody.
	if err := clients[0].WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatalf("write rematch vote: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "rematch_status")
	}
	// Drop the only voter's vote so the window expires with zero voters.
	room.mu.Lock()
	room.rematchVotes = map[int]bool{}
	room.mu.Unlock()

	// Remaining connected clients should receive room_closed on expiry.
	for _, idx := range []int{1, 2, 3} {
		readTypedMessage(t, clients[idx], "room_closed")
	}
}

func TestWebSocketRematchKeepsConfiguredRoomTurnTimer(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{TurnTimerSeconds: 30}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch-timer-30", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	forceGameOverRoom(t, server.rooms["room-rematch-timer-30"])
	server.rooms["room-rematch-timer-30"].broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	for voter, client := range clients {
		if err := client.WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
			t.Fatalf("write rematch vote: %v", err)
		}
		if voter < len(clients)-1 {
			for _, observerClient := range clients {
				readTypedMessage(t, observerClient, "rematch_status")
			}
		}
	}
	message := readTypedMessage(t, clients[0], "state_update")
	if message["turn_timer_seconds"] != float64(30) {
		t.Fatalf("expected rematch turn_timer_seconds=30, got %+v", message)
	}
	assertTurnEndsNear(t, message, 30*time.Second)
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

func TestWebSocketRematchVotesIgnoreBots(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-rematch-bots", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-rematch-bots", "Bob")
	defer second.Close()
	clients := []*websocket.Conn{host, second}

	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "state_update")
	}
	forceGameOverRoom(t, server.rooms["room-rematch-bots"])
	server.rooms["room-rematch-bots"].broadcastGameOver()
	for _, client := range clients {
		message := readTypedMessage(t, client, "game_over")
		results := message["results"].([]any)
		var botResults int
		for _, raw := range results {
			result := raw.(map[string]any)
			if result["is_bot"] == true {
				botResults++
			}
		}
		if botResults != 2 {
			t.Fatalf("expected 2 bot results in game_over, got %d in %+v", botResults, results)
		}
	}

	if err := host.WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatalf("write first rematch vote: %v", err)
	}
	for index, client := range clients {
		message := readTypedMessage(t, client, "rematch_status")
		if message["votes"] != float64(1) || message["total"] != float64(len(clients)) {
			t.Fatalf("client %d unexpected rematch status after first vote: %+v", index, message)
		}
		if !rematchStatusIncludesVote(message, "Alice") {
			t.Fatalf("client %d status missing Alice vote: %+v", index, message)
		}
	}

	if err := second.WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatalf("write second rematch vote: %v", err)
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

func TestWebSocketDisconnectDuringRematchStopsCountdown(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-rematch-keep", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	forceGameOverRoom(t, server.rooms["room-rematch-keep"])
	server.rooms["room-rematch-keep"].broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	// Alice votes, opening the countdown.
	if err := clients[0].WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatalf("write first rematch vote: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "rematch_status")
	}

	// Dave disconnects. A full-table rematch is now impossible, so the countdown
	// stops; the others see a re-tallied rematch_status then the disconnect notice.
	if err := clients[3].Close(); err != nil {
		t.Fatalf("close non-voter: %v", err)
	}
	for index, client := range clients[:3] {
		readTypedMessage(t, client, "rematch_status")
		message := readTypedMessage(t, client, "player_disconnected")
		if message["display_name"] != "Dave" {
			t.Fatalf("observer %d expected Dave disconnect, got %+v", index, message)
		}
	}

	// The countdown is cancelled (a rematch needs every human), but Alice's vote
	// is preserved so she can choose to move to the waiting room.
	room := server.rooms["room-rematch-keep"]
	room.mu.Lock()
	timerLive := room.rematchTimer != nil
	votes := len(room.rematchVotes)
	room.mu.Unlock()
	if timerLive {
		t.Fatalf("expected rematch countdown to stop after a human left")
	}
	if votes != 1 {
		t.Fatalf("expected 1 rematch vote to remain, got %d", votes)
	}
}

func TestWebSocketGoToWaitingRoomAfterPlayerLeft(t *testing.T) {
	server := NewGameServer("test-secret")
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	updater := &capturingStatusUpdater{calls: make(chan statusCall, 8)}
	server.statusUpdater = updater
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-goto-waiting", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)
	room := server.rooms["room-goto-waiting"]
	forceGameOverRoom(t, room)
	room.broadcastGameOver()
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	// Dave leaves during the results screen.
	if err := clients[3].Close(); err != nil {
		t.Fatalf("close player: %v", err)
	}
	for _, client := range clients[:3] {
		readTypedMessage(t, client, "player_disconnected")
	}

	// Alice opts to move the remaining humans back to the waiting room.
	if err := clients[0].WriteJSON(map[string]any{"type": "go_to_waiting_room"}); err != nil {
		t.Fatalf("write go_to_waiting_room: %v", err)
	}

	// The three connected humans receive lobby_state.
	for index, client := range clients[:3] {
		message := readTypedMessage(t, client, "lobby_state")
		players := message["players"].([]any)
		if len(players) != 3 {
			t.Fatalf("client %d expected 3 players in waiting room, got %+v", index, players)
		}
	}

	// Dave's membership row is dropped.
	select {
	case call := <-remover.calls:
		if call.userID != "Dave-id" {
			t.Fatalf("expected Dave removed, got %+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Dave removal")
	}

	room.mu.Lock()
	phase := room.phase
	started := room.started
	playerCount := len(room.players)
	room.mu.Unlock()
	if phase != phaseLobby || started {
		t.Fatalf("expected lobby phase, got phase=%d started=%v", phase, started)
	}
	if playerCount != 3 {
		t.Fatalf("expected 3 players after waiting-room return, got %d", playerCount)
	}
}
