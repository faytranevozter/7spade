package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
)

func (h *AdminHandler) CreateAchievement(c *gin.Context) {
	var req struct {
		ID           string                  `json:"id"`
		Name         string                  `json:"name"`
		Description  string                  `json:"description"`
		Icon         string                  `json:"icon"`
		DisplayOrder int                     `json:"display_order"`
		Enabled      bool                    `json:"enabled"`
		Rules        []model.AchievementRule `json:"rules"`
		Reason       string                  `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, http.StatusBadRequest, "id, name, description, icon, rules, and reason are required")
		return
	}
	normalizeAchievementRules(req.Rules)
	if !validAchievementID(req.ID) || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Description) == "" || strings.TrimSpace(req.Icon) == "" || strings.TrimSpace(req.Reason) == "" || req.DisplayOrder < 0 || !validAchievementRules(req.Rules) {
		jsonError(c, http.StatusBadRequest, "id, name, description, icon, rules, and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	achievement := model.Achievement{ID: strings.TrimSpace(req.ID), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Icon: strings.TrimSpace(req.Icon), DisplayOrder: req.DisplayOrder, Enabled: req.Enabled, Rules: req.Rules}
	event := h.requestAudit(c, admin.ID, "achievement.catalog.create", "achievement", achievement.ID, "success")
	event.Reason = strings.TrimSpace(req.Reason)
	created, err := h.store.CreateAchievement(c, achievement, event)
	if errors.Is(err, ErrConflict) {
		jsonError(c, http.StatusConflict, "Achievement ID already exists")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create achievement")
		return
	}
	c.JSON(http.StatusCreated, created)
}

func validAchievementID(id string) bool {
	if id = strings.TrimSpace(id); id == "" {
		return false
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_') {
			return false
		}
	}
	return true
}

func (h *AdminHandler) UpdateAchievement(c *gin.Context) {
	var req struct {
		Name         string                  `json:"name"`
		Description  string                  `json:"description"`
		Icon         string                  `json:"icon"`
		DisplayOrder int                     `json:"display_order"`
		Enabled      bool                    `json:"enabled"`
		Rules        []model.AchievementRule `json:"rules"`
		Reason       string                  `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, http.StatusBadRequest, "name, description, icon, rules, and reason are required")
		return
	}
	normalizeAchievementRules(req.Rules)
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Description) == "" || strings.TrimSpace(req.Icon) == "" || strings.TrimSpace(req.Reason) == "" || req.DisplayOrder < 0 || !validAchievementRules(req.Rules) {
		jsonError(c, http.StatusBadRequest, "name, description, icon, rules, and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	next := model.Achievement{Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Icon: strings.TrimSpace(req.Icon), DisplayOrder: req.DisplayOrder, Enabled: req.Enabled, Rules: req.Rules}
	event := h.requestAudit(c, admin.ID, "achievement.catalog.update", "achievement", c.Param("id"), "success")
	event.Reason = strings.TrimSpace(req.Reason)
	achievement, err := h.store.UpdateAchievement(c, c.Param("id"), next, event)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Achievement not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		jsonError(c, http.StatusConflict, "Rule configuration is immutable after grants")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update achievement")
		return
	}
	c.JSON(http.StatusOK, achievement)
}

func validAchievementRules(rules []model.AchievementRule) bool {
	if len(rules) == 0 {
		return false
	}
	booleanMetrics := map[string]bool{"is_winner": true, "all_zero_penalty": true, "ace_closed": true}
	integerMetrics := map[string]bool{"shared_win_count": true, "penalty": true, "games_played": true, "wins": true, "current_streak": true, "current_top2_streak": true, "first_place_count": true, "zero_penalty_games": true, "human_only_games": true, "game_duration_seconds": true}
	for _, rule := range rules {
		if booleanMetrics[rule.Metric] {
			if rule.Operator != "eq" {
				return false
			}
			if _, err := strconv.ParseBool(rule.Value); err != nil {
				return false
			}
			continue
		}
		if !integerMetrics[rule.Metric] {
			return false
		}
		switch rule.Operator {
		case "eq", "gte", "lte", "gt", "lt":
		default:
			return false
		}
		if _, err := strconv.Atoi(rule.Value); err != nil {
			return false
		}
	}
	return true
}

func normalizeAchievementRules(rules []model.AchievementRule) {
	for i := range rules {
		rules[i].Metric = strings.TrimSpace(rules[i].Metric)
		rules[i].Operator = strings.TrimSpace(rules[i].Operator)
		rules[i].Value = strings.TrimSpace(rules[i].Value)
	}
}

func (h *AdminHandler) ChangeAchievementEntitlement(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Reason         string `json:"reason"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
			jsonError(c, http.StatusBadRequest, "reason and idempotency_key are required")
			return
		}
		admin := c.MustGet("admin").(Admin)
		event := h.requestAudit(c, admin.ID, "achievement.entitlement."+action, "user_achievement_entitlement", c.Param("id")+":"+c.Param("achievementID"), "success")
		event.Reason = strings.TrimSpace(req.Reason)
		result, replayed, err := h.store.ChangeAchievementEntitlement(c, c.Param("id"), c.Param("achievementID"), action, event.Reason, strings.TrimSpace(req.IdempotencyKey), event)
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "User or achievement not found")
			return
		}
		if errors.Is(err, ErrConflict) {
			jsonError(c, http.StatusConflict, "Entitlement state conflict")
			return
		}
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to change entitlement")
			return
		}
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		c.JSON(status, result)
	}
}
