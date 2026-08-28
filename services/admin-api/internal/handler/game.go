package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// seasonBucketRE matches the UTC month-bucket season ids used by the leaderboard
// (e.g. "2026-06").
var seasonBucketRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type gameAnnotationRequest struct {
	Reason string `json:"reason"`
	Body   string `json:"body"`
}

func (h *AdminHandler) SearchGames(c *gin.Context) {
	filter, ok := gameFilterFromRequest(c)
	if !ok {
		return
	}
	result, err := h.store.SearchGames(c, filter)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to search games")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) GetGame(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid game ID")
		return
	}
	detail, err := h.store.GetGame(c, id)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Game not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load game")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *AdminHandler) FlagGame(c *gin.Context) {
	id, request, ok := gameFlagInput(c)
	if !ok {
		return
	}
	actor := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, actor.ID, "games.flag.create", "game_flag", id, "success")
	event.Reason = request.Reason
	result, err := h.store.FlagGame(c, id, request.Reason, event)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Game not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Unable to flag game")
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *AdminHandler) AddGameNote(c *gin.Context) {
	id, request, ok := gameNoteInput(c)
	if !ok {
		return
	}
	actor := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, actor.ID, "games.note.create", "game_note", id, "success")
	event.Reason = request.Reason
	result, err := h.store.AddGameNote(c, id, request.Reason, request.Body, event)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Game not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Unable to add game note")
		return
	}
	c.JSON(http.StatusCreated, result)
}

// gameFlagInput validates the game id and the required flag reason.
func gameFlagInput(c *gin.Context) (string, gameAnnotationRequest, bool) {
	id, request, ok := gameAnnotationInput(c)
	if !ok {
		return "", gameAnnotationRequest{}, false
	}
	return id, request, true
}

// gameNoteInput additionally requires the administrative note body.
func gameNoteInput(c *gin.Context) (string, gameAnnotationRequest, bool) {
	id, request, ok := gameAnnotationInput(c)
	if !ok {
		return "", gameAnnotationRequest{}, false
	}
	request.Body = strings.TrimSpace(request.Body)
	if request.Body == "" {
		jsonError(c, http.StatusBadRequest, "Note body is required")
		return "", gameAnnotationRequest{}, false
	}
	return id, request, true
}

func gameAnnotationInput(c *gin.Context) (string, gameAnnotationRequest, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid game ID")
		return "", gameAnnotationRequest{}, false
	}
	var request gameAnnotationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid request body")
		return "", gameAnnotationRequest{}, false
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" {
		jsonError(c, http.StatusBadRequest, "Reason is required")
		return "", gameAnnotationRequest{}, false
	}
	return id, request, true
}

func gameFilterFromRequest(c *gin.Context) (GameFilter, bool) {
	limit, offset, ok := pageFromRequest(c)
	if !ok {
		return GameFilter{}, false
	}
	filter := GameFilter{ID: strings.TrimSpace(c.Query("id")), RoomID: strings.TrimSpace(c.Query("room_id")), PlayerID: strings.TrimSpace(c.Query("player_id")), Mode: strings.TrimSpace(c.Query("mode")), SeasonID: strings.TrimSpace(c.Query("season_id")), Completion: strings.TrimSpace(c.Query("completion")), Limit: limit, Offset: offset}
	for _, value := range []string{filter.ID, filter.PlayerID} {
		if value != "" {
			if _, err := uuid.Parse(value); err != nil {
				jsonError(c, http.StatusBadRequest, "Invalid game filter ID")
				return GameFilter{}, false
			}
		}
	}
	if filter.SeasonID != "" && !seasonBucketRE.MatchString(filter.SeasonID) {
		jsonError(c, http.StatusBadRequest, "Invalid season")
		return GameFilter{}, false
	}
	if filter.Completion != "" && filter.Completion != "completed" {
		jsonError(c, http.StatusBadRequest, "Invalid completion state")
		return GameFilter{}, false
	}
	dateLayout := time.RFC3339
	for key, target := range map[string]**time.Time{"finished_from": &filter.FinishedFrom, "finished_to": &filter.FinishedTo} {
		value := strings.TrimSpace(c.Query(key))
		if value == "" {
			continue
		}
		if !strings.Contains(value, "T") {
			dateLayout = "2006-01-02"
		} else {
			dateLayout = time.RFC3339
		}
		parsed, err := time.Parse(dateLayout, value)
		if err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid "+key)
			return GameFilter{}, false
		}
		*target = &parsed
	}
	if filter.FinishedFrom != nil && filter.FinishedTo != nil && !filter.FinishedTo.After(*filter.FinishedFrom) {
		jsonError(c, http.StatusBadRequest, "finished_to must be after finished_from")
		return GameFilter{}, false
	}
	return filter, true
}
