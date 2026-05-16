package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

type GameServer struct {
	jwtSecret string
	rooms     map[string]*room
	store     stateStore
	mu        sync.Mutex
	upgrader  websocket.Upgrader
}

type room struct {
	id      string
	players []*player
	state   game.GameState
	store   stateStore
	started bool
	mu      sync.Mutex
}

type stateStore interface {
	Save(roomID string, state game.GameState)
}

type memoryStateStore struct {
	mu     sync.Mutex
	states map[string]game.GameState
}

type player struct {
	displayName string
	index       int
	conn        *websocket.Conn
	mu          sync.Mutex
}

var orderedSuits = []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs}

type tokenClaims struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	IsGuest     bool   `json:"is_guest"`
	jwt.RegisteredClaims
}

type clientMessage struct {
	Type   string `json:"type"`
	Suit   string `json:"suit"`
	Rank   string `json:"rank"`
	Method string `json:"method"`
}

func NewGameServer(jwtSecret string) *GameServer {
	return NewGameServerWithStateStore(jwtSecret, newMemoryStateStore())
}

func NewGameServerWithStateStore(jwtSecret string, store stateStore) *GameServer {
	return &GameServer{
		jwtSecret: jwtSecret,
		rooms:     map[string]*room{},
		store:     store,
		upgrader:  websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{states: map[string]game.GameState{}}
}

func (store *memoryStateStore) Save(roomID string, state game.GameState) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states[roomID] = cloneGameState(state)
}

func (store *memoryStateStore) Load(roomID string) (game.GameState, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.states[roomID]
	return cloneGameState(state), ok
}

func cloneGameState(state game.GameState) game.GameState {
	clone := game.GameState{
		Board:         make(map[game.Suit]game.SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[game.Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
	}
	for player := range state.Hands {
		clone.Hands[player] = append([]game.Card(nil), state.Hands[player]...)
		clone.FaceDown[player] = append([]game.Card(nil), state.FaceDown[player]...)
	}
	for suit, sequence := range state.Board {
		clone.Board[suit] = sequence
	}
	for suit, closed := range state.Closed {
		clone.Closed[suit] = closed
	}
	return clone
}

func (server *GameServer) routes(checks map[string]dependencyCheck) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("ws", checks))
	mux.HandleFunc("GET /ws", server.handleWebSocket)
	return mux
}

func (server *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}
	claims, err := parseToken(r.URL.Query().Get("token"), server.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	room, player, err := server.joinRoom(roomID, claims, conn)
	if err != nil {
		if writeErr := conn.WriteJSON(map[string]any{"type": "error", "message": err.Error()}); writeErr != nil {
			log.Printf("write websocket join error: %v", writeErr)
		}
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("close websocket after join error: %v", closeErr)
		}
		return
	}

	if room.started {
		room.broadcastState()
	}
	go room.readLoop(player)
}

func (server *GameServer) joinRoom(roomID string, claims *tokenClaims, conn *websocket.Conn) (*room, *player, error) {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	if gameRoom == nil {
		gameRoom = &room{id: roomID, store: server.store}
		server.rooms[roomID] = gameRoom
	}
	server.mu.Unlock()

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if gameRoom.started {
		return nil, nil, fmt.Errorf("game already started")
	}
	if len(gameRoom.players) >= game.PlayerCount {
		return nil, nil, fmt.Errorf("room is full")
	}
	player := &player{displayName: claims.DisplayName, index: len(gameRoom.players), conn: conn}
	gameRoom.players = append(gameRoom.players, player)
	if len(gameRoom.players) == game.PlayerCount {
		state, starter := game.Deal(time.Now().UnixNano())
		gameRoom.state = state
		gameRoom.state.CurrentPlayer = starter
		gameRoom.started = true
		gameRoom.store.Save(roomID, gameRoom.state)
	}
	return gameRoom, player, nil
}

func (room *room) readLoop(player *player) {
	defer func() {
		if err := player.conn.Close(); err != nil {
			log.Printf("close websocket read loop: %v", err)
		}
	}()
	for {
		var message clientMessage
		if err := player.conn.ReadJSON(&message); err != nil {
			return
		}
		room.handleMessage(player, message)
	}
}

func (room *room) handleMessage(player *player, message clientMessage) {
	room.mu.Lock()
	if !room.started {
		room.mu.Unlock()
		player.sendError("game has not started")
		return
	}
	if room.state.CurrentPlayer != player.index {
		room.mu.Unlock()
		player.sendError("not your turn")
		return
	}

	card, err := parseCard(message.Suit, message.Rank)
	if err != nil {
		room.mu.Unlock()
		player.sendError(err.Error())
		return
	}

	state, err := applyClientMessage(room.state, player.index, card, message)
	if err != nil {
		room.mu.Unlock()
		player.sendError(err.Error())
		return
	}
	room.state = state
	room.store.Save(room.id, room.state)
	gameOver := game.IsGameOver(room.state)
	room.mu.Unlock()

	if gameOver {
		room.broadcastGameOver()
		return
	}
	room.broadcastState()
}

func applyClientMessage(state game.GameState, playerIndex int, card game.Card, message clientMessage) (game.GameState, error) {
	switch message.Type {
	case "play_card":
		if card.Rank == game.Ace && message.Method != "" {
			return game.ApplyAceClose(state, playerIndex, card.Suit, game.CloseMethod(message.Method))
		}
		return game.ApplyMove(state, playerIndex, card, false)
	case "place_facedown":
		return game.ApplyMove(state, playerIndex, card, true)
	default:
		return game.GameState{}, fmt.Errorf("unknown message type: %s", message.Type)
	}
}

func (room *room) broadcastState() {
	room.mu.Lock()
	snapshots := make([]struct {
		player  *player
		message map[string]any
	}, len(room.players))
	for index, player := range room.players {
		snapshots[index].player = player
		snapshots[index].message = room.stateMessageFor(player.index)
	}
	room.mu.Unlock()
	for _, snapshot := range snapshots {
		snapshot.player.send(snapshot.message)
	}
}

func (room *room) stateMessageFor(playerIndex int) map[string]any {
	moves := game.ValidMoves(room.state, room.state.Hands[playerIndex])
	valid := map[game.Card]bool{}
	for _, card := range moves.Cards {
		valid[card] = true
	}

	yourHand := make([]map[string]any, 0, len(room.state.Hands[playerIndex]))
	for _, card := range room.state.Hands[playerIndex] {
		yourHand = append(yourHand, map[string]any{"suit": card.Suit, "rank": rankString(card.Rank), "valid": valid[card]})
	}

	opponents := make([]map[string]any, 0, game.PlayerCount-1)
	for _, player := range room.players {
		if player.index == playerIndex {
			continue
		}
		opponents = append(opponents, map[string]any{
			"display_name":   player.displayName,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
		})
	}

	return map[string]any{
		"type":             "state_update",
		"status":           "in_progress",
		"board":            boardPayload(room.state),
		"closed_suits":     closedSuits(room.state),
		"ace_close_method": room.state.CloseMethod,
		"your_hand":        yourHand,
		"opponents":        opponents,
		"current_turn":     room.players[room.state.CurrentPlayer].displayName,
		"turn_ends_at":     time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339),
	}
}

func (room *room) broadcastGameOver() {
	room.mu.Lock()
	message := map[string]any{"type": "game_over", "results": room.results()}
	players := append([]*player(nil), room.players...)
	room.mu.Unlock()
	for _, player := range players {
		player.send(message)
	}
}

func (room *room) results() []map[string]any {
	scores := game.CalculateScores(room.state)
	sortedScores := append([]int(nil), scores[:]...)
	sort.Ints(sortedScores)
	lowest := sortedScores[0]
	results := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		rank := 1
		for _, score := range sortedScores {
			if score < scores[player.index] {
				rank++
			}
		}
		results = append(results, map[string]any{
			"display_name":   player.displayName,
			"penalty_points": scores[player.index],
			"rank":           rank,
			"is_winner":      scores[player.index] == lowest,
		})
	}
	return results
}

func (player *player) send(message map[string]any) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if err := player.conn.WriteJSON(message); err != nil {
		log.Printf("write websocket message: %v", err)
	}
}

func (player *player) sendError(message string) {
	player.send(map[string]any{"type": "error", "message": message})
}

func boardPayload(state game.GameState) map[string]any {
	board := map[string]any{}
	for _, suit := range orderedSuits {
		sequence, ok := state.Board[suit]
		if !ok {
			board[string(suit)] = nil
			continue
		}
		board[string(suit)] = map[string]any{"low": sequence.Low, "high": sequence.High}
	}
	return board
}

func closedSuits(state game.GameState) []string {
	closed := []string{}
	for _, suit := range orderedSuits {
		if state.Closed[suit] {
			closed = append(closed, string(suit))
		}
	}
	return closed
}

func parseToken(tokenString string, secret string) (*tokenClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid || claims.Sub == "" || claims.DisplayName == "" {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func parseCard(suitValue string, rankValue string) (game.Card, error) {
	suit := game.Suit(strings.ToLower(suitValue))
	if suit != game.Spades && suit != game.Hearts && suit != game.Diamonds && suit != game.Clubs {
		return game.Card{}, fmt.Errorf("unknown suit: %s", suitValue)
	}
	rank, err := parseRank(rankValue)
	if err != nil {
		return game.Card{}, err
	}
	return game.Card{Suit: suit, Rank: rank}, nil
}

func parseRank(value string) (game.Rank, error) {
	switch strings.ToUpper(value) {
	case "J":
		return game.Jack, nil
	case "Q":
		return game.Queen, nil
	case "K":
		return game.King, nil
	case "A":
		return game.Ace, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < int(game.Two) || number > int(game.Ace) {
		return 0, fmt.Errorf("unknown rank: %s", value)
	}
	return game.Rank(number), nil
}

func rankString(rank game.Rank) string {
	switch rank {
	case game.Jack:
		return "J"
	case game.Queen:
		return "Q"
	case game.King:
		return "K"
	case game.Ace:
		return "A"
	default:
		return strconv.Itoa(int(rank))
	}
}
