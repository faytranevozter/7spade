package main

import (
	"fmt"
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
	mu        sync.Mutex
	upgrader  websocket.Upgrader
}

type room struct {
	id      string
	players []*player
	state   game.GameState
	started bool
	mu      sync.Mutex
}

type player struct {
	id          string
	displayName string
	index       int
	conn        *websocket.Conn
	mu          sync.Mutex
}

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
	return &GameServer{
		jwtSecret: jwtSecret,
		rooms:     map[string]*room{},
		upgrader:  websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func (server *GameServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("ws", map[string]dependencyCheck{
		"postgres": postgresCheck(""),
		"redis":    redisCheck(""),
	}))
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
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": err.Error()})
		_ = conn.Close()
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
		gameRoom = &room{id: roomID}
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
	player := &player{id: claims.Sub, displayName: claims.DisplayName, index: len(gameRoom.players), conn: conn}
	gameRoom.players = append(gameRoom.players, player)
	if len(gameRoom.players) == game.PlayerCount {
		gameRoom.state, _ = game.Deal(time.Now().UnixNano())
		gameRoom.started = true
	}
	return gameRoom, player, nil
}

func (room *room) readLoop(player *player) {
	defer player.conn.Close()
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
		player.send(map[string]any{"type": "error", "message": "game has not started"})
		return
	}
	if room.state.CurrentPlayer != player.index {
		room.mu.Unlock()
		player.send(map[string]any{"type": "error", "message": "not your turn"})
		return
	}

	card, err := parseCard(message.Suit, message.Rank)
	if err != nil {
		room.mu.Unlock()
		player.send(map[string]any{"type": "error", "message": err.Error()})
		return
	}

	state := room.state
	switch message.Type {
	case "play_card":
		if card.Rank == game.Ace && message.Method != "" {
			state, err = game.ApplyAceClose(room.state, player.index, card.Suit, game.CloseMethod(message.Method))
		} else {
			state, err = game.ApplyMove(room.state, player.index, card, false)
		}
	case "place_facedown":
		state, err = game.ApplyMove(room.state, player.index, card, true)
	default:
		err = fmt.Errorf("unknown message type: %s", message.Type)
	}
	if err != nil {
		room.mu.Unlock()
		player.send(map[string]any{"type": "error", "message": err.Error()})
		return
	}
	room.state = state
	gameOver := game.IsGameOver(room.state)
	room.mu.Unlock()

	if gameOver {
		room.broadcastGameOver()
		return
	}
	room.broadcastState()
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
	_ = player.conn.WriteJSON(message)
}

func boardPayload(state game.GameState) map[string]any {
	board := map[string]any{}
	for _, suit := range []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs} {
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
	for _, suit := range []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs} {
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
