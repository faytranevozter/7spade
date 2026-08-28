package model

import (
	"encoding/json"
	"time"
)

type SkinUnlockRule struct {
	ID              string          `json:"id,omitempty"`
	Name            string          `json:"name"`
	RuleType        string          `json:"rule_type"`
	AchievementID   string          `json:"achievement_id,omitempty"`
	MinimumLevel    *int            `json:"minimum_level,omitempty"`
	LoginStreakDays *int            `json:"login_streak_days,omitempty"`
	Retroactive     bool            `json:"retroactive"`
	Enabled         bool            `json:"enabled"`
	Conditions      json.RawMessage `json:"conditions,omitempty"`
}

type SkinRevision struct {
	ID          string     `json:"id"`
	SkinID      string     `json:"skin_id"`
	Version     int        `json:"version"`
	AssetKey    string     `json:"asset_key"`
	ContentType string     `json:"content_type"`
	Enabled     bool       `json:"enabled"`
	PublishedAt time.Time  `json:"published_at"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
}

type Skin struct {
	ID                string           `json:"id"`
	SkinType          string           `json:"skin_type"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	AssetKey          string           `json:"asset_key"`
	IsStarter         bool             `json:"is_starter"`
	DisplayOrder      int              `json:"display_order"`
	Enabled           bool             `json:"enabled"`
	CatalogVisible    bool             `json:"catalog_visible"`
	UnlockRulesLocked bool             `json:"unlock_rules_locked"`
	UnlockRules       []SkinUnlockRule `json:"unlock_rules"`
	Revisions         []SkinRevision   `json:"revisions"`
}

type SkinEntitlementEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	SkinID     string    `json:"skin_id"`
	RevisionID string    `json:"revision_id"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	AdminID    string    `json:"admin_id"`
	OccurredAt time.Time `json:"occurred_at"`
}
