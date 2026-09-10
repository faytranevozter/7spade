package protocol

import (
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/store"
)

type RoomSettings struct {
	TurnTimerSeconds int               `json:"turn_timer_seconds"`
	BotDifficulty    string            `json:"bot_difficulty"`
	PracticeMode     bool              `json:"practice_mode"`
	GameMode         string            `json:"game_mode"`
	MaxPlayers       int               `json:"max_players"`
	DeckCount        int               `json:"deck_count"`
	ScoringMode      string            `json:"scoring_mode"`
	CustomScores     map[game.Rank]int `json:"custom_scores,omitempty"`
	TeamMode         string            `json:"team_mode"`
}

type SkinGrant = store.SkinGrant

type PlayerDelta struct {
	UserID        string      `json:"user_id"`
	RatingDelta   *int        `json:"rating_delta,omitempty"`
	RatingAfter   *int        `json:"rating_after,omitempty"`
	XPDelta       int         `json:"xp_delta"`
	XPAfter       int64       `json:"xp_after"`
	Level         int         `json:"level"`
	NewSkinGrants []SkinGrant `json:"new_skin_grants,omitempty"`
}

type SavedGameResult struct {
	RoomID       string            `json:"room_id"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at"`
	Players      []SavedGamePlayer `json:"players"`
	InitialHands [][]SavedCard     `json:"initial_hands,omitempty"`
	Moves        []SavedReplayMove `json:"moves,omitempty"`
}

type SavedGamePlayer struct {
	UserID        string              `json:"user_id,omitempty"`
	SubjectID     string              `json:"subject_id,omitempty"`
	DisplayName   string              `json:"display_name"`
	PenaltyPoints int                 `json:"penalty_points"`
	Rank          int                 `json:"rank"`
	IsWinner      bool                `json:"is_winner"`
	IsBot         bool                `json:"is_bot"`
	IsGuest       bool                `json:"is_guest"`
	Team          *int                `json:"team,omitempty"`
	FaceDownCards []SavedRevealedCard `json:"facedown_cards,omitempty"`
	Index         int                 `json:"index"`
}

type SavedRevealedCard struct {
	Suit   string `json:"suit"`
	Rank   int    `json:"rank"`
	Points int    `json:"points"`
}

// savedCard is the wire form of a card used in replay payloads. Suit is the
// engine string (spades/hearts/diamonds/clubs); Rank is the engine int (2..14).
type SavedCard struct {
	Suit string `json:"suit"`
	Rank int    `json:"rank"`
}

// savedReplayMove is the wire form of a recorded move. Index is its 0-based
// position in the game's move sequence.
type SavedReplayMove struct {
	Index        int    `json:"index"`
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit"`
	Rank         int    `json:"rank"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

type ClientMessage struct {
	Type   string `json:"type"`
	Suit   string `json:"suit"`
	Rank   string `json:"rank"`
	Method string `json:"method"`
	Ready  bool   `json:"ready"`
	Emote  string `json:"emote"`
	Target int    `json:"target"`
	Team   int    `json:"team"`
}
