package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/relay"
)

func TestRoomInspectionRequiresDedicatedMachineSecret(t *testing.T) {
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	req := httptest.NewRequest(http.MethodGet, "/internal/rooms/room-1", nil)
	req.Header.Set("X-Internal-Secret", "inspect")
	res := httptest.NewRecorder()

	server.routes(testDependencyChecks()).ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestRoomInspectionRedactsSecretGameState(t *testing.T) {
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	room := &room{
		id:              "room-1",
		phase:           phasePlaying,
		started:         true,
		turnExpiresAt:   time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
		snapVersion:     7,
		snapshotSavedAt: time.Now().Add(-2 * time.Second),
		state: game.GameState{
			Hands:    [][]game.Card{{{Suit: game.Spades, Rank: game.Seven}}},
			FaceDown: [][]game.Card{{{Suit: game.Hearts, Rank: game.Ace}}},
		},
	}
	room.players = []*player{{sub: "user-1", displayName: "Alice", index: 0, room: room}, {sub: "bot-1", displayName: "Robo", index: 1, isBot: true, disconnected: true, room: room}}
	server.rooms[room.id] = room

	res := inspectRoom(t, server, room.id, "inspect")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, forbidden := range []string{"hands", "face_down", "hidden", "spades", "hearts"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}
	var got roomInspectionResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Phase != "playing" || got.StateVersion != 7 || got.Owner.Role != "owner" || len(got.Players) != 2 || got.Players[0].Connected {
		t.Fatalf("unexpected inspection: %+v", got)
	}
	if !got.Players[1].Bot || !got.Players[1].Connected {
		t.Fatalf("bot player not reported correctly: %+v", got.Players[1])
	}
	if got.TurnDeadline == nil || got.SnapshotAgeMS < 0 {
		t.Fatalf("missing timing contract: %+v", got)
	}
}

func TestHiddenRoomStateInspectionRequiresMachineCredentialAndReturnsState(t *testing.T) {
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.rooms["room-1"] = &room{id: "room-1", state: game.GameState{Hands: [][]game.Card{{{Suit: game.Spades, Rank: game.Seven}}}, FaceDown: [][]game.Card{{{Suit: game.Hearts, Rank: game.Ace}}}}}

	unauthorized := inspectHiddenRoomState(t, server, "room-1", "wrong")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}
	response := inspectHiddenRoomState(t, server, "room-1", "inspect")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"Hands"`) || !strings.Contains(response.Body.String(), "spades") {
		t.Fatalf("hidden state status=%d body=%s", response.Code, response.Body.String())
	}
	if regular := inspectRoom(t, server, "room-1", "inspect"); regular.Code != http.StatusOK || strings.Contains(regular.Body.String(), `"Hands"`) || strings.Contains(regular.Body.String(), `"FaceDown"`) {
		t.Fatalf("routine inspection status=%d body=%s", regular.Code, regular.Body.String())
	}
}

func TestHiddenRoomStateInspectionRejectsLeaseLossAndMissingRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.replicaID = "replica-A"
	server.leases = relay.NewLeaseManager(client, "replica-A", time.Minute)
	server.rooms["room-1"] = &room{id: "room-1", relay: &roomRelay{owner: true, token: 3}, state: game.GameState{Hands: [][]game.Card{{{Suit: game.Spades, Rank: game.Seven}}}}}
	mr.Set("roomlease:room-1", "replica-A")
	mr.Set("roomfence:room-1", "4")

	if response := inspectHiddenRoomState(t, server, "room-1", "inspect"); response.Code != http.StatusConflict || strings.Contains(response.Body.String(), "spades") {
		t.Fatalf("lease loss status=%d body=%s", response.Code, response.Body.String())
	}
	mr.Set("roomlease:missing", "replica-A")
	mr.Set("roomfence:missing", "1")
	if response := inspectHiddenRoomState(t, server, "missing", "inspect"); response.Code != http.StatusNotFound {
		t.Fatalf("missing room status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRoomInspectionReportsEdgeWithoutAcquiringOwnership(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.attachRelay("replica-B", nil, relay.NewLeaseManager(client, "replica-B", time.Minute), nil)
	server.broker = nil // inspection needs leases only; gameplay relay remains disabled.
	mr.Set("roomlease:room-1", "replica-A")
	mr.Set("roomfence:room-1", "4")

	res := inspectRoom(t, server, "room-1", "inspect")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var got roomInspectionResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Owner.Role != "edge" || got.Owner.ReplicaID != "replica-A" || got.Owner.FencingToken != 4 {
		t.Fatalf("owner = %+v", got.Owner)
	}
	if owner, _ := mr.Get("roomlease:room-1"); owner != "replica-A" {
		t.Fatalf("inspection changed owner to %q", owner)
	}
	if token, _ := mr.Get("roomfence:room-1"); token != "4" {
		t.Fatalf("inspection changed fencing token to %q", token)
	}
}

func TestRoomInspectionRejectsStaleLocalOwnerAfterLeaseLoss(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	leases := relay.NewLeaseManager(client, "replica-A", time.Minute)
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.replicaID = "replica-A"
	server.leases = leases
	gameRoom := &room{id: "room-1", relay: &roomRelay{owner: true, token: 3}}
	server.rooms[gameRoom.id] = gameRoom
	mr.Set("roomlease:room-1", "replica-A")
	mr.Set("roomfence:room-1", "4")

	res := inspectRoom(t, server, gameRoom.id, "inspect")
	if res.Code != http.StatusConflict || !strings.Contains(res.Body.String(), "lease_lost") {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestRoomInspectionReflectsOwnerFailoverWithoutMutation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.replicaID = "replica-edge"
	server.leases = relay.NewLeaseManager(client, "replica-edge", time.Minute)
	mr.Set("roomlease:room-1", "replica-A")
	mr.Set("roomfence:room-1", "8")

	first := inspectRoom(t, server, "room-1", "inspect")
	mr.Set("roomlease:room-1", "replica-B")
	mr.Set("roomfence:room-1", "9")
	second := inspectRoom(t, server, "room-1", "inspect")

	if !strings.Contains(first.Body.String(), `"replica_id":"replica-A"`) || !strings.Contains(second.Body.String(), `"replica_id":"replica-B"`) || !strings.Contains(second.Body.String(), `"fencing_token":9`) {
		t.Fatalf("failover responses: first=%s second=%s", first.Body.String(), second.Body.String())
	}
}

func TestRoomInspectionReportsOwnerFailoverWithoutLeakingLocalState(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.replicaID = "replica-A"
	server.leases = relay.NewLeaseManager(client, "replica-A", time.Minute)
	// This replica previously owned the room and still holds local state, but
	// the lease has since failed over to another replica.
	gameRoom := &room{
		id:          "room-1",
		phase:       phasePlaying,
		started:     true,
		snapVersion: 11,
		relay:       &roomRelay{owner: true, token: 8},
	}
	gameRoom.players = []*player{{sub: "user-1", displayName: "Alice", index: 0, room: gameRoom}}
	server.rooms[gameRoom.id] = gameRoom
	mr.Set("roomlease:room-1", "replica-B")
	mr.Set("roomfence:room-1", "9")

	res := inspectRoom(t, server, gameRoom.id, "inspect")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var got roomInspectionResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Owner.Role != "edge" || got.Owner.ReplicaID != "replica-B" || got.Owner.FencingToken != 9 {
		t.Fatalf("owner = %+v", got.Owner)
	}
	if got.Phase != "" || len(got.Players) != 0 || got.StateVersion != 0 {
		t.Fatalf("edge response leaked local game state: %+v", got)
	}
}

func TestRoomInspectionReportsUnavailableLeaseDependency(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: time.Millisecond, ReadTimeout: time.Millisecond})
	server := NewGameServerFromConfig(Config{JWTSecret: "jwt", InspectionSecret: "inspect"}, newMemoryStateStore())
	server.replicaID = "replica-A"
	server.leases = relay.NewLeaseManager(client, "replica-A", time.Minute)

	res := inspectRoom(t, server, "room-1", "inspect")
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func inspectHiddenRoomState(t *testing.T, server *GameServer, roomID, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/rooms/"+roomID+"/hidden-state", nil)
	req.Header.Set("X-WS-Inspection-Secret", secret)
	res := httptest.NewRecorder()
	server.routes(testDependencyChecks()).ServeHTTP(res, req)
	return res
}

func inspectRoom(t *testing.T, server *GameServer, roomID, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/rooms/"+roomID, nil)
	req.Header.Set("X-WS-Inspection-Secret", secret)
	res := httptest.NewRecorder()
	server.routes(testDependencyChecks()).ServeHTTP(res, req)
	return res
}
