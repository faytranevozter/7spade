package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type createRoomRequest struct {
	Visibility       string `json:"visibility"`
	TurnTimerSeconds int    `json:"turn_timer_seconds"`
}

type roomResponse struct {
	ID               string `json:"id"`
	InviteCode       string `json:"invite_code"`
	Visibility       string `json:"visibility"`
	TurnTimerSeconds int    `json:"turn_timer_seconds"`
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

// createRoomHandler handles POST /rooms — creates a room, adds the creator as the first player.
func createRoomHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		var req createRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		visibility := strings.ToLower(strings.TrimSpace(req.Visibility))
		if visibility != "public" && visibility != "private" {
			writeError(w, http.StatusBadRequest, "Visibility must be 'public' or 'private'")
			return
		}

		if !validTurnTimers[req.TurnTimerSeconds] {
			writeError(w, http.StatusBadRequest, "Turn timer must be 30, 60, 90, or 120 seconds")
			return
		}

		userID, err := uuid.Parse(claims.Sub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid user identity")
			return
		}

		room, err := CreateRoom(db, visibility, req.TurnTimerSeconds, userID)
		if err != nil {
			log.Printf("createRoomHandler: CreateRoom failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create room")
			return
		}

		// Add the creator as the first player.
		if _, err := AddPlayerToRoom(db, room.ID, userID, claims.DisplayName); err != nil {
			log.Printf("createRoomHandler: AddPlayerToRoom failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to join created room")
			return
		}

		writeJSON(w, http.StatusCreated, roomResponse{
			ID:               room.ID.String(),
			InviteCode:       room.InviteCode,
			Visibility:       room.Visibility,
			TurnTimerSeconds: room.TurnTimerSeconds,
			Status:           room.Status,
			PlayerCount:      1,
		})
	}
}

// listPublicRoomsHandler handles GET /rooms — returns public rooms with status='waiting'.
func listPublicRoomsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rooms, err := GetPublicWaitingRooms(db)
		if err != nil {
			log.Printf("listPublicRoomsHandler: GetPublicWaitingRooms failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to list rooms")
			return
		}

		responses := make([]roomResponse, 0, len(rooms))
		for _, room := range rooms {
			responses = append(responses, roomResponse{
				ID:               room.ID.String(),
				InviteCode:       room.InviteCode,
				Visibility:       room.Visibility,
				TurnTimerSeconds: room.TurnTimerSeconds,
				Status:           room.Status,
				PlayerCount:      room.PlayerCount,
			})
		}

		writeJSON(w, http.StatusOK, responses)
	}
}

// joinRoomHandler handles POST /rooms/{code}/join — adds the authenticated user.
func joinRoomHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
		if code == "" {
			writeError(w, http.StatusBadRequest, "Invite code is required")
			return
		}

		room, err := GetRoomByInviteCode(db, code)
		if err != nil {
			log.Printf("joinRoomHandler: GetRoomByInviteCode failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to load room")
			return
		}
		if room == nil {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}

		userID, err := uuid.Parse(claims.Sub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid user identity")
			return
		}

		newCount, err := AddPlayerToRoom(db, room.ID, userID, claims.DisplayName)
		if err != nil {
			status, msg := classifyJoinError(err)
			writeJSON(w, status, errorResponse{Error: msg})
			return
		}

		// Re-read final status (in case it transitioned to in_progress).
		updated, err := GetRoomByID(db, room.ID)
		if err != nil || updated == nil {
			// Fall back to derived values.
			finalStatus := room.Status
			if newCount == 4 {
				finalStatus = "in_progress"
			}
			writeJSON(w, http.StatusOK, joinRoomResponse{
				ID:          room.ID.String(),
				InviteCode:  room.InviteCode,
				Status:      finalStatus,
				PlayerCount: newCount,
			})
			return
		}

		writeJSON(w, http.StatusOK, joinRoomResponse{
			ID:          updated.ID.String(),
			InviteCode:  updated.InviteCode,
			Status:      updated.Status,
			PlayerCount: updated.PlayerCount,
		})
	}
}

// getRoomHandler handles GET /rooms/{id} — returns room status and player count.
func getRoomHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid room ID")
			return
		}

		room, err := GetRoomByID(db, id)
		if err != nil {
			log.Printf("getRoomHandler: GetRoomByID failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to load room")
			return
		}
		if room == nil {
			writeError(w, http.StatusNotFound, "Room not found")
			return
		}

		writeJSON(w, http.StatusOK, roomResponse{
			ID:               room.ID.String(),
			InviteCode:       room.InviteCode,
			Visibility:       room.Visibility,
			TurnTimerSeconds: room.TurnTimerSeconds,
			Status:           room.Status,
			PlayerCount:      room.PlayerCount,
		})
	}
}

// classifyJoinError maps domain errors from AddPlayerToRoom to HTTP statuses.
func classifyJoinError(err error) (int, string) {
	if err == nil {
		return http.StatusInternalServerError, "Unknown error"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "room is full"):
		return http.StatusConflict, "Room is full"
	case strings.Contains(msg, "not accepting players"):
		return http.StatusConflict, "Room is not accepting players"
	case strings.Contains(msg, "already in room"):
		return http.StatusConflict, "Already in room"
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound, "Room not found"
	default:
		log.Printf("classifyJoinError: unexpected error: %v", err)
		return http.StatusInternalServerError, "Failed to join room"
	}
}
