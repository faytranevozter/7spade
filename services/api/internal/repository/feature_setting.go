package repository

import (
	"database/sql"
	"fmt"
)

const (
	SettingNewRegistrations = "new_registrations"
	SettingGuestAccess      = "guest_access"
	SettingRoomCreation     = "room_creation"
	SettingQuickPlay        = "quick_play"
)

func FeatureSettingEnabled(db *sql.DB, key string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("get feature setting %s: database unavailable", key)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT enabled FROM feature_settings WHERE key = $1`, key).Scan(&enabled); err != nil {
		return false, fmt.Errorf("get feature setting %s: %w", key, err)
	}
	return enabled, nil
}
