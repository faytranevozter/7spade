package model

import (
	"encoding/json"
	"time"
)

const (
	EventDraft     = "draft"
	EventScheduled = "scheduled"
	EventPublished = "published"
	EventArchived  = "archived"
)

type Event struct {
	ID           string          `json:"id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Summary      string          `json:"summary"`
	Description  string          `json:"description"`
	StartsAt     time.Time       `json:"starts_at"`
	EndsAt       time.Time       `json:"ends_at"`
	HeroAssetKey *string         `json:"hero_asset_key,omitempty"`
	AccentColor  *string         `json:"accent_color,omitempty"`
	RewardConfig json.RawMessage `json:"reward_config"`
	State        string          `json:"state"`
	Revision     int             `json:"revision"`
	Version      int             `json:"version"`
	PublishedAt  *time.Time      `json:"published_at,omitempty"`
	ArchivedAt   *time.Time      `json:"archived_at,omitempty"`
}
