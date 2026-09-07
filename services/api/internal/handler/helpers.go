package handler

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
)

func JSONError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func requireFeatureEnabled(c *gin.Context, db *sql.DB, lookup func(*sql.DB, string) (bool, error), key, message string) bool {
	if lookup == nil {
		lookup = repository.FeatureSettingEnabled
	}
	enabled, err := lookup(db, key)
	if err != nil {
		log.Printf("feature setting %s: %v", key, err)
		JSONError(c, http.StatusInternalServerError, "Failed to load application settings")
		return false
	}
	if !enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": message, "code": "feature_disabled"})
		return false
	}
	return true
}
