package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const dailyLoginSettingKey = "daily_login"

var applicationSettingKeys = map[string]bool{
	"daily_login":       true,
	"new_registrations": true,
	"guest_access":      true,
	"room_creation":     true,
	"quick_play":        true,
}

func (h *AdminHandler) ListApplicationSettings(c *gin.Context) {
	settings, err := h.store.ListFeatureSettings(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load application settings")
		return
	}
	filtered := settings[:0]
	for _, setting := range settings {
		if applicationSettingKeys[setting.Key] {
			filtered = append(filtered, setting)
		}
	}
	c.JSON(http.StatusOK, filtered)
}

func (h *AdminHandler) UpdateApplicationSetting(c *gin.Context) {
	key := c.Param("key")
	if !applicationSettingKeys[key] {
		jsonError(c, http.StatusNotFound, "Application setting not found")
		return
	}
	h.updateApplicationSetting(c, key)
}

func (h *AdminHandler) GetDailyLoginSetting(c *gin.Context) {
	setting, err := h.store.GetFeatureSetting(c, dailyLoginSettingKey)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load daily login setting")
		return
	}
	c.JSON(http.StatusOK, setting)
}

func (h *AdminHandler) UpdateDailyLoginSetting(c *gin.Context) {
	h.updateApplicationSetting(c, dailyLoginSettingKey)
}

func (h *AdminHandler) updateApplicationSetting(c *gin.Context, key string) {
	var req struct {
		Enabled *bool  `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Enabled == nil || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "enabled and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	audit := h.requestAudit(c, admin.ID, "setting."+key+".update", "feature_setting", key, "success")
	audit.Reason = strings.TrimSpace(req.Reason)
	setting, err := h.store.UpdateFeatureSetting(c, key, *req.Enabled, audit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update application setting")
		return
	}
	c.JSON(http.StatusOK, setting)
}

func featureSettingState(enabled bool) json.RawMessage {
	value, _ := json.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled})
	return value
}
