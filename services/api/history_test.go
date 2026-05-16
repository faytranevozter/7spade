package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHistoryHandlerReturnsOnlyAuthenticatedPlayersGames(t *testing.T) {
	db := setupRoomsTestDB(t)
	defer db.Close()

	userID := createTestUser(t, db, "winner@example.com", "Winner")
	otherUserID := createTestUser(t, db, "other@example.com", "Other")
	roomID := uuid.New()
	finishedAt := time.Now().UTC()

	result := GameResult{
		RoomID:     roomID.String(),
		StartedAt:  finishedAt.Add(-20 * time.Minute),
		FinishedAt: finishedAt,
		Players: []GameResultPlayer{
			{UserID: userID.String(), DisplayName: "Winner", PenaltyPoints: 4, Rank: 1, IsWinner: true},
			{UserID: otherUserID.String(), DisplayName: "Other", PenaltyPoints: 11, Rank: 2, IsWinner: false},
			{DisplayName: "Guest", PenaltyPoints: 15, Rank: 3, IsWinner: false},
		},
	}
	if _, err := SaveGame(db, result); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}

	req := authedRequest(http.MethodGet, "/history?page=1&per_page=10", nil, userID, "Winner")
	rec := httptest.NewRecorder()
	historyHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var response historyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Total != 1 || response.Page != 1 || len(response.Games) != 1 {
		t.Fatalf("unexpected history response: %+v", response)
	}
	game := response.Games[0]
	if game.RoomID != roomID.String() || game.PenaltyPoints != 4 || game.Rank != 1 || !game.IsWinner {
		t.Fatalf("unexpected game history row: %+v", game)
	}
}
