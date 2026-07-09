package main

import (
	"encoding/json"
	"fmt"
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

// TestRelayCrossReplicaSpectator connects four players to the owner replica and
// a spectator to the OTHER replica (served as an edge). It verifies the edge
// spectator receives the initial snapshot, a live state update after a move, and
// that a spectator emote sent from the edge is broadcast back across replicas to
// the seated players — the relay-aware spectator path end to end.
func TestRelayCrossReplicaSpectator(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	a1 := connectPlayer(t, tr.urlA, "test-secret", "xroom-spec", "Alice")
	defer a1.Close()
	time.Sleep(50 * time.Millisecond) // let A acquire ownership first
	a2 := connectPlayer(t, tr.urlA, "test-secret", "xroom-spec", "Bob")
	defer a2.Close()
	a3 := connectPlayer(t, tr.urlA, "test-secret", "xroom-spec", "Carol")
	defer a3.Close()
	a4 := connectPlayer(t, tr.urlA, "test-secret", "xroom-spec", "Dave")
	defer a4.Close()

	players := []*websocket.Conn{a1, a2, a3, a4}
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	waitForLobbyPlayers(t, a1, 4)
	startGameAndDrainLobby(t, players)

	starter := -1
	for i, c := range players {
		msg := readTypedMessage(t, c, "state_update")
		if msg["current_turn"] == names[i] {
			starter = i
		}
	}
	if starter < 0 {
		t.Fatal("no starter found among the four players")
	}

	// Spectator connects to replica B, which does not own the room, so it is
	// served as an edge proxied to A.
	spec := dialSpectator(t, tr.urlB, "test-secret", "xroom-spec", "Watcher")
	defer func() { _ = spec.Close() }()

	// Initial redacted snapshot, delivered across replicas via the relay.
	snap, ok := readUntilTypeOptional(t, spec, "spectator_state", 3*time.Second)
	if !ok {
		t.Fatal("edge spectator did not receive its initial spectator_state")
	}
	if _, leaked := snap["your_hand"]; leaked {
		t.Fatalf("edge spectator snapshot leaked your_hand: %+v", snap)
	}

	// A move by the owner-side starter should reach the edge spectator live.
	// Several spectator_state frames may be in flight (the join also triggers a
	// count-refresh broadcast), so poll until one reflects the move.
	if err := players[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write move: %v", err)
	}
	sawMove := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		update, ok := readUntilTypeOptional(t, spec, "spectator_state", 3*time.Second)
		if !ok {
			break
		}
		board, _ := update["board"].(map[string]any)
		if spades, ok := board["spades"].(map[string]any); ok {
			if low, ok := spades["low"].(float64); ok && low == 7 {
				sawMove = true
				break
			}
		}
	}
	if !sawMove {
		t.Fatal("edge spectator did not see the 7 of spades after a live move")
	}

	// An emote sent from the edge spectator is forwarded to the owner and
	// broadcast back to the seated players on the owner replica.
	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "celebrate"}); err != nil {
		t.Fatalf("write spectator emote: %v", err)
	}
	playerMsg, ok := readUntilTypeOptional(t, players[0], "spectator_emote", 3*time.Second)
	if !ok {
		t.Fatal("owner-side player did not receive the cross-replica spectator_emote")
	}
	if playerMsg["emote"] != "celebrate" {
		t.Fatalf("spectator_emote = %v, want celebrate", playerMsg["emote"])
	}
	// And the spectator sees its own echoed emote across the relay.
	specEcho, ok := readUntilTypeOptional(t, spec, "spectator_emote", 3*time.Second)
	if !ok {
		t.Fatal("edge spectator did not receive its own echoed spectator_emote")
	}
	if specEcho["emote"] != "celebrate" {
		t.Fatalf("echoed spectator_emote = %v, want celebrate", specEcho["emote"])
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
	deadline := time.Now().Add(8 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
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
				_ = conn.SetReadDeadline(time.Time{})
				return
			}
		}
	}
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// TestRelayOwnerFailover starts a game owned by replica A, then simulates A
// dying: its lease is allowed to expire and a player reconnects to replica B.
// B must claim ownership, rehydrate the room from the shared Redis snapshot, and
// serve the reconnecting player the live game state.
func TestRelayOwnerFailover(t *testing.T) {
	tr := newTwoReplica(t)
	defer tr.Close()

	// Unique room id per run so parallel -count iterations never share lease keys.
	roomID := fmt.Sprintf("failover-%d", time.Now().UnixNano())

	// All four players connect to A so A owns the room and persists snapshots.
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	clients := make([]*websocket.Conn, 0, 4)
	for _, name := range names {
		clients = append(clients, connectPlayer(t, tr.urlA, "test-secret", roomID, name))
	}
	waitForLobbyPlayers(t, clients[0], 4)
	// Force-start server-side so ready-up websocket races can't flake this test.
	if err := tr.a.forceStartForTest(roomID); err != nil {
		t.Fatalf("force start: %v", err)
	}
	for _, c := range clients {
		readTypedMessage(t, c, "state_update")
	}

	// SaveRoom is async; wait until the started-game snapshot is durable so B
	// can rehydrate after failover.
	waitForRoomSnapshot(t, tr.mr, roomID)

	// Simulate A crashing: demote its ownership locally and let the lease lapse.
	// (In production the process exits; here we drop the in-memory owner flag and
	// fast-forward Redis past the lease TTL so B can claim it.)
	tr.a.demoteRoomForTest(roomID)
	for _, c := range clients {
		_ = c.Close()
	}
	tr.mr.FastForward(2 * time.Second)

	// Carol reconnects — now to replica B. B should acquire the now-free lease,
	// rehydrate the room from the shared snapshot, and replay the live state.
	carol := connectPlayer(t, tr.urlB, "test-secret", roomID, "Carol")
	defer carol.Close()

	msg := readTypedMessage(t, carol, "state_update")
	if msg["status"] != "in_progress" {
		t.Fatalf("reconnect after failover: status %v, want in_progress", msg["status"])
	}
	hand, _ := msg["your_hand"].([]any)
	if len(hand) == 0 {
		t.Fatalf("reconnect after failover: empty hand, want rehydrated cards")
	}
	if !tr.b.ownsRoomForTest(roomID) {
		t.Fatal("replica B did not take ownership after failover")
	}
}

// waitForRoomSnapshot polls miniredis until a room snapshot key exists with a
// started game (started=true). SaveRoom writes are async, so failover tests
// must not reconnect until the durable snapshot is present.
func waitForRoomSnapshot(t *testing.T, mr *miniredis.Miniredis, roomID string) {
	t.Helper()
	key := store.StateKey(roomID)
	deadline := time.Now().Add(5 * time.Second)
	var lastErr string
	for time.Now().Before(deadline) {
		payload, err := mr.Get(key)
		if err != nil {
			lastErr = err.Error()
			time.Sleep(20 * time.Millisecond)
			continue
		}
		var snap store.RoomSnapshot
		if err := json.Unmarshal([]byte(payload), &snap); err != nil {
			lastErr = "unmarshal: " + err.Error()
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if snap.Started {
			return
		}
		lastErr = "snapshot present but started=false"
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for durable snapshot of room %q (last: %s)", roomID, lastErr)
}
