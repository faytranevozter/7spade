package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type GameResult struct {
	RoomID     string             `json:"room_id"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	Players    []GameResultPlayer `json:"players"`
}

type GameResultPlayer struct {
	UserID        string `json:"user_id,omitempty"`
	DisplayName   string `json:"display_name"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
}

type historyGameResponse struct {
	GameID        string `json:"game_id"`
	RoomID        string `json:"room_id"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
}

type historyResponse struct {
	Games []historyGameResponse `json:"games"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
}

func SaveGame(db *sql.DB, result GameResult) (uuid.UUID, error) {
	tx, err := db.Begin()
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin save game: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.Rollback(); err != nil {
			log.Printf("rollback save game: %v", err)
		}
	}()

	gameID := uuid.New()
	if _, err := tx.Exec(`INSERT INTO games (id, room_id, started_at, finished_at) VALUES ($1, $2, $3, $4)`, gameID, result.RoomID, result.StartedAt, result.FinishedAt); err != nil {
		return uuid.Nil, fmt.Errorf("insert game: %w", err)
	}
	for _, player := range result.Players {
		var userID *uuid.UUID
		if player.UserID != "" {
			parsed, err := uuid.Parse(player.UserID)
			if err != nil {
				return uuid.Nil, fmt.Errorf("parse player user id: %w", err)
			}
			userID = &parsed
		}
		_, err := tx.Exec(`
			INSERT INTO game_players (game_id, user_id, display_name, penalty_points, rank, is_winner)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, gameID, userID, player.DisplayName, player.PenaltyPoints, player.Rank, player.IsWinner)
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert game player: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("commit save game: %w", err)
	}
	committed = true
	return gameID, nil
}

func historyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		userID, err := uuid.Parse(claims.Sub)
		if err != nil || claims.IsGuest {
			writeError(w, http.StatusUnauthorized, "Logged-in user required")
			return
		}

		page := positiveQueryInt(r, "page", 1)
		perPage := positiveQueryInt(r, "per_page", 10)
		if perPage > 50 {
			perPage = 50
		}
		games, total, err := GetPlayerHistory(db, userID, page, perPage)
		if err != nil {
			log.Printf("historyHandler: GetPlayerHistory failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to load history")
			return
		}
		writeJSON(w, http.StatusOK, historyResponse{Games: games, Total: total, Page: page})
	}
}

func GetPlayerHistory(db *sql.DB, userID uuid.UUID, page int, perPage int) ([]historyGameResponse, int, error) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM game_players WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count history: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := db.Query(`
		SELECT g.id, g.room_id, g.started_at, g.finished_at, gp.penalty_points, gp.rank, gp.is_winner
		FROM game_players gp
		JOIN games g ON g.id = gp.game_id
		WHERE gp.user_id = $1
		ORDER BY g.finished_at DESC
		LIMIT $2 OFFSET $3
	`, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	games := []historyGameResponse{}
	for rows.Next() {
		var game historyGameResponse
		var startedAt time.Time
		var finishedAt time.Time
		if err := rows.Scan(&game.GameID, &game.RoomID, &startedAt, &finishedAt, &game.PenaltyPoints, &game.Rank, &game.IsWinner); err != nil {
			return nil, 0, fmt.Errorf("scan history: %w", err)
		}
		game.StartedAt = startedAt.UTC().Format(time.RFC3339)
		game.FinishedAt = finishedAt.UTC().Format(time.RFC3339)
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate history: %w", err)
	}
	return games, total, nil
}

func saveGameHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var result GameResult
		if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		gameID, err := SaveGame(db, result)
		if err != nil {
			log.Printf("saveGameHandler: SaveGame failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to save game")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"game_id": gameID.String()})
	}
}

func positiveQueryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
