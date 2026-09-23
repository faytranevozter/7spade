package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
)

const dailyLoginSettingKey = "daily_login"

var applicationSettingKeys = map[string]bool{
	"daily_login":         true,
	"daily_login_xp_base": true,
	"daily_login_xp_step": true,
	"daily_login_xp_max":  true,
	"new_registrations":   true,
	"guest_access":        true,
	"room_creation":       true,
	"quick_play":          true,
	"new_game_starts":     true,
	"spectator_access":    true,
	"emotes":              true,
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
		Value  json.RawMessage `json:"value"`
		Reason string          `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Value) == 0 || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "value and reason are required")
		return
	}
	current, err := h.store.GetFeatureSetting(c, key)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load application setting")
		return
	}
	value := json.RawMessage(bytes.TrimSpace(req.Value))
	if !validSettingValue(current.Type, value) {
		jsonError(c, http.StatusBadRequest, "Value does not match setting type")
		return
	}
	if strings.HasPrefix(key, "daily_login_xp_") {
		valid, err := h.validDailyLoginXP(c, key, value)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to validate daily login settings")
			return
		}
		if !valid {
			jsonError(c, http.StatusBadRequest, "Daily login XP values must be positive and max must be at least base")
			return
		}
	}
	admin := c.MustGet("admin").(Admin)
	audit := h.requestAudit(c, admin.ID, "setting."+key+".update", "feature_setting", key, "success")
	audit.Reason = strings.TrimSpace(req.Reason)
	setting, err := h.store.UpdateFeatureSetting(c, key, value, audit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update application setting")
		return
	}
	c.JSON(http.StatusOK, setting)
}

func validSettingValue(settingType string, value json.RawMessage) bool {
	switch settingType {
	case "boolean":
		var decoded bool
		return json.Unmarshal(value, &decoded) == nil
	case "integer":
		var decoded int
		return json.Unmarshal(value, &decoded) == nil
	case "float":
		var decoded float64
		return json.Unmarshal(value, &decoded) == nil
	case "string":
		var decoded string
		return json.Unmarshal(value, &decoded) == nil
	case "options":
		var decoded any
		if json.Unmarshal(value, &decoded) != nil {
			return false
		}
		switch decoded.(type) {
		case []any, map[string]any:
			return true
		}
	}
	return false
}

func (h *AdminHandler) validDailyLoginXP(c *gin.Context, changedKey string, changedValue json.RawMessage) (bool, error) {
	values := map[string]int{}
	for _, key := range []string{"daily_login_xp_base", "daily_login_xp_step", "daily_login_xp_max"} {
		setting, err := h.store.GetFeatureSetting(c, key)
		if err != nil {
			return false, err
		}
		value := setting.Value
		if key == changedKey {
			value = changedValue
		}
		var decoded int
		if err := json.Unmarshal(value, &decoded); err != nil {
			return false, err
		}
		values[key] = decoded
	}
	return values["daily_login_xp_base"] > 0 && values["daily_login_xp_step"] > 0 &&
		values["daily_login_xp_max"] >= values["daily_login_xp_base"], nil
}

func featureSettingState(setting model.FeatureSetting) json.RawMessage {
	value, _ := json.Marshal(struct {
		Key   string          `json:"key"`
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}{Key: setting.Key, Type: setting.Type, Value: setting.Value})
	return value
}
