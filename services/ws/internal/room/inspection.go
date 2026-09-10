package room

import (
	"context"
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

// Inspect returns an isolated view; no live mutable state escapes the lock.
func (server *Manager) Inspect(ctx context.Context, roomID string, hidden bool) (any, int) {
	owner := ""
	if server.cluster != nil {
		owner = server.cluster.ID()
	}
	var token int64
	if server.relayEnabled() {
		var err error
		owner, token, err = server.cluster.Inspect(ctx, roomID)
		if err != nil {
			return map[string]string{"error": "lease dependency unavailable"}, 503
		}
		if owner != server.cluster.ID() {
			role := "edge"
			if owner == "" {
				role = "unowned"
			}
			if hidden {
				return map[string]string{"error": "room not owned by this replica"}, 409
			}
			return roomInspectionResponse{RoomID: roomID, Owner: roomInspectionOwner{Role: role, ReplicaID: owner, FencingToken: token}}, 200
		}
	}

	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil {
		return map[string]string{"error": "live room not found"}, 404
	}

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if gameRoom.relay != nil {
		owns := gameRoom.relay.Matches(token)
		if !owns {
			return roomInspectionResponse{RoomID: roomID, Owner: roomInspectionOwner{Role: "lease_lost", ReplicaID: owner, FencingToken: token}}, 409
		}
	}

	if hidden {
		return cloneGameState(gameRoom.state), 200
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
	return roomInspectionResponse{
		RoomID: roomID, Phase: phase, Players: players, TurnDeadline: deadline,
		SnapshotAgeMS: age, StateVersion: gameRoom.snapVersion,
		Owner: roomInspectionOwner{Role: "owner", ReplicaID: owner, FencingToken: token},
	}, 200
}
