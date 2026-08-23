package handler

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SkinHandler struct {
	DB *sql.DB
}

type equipSkinRequest struct {
	SkinID string `json:"skin_id"`
}

func (h SkinHandler) Catalog(c *gin.Context) {
	catalog, err := repository.GetSkinCatalog(h.DB)
	if err != nil {
		log.Printf("skins: get catalog: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load skins")
		return
	}
	c.JSON(http.StatusOK, gin.H{"skins": catalog})
}

// UserSkins returns the player's owned and currently equipped cosmetics. It is
// public so public profile pages can render a player's selected appearance.
func (h SkinHandler) UserSkins(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	equipped, err := repository.GetEquippedSkins(h.DB, userID)
	if err != nil {
		log.Printf("skins: get public user skins: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load skins")
		return
	}
	c.JSON(http.StatusOK, gin.H{"equipped": equipped})
}

func (h SkinHandler) MySkins(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	h.respondUserSkins(c, userID)
}

func (h SkinHandler) respondUserSkins(c *gin.Context, userID uuid.UUID) {
	owned, equipped, err := repository.GetUserSkins(h.DB, userID)
	if err != nil {
		log.Printf("skins: get user skins: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load skins")
		return
	}
	c.JSON(http.StatusOK, gin.H{"owned": owned, "equipped": equipped})
}

func (h SkinHandler) Equip(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	var req equipSkinRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SkinID) == "" {
		JSONError(c, http.StatusBadRequest, "skin_id is required")
		return
	}
	if err := repository.EquipSkin(h.DB, userID, c.Param("type"), strings.TrimSpace(req.SkinID)); err != nil {
		switch {
		case errors.Is(err, repository.ErrSkinType):
			JSONError(c, http.StatusBadRequest, "Invalid skin type")
		case errors.Is(err, repository.ErrSkinNotOwned):
			JSONError(c, http.StatusForbidden, "Skin is not owned by this user")
		default:
			log.Printf("skins: equip: %v", err)
			JSONError(c, http.StatusInternalServerError, "Failed to equip skin")
		}
		return
	}
	h.respondUserSkins(c, userID)
}

func (h SkinHandler) Unequip(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "Invalid user identity")
		return
	}
	if err := repository.UnequipSkin(h.DB, userID, c.Param("type")); err != nil {
		if errors.Is(err, repository.ErrSkinType) {
			JSONError(c, http.StatusBadRequest, "Invalid skin type")
			return
		}
		log.Printf("skins: unequip: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to unequip skin")
		return
	}
	h.respondUserSkins(c, userID)
}
