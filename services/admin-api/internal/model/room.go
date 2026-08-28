package model

import "time"

type Room struct {
	ID               string    `json:"id"`
	InviteCode       string    `json:"invite_code"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Visibility       string    `json:"visibility"`
	GameMode         string    `json:"game_mode"`
	PracticeMode     bool      `json:"practice_mode"`
	MaxPlayers       int       `json:"max_players"`
	DeckCount        int       `json:"deck_count"`
	ScoringMode      string    `json:"scoring_mode"`
	TeamMode         string    `json:"team_mode"`
	TurnTimerSeconds int       `json:"turn_timer_seconds"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	PlayerCount      int       `json:"player_count"`
}

type RoomPlayer struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
}

type RoomFilter struct {
	ID, InviteCode, Status, Visibility, Mode string
	CreatedFrom, CreatedTo                   *time.Time
	Limit, Offset                            int
}

type RoomPage struct {
	Rooms  []Room `json:"rooms"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type RoomDetail struct {
	Room    Room         `json:"room"`
	Players []RoomPlayer `json:"players"`
}

type LiveRoomPlayer struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Connected   bool   `json:"connected"`
	IsBot       bool   `json:"is_bot,omitempty"`
	Seat        int    `json:"seat,omitempty"`
}

type LiveRoomSummary struct {
	Role               string           `json:"role"`
	Phase              string           `json:"phase"`
	Players            []LiveRoomPlayer `json:"players"`
	TurnDeadline       *time.Time       `json:"turn_deadline,omitempty"`
	SnapshotAgeSeconds float64          `json:"snapshot_age_seconds"`
	StateVersion       int64            `json:"state_version"`
	OwnerID            string           `json:"owner_id"`
	FenceToken         int64            `json:"fence_token"`
}

type LiveRoomResult struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason,omitempty"`
	Summary   *LiveRoomSummary `json:"summary,omitempty"`
}

type RoomInvestigation struct {
	RoomDetail
	Live LiveRoomResult `json:"live"`
}
