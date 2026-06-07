package handler

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoomHandler struct{ DB *sql.DB }

type createRoomRequest struct {
	Visibility       string `json:"visibility"`
	TurnTimerSeconds int    `json:"turn_timer_seconds"`
	BotDifficulty    string `json:"bot_difficulty"`
}

type roomResponse struct {
	ID               string `json:"id"`
	InviteCode       string `json:"invite_code"`
	Visibility       string `json:"visibility"`
	TurnTimerSeconds int    `json:"turn_timer_seconds"`
	BotDifficulty    string `json:"bot_difficulty"`
	Status           string `json:"status"`
	PlayerCount      int    `json:"player_count"`
}

type joinRoomResponse struct {
	ID          string `json:"id"`
	InviteCode  string `json:"invite_code"`
	Status      string `json:"status"`
	PlayerCount int    `json:"player_count"`
}

var validTurnTimers = map[int]bool{30: true, 60: true, 90: true, 120: true}
var validBotDifficulties = map[string]bool{"easy": true, "medium": true, "hard": true}

func (h RoomHandler) Create(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	visibility := strings.ToLower(strings.TrimSpace(req.Visibility))
	if visibility != "public" && visibility != "private" {
		JSONError(c, http.StatusBadRequest, "Visibility must be 'public' or 'private'")
		return
	}
	if !validTurnTimers[req.TurnTimerSeconds] {
		JSONError(c, http.StatusBadRequest, "Turn timer must be 30, 60, 90, or 120 seconds")
		return
	}
	botDifficulty := strings.ToLower(strings.TrimSpace(req.BotDifficulty))
	if botDifficulty == "" {
		botDifficulty = "medium"
	}
	if !validBotDifficulties[botDifficulty] {
		JSONError(c, http.StatusBadRequest, "Bot difficulty must be 'easy', 'medium', or 'hard'")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	room, err := repository.CreateRoom(h.DB, visibility, req.TurnTimerSeconds, botDifficulty, userID)
	if err != nil {
		log.Printf("rooms: create room: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to create room")
		return
	}
	if _, err := repository.AddPlayerToRoom(h.DB, room.ID, userID, claims.DisplayName); err != nil {
		log.Printf("rooms: join created room: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to join created room")
		return
	}
	c.JSON(http.StatusCreated, newRoomResponse(repository.RoomWithPlayerCount{Room: *room, PlayerCount: 1}))
}

func (h RoomHandler) ListPublic(c *gin.Context) {
	rooms, err := repository.GetPublicWaitingRooms(h.DB)
	if err != nil {
		log.Printf("rooms: list public: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to list rooms")
		return
	}
	responses := make([]roomResponse, 0, len(rooms))
	for _, room := range rooms {
		responses = append(responses, newRoomResponse(room))
	}
	c.JSON(http.StatusOK, responses)
}

// LiveGames is public: in-progress public rooms a spectator can watch. Returns
// the seated players' identities so the client can deep-link /watch/:roomId.
func (h RoomHandler) LiveGames(c *gin.Context) {
	games, err := repository.GetLiveGames(h.DB)
	if err != nil {
		log.Printf("rooms: list live games: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to list live games")
		return
	}
	c.JSON(http.StatusOK, gin.H{"games": games})
}

func (h RoomHandler) Join(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	if code == "" {
		JSONError(c, http.StatusBadRequest, "Invite code is required")
		return
	}
	room, err := repository.GetRoomByInviteCode(h.DB, code)
	if err != nil {
		log.Printf("rooms: get by invite: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load room")
		return
	}
	if room == nil {
		JSONError(c, http.StatusNotFound, "Room not found")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	newCount, err := repository.AddPlayerToRoom(h.DB, room.ID, userID, claims.DisplayName)
	if err != nil {
		status, msg := classifyJoinError(err)
		JSONError(c, status, msg)
		return
	}
	updated, err := repository.GetRoomByID(h.DB, room.ID)
	if err != nil || updated == nil {
		finalStatus := room.Status
		if newCount == 4 {
			finalStatus = "in_progress"
		}
		c.JSON(http.StatusOK, joinRoomResponse{ID: room.ID.String(), InviteCode: room.InviteCode, Status: finalStatus, PlayerCount: newCount})
		return
	}
	c.JSON(http.StatusOK, joinRoomResponse{ID: updated.ID.String(), InviteCode: updated.InviteCode, Status: updated.Status, PlayerCount: updated.PlayerCount})
}

func (h RoomHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid room ID")
		return
	}
	room, err := repository.GetRoomByID(h.DB, id)
	if err != nil {
		log.Printf("rooms: get by id: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load room")
		return
	}
	if room == nil {
		JSONError(c, http.StatusNotFound, "Room not found")
		return
	}
	c.JSON(http.StatusOK, newRoomResponse(*room))
}

type updateRoomStatusRequest struct {
	Status string `json:"status"`
}

// UpdateStatus is the internal endpoint the WS service calls when a game
// transitions to in_progress (host pressed start) or finished. It mirrors
// /internal/games in being unauthenticated and intended for the docker-internal
// network only.
func (h RoomHandler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid room ID")
		return
	}
	var req updateRoomStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "in_progress" && status != "finished" {
		JSONError(c, http.StatusBadRequest, "Status must be 'in_progress' or 'finished'")
		return
	}
	if err := repository.UpdateRoomStatus(h.DB, id, status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			JSONError(c, http.StatusNotFound, "Room not found")
			return
		}
		log.Printf("rooms: update status: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to update room status")
		return
	}
	c.Status(http.StatusNoContent)
}

// RemovePlayer is the internal endpoint the WS service calls when a player
// leaves a room during the lobby phase. Removing the membership row keeps the
// public player count accurate and lets the same user re-join later instead of
// hitting "already in room". Like /internal/games and /internal/rooms/:id/status
// it is unauthenticated and intended for the docker-internal network only.
func (h RoomHandler) RemovePlayer(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid room ID")
		return
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if _, err := repository.RemovePlayerFromRoom(h.DB, roomID, userID); err != nil {
		log.Printf("rooms: remove player: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to remove player")
		return
	}
	c.Status(http.StatusNoContent)
}

type reconcileRoomsRequest struct {
	ActiveRoomIDs []string `json:"active_room_ids"`
}

// staleRoomTTL is how old a 'waiting' room must be before the reconcile sweep
// will delete it when the WS service reports no live presence for it. It covers
// the create->WS-connect window and brief WS restarts.
const staleRoomTTL = 2 * time.Minute

// Reconcile is the internal endpoint the WS service calls periodically with the
// set of room IDs it is actively tracking in memory. Any 'waiting' room not in
// that set (and older than staleRoomTTL) has no live presence and is deleted so
// orphaned lobbies — a DB membership row whose player never connected over
// WebSocket — stop lingering in the public list. Unauthenticated and intended
// for the docker-internal network only, like the other /internal endpoints.
func (h RoomHandler) Reconcile(c *gin.Context) {
	var req reconcileRoomsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	activeIDs := make([]uuid.UUID, 0, len(req.ActiveRoomIDs))
	for _, raw := range req.ActiveRoomIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			// Skip unparseable IDs rather than failing the whole sweep; a bad
			// entry shouldn't keep stale rooms alive.
			continue
		}
		activeIDs = append(activeIDs, id)
	}
	deleted, err := repository.DeleteStaleWaitingRooms(h.DB, activeIDs, time.Now().Add(-staleRoomTTL))
	if err != nil {
		log.Printf("rooms: reconcile stale rooms: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to reconcile rooms")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func newRoomResponse(room repository.RoomWithPlayerCount) roomResponse {
	return roomResponse{ID: room.ID.String(), InviteCode: room.InviteCode, Visibility: room.Visibility, TurnTimerSeconds: room.TurnTimerSeconds, BotDifficulty: room.BotDifficulty, Status: room.Status, PlayerCount: room.PlayerCount}
}

func classifyJoinError(err error) (int, string) {
	switch {
	case errors.Is(err, repository.ErrRoomFull):
		return http.StatusConflict, "Room is full"
	case errors.Is(err, repository.ErrRoomNotAcceptingPlayers):
		return http.StatusConflict, "Room is not accepting players"
	case errors.Is(err, repository.ErrPlayerAlreadyInRoom):
		return http.StatusConflict, "Already in room"
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound, "Room not found"
	default:
		log.Printf("rooms: unexpected join error: %v", err)
		return http.StatusInternalServerError, "Failed to join room"
	}
}
