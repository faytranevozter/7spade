package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
)

type eventRequest struct {
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Summary      string          `json:"summary"`
	Description  string          `json:"description"`
	StartsAt     time.Time       `json:"starts_at"`
	EndsAt       time.Time       `json:"ends_at"`
	HeroAssetKey *string         `json:"hero_asset_key"`
	AccentColor  *string         `json:"accent_color"`
	RewardConfig json.RawMessage `json:"reward_config"`
	Version      int             `json:"version"`
	Reason       string          `json:"reason"`
}

func (r eventRequest) valid(requireVersion bool) bool {
	return strings.TrimSpace(r.Slug) != "" && strings.TrimSpace(r.Name) != "" && strings.TrimSpace(r.Summary) != "" && strings.TrimSpace(r.Description) != "" && strings.TrimSpace(r.Reason) != "" && r.StartsAt.Before(r.EndsAt) && len(r.RewardConfig) > 0 && (!requireVersion || r.Version > 0)
}
func (r eventRequest) event() model.Event {
	return model.Event{Slug: strings.TrimSpace(r.Slug), Name: strings.TrimSpace(r.Name), Summary: strings.TrimSpace(r.Summary), Description: strings.TrimSpace(r.Description), StartsAt: r.StartsAt, EndsAt: r.EndsAt, HeroAssetKey: r.HeroAssetKey, AccentColor: r.AccentColor, RewardConfig: r.RewardConfig}
}

func (h *AdminHandler) ListEvents(c *gin.Context) {
	events, err := h.store.ListEvents(c)
	if err != nil {
		jsonError(c, 500, "Failed to load events")
		return
	}
	c.JSON(200, gin.H{"events": events})
}
func (h *AdminHandler) GetEvent(c *gin.Context) {
	event, err := h.store.GetEvent(c, c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		jsonError(c, 404, "Event not found")
		return
	}
	if err != nil {
		jsonError(c, 500, "Failed to load event")
		return
	}
	c.JSON(200, event)
}
func (h *AdminHandler) CreateEvent(c *gin.Context) {
	var req eventRequest
	if c.ShouldBindJSON(&req) != nil || !req.valid(false) {
		jsonError(c, 400, "valid event properties and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	event := req.event()
	audit := h.requestAudit(c, admin.ID, "event.create", "event", "", "success")
	audit.Reason = strings.TrimSpace(req.Reason)
	created, err := h.store.CreateEvent(c, event, audit)
	if errors.Is(err, ErrConflict) {
		jsonError(c, 409, "Event slug already exists")
		return
	}
	if err != nil {
		jsonError(c, 500, "Failed to create event")
		return
	}
	c.JSON(201, created)
}
func (h *AdminHandler) UpdateEvent(c *gin.Context) {
	var req eventRequest
	if c.ShouldBindJSON(&req) != nil || !req.valid(true) {
		jsonError(c, 400, "valid event properties, version, and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	audit := h.requestAudit(c, admin.ID, "event.update", "event", c.Param("id"), "success")
	audit.Reason = strings.TrimSpace(req.Reason)
	event, err := h.store.UpdateEvent(c, c.Param("id"), req.Version, req.event(), audit)
	eventResponse(c, event, err)
}
func (h *AdminHandler) ScheduleEvent(c *gin.Context) { h.transitionEvent(c, model.EventScheduled) }
func (h *AdminHandler) PublishEvent(c *gin.Context)  { h.transitionEvent(c, model.EventPublished) }
func (h *AdminHandler) ArchiveEvent(c *gin.Context)  { h.transitionEvent(c, model.EventArchived) }
func (h *AdminHandler) transitionEvent(c *gin.Context, state string) {
	var req struct {
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Version < 1 || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, 400, "version and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	audit := h.requestAudit(c, admin.ID, "event."+strings.TrimSuffix(state, "d"), "event", c.Param("id"), "success")
	if state == model.EventPublished {
		audit.Action = "event.publish"
	}
	if state == model.EventArchived {
		audit.Action = "event.archive"
	}
	audit.Reason = strings.TrimSpace(req.Reason)
	event, err := h.store.TransitionEvent(c, c.Param("id"), req.Version, state, audit)
	eventResponse(c, event, err)
}
func eventResponse(c *gin.Context, event model.Event, err error) {
	if errors.Is(err, ErrNotFound) {
		jsonError(c, 404, "Event not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		jsonError(c, 409, "Event version or lifecycle conflict")
		return
	}
	if err != nil {
		jsonError(c, 500, "Failed to mutate event")
		return
	}
	c.JSON(http.StatusOK, event)
}
