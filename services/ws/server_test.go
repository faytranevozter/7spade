package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

type memoryGameHistoryStore struct {
	results []savedGameResult
}

func (store *memoryGameHistoryStore) SaveGame(result savedGameResult) (string, []playerDelta, error) {
	store.results = append(store.results, result)
	return "", nil, nil
}

type staticRoomSettingsStore struct {
	settings roomSettings
}

func (store staticRoomSettingsStore) GetRoomSettings(string, string) (roomSettings, error) {
	return store.settings, nil
}

type removeCall struct {
	roomID string
	userID string
	kick   bool
}

type capturingMemberRemover struct {
	calls chan removeCall
}

func (r *capturingMemberRemover) RemoveRoomPlayer(roomID, userID string) error {
	r.calls <- removeCall{roomID: roomID, userID: userID}
	return nil
}

func (r *capturingMemberRemover) KickRoomPlayer(roomID, userID string) error {
	r.calls <- removeCall{roomID: roomID, userID: userID, kick: true}
	return nil
}

type statusCall struct {
	roomID string
	status string
}

type capturingStatusUpdater struct {
	calls chan statusCall
}

func (u *capturingStatusUpdater) UpdateRoomStatus(roomID, status string) error {
	select {
	case u.calls <- statusCall{roomID: roomID, status: status}:
	default:
	}
	return nil
}

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

func TestWebSocketRoomStartsGameWhenFourthPlayerJoins(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-start", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		if got := len(message["your_hand"].([]any)); got != 13 {
			t.Fatalf("client %d got %d cards, want 13", index, got)
		}
		if got := len(message["opponents"].([]any)); got != 3 {
			t.Fatalf("client %d got %d opponents, want 3", index, got)
		}
	}
}

func TestWebSocketCustomGame6PlayersStartsAndDeals(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       6,
		DeckCount:        1,
		ScoringMode:      "rank_value",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-6p", []string{"Alice", "Bob", "Carol", "Dave", "Eve", "Frank"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		hand := message["your_hand"].([]any)
		if len(hand) == 0 {
			t.Fatalf("client %d got 0 cards", index)
		}
		opponents := message["opponents"].([]any)
		if len(opponents) != 5 {
			t.Fatalf("client %d got %d opponents, want 5", index, len(opponents))
		}
	}
}

func TestWebSocketCustomGameDoubleDeck(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        2,
		ScoringMode:      "rank_value",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-2deck", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
		hand := message["your_hand"].([]any)
		if len(hand) != 26 {
			t.Fatalf("client %d got %d cards, want 26", index, len(hand))
		}
	}
}

func TestWebSocketCustomGameFlatScoring(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        1,
		ScoringMode:      "flat",
		TeamMode:         "ffa",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-flat", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
	}
}

func TestWebSocketCustomGameTeamMode(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{
		TurnTimerSeconds: 60,
		MaxPlayers:       4,
		DeckCount:        1,
		ScoringMode:      "rank_value",
		TeamMode:         "2v2",
	}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := make([]*websocket.Conn, 0, 4)
	names := []string{"Alice", "Bob", "Carol", "Dave"}
	for _, name := range names {
		clients = append(clients, connectPlayer(t, httpServer.URL, "test-secret", "room-team", name))
	}
	for i := 2; i < 4; i++ {
		if err := clients[i].WriteJSON(map[string]any{"type": "set_team", "team": 1}); err != nil {
			t.Fatalf("write set_team %d: %v", i, err)
		}
	}
	for i, client := range clients {
		if i == 0 {
			continue
		}
		if err := client.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready %d: %v", i, err)
		}
	}
	waitForLobbyCanStart(t, clients[0])
	if err := clients[0].WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	defer closeClients(clients)

	for index, client := range clients {
		message := readTypedMessage(t, client, "state_update")
		if message["status"] != "in_progress" {
			t.Fatalf("client %d expected in_progress, got %+v", index, message)
		}
	}
}

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

func TestWebSocketUnknownMessageTypeReturnsTypeError(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-unknown", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)

	starter := readInitialUpdatesAndFindStarter(t, clients)

	if err := clients[starter].WriteJSON(map[string]any{"type": "dance"}); err != nil {
		t.Fatalf("write unknown message: %v", err)
	}
	errorMessage := readTypedMessage(t, clients[starter], "error")
	if errorMessage["message"] != "unknown message type: dance" {
		t.Fatalf("unexpected error message: %+v", errorMessage)
	}
}

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

func TestWebSocketEmoteWorksInLobby(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	alice := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-emote", "Alice")
	bob := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-emote", "Bob")
	defer closeClients([]*websocket.Conn{alice, bob})

	// Ensure both players are registered server-side (Bob's lobby view lists
	// both) before emoting, so the broadcast can reach Bob.
	waitForLobbyPlayerCount(t, bob, 2)

	if err := alice.WriteJSON(map[string]any{"type": "emote", "emote": "gg"}); err != nil {
		t.Fatalf("write emote: %v", err)
	}
	msg := readTypedMessage(t, bob, "emote")
	if msg["display_name"] != "Alice" || msg["emote"] != "gg" {
		t.Fatalf("unexpected lobby emote: %+v", msg)
	}
}

// readEmoteOptional reads the next message within the window, returning ok=false
// when nothing arrives (a read deadline timeout). Used to assert an emote was
// dropped without a flaky fixed sleep.
func readEmoteOptional(t *testing.T, conn *websocket.Conn, within time.Duration) (map[string]any, bool) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(within)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return nil, false
	}
	var message map[string]any
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("decode message %s: %v", payload, err)
	}
	return message, true
}

// waitForLobbyPlayerCount reads lobby_state messages until one lists exactly
// want players, confirming all of them are registered in the room.
func waitForLobbyPlayerCount(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read while waiting for lobby player count: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		if message["type"] == "lobby_state" {
			if players, ok := message["players"].([]any); ok && len(players) == want {
				return
			}
		}
	}
	t.Fatalf("timed out waiting for lobby player count = %d", want)
}

func TestWebSocketLobbyIncludesAvatar(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	// Alice's token carries an avatar claim; Bob's does not.
	aliceToken := signTestTokenWithAvatar(t, "test-secret", "Alice", "https://cdn/alice.png")
	alice := dialPlayer(t, httpServer.URL, "room-avatar", aliceToken)
	bob := connectPlayer(t, httpServer.URL, "test-secret", "room-avatar", "Bob")
	defer closeClients([]*websocket.Conn{alice, bob})

	// Read lobby_state messages on Bob until both players are present, and assert
	// Alice's entry carries her avatar while Bob's is empty.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := bob.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := bob.ReadMessage()
		if err != nil {
			t.Fatalf("read lobby_state: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if msg["type"] != "lobby_state" {
			continue
		}
		players, _ := msg["players"].([]any)
		if len(players) != 2 {
			continue
		}
		avatars := map[string]string{}
		for _, raw := range players {
			p := raw.(map[string]any)
			name, _ := p["display_name"].(string)
			avatar, _ := p["avatar_url"].(string)
			avatars[name] = avatar
		}
		if avatars["Alice"] != "https://cdn/alice.png" {
			t.Fatalf("Alice avatar = %q, want https://cdn/alice.png", avatars["Alice"])
		}
		if avatars["Bob"] != "" {
			t.Fatalf("Bob avatar = %q, want empty", avatars["Bob"])
		}
		return
	}
	t.Fatal("timed out waiting for two-player lobby_state with avatars")
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

// dialSpectator opens a read-only spectator connection to a room.
func dialSpectator(t *testing.T, baseURL, secret, roomID, name string) *websocket.Conn {
	t.Helper()
	token := signTestToken(t, secret, name)
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token+"&role=spectator", nil)
	if err != nil {
		t.Fatalf("dial spectator: %v", err)
	}
	return conn
}

// A spectator receives a redacted snapshot with no hand and per-player counts,
// and is never leaked any hidden information.
func TestWebSocketSpectatorReceivesRedactedState(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate", "Watcher")
	defer func() { _ = spec.Close() }()

	msg := readTypedMessage(t, spec, "spectator_state")
	if _, hasHand := msg["your_hand"]; hasHand {
		t.Fatalf("spectator payload leaked your_hand: %+v", msg)
	}
	if _, hasAce := msg["ace_close_options"]; hasAce {
		t.Fatalf("spectator payload leaked ace_close_options: %+v", msg)
	}
	players, ok := msg["players"].([]any)
	if !ok || len(players) != 4 {
		t.Fatalf("expected 4 players in spectator state, got %+v", msg["players"])
	}
	first := players[0].(map[string]any)
	if _, hasCount := first["hand_count"]; !hasCount {
		t.Fatalf("expected hand_count in spectator player info: %+v", first)
	}
	// No player object should carry hand card identities.
	for _, raw := range players {
		p := raw.(map[string]any)
		if _, leaked := p["hand"]; leaked {
			t.Fatalf("spectator player leaked hand cards: %+v", p)
		}
	}
}

// Spectating does not add the viewer to the lobby/player set, and players see
// the spectator_count.
func TestWebSocketSpectatorDoesNotJoinPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate-count", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate-count", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	room := server.rooms["room-spectate-count"]
	room.mu.Lock()
	playerCount := len(room.players)
	spectatorCount := len(room.spectators)
	room.mu.Unlock()
	if playerCount != 4 {
		t.Fatalf("spectator changed player count: got %d, want 4", playerCount)
	}
	if spectatorCount != 1 {
		t.Fatalf("expected 1 spectator, got %d", spectatorCount)
	}

	// A seated player gets a refreshed state_update carrying spectator_count=1.
	update := readTypedMessage(t, clients[0], "state_update")
	if int(update["spectator_count"].(float64)) != 1 {
		t.Fatalf("expected spectator_count=1 in player state, got %v", update["spectator_count"])
	}
}

// A spectator receives board updates after a move and the final game_over.
func TestWebSocketSpectatorReceivesUpdatesAndGameOver(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spectate-go", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	starter := readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spectate-go", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	// Starter plays 7 of spades; spectator should see a fresh spectator_state.
	if err := clients[starter].WriteJSON(map[string]any{"type": "play_card", "suit": "spades", "rank": "7"}); err != nil {
		t.Fatalf("write play: %v", err)
	}
	readTypedMessage(t, spec, "spectator_state")

	// Force game over and broadcast; spectator should receive game_over results.
	// Bot auto-plays may emit extra spectator_state frames first, so drain until
	// game_over arrives.
	room := server.rooms["room-spectate-go"]
	forceGameOverRoom(t, room)
	room.broadcastGameOver()
	msg := readSpectatorUntil(t, spec, "game_over")
	if _, ok := msg["results"].([]any); !ok {
		t.Fatalf("spectator game_over missing results: %+v", msg)
	}
}

// readSpectatorUntil reads frames until one of wantType arrives, skipping
// intervening spectator_state updates (bot auto-plays can emit several).
func readSpectatorUntil(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode %s: %v", payload, err)
		}
		if message["type"] == wantType {
			return message
		}
		if message["type"] == "spectator_state" {
			continue
		}
		t.Fatalf("unexpected message type %v, want %s", message["type"], wantType)
	}
	t.Fatalf("timed out waiting for %s", wantType)
	return nil
}

// Spectating a room that does not exist is rejected.
func TestWebSocketSpectatorUnknownRoomRejected(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	spec := dialSpectator(t, httpServer.URL, "test-secret", "no-such-room", "Watcher")
	defer func() { _ = spec.Close() }()
	msg := readTypedMessage(t, spec, "error")
	if msg["message"] != "room not found" {
		t.Fatalf("unexpected error: %+v", msg)
	}
}

// readUntilTypeOptional reads frames (skipping state_update / lobby_state
// heartbeats) until one of wantType arrives or the deadline passes. Used by the
// spectator-emote tests where a spectator join triggers a state_update refresh
// to seated players just before the emote broadcast.
func readUntilTypeOptional(t *testing.T, conn *websocket.Conn, wantType string, within time.Duration) (map[string]any, bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return nil, false
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		switch message["type"] {
		case wantType:
			return message, true
		case "state_update", "spectator_state", "lobby_state", "player_connected", "player_reconnected":
			continue
		default:
			continue
		}
	}
	return nil, false
}

// A spectator emote reaches every seated player and the spectator itself, tagged
// with the distinct spectator_emote type and the spectator's id.
func TestWebSocketSpectatorEmoteBroadcastsToPlayersAndSpectator(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-emote", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-emote", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "celebrate"}); err != nil {
		t.Fatalf("write spectator emote: %v", err)
	}

	// The sending spectator sees its own emote.
	specMsg, ok := readUntilTypeOptional(t, spec, "spectator_emote", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive its own spectator_emote")
	}
	if specMsg["emote"] != "celebrate" {
		t.Fatalf("spectator emote = %v, want celebrate", specMsg["emote"])
	}
	specID, _ := specMsg["spectator_id"].(string)
	if specID == "" {
		t.Fatalf("spectator_emote missing spectator_id: %+v", specMsg)
	}

	// Every seated player receives the same spectator_emote.
	for i, client := range clients {
		msg, ok := readUntilTypeOptional(t, client, "spectator_emote", 2*time.Second)
		if !ok {
			t.Fatalf("player %d did not receive spectator_emote", i)
		}
		if msg["emote"] != "celebrate" {
			t.Fatalf("player %d: emote = %v, want celebrate", i, msg["emote"])
		}
		if msg["spectator_id"] != specID {
			t.Fatalf("player %d: spectator_id = %v, want %s", i, msg["spectator_id"], specID)
		}
	}
}

// An unknown spectator emote id is rejected with an error frame to the sender.
func TestWebSocketSpectatorUnknownEmoteReturnsError(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-bad-emote", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-bad-emote", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "definitely_not_real"}); err != nil {
		t.Fatalf("write spectator emote: %v", err)
	}
	msg, ok := readUntilTypeOptional(t, spec, "error", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive an error for an unknown emote")
	}
	if msg["message"] != "unknown emote" {
		t.Fatalf("unexpected error: %+v", msg)
	}
}

// A spectator's second emote inside the cooldown window is silently dropped.
func TestWebSocketSpectatorEmoteRateLimited(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	clients := connectPlayers(t, httpServer.URL, "test-secret", "room-spec-emote-rate", []string{"Alice", "Bob", "Carol", "Dave"})
	defer closeClients(clients)
	readInitialUpdatesAndFindStarter(t, clients)

	spec := dialSpectator(t, httpServer.URL, "test-secret", "room-spec-emote-rate", "Watcher")
	defer func() { _ = spec.Close() }()
	readTypedMessage(t, spec, "spectator_state")

	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "thumbs_up"}); err != nil {
		t.Fatalf("write first spectator emote: %v", err)
	}
	if err := spec.WriteJSON(map[string]any{"type": "emote", "emote": "laugh"}); err != nil {
		t.Fatalf("write second spectator emote: %v", err)
	}

	first, ok := readUntilTypeOptional(t, spec, "spectator_emote", 2*time.Second)
	if !ok {
		t.Fatalf("spectator did not receive its first emote")
	}
	if first["emote"] != "thumbs_up" {
		t.Fatalf("first spectator emote = %v, want thumbs_up", first["emote"])
	}
	if msg, ok := readUntilTypeOptional(t, spec, "spectator_emote", 400*time.Millisecond); ok {
		t.Fatalf("expected the second spectator emote to be dropped, but received: %+v", msg)
	}
}

func aceCloseTestState() game.GameState {
	state := game.NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Ace}}
	state.Board[game.Spades] = game.SuitSequence{Low: game.Two, High: game.King}
	return state
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

func readInitialUpdatesAndFindStarter(t *testing.T, clients []*websocket.Conn) int {
	t.Helper()

	starter := -1
	for index, client := range clients {
		update := readTypedMessage(t, client, "state_update")
		if hasCard(update, "spades", "7") {
			starter = index
		}
	}
	if starter == -1 {
		t.Fatal("no player received seven of spades")
	}
	return starter
}

func forceGameOverRoom(t *testing.T, room *room) {
	t.Helper()
	room.mu.Lock()
	defer room.mu.Unlock()
	room.state = game.NewGameState()
	room.state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Five}}
	room.state.FaceDown[1] = []game.Card{{Suit: game.Hearts, Rank: game.Five}}
	room.state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	room.state.FaceDown[3] = []game.Card{{Suit: game.Spades, Rank: game.King}}
}

func rematchStatusIncludesVote(message map[string]any, displayName string) bool {
	players, ok := message["players"].([]any)
	if !ok {
		return false
	}
	for _, rawPlayer := range players {
		player := rawPlayer.(map[string]any)
		if player["display_name"] == displayName {
			return player["voted"] == true
		}
	}
	return false
}

func connectPlayers(t *testing.T, baseURL, secret, roomID string, names []string) []*websocket.Conn {
	t.Helper()
	clients := make([]*websocket.Conn, 0, len(names))
	for _, name := range names {
		clients = append(clients, connectPlayer(t, baseURL, secret, roomID, name))
	}
	startGameAndDrainLobby(t, clients)
	return clients
}

// startGameAndDrainLobby reads any pending lobby_state messages emitted as
// players join, marks every non-host player ready, waits until the host sees
// can_start=true, then asks the host to start the game. After this returns,
// each client's next message will be the initial state_update.
func startGameAndDrainLobby(t *testing.T, clients []*websocket.Conn) {
	t.Helper()
	if len(clients) == 0 {
		return
	}
	host := clients[0]
	for index, client := range clients {
		if index == 0 {
			continue
		}
		if err := client.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready %d: %v", index, err)
		}
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
}

func waitForLobbyCanStart(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read while waiting for can_start: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		if message["type"] == "lobby_state" && message["can_start"] == true {
			return
		}
	}
	t.Fatal("timed out waiting for lobby can_start=true")
}

func connectPlayer(t *testing.T, baseURL, secret, roomID string, name string) *websocket.Conn {
	t.Helper()
	token := signTestToken(t, secret, name)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	return conn
}

func signTestToken(t *testing.T, secret, displayName string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          displayName + "-id",
		"display_name": displayName,
		"is_guest":     false,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func signTestTokenWithAvatar(t *testing.T, secret, displayName, avatarURL string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":          displayName + "-id",
		"display_name": displayName,
		"is_guest":     false,
		"avatar_url":   avatarURL,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func assertTurnEndsNear(t *testing.T, message map[string]any, want time.Duration) {
	t.Helper()
	raw, ok := message["turn_ends_at"].(string)
	if !ok || raw == "" {
		t.Fatalf("state update missing turn_ends_at: %+v", message)
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse turn_ends_at %q: %v", raw, err)
	}
	remaining := time.Until(expiresAt)
	if remaining < want-5*time.Second || remaining > want+5*time.Second {
		t.Fatalf("turn timer remaining %v, want near %v in %+v", remaining, want, message)
	}
}

// dialPlayer opens a WebSocket with a caller-supplied token (e.g. one carrying
// an avatar claim), unlike connectPlayer which signs a plain token by name.
func dialPlayer(t *testing.T, baseURL, roomID, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+baseURL[len("http"):]+"/ws?room_id="+roomID+"&token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func readTypedMessage(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	for {
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket message: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode message %s: %v", payload, err)
		}
		// Skip lobby_state heartbeats so existing tests don't need to be aware
		// of the lobby phase that precedes the first state_update.
		if message["type"] == "lobby_state" && wantType != "lobby_state" {
			continue
		}
		// Skip the rematch countdown banner that now precedes the first
		// rematch_status so vote-flow tests don't each need to consume it.
		if message["type"] == "rematch_countdown" && wantType != "rematch_countdown" {
			continue
		}
		if message["type"] != wantType {
			t.Fatalf("message type = %v, want %s: %+v", message["type"], wantType, message)
		}
		return message
	}
}

func hasCard(update map[string]any, suit string, rank string) bool {
	for _, rawCard := range update["your_hand"].([]any) {
		card := rawCard.(map[string]any)
		if card["suit"] == suit && card["rank"] == rank {
			return true
		}
	}
	return false
}

func hasGameCard(cards []game.Card, suit string, rank string) bool {
	card, err := parseCard(suit, rank)
	if err != nil {
		return false
	}
	for _, candidate := range cards {
		if candidate == card {
			return true
		}
	}
	return false
}

func closeClients(clients []*websocket.Conn) {
	for _, client := range clients {
		_ = client.Close()
	}
}

func opponentDisconnected(opponents []any, displayName string) bool {
	for _, rawOpponent := range opponents {
		opponent := rawOpponent.(map[string]any)
		if opponent["display_name"] == displayName {
			return opponent["disconnected"] == true
		}
	}
	return false
}

func testDependencyChecks() map[string]dependencyCheck {
	return map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	}
}

// --- Lobby phase tests ---------------------------------------------------

func TestWebSocketLobbyBroadcastsStateOnJoin(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-join", "Alice")
	defer host.Close()

	first := readTypedMessage(t, host, "lobby_state")
	if first["host_display_name"] != "Alice" {
		t.Fatalf("expected Alice as host, got %+v", first)
	}
	if first["can_start"] != false {
		t.Fatalf("expected can_start=false with single player: %+v", first)
	}
	if got := len(first["players"].([]any)); got != 1 {
		t.Fatalf("expected one player in lobby state, got %d", got)
	}

	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-join", "Bob")
	defer second.Close()

	hostUpdate := readTypedMessage(t, host, "lobby_state")
	if got := len(hostUpdate["players"].([]any)); got != 2 {
		t.Fatalf("host expected two-player lobby state, got %+v", hostUpdate)
	}
	bobUpdate := readTypedMessage(t, second, "lobby_state")
	if bobUpdate["host_display_name"] != "Alice" {
		t.Fatalf("Bob expected Alice as host, got %+v", bobUpdate)
	}
}

func TestWebSocketLobbyHostStartsGameWithBotsFillingSeats(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bots", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bots", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	hostState := readTypedMessage(t, host, "state_update")
	bobState := readTypedMessage(t, second, "state_update")
	for _, msg := range []map[string]any{hostState, bobState} {
		opponents, ok := msg["opponents"].([]any)
		if !ok {
			t.Fatalf("expected opponents in state_update: %+v", msg)
		}
		if len(opponents) != 3 {
			t.Fatalf("expected 3 opponents (1 human + 2 bots), got %d", len(opponents))
		}
		var botCount int
		for _, raw := range opponents {
			opp := raw.(map[string]any)
			if name, _ := opp["display_name"].(string); name == "Bot 1" || name == "Bot 2" {
				if opp["is_bot"] != true {
					t.Fatalf("expected bot opponent to include is_bot=true, got %+v", opp)
				}
				botCount++
			}
		}
		if botCount != 2 {
			t.Fatalf("expected 2 bot opponents, got %d in %+v", botCount, opponents)
		}
	}
}

func TestWebSocketLobbyRejectsStartFromNonHost(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-host", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-host", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game from non-host: %v", err)
	}
	errMsg := readTypedMessage(t, second, "error")
	if errMsg["message"] != "only the host can start the game" {
		t.Fatalf("unexpected error from non-host start: %+v", errMsg)
	}
}

func TestWebSocketLobbyRejectsStartBelowMinimumPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-min", "Alice")
	defer host.Close()
	readTypedMessage(t, host, "lobby_state")

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game with one player: %v", err)
	}
	errMsg := readTypedMessage(t, host, "error")
	if errMsg["message"] != "need at least 2 players to start" {
		t.Fatalf("unexpected error with one player: %+v", errMsg)
	}
}

func TestWebSocketLobbyHostPromotionWhenHostLeaves(t *testing.T) {
	server := NewGameServer("test-secret")
	// Short grace so the leave finalizes (and Bob is promoted) within the test
	// deadline rather than after the production 10s window.
	server.lobbyLeaveGrace = 50 * time.Millisecond
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-promote", "Alice")
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-promote", "Bob")
	defer second.Close()

	// Drain Bob's initial lobby_state messages (join broadcast).
	readTypedMessage(t, second, "lobby_state")

	if err := host.Close(); err != nil {
		t.Fatalf("close host: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := second.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, payload, err := second.ReadMessage()
		if err != nil {
			t.Fatalf("read after host leave: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode lobby state: %v", err)
		}
		if message["type"] != "lobby_state" {
			continue
		}
		if message["host_display_name"] == "Bob" {
			players := message["players"].([]any)
			if len(players) != 1 {
				t.Fatalf("expected 1 player after host leave, got %+v", players)
			}
			bob := players[0].(map[string]any)
			if bob["is_host"] != true || bob["ready"] != true {
				t.Fatalf("expected promoted Bob to be host+ready, got %+v", bob)
			}
			return
		}
	}
	t.Fatal("timed out waiting for host promotion")
}

func TestWebSocketLobbyLeaveNotifiesAPIToRemovePlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	// Short grace so the leave finalizes within the test deadline.
	server.lobbyLeaveGrace = 50 * time.Millisecond
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	// Two players so the room isn't empty after one leaves; the remaining
	// player keeps the room alive and receives the updated lobby state.
	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-leave", "Bob")

	// Drain Bob's join broadcast so we know both players are seated.
	readTypedMessage(t, host, "lobby_state")

	if err := second.Close(); err != nil {
		t.Fatalf("close second player: %v", err)
	}

	select {
	case call := <-remover.calls:
		if call.roomID != "room-lobby-leave" {
			t.Fatalf("remove call room = %q, want room-lobby-leave", call.roomID)
		}
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer to be called on lobby leave")
	}
}

func TestWebSocketLobbyExplicitLeaveRemovesImmediately(t *testing.T) {
	server := NewGameServer("test-secret")
	// Long grace: an explicit leave must NOT wait for it.
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-explicit-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-explicit-leave", "Bob")
	defer second.Close()
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast

	if err := second.WriteJSON(map[string]any{"type": "leave"}); err != nil {
		t.Fatalf("write leave: %v", err)
	}

	// Bob's removal should be reported to the API immediately (well within the
	// 1h grace), proving the explicit leave bypasses the reconnect grace.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for immediate RemoveRoomPlayer on explicit leave")
	}

	// Host should see a lobby_state with only Alice remaining.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, host, "lobby_state")
		players := state["players"].([]any)
		if len(players) == 1 {
			only := players[0].(map[string]any)
			if only["display_name"] == "Alice" {
				return
			}
		}
	}
	t.Fatal("timed out waiting for Bob to disappear from the lobby roster")
}

func TestWebSocketLobbyDuplicateLeaveIsIdempotent(t *testing.T) {
	server := NewGameServer("test-secret")
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-dup-leave", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-dup-leave", "Bob")
	defer second.Close()
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast

	// Send leave twice; only the first should produce a removal.
	for i := 0; i < 2; i++ {
		if err := second.WriteJSON(map[string]any{"type": "leave"}); err != nil {
			t.Fatalf("write leave %d: %v", i, err)
		}
	}

	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer on explicit leave")
	}

	// A second removal must NOT fire for the duplicate leave.
	select {
	case call := <-remover.calls:
		t.Fatalf("unexpected second RemoveRoomPlayer on duplicate leave: %+v", call)
	case <-time.After(300 * time.Millisecond):
		// No duplicate — expected.
	}
}

func TestWebSocketLobbyDisconnectDisablesCanStart(t *testing.T) {
	server := NewGameServer("test-secret")
	// Long grace so Bob stays listed-but-disconnected during the assertion.
	server.lobbyLeaveGrace = time.Hour
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-disc-canstart", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-disc-canstart", "Bob")

	// Bob marks ready -> host should see can_start=true.
	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)

	// Bob drops (no explicit leave) -> within the grace he's still listed but
	// must no longer count toward can_start.
	if err := second.Close(); err != nil {
		t.Fatalf("close second: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, host, "lobby_state")
		if state["can_start"] == false {
			// Bob should still be listed as disconnected during the grace.
			players := state["players"].([]any)
			for _, raw := range players {
				p := raw.(map[string]any)
				if p["display_name"] == "Bob" && p["disconnected"] != true {
					t.Fatalf("expected Bob listed as disconnected, got %+v", p)
				}
			}
			return
		}
	}
	t.Fatal("timed out waiting for can_start to flip false after disconnect")
}

func TestWebSocketLobbyStartDropsDisconnectedPlayers(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
	server.lobbyLeaveGrace = time.Hour
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Bob")
	third := connectPlayer(t, httpServer.URL, "test-secret", "room-start-drop", "Carol")
	defer third.Close()

	for _, c := range []*websocket.Conn{second, third} {
		if err := c.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
			t.Fatalf("write set_ready: %v", err)
		}
	}
	waitForLobbyCanStart(t, host)

	// Bob drops within the grace (still seated server-side); Carol stays.
	if err := second.Close(); err != nil {
		t.Fatalf("close Bob: %v", err)
	}
	// Wait until the host observes can_start again (Alice + Carol, both ready).
	waitForLobbyCanStart(t, host)

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	// The dealt game must contain Alice + Carol + 2 bots, never the dropped Bob.
	state := readTypedMessage(t, host, "state_update")
	opponents := state["opponents"].([]any)
	for _, raw := range opponents {
		opp := raw.(map[string]any)
		if opp["display_name"] == "Bob" {
			t.Fatalf("dropped player Bob was dealt into the game: %+v", opponents)
		}
	}
	var botCount int
	for _, raw := range opponents {
		opp := raw.(map[string]any)
		if name, _ := opp["display_name"].(string); name == "Bot 1" || name == "Bot 2" {
			botCount++
		}
	}
	if botCount != 2 {
		t.Fatalf("expected 2 bots backfilling after dropping Bob, got %d in %+v", botCount, opponents)
	}

	// Bob's DB membership row must be removed when he's dropped at start, so
	// player_count doesn't describe a participant who isn't in the game.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" {
			t.Fatalf("remove call user = %q, want Bob-id", call.userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveRoomPlayer for the dropped player on start")
	}
}

func TestWebSocketLobbyReconnectWithinGraceKeepsPlayer(t *testing.T) {
	server := NewGameServer("test-secret")
	// Generous grace so the reconnect comfortably lands inside the window.
	server.lobbyLeaveGrace = time.Second
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-reconnect", "Alice")
	defer host.Close()
	// Drain the host's own join broadcast.
	readTypedMessage(t, host, "lobby_state")

	// Alice drops her socket (e.g. a page refresh) then reconnects with the
	// same identity before the grace period elapses.
	if err := host.Close(); err != nil {
		t.Fatalf("close host socket: %v", err)
	}
	reconnected := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-reconnect", "Alice")
	defer reconnected.Close()

	// The reconnect resumes the same single-player lobby with Alice still host.
	state := readTypedMessage(t, reconnected, "lobby_state")
	if state["host_display_name"] != "Alice" {
		t.Fatalf("expected Alice still host after reconnect, got %+v", state)
	}
	players, ok := state["players"].([]any)
	if !ok || len(players) != 1 {
		t.Fatalf("expected single seated player after reconnect, got %+v", state["players"])
	}
	alice := players[0].(map[string]any)
	if alice["is_host"] != true {
		t.Fatalf("expected reconnected Alice to remain host, got %+v", alice)
	}
	if alice["disconnected"] != false {
		t.Fatalf("expected reconnected Alice to be marked connected, got %+v", alice)
	}

	// No removal should fire: the grace timer was cancelled by the reconnect.
	select {
	case call := <-remover.calls:
		t.Fatalf("unexpected RemoveRoomPlayer after reconnect within grace: %+v", call)
	case <-time.After(1500 * time.Millisecond):
		// Past the grace window with no removal — the seat was preserved.
	}
}

func TestWebSocketLobbyBotAutoPlaysOnItsTurn(t *testing.T) {
	server := NewGameServerWithOptions(Config{JWTSecret: "test-secret"}, newMemoryStateStore(), time.Hour)
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bot-play", "Alice")
	defer host.Close()
	second := connectPlayer(t, httpServer.URL, "test-secret", "room-lobby-bot-play", "Bob")
	defer second.Close()

	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	// Read initial state_update for both clients.
	first := readTypedMessage(t, host, "state_update")
	readTypedMessage(t, second, "state_update")

	// If a bot is the starter, both clients will receive a follow-up state_update
	// after the bot plays without anyone touching the turn timer.
	if firstName, _ := first["current_turn"].(string); firstName == "Bot 1" || firstName == "Bot 2" {
		next := readTypedMessage(t, host, "state_update")
		if next["current_turn"] == firstName {
			t.Fatalf("expected bot to advance turn after auto-play: %+v", next)
		}
	}
}

type capturingReconciler struct {
	calls chan []string
}

func (r *capturingReconciler) ReconcileRooms(activeRoomIDs []string) error {
	r.calls <- append([]string(nil), activeRoomIDs...)
	return nil
}

func TestActiveRoomIDsSnapshotsInMemoryRooms(t *testing.T) {
	server := NewGameServer("test-secret")
	server.rooms["room-a"] = &room{id: "room-a"}
	server.rooms["room-b"] = &room{id: "room-b"}

	ids := server.activeRoomIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 active room ids, got %+v", ids)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen["room-a"] || !seen["room-b"] {
		t.Fatalf("expected room-a and room-b in active set, got %+v", ids)
	}
}

func TestRoomReconcilerReportsActiveRoomIDs(t *testing.T) {
	server := NewGameServer("test-secret")
	reconciler := &capturingReconciler{calls: make(chan []string, 1)}
	server.reconciler = reconciler
	server.rooms["room-live"] = &room{id: "room-live"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Drive one reconcile tick directly rather than waiting the production
	// interval; this exercises the same snapshot + report path.
	go func() {
		_ = reconciler.ReconcileRooms(server.activeRoomIDs())
		<-ctx.Done()
	}()

	select {
	case ids := <-reconciler.calls:
		if len(ids) != 1 || ids[0] != "room-live" {
			t.Fatalf("expected [room-live], got %+v", ids)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reconcile call")
	}
}

func TestStartRoomReconcilerNoopWithoutReconciler(t *testing.T) {
	server := NewGameServer("test-secret") // no API URL -> reconciler is nil
	if server.reconciler != nil {
		t.Fatal("expected nil reconciler when no API URL is configured")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should return promptly without panicking when reconciler is nil.
	server.StartRoomReconciler(ctx)
}

func TestAPIClientsSendInternalSecretHeader(t *testing.T) {
	gotHeader := make(chan string, 1)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("X-Internal-Secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	server := NewGameServerWithOptions(
		Config{JWTSecret: "test-secret", APIURL: apiServer.URL, InternalSecret: "top-secret"},
		newMemoryStateStore(),
		time.Hour,
	)

	if err := server.reconciler.ReconcileRooms([]string{"room-x"}); err != nil {
		t.Fatalf("reconcile rooms: %v", err)
	}
	select {
	case h := <-gotHeader:
		if h != "top-secret" {
			t.Fatalf("X-Internal-Secret = %q, want %q", h, "top-secret")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconcile request")
	}
}

func TestAPIClientsOmitInternalSecretWhenUnset(t *testing.T) {
	gotHeader := make(chan string, 1)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("X-Internal-Secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	server := NewGameServerWithOptions(
		Config{JWTSecret: "test-secret", APIURL: apiServer.URL},
		newMemoryStateStore(),
		time.Hour,
	)

	if err := server.reconciler.ReconcileRooms([]string{"room-x"}); err != nil {
		t.Fatalf("reconcile rooms: %v", err)
	}
	select {
	case h := <-gotHeader:
		if h != "" {
			t.Fatalf("expected no X-Internal-Secret header, got %q", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconcile request")
	}
}

func TestWebSocketRehydratesInProgressGameAfterRestart(t *testing.T) {
	// A shared store stands in for Redis surviving a process restart.
	sharedStore := newMemoryStateStore()

	// First "process": four players start a game, then everyone disconnects.
	server1 := NewGameServerWithStateStore("test-secret", sharedStore)
	http1 := httptest.NewServer(server1.routes(testDependencyChecks()))
	clients := connectPlayers(t, http1.URL, "test-secret", "room-restart", []string{"Alice", "Bob", "Carol", "Dave"})
	readInitialUpdatesAndFindStarter(t, clients)
	closeClients(clients)
	http1.Close()

	// Second "process": a brand-new server backed by the same store. Alice
	// reconnects and must land back in the in-progress game (state_update),
	// not a fresh lobby.
	server2 := NewGameServerWithStateStore("test-secret", sharedStore)
	http2 := httptest.NewServer(server2.routes(testDependencyChecks()))
	defer http2.Close()

	alice := connectPlayer(t, http2.URL, "test-secret", "room-restart", "Alice")
	defer alice.Close()

	msg := readTypedMessage(t, alice, "state_update")
	if msg["status"] != "in_progress" {
		t.Fatalf("expected in_progress after restart, got %+v", msg)
	}
	hand, ok := msg["your_hand"].([]any)
	if !ok || len(hand) == 0 {
		t.Fatalf("expected Alice's hand restored after restart, got %+v", msg["your_hand"])
	}
	opponents, ok := msg["opponents"].([]any)
	if !ok || len(opponents) != 3 {
		t.Fatalf("expected 3 opponents after restart, got %+v", msg["opponents"])
	}
}

func TestWebSocketRehydratesLobbyAfterRestart(t *testing.T) {
	sharedStore := newMemoryStateStore()

	// First process: two players in a lobby (Bob ready), then both disconnect.
	server1 := NewGameServerWithStateStore("test-secret", sharedStore)
	server1.lobbyLeaveGrace = time.Hour // hold seats so the snapshot keeps both players
	http1 := httptest.NewServer(server1.routes(testDependencyChecks()))
	host := connectPlayer(t, http1.URL, "test-secret", "room-lobby-restart", "Alice")
	second := connectPlayer(t, http1.URL, "test-secret", "room-lobby-restart", "Bob")
	readTypedMessage(t, host, "lobby_state") // drain Bob's join broadcast
	if err := second.WriteJSON(map[string]any{"type": "set_ready", "ready": true}); err != nil {
		t.Fatalf("write set_ready: %v", err)
	}
	waitForLobbyCanStart(t, host)
	_ = host.Close()
	_ = second.Close()
	http1.Close()

	// Second process: Alice reconnects and should see the restored roster
	// (both seats), with Bob shown disconnected until he returns.
	server2 := NewGameServerWithStateStore("test-secret", sharedStore)
	server2.lobbyLeaveGrace = time.Hour
	http2 := httptest.NewServer(server2.routes(testDependencyChecks()))
	defer http2.Close()

	alice := connectPlayer(t, http2.URL, "test-secret", "room-lobby-restart", "Alice")
	defer alice.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := readTypedMessage(t, alice, "lobby_state")
		players, _ := state["players"].([]any)
		if len(players) != 2 {
			continue
		}
		var bob map[string]any
		for _, raw := range players {
			p := raw.(map[string]any)
			if p["display_name"] == "Bob" {
				bob = p
			}
		}
		if bob == nil {
			t.Fatalf("expected Bob in restored roster, got %+v", players)
		}
		if bob["disconnected"] != true {
			t.Fatalf("expected Bob restored as disconnected, got %+v", bob)
		}
		return
	}
	t.Fatal("timed out waiting for restored lobby roster")
}

func TestPracticeLobbyStartsSoloWithThreeBots(t *testing.T) {
	server := NewGameServer("test-secret")
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{PracticeMode: true}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-practice", "Alice")
	defer host.Close()

	// The host alone can start a practice room: min_to_start is 1, can_start is
	// already true on the first lobby_state, and practice_mode is reported.
	lobby := readTypedMessage(t, host, "lobby_state")
	if lobby["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in lobby_state, got %+v", lobby)
	}
	if lobby["min_to_start"] != float64(1) {
		t.Fatalf("expected min_to_start 1 for practice, got %+v", lobby["min_to_start"])
	}
	if lobby["can_start"] != true {
		t.Fatalf("expected can_start true for solo practice host, got %+v", lobby)
	}

	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}

	state := readTypedMessage(t, host, "state_update")
	if state["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in state_update, got %+v", state)
	}
	if got := len(state["opponents"].([]any)); got != 3 {
		t.Fatalf("expected 3 bot opponents, got %d in %+v", got, state)
	}
	for _, raw := range state["opponents"].([]any) {
		if raw.(map[string]any)["is_bot"] != true {
			t.Fatalf("expected every opponent to be a bot, got %+v", raw)
		}
	}
}

func TestPracticeGameSkipsHistorySave(t *testing.T) {
	server := NewGameServer("test-secret")
	history := &memoryGameHistoryStore{}
	server.gameHistory = history
	server.roomSettings = staticRoomSettingsStore{settings: roomSettings{PracticeMode: true}}
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-practice-save", "Alice")
	defer host.Close()
	waitForLobbyCanStart(t, host)
	if err := host.WriteJSON(map[string]any{"type": "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	readTypedMessage(t, host, "state_update")

	room := server.rooms["room-practice-save"]
	forceGameOverRoom(t, room)
	room.saveGameResult()

	if len(history.results) != 0 {
		t.Fatalf("expected no history save for practice game, got %+v", history.results)
	}

	room.broadcastGameOver()
	gameOver := readTypedMessage(t, host, "game_over")
	if gameOver["practice_mode"] != true {
		t.Fatalf("expected practice_mode true in game_over, got %+v", gameOver)
	}
}

func TestPracticeModePersistsInSnapshot(t *testing.T) {
	original := roomSnapshot{
		practiceMode:     true,
		botDifficulty:    game.BotHard,
		turnTimerSeconds: 60,
	}
	roundTrip := fromStoreSnapshot(toStoreSnapshot(original))
	if !roundTrip.practiceMode {
		t.Fatalf("expected practiceMode to survive snapshot round-trip, got %+v", roundTrip)
	}
}

func TestNonPracticeRoomStillRequiresTwoPlayers(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-solo-blocked", "Alice")
	defer host.Close()

	lobby := readTypedMessage(t, host, "lobby_state")
	if lobby["practice_mode"] != false {
		t.Fatalf("expected practice_mode false for normal room, got %+v", lobby)
	}
	if lobby["can_start"] != false {
		t.Fatalf("expected can_start false with a single non-practice player, got %+v", lobby)
	}
	if lobby["min_to_start"] != float64(2) {
		t.Fatalf("expected min_to_start 2 for normal room, got %+v", lobby["min_to_start"])
	}
}

func TestWebSocketHostKicksPlayerFromLobby(t *testing.T) {
	server := NewGameServer("test-secret")
	remover := &capturingMemberRemover{calls: make(chan removeCall, 4)}
	server.memberRemover = remover
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-kick", "Alice")
	defer host.Close()
	guest := connectPlayer(t, httpServer.URL, "test-secret", "room-kick", "Bob")
	defer guest.Close()

	// Wait until the host's lobby_state shows both players, then read Bob's slot.
	var bobSlot int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readTypedMessage(t, host, "lobby_state")
		players := msg["players"].([]any)
		if len(players) < 2 {
			continue
		}
		for _, raw := range players {
			p := raw.(map[string]any)
			if p["display_name"] == "Bob" {
				bobSlot = int(p["slot"].(float64))
			}
		}
		break
	}
	if bobSlot == 0 {
		t.Fatalf("expected Bob to have a non-host slot")
	}

	if err := host.WriteJSON(map[string]any{"type": "kick", "target": bobSlot}); err != nil {
		t.Fatalf("write kick: %v", err)
	}

	// The kicked player is told to leave.
	readTypedMessage(t, guest, "room_closed")

	// The host sees an updated roster without Bob.
	roster := readTypedMessage(t, host, "lobby_state")
	for _, raw := range roster["players"].([]any) {
		if raw.(map[string]any)["display_name"] == "Bob" {
			t.Fatalf("expected Bob removed from roster, got %+v", roster["players"])
		}
	}

	// Bob's DB membership row is dropped and the kick is recorded.
	select {
	case call := <-remover.calls:
		if call.userID != "Bob-id" || !call.kick {
			t.Fatalf("expected Bob-id kicked, got %+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kicked player removal")
	}

	// A kicked player cannot rejoin the same room.
	token := signTestToken(t, "test-secret", "Bob")
	rejoin, _, err := websocket.DefaultDialer.Dial("ws"+httpServer.URL[len("http"):]+"/ws?room_id=room-kick&token="+token, nil)
	if err != nil {
		t.Fatalf("dial rejoin: %v", err)
	}
	defer rejoin.Close()
	errMsg := readTypedMessage(t, rejoin, "error")
	if msg, _ := errMsg["message"].(string); msg == "" {
		t.Fatalf("expected a join error for the kicked player, got %+v", errMsg)
	}
}

func TestWebSocketNonHostCannotKick(t *testing.T) {
	server := NewGameServer("test-secret")
	httpServer := httptest.NewServer(server.routes(testDependencyChecks()))
	defer httpServer.Close()

	host := connectPlayer(t, httpServer.URL, "test-secret", "room-kick-deny", "Alice")
	defer host.Close()
	guest := connectPlayer(t, httpServer.URL, "test-secret", "room-kick-deny", "Bob")
	defer guest.Close()

	// Wait for Bob to see both players seated.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readTypedMessage(t, guest, "lobby_state")
		if len(msg["players"].([]any)) >= 2 {
			break
		}
	}

	// Bob (non-host) tries to kick the host at slot 0.
	if err := guest.WriteJSON(map[string]any{"type": "kick", "target": 0}); err != nil {
		t.Fatalf("write kick: %v", err)
	}
	errMsg := readTypedMessage(t, guest, "error")
	if errMsg["message"] != "only the host can remove players" {
		t.Fatalf("expected host-only error, got %+v", errMsg)
	}
}
