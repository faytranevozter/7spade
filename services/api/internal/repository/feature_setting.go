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
	SettingNewGameStarts    = "new_game_starts"
	SettingSpectatorAccess  = "spectator_access"
	SettingEmotes           = "emotes"
	SettingDailyLogin       = "daily_login"
	SettingDailyLoginXPBase = "daily_login_xp_base"
	SettingDailyLoginXPStep = "daily_login_xp_step"
	SettingDailyLoginXPMax  = "daily_login_xp_max"
)

func FeatureSettingEnabled(db *sql.DB, key string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("get feature setting %s: database unavailable", key)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT (value #>> '{}')::boolean FROM feature_settings WHERE key = $1 AND type = 'boolean'`, key).Scan(&enabled); err != nil {
		return false, fmt.Errorf("get feature setting %s: %w", key, err)
	}
	return enabled, nil
}
