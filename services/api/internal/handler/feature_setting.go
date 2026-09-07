package handler

import (
	"database/sql"
	"net/http"

	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
)

type FeatureSettingHandler struct {
	DB *sql.DB
}

func (h FeatureSettingHandler) ApplicationControls(c *gin.Context) {
	keys := []string{
		repository.SettingNewRegistrations,
		repository.SettingGuestAccess,
		repository.SettingRoomCreation,
		repository.SettingQuickPlay,
		repository.SettingNewGameStarts,
		repository.SettingSpectatorAccess,
		repository.SettingEmotes,
	}
	controls := make(map[string]bool, len(keys))
	for _, key := range keys {
		enabled, err := repository.FeatureSettingEnabled(h.DB, key)
		if err != nil {
			JSONError(c, http.StatusInternalServerError, "Failed to load application controls")
			return
		}
		controls[key] = enabled
	}
	c.JSON(http.StatusOK, controls)
}
