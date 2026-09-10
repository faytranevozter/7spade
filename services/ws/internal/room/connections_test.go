package room

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketRejectsSuspendedPlayerBeforeUpgrade(t *testing.T) {
	server := NewGameServer("test-secret")
	server.accessChecker = rejectingAccessChecker{}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	token := signTestTokenWithSub(t, "test-secret", "suspended-user", "Suspended Player")
	_, response, err := websocket.DefaultDialer.Dial("ws"+httpServer.URL[len("http"):]+"/ws?room_id=suspension-test&token="+token, nil)
	if err == nil {
		t.Fatal("suspended player websocket connection succeeded")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("suspended player websocket status = %#v", response)
	}
}

func TestWebSocketDisconnectActivatesBotAndBroadcastsDisconnect(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), 20*time.Millisecond)
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
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
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

// TestWebSocketHeartbeatDropsSilentPlayer verifies the server-side liveness
// check: a client that stops reading (and so stops answering pings with pongs)
// is dropped once the pong window elapses, and the remaining players are told.
func TestWebSocketHeartbeatDropsSilentPlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	// Fast clock: ping every 300ms, drop after 3s without a pong. The initial
	// read deadline (conn open + 3s) comfortably outlasts the join/start
	// handshake below, so Alice is only dropped once she goes silent afterwards.
	server.wsPingEvery = 300 * time.Millisecond
	server.wsPongWait = 3 * time.Second
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-heartbeat-silent", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	// Alice stops reading entirely: pings pile up unanswered, no pong reaches
	// the server, and the pong deadline drops her. Bob stays blocked in
	// readTypedMessage (which keeps reading, so his own heartbeat is healthy)
	// and must be notified of Alice's disconnect.
	readTypedMessage(t, clients[1], "player_disconnected")
}

// TestWebSocketHeartbeatKeepsReadingPlayer is the counterpart: clients that
// keep reading answer every ping with a pong, so the pong handler keeps
// extending the read deadline and nobody is dropped — even over several pong
// windows with an aggressive clock. Guards against a regression where the pong
// handler stops refreshing the deadline.
func TestWebSocketHeartbeatKeepsReadingPlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	server.wsPingEvery = 300 * time.Millisecond
	server.wsPongWait = 3 * time.Second
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-heartbeat-alive", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	// All clients drain frames continuously for ~2 pong windows. gorilla
	// answers each ping with a pong inside ReadMessage, so everyone stays live.
	disconnected := make(chan string, len(clients))
	readUntil := time.Now().Add(2500 * time.Millisecond)
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(conn *websocket.Conn) {
			defer wg.Done()
			for {
				_ = conn.SetReadDeadline(readUntil)
				_, payload, err := conn.ReadMessage()
				if err != nil {
					// Deadline reached with no disconnect frame. Do not re-enter
					// ReadMessage on a timed-out conn (gorilla panics).
					return
				}
				var message map[string]any
				if err := json.Unmarshal(payload, &message); err != nil {
					continue
				}
				if message["type"] == "player_disconnected" {
					if name, ok := message["display_name"].(string); ok {
						disconnected <- name
					} else {
						disconnected <- "unknown"
					}
				}
			}
		}(client)
	}
	wg.Wait()
	close(disconnected)
	for name := range disconnected {
		t.Fatalf("active client %q was declared disconnected despite answering pings", name)
	}
}
