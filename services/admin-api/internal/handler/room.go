package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *AdminHandler) SearchRooms(c *gin.Context) {
	filter, ok := roomFilterFromRequest(c)
	if !ok {
		return
	}
	result, err := h.store.SearchRooms(c, filter)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to search rooms")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("id")
	if _, err := uuid.Parse(roomID); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid room ID")
		return
	}
	detail, err := h.store.GetRoom(c, roomID)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Room not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load room")
		return
	}
	result := RoomInvestigation{RoomDetail: detail}
	if h.liveRooms == nil {
		result.Live.Reason = "not_configured"
	} else if summary, err := h.liveRooms.RoomSummary(c, roomID); err == nil {
		if summary.Role != "" && summary.Role != "owner" {
			result.Live.Reason = summary.Role
		} else {
			result.Live.Available = true
			result.Live.Summary = &summary
		}
	} else {
		result.Live.Reason = "unavailable"
	}
	c.JSON(http.StatusOK, result)
}

func roomFilterFromRequest(c *gin.Context) (RoomFilter, bool) {
	limit, offset, ok := pageFromRequest(c)
	if !ok {
		return RoomFilter{}, false
	}
	filter := RoomFilter{ID: strings.TrimSpace(c.Query("id")), InviteCode: strings.ToUpper(strings.TrimSpace(c.Query("invite_code"))), Status: strings.TrimSpace(c.Query("status")), Visibility: strings.TrimSpace(c.Query("visibility")), Mode: strings.TrimSpace(c.Query("mode")), Limit: limit, Offset: offset}
	if filter.ID != "" {
		if _, err := uuid.Parse(filter.ID); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid room ID")
			return RoomFilter{}, false
		}
	}
	for key, target := range map[string]**time.Time{"created_from": &filter.CreatedFrom, "created_to": &filter.CreatedTo} {
		if value := c.Query(key); value != "" {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				jsonError(c, http.StatusBadRequest, "Invalid "+key)
				return RoomFilter{}, false
			}
			*target = &parsed
		}
	}
	if filter.CreatedFrom != nil && filter.CreatedTo != nil && !filter.CreatedTo.After(*filter.CreatedFrom) {
		jsonError(c, http.StatusBadRequest, "created_to must be after created_from")
		return RoomFilter{}, false
	}
	return filter, true
}
