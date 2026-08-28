package model

import "time"

type Game struct {
	ID              string     `json:"game_id"`
	RoomID          string     `json:"room_id"`
	RoomName        string     `json:"room_name"`
	Mode            string     `json:"mode"`
	SeasonID        string     `json:"season_id,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	ReplayAvailable bool       `json:"replay_available"`
}

type GamePlayer struct {
	UserID        string     `json:"user_id,omitempty"`
	DisplayName   string     `json:"display_name"`
	PenaltyPoints int        `json:"penalty_points"`
	Rank          int        `json:"rank"`
	IsWinner      bool       `json:"is_winner"`
	IsBot         bool       `json:"is_bot"`
	IsGuest       bool       `json:"is_guest"`
	Team          *int       `json:"team,omitempty"`
	FaceDownCards []GameCard `json:"facedown_cards"`
}

type GameCard struct {
	Suit   string `json:"suit"`
	Rank   int    `json:"rank"`
	Points int    `json:"points"`
}

type GameMove struct {
	Index        int    `json:"index"`
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit,omitempty"`
	Rank         int    `json:"rank,omitempty"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

type GameFlag struct {
	ID        string    `json:"id"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
type GameNote struct {
	ID        string    `json:"id"`
	Reason    string    `json:"reason"`
	Body      string    `json:"body"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
type GameDetail struct {
	Game    Game         `json:"game"`
	Players []GamePlayer `json:"players"`
	Moves   []GameMove   `json:"moves"`
	Flags   []GameFlag   `json:"flags"`
	Notes   []GameNote   `json:"notes"`
}
type GameFilter struct {
	ID, RoomID, PlayerID, Mode, SeasonID, Completion string
	FinishedFrom, FinishedTo                         *time.Time
	Limit, Offset                                    int
}
type GamePage struct {
	Games  []Game `json:"games"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Total  int    `json:"total"`
}
