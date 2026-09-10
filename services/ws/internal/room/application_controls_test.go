package room

import (
	"net/http/httptest"
	"testing"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/gorilla/websocket"
)

func TestGameServerApplicationControlsWithoutAPI(t *testing.T) {
	server := NewGameServerFromConfig(Config{}, newMemoryStateStore())
	if server.applicationControls != nil {
		t.Fatal("API-less server must not initialize application controls for startup refresh")
	}
	for _, key := range requiredWSControls {
		if !server.controlEnabled(key) {
			t.Fatalf("API-less server must allow %s", key)
		}
	}
}

func TestDisabledWSControlsBlockStartsRematchesAndEmotes(t *testing.T) {
	server := NewGameServer("test-secret")
	server.applicationControls = staticApplicationControls{
		controlNewGameStarts:   false,
		controlSpectatorAccess: true,
		controlEmotes:          false,
	}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "controlled-room", "Alice")
	second := connectPlayer(t, httpServer.URL, "test-secret", "controlled-room", "Bob")
	defer closeClients([]*websocket.Conn{host, second})
	waitForLobbyPlayerCount(t, host, 2)
	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatal(err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatal(err)
	}
	if msg := readTypedMessage(t, host, "error"); msg["message"] != "new game starts are temporarily unavailable" {
		t.Fatalf("start error = %+v", msg)
	}
	if err := second.WriteJSON(map[string]any{"type": "emote", "emote": "gg"}); err != nil {
		t.Fatal(err)
	}
	if msg := readTypedMessage(t, second, "error"); msg["message"] != "emotes are temporarily unavailable" {
		t.Fatalf("emote error = %+v", msg)
	}

	room := server.rooms["controlled-room"]
	room.mu.Lock()
	room.phase = phasePlaying
	room.started = true
	room.state = finishedGameStateForControls()
	room.mu.Unlock()
	if err := host.WriteJSON(map[string]any{"type": "rematch_vote"}); err != nil {
		t.Fatal(err)
	}
	if msg := readTypedMessage(t, host, "error"); msg["message"] != "new game starts are temporarily unavailable" {
		t.Fatalf("rematch error = %+v", msg)
	}
	room.mu.Lock()
	defer room.mu.Unlock()
	if room.rematchTimer != nil || len(room.rematchVotes) != 0 {
		t.Fatal("blocked rematch must not open a countdown or record a vote")
	}
}

func TestDisabledSpectatorAccessRejectsNewAdmission(t *testing.T) {
	server := NewGameServer("test-secret")
	server.applicationControls = staticApplicationControls{
		controlNewGameStarts:   true,
		controlSpectatorAccess: false,
		controlEmotes:          true,
	}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()
	conn := dialSpectator(t, httpServer.URL, "test-secret", "any-room", "Watcher")
	defer conn.Close()
	msg := readTypedMessage(t, conn, "error")
	if msg["message"] != "spectator access is temporarily unavailable" || msg["fatal"] != true {
		t.Fatalf("spectator rejection = %+v", msg)
	}
}

func finishedGameStateForControls() game.GameState {
	state := game.NewGameState()
	for i := range state.Hands {
		state.Hands[i] = nil
	}
	return state
}
