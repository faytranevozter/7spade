package handler

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type HistoryHandler struct{ DB *sql.DB }

func (h HistoryHandler) List(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}
	page := positiveQueryInt(c, "page", 1)
	perPage := positiveQueryInt(c, "per_page", 10)
	if perPage > 50 {
		perPage = 50
	}
	games, total, err := repository.GetPlayerHistory(h.DB, userID, page, perPage)
	if err != nil {
		log.Printf("history: get player history: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load history")
		return
	}
	c.JSON(http.StatusOK, gin.H{"games": games, "total": total, "page": page})
}

func (h HistoryHandler) Save(c *gin.Context) {
	var result repository.GameResult
	if err := c.ShouldBindJSON(&result); err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	saveResult, err := repository.SaveGame(h.DB, result)
	if err != nil {
		log.Printf("history: save game: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to save game: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"game_id": saveResult.GameID.String(), "deltas": saveResult.Deltas})
}

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
