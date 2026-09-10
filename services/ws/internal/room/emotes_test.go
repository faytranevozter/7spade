package room

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebSocketEmoteBroadcastsToAllIncludingSender(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	names := []string{"Alice", "Bob", "Carol", "Dave"}
	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-emote", names)
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	// Send from a non-starter to prove emotes are not gated by turn ownership.
	sender := (starter + 1) % len(clients)

	if err := clients[sender].WriteJSON(map[string]any{"type": "emote", "emote": "thumbs_up"}); err != nil {
		t.Fatalf("write emote: %v", err)
	}

	for i, client := range clients {
		msg := readTypedMessage(t, client, "emote")
		if msg["display_name"] != names[sender] {
			t.Fatalf("client %d: display_name = %v, want %s", i, msg["display_name"], names[sender])
		}
		if msg["emote"] != "thumbs_up" {
			t.Fatalf("client %d: emote = %v, want thumbs_up", i, msg["emote"])
		}
	}
}

func TestWebSocketUnknownEmoteReturnsError(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-bad-emote", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[0].WriteJSON(map[string]any{"type": "emote", "emote": "definitely_not_real"}); err != nil {
		t.Fatalf("write emote: %v", err)
	}
	msg := readTypedMessage(t, clients[0], "error")
	if msg["message"] != "unknown emote" {
		t.Fatalf("unexpected error: %+v", msg)
	}
}

func TestWebSocketEmoteRateLimited(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	names := []string{"Alice", "Bob", "Carol", "Dave"}
	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-emote-rate", names)
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	receiver := (starter + 1) % len(clients)

	// Two emotes back-to-back: the first broadcasts, the second falls inside the
	// cooldown and is silently dropped.
	if err := clients[starter].WriteJSON(map[string]any{"type": "emote", "emote": "thumbs_up"}); err != nil {
		t.Fatalf("write first emote: %v", err)
	}
	if err := clients[starter].WriteJSON(map[string]any{"type": "emote", "emote": "laugh"}); err != nil {
		t.Fatalf("write second emote: %v", err)
	}

	first := readTypedMessage(t, clients[receiver], "emote")
	if first["emote"] != "thumbs_up" {
		t.Fatalf("first emote = %v, want thumbs_up", first["emote"])
	}
	if msg, ok := readEmoteOptional(t, clients[receiver], 300*time.Millisecond); ok {
		t.Fatalf("expected the second emote to be dropped, but received: %+v", msg)
	}
}

func TestPlayerAllowInboundFloodGuard(t *testing.T) {
	p := &player{}
	for i := 0; i < inboundFloodLimit; i++ {
		ok, closeConn := p.allowInbound()
		if !ok || closeConn {
			t.Fatalf("under limit i=%d: ok=%v close=%v", i, ok, closeConn)
		}
	}
	ok, closeConn := p.allowInbound()
	if ok || closeConn {
		t.Fatalf("over limit: ok=%v close=%v, want false,false", ok, closeConn)
	}
	// Drive up to the close threshold.
	for len(p.inboundAt) < inboundFloodClose {
		p.allowInbound()
	}
	ok, closeConn = p.allowInbound()
	if ok || !closeConn {
		t.Fatalf("close threshold: ok=%v close=%v, want false,true (n=%d)", ok, closeConn, len(p.inboundAt))
	}
}
