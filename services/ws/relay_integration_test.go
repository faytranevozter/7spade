package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/relay"
	"github.com/faytranevozter/7spade/services/ws/store"
)

// twoReplica spins up two GameServers sharing one miniredis-backed relay, each
// behind its own httptest server, so a test can connect players to different
// replicas of the same logical room.
type twoReplica struct {
	mr      *miniredis.Miniredis
	a, b    *GameServer
	urlA    string
	urlB    string
	closeFn func()
}

func newTwoReplica(t *testing.T) *twoReplica {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	build := func(replicaID string) (*GameServer, *httptest.Server) {
		// Both replicas share one Redis-backed snapshot store so a room owned by
		// one replica can be rehydrated by another on failover.
		s := NewGameServerWithStateStore("test-secret", newRedisStateStore(store.New(client, store.DefaultTTL)))
		broker := relay.NewBroker(client)
		// Short lease TTL so the failover test doesn't wait long.
		leases := relay.NewLeaseManager(client, replicaID, time.Second)
		coordinator := relay.NewCoordinator(client, replicaID)
		s.attachRelay(replicaID, broker, leases, coordinator)
		hs := httptest.NewServer(s.routes(testDependencyChecks()))
		return s, hs
	}

	a, hsA := build("replica-A")
	b, hsB := build("replica-B")

	return &twoReplica{
		mr:   mr,
		a:    a,
		b:    b,
		urlA: hsA.URL,
		urlB: hsB.URL,
		closeFn: func() {
			a.shutdownRelay()
			b.shutdownRelay()
			hsA.Close()
			hsB.Close()
			_ = client.Close()
			mr.Close()
		},
	}
}

func (tr *twoReplica) Close() { tr.closeFn() }

// TestRelayCrossReplicaGameStart connects four players split across two replicas
// of the same room and verifies the lobby ready/start handshake works across
// replicas and every player — local or edge-relayed — receives a correct
// per-seat initial state.
func TestRelayCrossReplicaGameStart(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	// Alice (host) on A, so A wins the lease and owns the room. Bob on A; Carol
	// + Dave on B are served as edge sockets proxied to A.
	a1 := connectPlayer(t, tr.urlA, "test-secret", "xroom", "Alice")
	defer a1.Close()
	time.Sleep(50 * time.Millisecond) // let A acquire ownership first
	a2 := connectPlayer(t, tr.urlA, "test-secret", "xroom", "Bob")
	defer a2.Close()
	b1 := connectPlayer(t, tr.urlB, "test-secret", "xroom", "Carol")
	defer b1.Close()
	b2 := connectPlayer(t, tr.urlB, "test-secret", "xroom", "Dave")
	defer b2.Close()

	clients := []*websocket.Conn{a1, a2, b1, b2}
	waitForLobbyPlayers(t, a1, 4)
	startGameAndDrainLobby(t, clients)

	for i, c := range clients {
		msg := readTypedMessage(t, c, "state_update")
		if msg["status"] != "in_progress" {
			t.Fatalf("client %d: status %v, want in_progress", i, msg)
		}
		if got := len(msg["your_hand"].([]any)); got != 13 {
			t.Fatalf("client %d: %d cards, want 13", i, got)
		}
		if got := len(msg["opponents"].([]any)); got != 3 {
			t.Fatalf("client %d: %d opponents, want 3", i, got)
		}
	}
}

// TestRelayCrossReplicaMovePropagates verifies a move made by a player on the
// edge replica is applied by the owner and broadcast back to all four players
// across both replicas.
func TestRelayCrossReplicaMovePropagates(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	a1 := connectPlayer(t, tr.urlA, "test-secret", "xroom2", "Alice")
	defer a1.Close()
	time.Sleep(50 * time.Millisecond)
	a2 := connectPlayer(t, tr.urlA, "test-secret", "xroom2", "Bob")
	defer a2.Close()
	b1 := connectPlayer(t, tr.urlB, "test-secret", "xroom2", "Carol")
	defer b1.Close()
	b2 := connectPlayer(t, tr.urlB, "test-secret", "xroom2", "Dave")
	defer b2.Close()

	clients := []*websocket.Conn{a1, a2, b1, b2}
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	waitForLobbyPlayers(t, a1, 4)
	startGameAndDrainLobby(t, clients)

	starter := -1
	for i, c := range clients {
		msg := readTypedMessage(t, c, "state_update")
		if msg["current_turn"] == names[i] {
			starter = i
		}
	}
	if starter < 0 {
		t.Fatal("no starter found among the four players")
	}

	// The starter plays the 7 of spades (always a legal opening move). It may be
	// on the edge replica (Carol/Dave), exercising the edge->owner forward path.
	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write move: %v", err)
	}

	for i, c := range clients {
		msg := readTypedMessage(t, c, "state_update")
		board := msg["board"].(map[string]any)
		spades, ok := board["spades"].(map[string]any)
		if !ok || spades["low"].(float64) != 7 || spades["high"].(float64) != 7 {
			t.Fatalf("client %d (%s) did not see the 7 of spades on the board: %v", i, names[i], board)
		}
	}
}

// TestRelaySingleOwner verifies only one replica holds the room lease even when
// players race in from both replicas.
func TestRelaySingleOwner(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	a1 := connectPlayer(t, tr.urlA, "test-secret", "xroom3", "Alice")
	defer a1.Close()
	b1 := connectPlayer(t, tr.urlB, "test-secret", "xroom3", "Bob")
	defer b1.Close()
	// Let both joins settle.
	time.Sleep(150 * time.Millisecond)

	ownerA := tr.a.ownsRoomForTest("xroom3")
	ownerB := tr.b.ownsRoomForTest("xroom3")
	if ownerA == ownerB {
		t.Fatalf("expected exactly one owner, got A=%v B=%v", ownerA, ownerB)
	}
}

// waitForLobbyPlayers reads the host's lobby_state messages until the roster
// shows wantCount players, so a test doesn't start the game before all
// edge-relayed joins have been seated by the owner.
func waitForLobbyPlayers(t *testing.T, conn *websocket.Conn, wantCount int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read while waiting for lobby roster: %v", err)
		}
		var message map[string]any
		if err := jsonUnmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		if message["type"] == "lobby_state" {
			if players, ok := message["players"].([]any); ok && len(players) >= wantCount {
				return
			}
		}
	}
	t.Fatalf("timed out waiting for %d players in lobby", wantCount)
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// TestRelayOwnerFailover starts a game owned by replica A, then simulates A
// dying: its lease is allowed to expire and a player reconnects to replica B.
// B must claim ownership, rehydrate the room from the shared Redis snapshot, and
// serve the reconnecting player the live game state.
func TestRelayOwnerFailover(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	// All four players connect to A so A owns the room and persists snapshots.
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	clients := make([]*websocket.Conn, 0, 4)
	for _, name := range names {
		clients = append(clients, connectPlayer(t, tr.urlA, "test-secret", "failover", name))
	}
	waitForLobbyPlayers(t, clients[0], 4)
	startGameAndDrainLobby(t, clients)
	for _, c := range clients {
		readTypedMessage(t, c, "state_update")
	}

	// Simulate A crashing: demote its ownership locally and let the lease lapse.
	// (In production the process exits; here we drop the in-memory owner flag and
	// fast-forward Redis past the lease TTL so B can claim it.)
	tr.a.demoteRoomForTest("failover")
	for _, c := range clients {
		_ = c.Close()
	}
	tr.mr.FastForward(2 * time.Second)

	// Carol reconnects — now to replica B. B should acquire the now-free lease,
	// rehydrate the room from the shared snapshot, and replay the live state.
	carol := connectPlayer(t, tr.urlB, "test-secret", "failover", "Carol")
	defer carol.Close()

	msg := readTypedMessage(t, carol, "state_update")
	if msg["status"] != "in_progress" {
		t.Fatalf("reconnect after failover: status %v, want in_progress", msg["status"])
	}
	if got := len(msg["your_hand"].([]any)); got == 0 {
		t.Fatalf("reconnect after failover: empty hand, want rehydrated cards")
	}
	if !tr.b.ownsRoomForTest("failover") {
		t.Fatal("replica B did not take ownership after failover")
	}
}
