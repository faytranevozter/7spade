package room

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/gorilla/websocket"
)

func TestGameOverMessageIncludesSkinGrantsOnlyForRecipient(t *testing.T) {
	room := &room{
		state: game.GameState{Hands: [][]game.Card{{}, {}}, FaceDown: [][]game.Card{{}, {}}},
		players: []*player{
			{sub: "user-1", displayName: "Alice", index: 0},
			{sub: "user-2", displayName: "Bob", index: 1},
		},
		gameDeltas: map[string]playerDelta{
			"user-1": {UserID: "user-1", NewSkinGrants: []skinGrant{{ID: "skin-1", Name: "Victor Frame", Source: "achievement:first_win"}}},
		},
	}

	mine := room.gameOverMessageFor("user-1")
	if grants, ok := mine["new_skin_grants"].([]skinGrant); !ok || len(grants) != 1 {
		t.Fatalf("recipient grants = %#v", mine["new_skin_grants"])
	}
	if _, ok := room.gameOverMessageFor("user-2")["new_skin_grants"]; ok {
		t.Fatal("other player received private grants")
	}
	if _, ok := room.gameOverMessage()["new_skin_grants"]; ok {
		t.Fatal("spectator/reconnect payload received grants")
	}
}

func TestRoomResultsRanksPlayersByPenaltyTotalWithSkippedTieRanks(t *testing.T) {
	room := &room{
		players: []*player{
			{displayName: "Alice", index: 0},
			{displayName: "Bob", index: 1},
			{displayName: "Carol", index: 2},
			{displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Nine}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}

	results := room.results()

	if results[0]["rank"] != 1 || results[1]["rank"] != 1 || results[2]["rank"] != 3 || results[3]["rank"] != 4 {
		t.Fatalf("expected competition ranks 1, 1, 3, 4, got %+v", results)
	}
}

func TestWebSocketBroadcastsGameOverAfterFinalMove(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-game-over", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	room := server.rooms["room-game-over"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Seven, High: game.Seven}
	room.state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "6"}); err != nil {
		t.Fatalf("write final move: %v", err)
	}

	for index, client := range clients {
		message := readTypedMessage(t, client, "game_over")
		results := message["results"].([]any)
		if len(results) != 4 {
			t.Fatalf("client %d expected four results, got %+v", index, message)
		}
		alice := results[0].(map[string]any)
		if alice["display_name"] != "Alice" || alice["penalty_points"] != float64(5) || alice["rank"] != float64(1) || alice["is_winner"] != true {
			t.Fatalf("client %d unexpected Alice result: %+v", index, alice)
		}
		bob := results[1].(map[string]any)
		if bob["rank"] != float64(1) || bob["is_winner"] != true {
			t.Fatalf("client %d expected Bob to share winner rank: %+v", index, bob)
		}
		cards := alice["facedown_cards"].([]any)
		if len(cards) != 1 {
			t.Fatalf("client %d expected Alice revealed card, got %+v", index, alice)
		}
		card := cards[0].(map[string]any)
		if card["rank"] != "5" || card["suit"] != "clubs" || card["points"] != float64(5) {
			t.Fatalf("client %d unexpected revealed card: %+v", index, card)
		}
	}
}

func TestWebSocketBroadcastsGameOverAfterStalemate(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-stalemate", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	// Set up a dead table: Spades open at 6–8, but after Alice takes her forced
	// face-down nobody holds a playable card. The engine should sweep the
	// remaining hands into face-down piles and end the game immediately.
	room := server.rooms["room-stalemate"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Six, High: game.Eight}
	room.state.Hands[0] = []game.Card{{Suit: game.Hearts, Rank: game.Ten}}   // Alice: forced face-down
	room.state.Hands[1] = []game.Card{{Suit: game.Clubs, Rank: game.Three}}  // Bob: stuck, swept
	room.state.Hands[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Ten}} // Carol: stuck, swept
	room.state.Hands[3] = []game.Card{{Suit: game.Hearts, Rank: game.Two}}   // Dave: stuck, swept
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "place_facedown", "suit": "hearts", "rank": "10"}); err != nil {
		t.Fatalf("write forced face-down: %v", err)
	}

	for index, client := range clients {
		message := readTypedMessage(t, client, "game_over")
		results := message["results"].([]any)
		if len(results) != 4 {
			t.Fatalf("client %d expected four results, got %+v", index, message)
		}
		// Every player's remaining card became a face-down penalty.
		byName := map[string]map[string]any{}
		for _, raw := range results {
			r := raw.(map[string]any)
			byName[r["display_name"].(string)] = r
		}
		if byName["Alice"]["penalty_points"] != float64(10) {
			t.Fatalf("client %d unexpected Alice penalty: %+v", index, byName["Alice"])
		}
		if byName["Bob"]["penalty_points"] != float64(3) {
			t.Fatalf("client %d unexpected Bob penalty: %+v", index, byName["Bob"])
		}
		// Dave's swept Two of Hearts (penalty 2) is the lowest, so Dave wins.
		if byName["Dave"]["penalty_points"] != float64(2) {
			t.Fatalf("client %d unexpected Dave penalty: %+v", index, byName["Dave"])
		}
		if byName["Dave"]["rank"] != float64(1) || byName["Dave"]["is_winner"] != true {
			t.Fatalf("client %d expected Dave to win: %+v", index, byName["Dave"])
		}
	}
}

func TestWebSocketReconnectToFinishedGameReceivesResults(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-finished-reconnect", []string{"Alice", "Bob", "Carol", "Dave"})
	readInitialUpdatesAndFindStarter(t, clients)

	room := server.rooms["room-finished-reconnect"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Seven, High: game.Seven}
	room.state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "6"}); err != nil {
		t.Fatalf("write final move: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "game_over")
	}

	// Alice drops and reconnects to the now-finished room. She must receive the
	// game_over results, not a live state_update, so the results screen renders.
	closeClients(clients)
	reconnect := connectPlayer(t, httpServer.URL, "test-secret", "room-finished-reconnect", "Alice")
	defer reconnect.Close()

	message := readTypedMessage(t, reconnect, "game_over")
	results, ok := message["results"].([]any)
	if !ok || len(results) != 4 {
		t.Fatalf("expected four results on reconnect to finished game, got %+v", message)
	}
}

func TestWebSocketSavesGameResultAfterFinalMove(t *testing.T) {
	history := &memoryGameHistoryStore{}
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
	server.gameHistory = history
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-save-game", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	room := server.rooms["room-save-game"]
	room.mu.Lock()
	room.state = game.NewGameState()
	room.state.Board[game.Spades] = game.SuitSequence{Low: game.Seven, High: game.Seven}
	room.state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Four}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Nine}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Ten}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
	room.state.CurrentPlayer = 0
	room.mu.Unlock()

	if err := clients[0].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "6"}); err != nil {
		t.Fatalf("write final move: %v", err)
	}
	readTypedMessage(t, clients[0], "game_over")

	if len(history.results) != 1 {
		t.Fatalf("expected one saved game result, got %+v", history.results)
	}
	result := history.results[0]
	if result.RoomID != "room-save-game" || result.StartedAt.IsZero() || result.FinishedAt.IsZero() {
		t.Fatalf("saved result missing game metadata: %+v", result)
	}
	if len(result.Players) != 4 {
		t.Fatalf("expected four saved players, got %+v", result.Players)
	}
	if result.Players[0].UserID != "Alice-id" || result.Players[0].DisplayName != "Alice" || result.Players[0].PenaltyPoints != 4 || result.Players[0].Rank != 1 || !result.Players[0].IsWinner {
		t.Fatalf("unexpected saved Alice result: %+v", result.Players[0])
	}
}

func TestSavedGameResultOmitsGuestUserIDs(t *testing.T) {
	room := &room{
		id:        "room-guests",
		startedAt: time.Now().UTC().Add(-15 * time.Minute),
		players: []*player{
			{sub: "Alice-id", displayName: "Alice", index: 0},
			{sub: "Guest-id", displayName: "Guest", isGuest: true, index: 1},
			{sub: "Carol-id", displayName: "Carol", index: 2},
			{sub: "Dave-id", displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Four}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Nine}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Ten}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}

	result := room.savedResultLocked(time.Now().UTC())

	if result.Players[0].UserID != "Alice-id" {
		t.Fatalf("expected authenticated player user id to be saved, got %+v", result.Players[0])
	}
	if result.Players[1].UserID != "" || result.Players[1].DisplayName != "Guest" {
		t.Fatalf("expected guest player display name without user id, got %+v", result.Players[1])
	}
}

func TestWebSocketGameOverIncludesAvatar(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	aliceToken := signTestTokenWithAvatar(t, "test-secret", "Alice", "https://cdn/alice.png")
	alice := dialPlayer(t, httpServer.URL, "room-avatar-go", aliceToken)
	others := []*websocket.Conn{
		connectPlayer(t, httpServer.URL, "test-secret", "room-avatar-go", "Bob"),
		connectPlayer(t, httpServer.URL, "test-secret", "room-avatar-go", "Carol"),
		connectPlayer(t, httpServer.URL, "test-secret", "room-avatar-go", "Dave"),
	}
	clients := append([]*websocket.Conn{alice}, others...)
	defer closeClients(clients)
	startGameAndDrainLobby(t, clients)
	readInitialUpdatesAndFindStarter(t, clients)

	gameRoom := server.rooms["room-avatar-go"]
	forceGameOverRoom(t, gameRoom)
	gameRoom.broadcastGameOver()

	msg := readTypedMessage(t, alice, "game_over")
	results, _ := msg["results"].([]any)
	found := false
	for _, raw := range results {
		r := raw.(map[string]any)
		if r["display_name"] == "Alice" {
			found = true
			if r["avatar_url"] != "https://cdn/alice.png" {
				t.Fatalf("Alice result avatar = %v, want https://cdn/alice.png", r["avatar_url"])
			}
		}
	}
	if !found {
		t.Fatal("Alice not found in results")
	}
}
