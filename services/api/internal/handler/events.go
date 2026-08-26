package handler

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventHandler struct {
	DB          *sql.DB
	AppTimezone *time.Location
}

func (h EventHandler) Detail(c *gin.Context) {
	var userID *uuid.UUID
	if claims, ok := middleware.ClaimsFromContext(c); ok && !claims.IsGuest {
		if id, err := uuid.Parse(claims.Sub); err == nil {
			userID = &id
		}
	}
	detail, err := repository.GetEventDetail(h.DB, c.Param("slug"), userID, time.Now(), h.AppTimezone)
	if err != nil {
		log.Printf("events: detail: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load event")
		return
	}
	if detail == nil {
		JSONError(c, http.StatusNotFound, "Event not found")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h EventHandler) ClaimCheckIn(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Registered account required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	result, err := repository.ClaimEventCheckIn(h.DB, c.Param("slug"), userID, time.Now(), h.AppTimezone)
	if err != nil {
		if errors.Is(err, repository.ErrEventNotActive) {
			JSONError(c, http.StatusConflict, "Event is not active")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			JSONError(c, http.StatusNotFound, "Event not found")
			return
		}
		log.Printf("events: claim check-in: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to claim event check-in")
		return
	}
	c.JSON(http.StatusOK, result)
}
