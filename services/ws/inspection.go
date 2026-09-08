package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

const inspectionSecretHeader = "X-WS-Inspection-Secret"

type roomInspectionResponse struct {
	RoomID        string                 `json:"room_id"`
	Phase         string                 `json:"phase,omitempty"`
	Players       []roomInspectionPlayer `json:"players,omitempty"`
	TurnDeadline  *time.Time             `json:"turn_deadline,omitempty"`
	SnapshotAgeMS int64                  `json:"snapshot_age_ms"`
	StateVersion  int64                  `json:"state_version"`
	Owner         roomInspectionOwner    `json:"owner"`
}

type roomInspectionPlayer struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Seat        int    `json:"seat"`
	Bot         bool   `json:"is_bot"`
	Connected   bool   `json:"connected"`
}

type roomInspectionOwner struct {
	Role         string `json:"role"`
	ReplicaID    string `json:"replica_id,omitempty"`
	FencingToken int64  `json:"fencing_token,omitempty"`
}

func (server *GameServer) handleRoomInspection(w http.ResponseWriter, r *http.Request) {
	if server.inspectionSecret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get(inspectionSecretHeader)), []byte(server.inspectionSecret)) != 1 {
		writeInspectionJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	roomID := r.PathValue("roomID")
	owner := server.replicaID
	var token int64
	if server.leases != nil {
		var err error
		owner, token, err = server.leases.Inspect(r.Context(), roomID)
		if err != nil {
			writeInspectionJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "lease dependency unavailable"})
			return
		}
		if owner != server.replicaID {
			role := "edge"
			if owner == "" {
				role = "unowned"
			}
			writeInspectionJSON(w, http.StatusOK, roomInspectionResponse{RoomID: roomID, Owner: roomInspectionOwner{Role: role, ReplicaID: owner, FencingToken: token}})
			return
		}
	}

	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil {
		writeInspectionJSON(w, http.StatusNotFound, map[string]string{"error": "live room not found"})
		return
	}

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if gameRoom.relay != nil {
		gameRoom.relay.mu.Lock()
		owns := gameRoom.relay.owner && gameRoom.relay.token == token
		gameRoom.relay.mu.Unlock()
		if !owns {
			writeInspectionJSON(w, http.StatusConflict, roomInspectionResponse{RoomID: roomID, Owner: roomInspectionOwner{Role: "lease_lost", ReplicaID: owner, FencingToken: token}})
			return
		}
	}

	phase := "lobby"
	if gameRoom.phase == phasePlaying {
		phase = "playing"
		if game.IsGameOver(gameRoom.state) {
			phase = "finished"
		}
	}
	players := make([]roomInspectionPlayer, 0, len(gameRoom.players))
	for _, p := range gameRoom.players {
		p.mu.Lock()
		connected := p.isBot || !p.disconnected
		p.mu.Unlock()
		players = append(players, roomInspectionPlayer{UserID: p.sub, DisplayName: p.displayName, Seat: p.index, Bot: p.isBot, Connected: connected})
	}
	var deadline *time.Time
	if !gameRoom.turnExpiresAt.IsZero() {
		value := gameRoom.turnExpiresAt
		deadline = &value
	}
	age := int64(0)
	if !gameRoom.snapshotSavedAt.IsZero() {
		age = max(time.Since(gameRoom.snapshotSavedAt).Milliseconds(), 0)
	}
	writeInspectionJSON(w, http.StatusOK, roomInspectionResponse{
		RoomID: roomID, Phase: phase, Players: players, TurnDeadline: deadline,
		SnapshotAgeMS: age, StateVersion: gameRoom.snapVersion,
		Owner: roomInspectionOwner{Role: "owner", ReplicaID: owner, FencingToken: token},
	})
}

func (server *GameServer) handleHiddenRoomStateInspection(w http.ResponseWriter, r *http.Request) {
	if server.inspectionSecret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get(inspectionSecretHeader)), []byte(server.inspectionSecret)) != 1 {
		writeInspectionJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	roomID := r.PathValue("roomID")
	owner := server.replicaID
	var token int64
	if server.leases != nil {
		var err error
		owner, token, err = server.leases.Inspect(r.Context(), roomID)
		if err != nil {
			writeInspectionJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "lease dependency unavailable"})
			return
		}
		if owner != server.replicaID {
			writeInspectionJSON(w, http.StatusConflict, map[string]string{"error": "room not owned by this replica"})
			return
		}
	}

	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil {
		writeInspectionJSON(w, http.StatusNotFound, map[string]string{"error": "live room not found"})
		return
	}

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if gameRoom.relay != nil {
		gameRoom.relay.mu.Lock()
		owns := gameRoom.relay.owner && gameRoom.relay.token == token
		gameRoom.relay.mu.Unlock()
		if !owns {
			writeInspectionJSON(w, http.StatusConflict, roomInspectionResponse{RoomID: roomID, Owner: roomInspectionOwner{Role: "lease_lost", ReplicaID: owner, FencingToken: token}})
			return
		}
	}
	writeInspectionJSON(w, http.StatusOK, gameRoom.state)
}

func writeInspectionJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
