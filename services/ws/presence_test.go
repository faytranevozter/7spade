package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/store"
)

// A registered player connecting marks them online in Redis; a guest does not.
func TestPresenceMarkedOnConnect(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	server := NewGameServer("test-secret")
	server.presence = store.NewPresence(client, time.Minute)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	// A registered player joins the lobby.
	conn := connectPlayer(t, httpServer.URL, "test-secret", "room-presence", "Alice")
	defer func() { _ = conn.Close() }()

	// Presence is marked after join; poll briefly since it runs in handleWebSocket.
	userID := "Alice-id" // signTestToken sets sub = displayName + "-id"
	waitForPresence(t, mr, userID, true)
}

func TestPresenceSkippedForGuest(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	server := NewGameServer("test-secret")
	server.presence = store.NewPresence(client, time.Minute)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	conn := connectGuestPlayer(t, httpServer.URL, "test-secret", "room-presence-guest", "Guesty")
	defer func() { _ = conn.Close() }()

	// Give the connect path a moment; a guest must never get a presence key.
	time.Sleep(150 * time.Millisecond)
	if mr.Exists(store.PresenceKey("Guesty-id")) {
		t.Fatal("guest should not have a presence key")
	}
}

// connectGuestPlayer dials with a guest token (is_guest=true).
func connectGuestPlayer(t *testing.T, baseURL, secret, roomID, name string) *websocket.Conn {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          name + "-id",
		"display_name": name,
		"is_guest":     true,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign guest token: %v", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial guest: %v", err)
	}
	return conn
}

func waitForPresence(t *testing.T, mr *miniredis.Miniredis, userID string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mr.Exists(store.PresenceKey(userID)) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("presence for %s: exists != %v after timeout", userID, want)
}
