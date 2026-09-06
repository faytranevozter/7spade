package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const dailyLoginSettingKey = "daily_login"

func (h *AdminHandler) GetDailyLoginSetting(c *gin.Context) {
	setting, err := h.store.GetFeatureSetting(c, dailyLoginSettingKey)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load daily login setting")
		return
	}
	c.JSON(http.StatusOK, setting)
}

func (h *AdminHandler) UpdateDailyLoginSetting(c *gin.Context) {
	var req struct {
		Enabled *bool  `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Enabled == nil || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "enabled and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	audit := h.requestAudit(c, admin.ID, "setting.daily_login.update", "feature_setting", dailyLoginSettingKey, "success")
	audit.Reason = strings.TrimSpace(req.Reason)
	setting, err := h.store.UpdateFeatureSetting(c, dailyLoginSettingKey, *req.Enabled, audit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update daily login setting")
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
