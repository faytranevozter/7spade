package room

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func TestWebSocketUsesConfiguredRoomTurnTimer(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{TurnTimerSeconds: 30}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-timer-30", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	message := readTypedMessage(t, clients[0], "state_update")
	if message["turn_timer_seconds"] != float64(30) {
		t.Fatalf("expected turn_timer_seconds=30, got %+v", message)
	}
	assertTurnEndsNear(t, message, 30*time.Second)
}

func TestWebSocketPlayCardRejectsOutOfTurnAndBroadcastsLegalMove(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-play", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)
	notStarter := (starter + 1) % len(clients)

	if err := clients[notStarter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write out-of-turn move: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[notStarter], "error")
	if errorMessage["message"] != "not your turn" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}

	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write legal move: %v", err)
	}
	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		board := message["board"].(map[string]any)
		spades := board["spades"].(map[string]any)
		if spades["low"].(float64) != 7 || spades["high"].(float64) != 7 {
			t.Fatalf("client %d unexpected spades board: %+v", index, spades)
		}
	}
}

func TestWebSocketPlayCardPersistsUpdatedRoomState(t *testing.T) {
	store := newMemoryStateStore()
	server := NewGameServerWithStateStore("test-secret", store)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-persist", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write legal move: %v", err)
	}
	for _, client := range clients {
		readTypedMessage(t, client, "state_update")
	}

	snap, ok := store.LoadRoom("room-persist")
	if !ok {
		t.Fatal("expected room state to be persisted")
	}
	saved := snap.state
	spades := saved.Board["spades"]
	if spades.Low != 7 || spades.High != 7 {
		t.Fatalf("unexpected persisted spades board: %+v", spades)
	}
	if hasGameCard(saved.Hands[starter], "spades", "7") {
		t.Fatal("persisted hand still contains played seven of spades")
	}
}

func TestWebSocketTurnTimerExpiryAutoPlaysAndBroadcastsStateUpdate(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), 20*time.Millisecond)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-autoplay", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	message := readTypedMessage(t, clients[starter], "state_update")
	if hasCard(message, "spades", "7") {
		t.Fatal("timer expiry did not auto-play the seven of spades")
	}
	board := message["board"].(map[string]any)
	spades := board["spades"].(map[string]any)
	if spades["low"].(float64) != 7 || spades["high"].(float64) != 7 {
		t.Fatalf("unexpected spades board after auto-play: %+v", spades)
	}
}

func TestWebSocketPlaceFaceDownRejectsWhenValidMoveExists(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-facedown", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "place_facedown", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write face-down move: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[starter], "error")
	if errorMessage["message"] != "cannot place face-down while a legal play is available" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}
}

func TestRoomResultsIncludeRevealedFaceDownCardsWithPointValues(t *testing.T) {
	room := &room{
		players: []*player{
			{displayName: "Alice", index: 0},
			{displayName: "Bob", index: 1},
			{displayName: "Carol", index: 2},
			{displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	room.state.CloseMethod = game.CloseLow
	room.state.FaceDown[0] = []game.Card{{Suit: game.Hearts, Rank: game.Ace}, {Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Spades, Rank: game.Six}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Nine}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Clubs, Rank: game.Ten}}

	results := room.results()
	alice := results[0]
	if alice["penalty_points"] != 6 {
		t.Fatalf("expected Alice score 6, got %+v", alice)
	}
	cards := alice["facedown_cards"].([]map[string]any)
	if len(cards) != 2 {
		t.Fatalf("expected two revealed cards, got %+v", cards)
	}
	if cards[0]["rank"] != "A" || cards[0]["suit"] != "hearts" || cards[0]["points"] != 1 {
		t.Fatalf("unexpected revealed ace payload: %+v", cards[0])
	}
	if cards[1]["rank"] != "5" || cards[1]["suit"] != "clubs" || cards[1]["points"] != 5 {
		t.Fatalf("unexpected revealed five payload: %+v", cards[1])
	}
	if alice["rank"] != 1 || alice["is_winner"] != true || results[1]["rank"] != 1 || results[1]["is_winner"] != true {
		t.Fatalf("expected Alice and Bob to share rank 1: %+v", results)
	}
}

func TestRoomResultsScoreUnclosedAceAsSeven(t *testing.T) {
	room := &room{
		players: []*player{
			{displayName: "Alice", index: 0},
			{displayName: "Bob", index: 1},
			{displayName: "Carol", index: 2},
			{displayName: "Dave", index: 3},
		},
		state: game.NewGameState(),
	}
	// No suit was ever closed with an Ace, so CloseMethod stays unset and a
	// face-down Ace is scored as a Seven (not its full rank of 14).
	room.state.FaceDown[0] = []game.Card{{Suit: game.Hearts, Rank: game.Ace}, {Suit: game.Clubs, Rank: game.Three}}

	results := room.results()
	alice := results[0]
	if alice["penalty_points"] != 10 {
		t.Fatalf("expected Alice score 10 (7+3 with unclosed ace), got %+v", alice)
	}
	cards := alice["facedown_cards"].([]map[string]any)
	if cards[0]["rank"] != "A" || cards[0]["points"] != 7 {
		t.Fatalf("expected unclosed ace to reveal 7 points, got %+v", cards[0])
	}
}

func TestApplyClientMessageClosesAceLowWithExplicitMethod(t *testing.T) {
	state := aceCloseTestState()

	updated, _, err := applyClientMessage(state, 0, clientMessage{
		Type: messageTypePlayCard, Suit: "spades", Rank: "A", Method: "low",
	})
	if err != nil {
		t.Fatalf("expected explicit low close to succeed: %v", err)
	}
	if !updated.Closed[game.Spades] {
		t.Fatal("expected spades to be closed")
	}
	if updated.CloseMethod != game.CloseLow {
		t.Fatalf("expected close method low, got %s", updated.CloseMethod)
	}
}

func TestApplyClientMessageAceWithoutMethodIsAmbiguousWhenBothEnds(t *testing.T) {
	// Sequence reaches both 2 and King with no locked method: the server can't
	// guess which end, so it must ask the client to specify.
	state := aceCloseTestState()

	_, _, err := applyClientMessage(state, 0, clientMessage{
		Type: messageTypePlayCard, Suit: "spades", Rank: "A",
	})
	if err == nil {
		t.Fatal("expected ambiguous close to be rejected when both ends are open")
	}
}

func TestApplyClientMessageAceWithoutMethodInfersSingleEnd(t *testing.T) {
	// Only the low end is reachable (high is Nine, not King): the server infers
	// low without the client supplying a method.
	state := game.NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []game.Card{{Suit: game.Hearts, Rank: game.Ace}}
	state.Board[game.Hearts] = game.SuitSequence{Low: game.Two, High: game.Nine}

	updated, _, err := applyClientMessage(state, 0, clientMessage{
		Type: messageTypePlayCard, Suit: "hearts", Rank: "A",
	})
	if err != nil {
		t.Fatalf("expected inferred low close to succeed: %v", err)
	}
	if !updated.Closed[game.Hearts] || updated.CloseMethod != game.CloseLow {
		t.Fatalf("expected hearts closed low, got closed=%v method=%s", updated.Closed[game.Hearts], updated.CloseMethod)
	}
}

func TestApplyClientMessageAceWithoutMethodUsesLockedMethod(t *testing.T) {
	state := aceCloseTestState()
	state.CloseMethod = game.CloseLow

	updated, _, err := applyClientMessage(state, 0, clientMessage{
		Type: messageTypePlayCard, Suit: "spades", Rank: "A",
	})
	if err != nil {
		t.Fatalf("expected locked-method close to succeed: %v", err)
	}
	if !updated.Closed[game.Spades] {
		t.Fatal("expected spades closed using locked low method")
	}
}

func TestApplyClientMessageAcePlayNeverExtendsBoard(t *testing.T) {
	// Regression for the board-blanking bug: an Ace play must never set the
	// sequence High to 14. When the suit can't be closed the move is rejected
	// rather than silently corrupting the board.
	state := game.NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Ace}}
	state.Board[game.Spades] = game.SuitSequence{Low: game.Five, High: game.Nine}

	_, _, err := applyClientMessage(state, 0, clientMessage{
		Type: messageTypePlayCard, Suit: "spades", Rank: "A",
	})
	if err == nil {
		t.Fatal("expected ace play to be rejected when the suit cannot be closed")
	}
}
