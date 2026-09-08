package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrEventNotActive = errors.New("event is not active")
var ErrEventDailyLoginDisabled = errors.New("event daily login is disabled")

type EventDailyLogin struct {
	Enabled    bool `json:"enabled"`
	XPPerClaim int  `json:"xp_per_claim"`
}

type Event struct {
	ID           string          `json:"id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Summary      string          `json:"summary"`
	Description  string          `json:"description"`
	StartsAt     time.Time       `json:"starts_at"`
	EndsAt       time.Time       `json:"ends_at"`
	AppTimezone  string          `json:"app_timezone"`
	HeroAssetKey *string         `json:"hero_asset_key,omitempty"`
	AccentColor  *string         `json:"accent_color,omitempty"`
	Status       string          `json:"status"`
	ServerTime   time.Time       `json:"server_time"`
	DailyLogin   EventDailyLogin `json:"daily_login"`
}

type EventSummary struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Summary      string    `json:"summary"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	AppTimezone  string    `json:"app_timezone"`
	HeroAssetKey *string   `json:"hero_asset_key,omitempty"`
	AccentColor  *string   `json:"accent_color,omitempty"`
	Status       string    `json:"status"`
	ServerTime   time.Time `json:"server_time"`
	RewardCount  int       `json:"reward_count"`
}

func ListEvents(db *sql.DB, now time.Time, location *time.Location) ([]EventSummary, error) {
	rows, err := db.Query(`
		SELECT e.id, e.slug, e.name, e.summary, e.starts_at, e.ends_at,
		       e.hero_asset_key, e.accent_color, COUNT(DISTINCT s.id)
		FROM events e
		LEFT JOIN skin_unlock_rules r ON r.event_id = e.id AND r.enabled = TRUE
		LEFT JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
		WHERE e.enabled = TRUE
		GROUP BY e.id, e.slug, e.name, e.summary, e.starts_at, e.ends_at,
		         e.hero_asset_key, e.accent_color
		ORDER BY CASE
		           WHEN $1 >= e.starts_at AND $1 < e.ends_at THEN 0
		           WHEN $1 < e.starts_at THEN 1
		           ELSE 2
		         END,
		         CASE WHEN $1 < e.starts_at THEN e.starts_at END ASC,
		         CASE WHEN $1 >= e.ends_at THEN e.ends_at END DESC,
		         e.starts_at ASC, e.id ASC
	`, now)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := []EventSummary{}
	for rows.Next() {
		var event EventSummary
		if err := rows.Scan(
			&event.ID, &event.Slug, &event.Name, &event.Summary, &event.StartsAt, &event.EndsAt,
			&event.HeroAssetKey, &event.AccentColor, &event.RewardCount,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.AppTimezone = appTimezoneLabel(location, now)
		event.Status = eventStatus(now, event.StartsAt, event.EndsAt)
		event.ServerTime = now.UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

type EventCheckIn struct {
	Authenticated bool       `json:"authenticated"`
	Count         int        `json:"count"`
	ClaimedToday  bool       `json:"claimed_today"`
	NextClaimAt   *time.Time `json:"next_claim_at,omitempty"`
}

type EventRewardRequirement struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Progress    *int   `json:"progress,omitempty"`
	Target      *int   `json:"target,omitempty"`
	Completed   bool   `json:"completed"`
}

type EventSkinReward struct {
	Skin        Skin                   `json:"skin"`
	Requirement EventRewardRequirement `json:"requirement"`
	Owned       bool                   `json:"owned"`
}

type EventDetail struct {
	Event       Event             `json:"event"`
	CheckIn     EventCheckIn      `json:"check_in"`
	SkinRewards []EventSkinReward `json:"skin_rewards"`
}

type EventClaimResult struct {
	NewlyClaimed bool         `json:"newly_claimed"`
	CheckIn      EventCheckIn `json:"check_in"`
	XPDelta      int          `json:"xp_delta"`
	XPAfter      int64        `json:"xp_after"`
	Level        int          `json:"level"`
	SkinGrants   []SkinGrant  `json:"skin_grants"`
}

func GetEventDetail(db *sql.DB, slug string, userID *uuid.UUID, now time.Time, location *time.Location) (*EventDetail, error) {
	var event Event
	err := db.QueryRow(`
		SELECT id, slug, name, summary, description, starts_at, ends_at,
		       hero_asset_key, accent_color,
		       COALESCE((reward_config->'daily_login'->>'enabled')::boolean, TRUE),
		       COALESCE((reward_config->'daily_login'->>'xp_per_claim')::integer, 100)
		FROM events WHERE slug = $1 AND enabled = TRUE
	`, slug).Scan(&event.ID, &event.Slug, &event.Name, &event.Summary, &event.Description,
		&event.StartsAt, &event.EndsAt, &event.HeroAssetKey, &event.AccentColor,
		&event.DailyLogin.Enabled, &event.DailyLogin.XPPerClaim)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	event.ServerTime = now.UTC()
	event.AppTimezone = appTimezoneLabel(location, now)
	event.Status = eventStatus(now, event.StartsAt, event.EndsAt)

	detail := &EventDetail{Event: event, SkinRewards: []EventSkinReward{}}
	detail.CheckIn.Authenticated = userID != nil
	if userID != nil {
		progress, err := getEventCheckIn(db, event.ID, *userID, now, location)
		if err != nil {
			return nil, err
		}
		detail.CheckIn = progress
	}
	rewards, err := getEventSkinRewards(db, event.ID, userID, detail.CheckIn.Count, event.Status)
	if err != nil {
		return nil, err
	}
	for i := range rewards {
		for j := range rewards[i].Skin.UnlockRules {
			rewards[i].Skin.UnlockRules[j].Event = &SkinUnlockRuleEvent{
				Slug: event.Slug, Name: event.Name, StartsAt: event.StartsAt, EndsAt: event.EndsAt,
			}
		}
	}
	detail.SkinRewards = rewards
	return detail, nil
}

func ClaimEventCheckIn(db *sql.DB, slug string, userID uuid.UUID, now time.Time, location *time.Location) (EventClaimResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return EventClaimResult{}, fmt.Errorf("begin event check-in: %w", err)
	}
	defer tx.Rollback()
	var eventID string
	var eventRevision int
	var startsAt, endsAt time.Time
	var dailyLogin EventDailyLogin
	if err := tx.QueryRow(`SELECT id, revision, starts_at, ends_at,
		COALESCE((reward_config->'daily_login'->>'enabled')::boolean, TRUE),
		COALESCE((reward_config->'daily_login'->>'xp_per_claim')::integer, 100)
		FROM events WHERE slug = $1 AND enabled = TRUE FOR SHARE`, slug).Scan(
		&eventID, &eventRevision, &startsAt, &endsAt, &dailyLogin.Enabled, &dailyLogin.XPPerClaim,
	); err != nil {
		return EventClaimResult{}, err
	}
	if eventStatus(now, startsAt, endsAt) != "active" {
		return EventClaimResult{}, ErrEventNotActive
	}
	if !dailyLogin.Enabled {
		return EventClaimResult{}, ErrEventDailyLoginDisabled
	}
	day := eventDay(now, location)
	result, err := tx.Exec(`INSERT INTO event_check_ins (event_id, event_revision, user_id, event_day) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, eventID, eventRevision, userID, day)
	if err != nil {
		return EventClaimResult{}, fmt.Errorf("claim event check-in: %w", err)
	}
	affected, _ := result.RowsAffected()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM event_check_ins WHERE event_id = $1 AND user_id = $2`, eventID, userID).Scan(&count); err != nil {
		return EventClaimResult{}, err
	}
	claimResult := EventClaimResult{NewlyClaimed: affected == 1, CheckIn: EventCheckIn{Authenticated: true, Count: count, ClaimedToday: true}, SkinGrants: []SkinGrant{}}
	if _, err := tx.Exec(`INSERT INTO user_stats (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return EventClaimResult{}, fmt.Errorf("create event check-in xp stats: %w", err)
	}
	if claimResult.NewlyClaimed {
		claimResult.XPDelta = dailyLogin.XPPerClaim
		if err := tx.QueryRow(`UPDATE user_stats SET xp = xp + $1, updated_at = NOW() WHERE user_id = $2 RETURNING xp`, claimResult.XPDelta, userID).Scan(&claimResult.XPAfter); err != nil {
			return EventClaimResult{}, fmt.Errorf("award event check-in xp: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO event_check_in_xp_events (event_id, event_revision, user_id, claim_date, xp_before, xp_after, xp_delta)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, eventID, eventRevision, userID, day, claimResult.XPAfter-int64(claimResult.XPDelta), claimResult.XPAfter, claimResult.XPDelta); err != nil {
			return EventClaimResult{}, fmt.Errorf("record event check-in xp: %w", err)
		}
	} else if err := tx.QueryRow(`SELECT xp FROM user_stats WHERE user_id = $1`, userID).Scan(&claimResult.XPAfter); err != nil {
		return EventClaimResult{}, fmt.Errorf("read event check-in xp: %w", err)
	}
	claimResult.Level = LevelFromXP(claimResult.XPAfter)
	grants, err := grantEventCheckInSkins(tx, eventID, eventRevision, userID, count)
	if err != nil {
		return EventClaimResult{}, err
	}
	levelGrants, err := GrantMinimumLevelSkins(tx, userID, claimResult.Level, now)
	if err != nil {
		return EventClaimResult{}, err
	}
	claimResult.SkinGrants = append(grants, levelGrants...)
	if err := tx.Commit(); err != nil {
		return EventClaimResult{}, err
	}
	return claimResult, nil
}

func getEventCheckIn(db *sql.DB, eventID string, userID uuid.UUID, now time.Time, location *time.Location) (EventCheckIn, error) {
	day := eventDay(now, location)
	var progress EventCheckIn
	progress.Authenticated = true
	if err := db.QueryRow(`SELECT COUNT(*), EXISTS(SELECT 1 FROM event_check_ins WHERE event_id = $1 AND user_id = $2 AND event_day = $3) FROM event_check_ins WHERE event_id = $1 AND user_id = $2`, eventID, userID, day).Scan(&progress.Count, &progress.ClaimedToday); err != nil {
		return progress, err
	}
	return progress, nil
}

func getEventSkinRewards(db *sql.DB, eventID string, userID *uuid.UUID, checkInCount int, status string) ([]EventSkinReward, error) {
	user := uuid.Nil
	authenticated := userID != nil
	if authenticated {
		user = *userID
	}
	rows, err := db.Query(`
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
		       r.rule_type, r.name, r.minimum_level, r.login_streak_days, r.event_check_in_count,
		       r.achievement_id, a.name,
		       COALESCE((SELECT jsonb_agg(jsonb_build_object('metric', c.metric, 'operator', c.operator, 'value', c.value) ORDER BY c.created_at, c.id)
		                 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id = r.id), '[]'::jsonb),
		       EXISTS(SELECT 1 FROM user_skins us WHERE us.user_id = $2 AND us.skin_id = s.id),
		       EXISTS(SELECT 1 FROM user_achievements ua WHERE ua.user_id = $2 AND ua.achievement_id = r.achievement_id),
		       COALESCE((SELECT xp FROM user_stats us WHERE us.user_id = $2), 0),
		       COALESCE((SELECT current_streak FROM user_login_progress ulp WHERE ulp.user_id = $2), 0)
		FROM skin_unlock_rules r
		JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
		LEFT JOIN achievements a ON a.id = r.achievement_id
		WHERE r.event_id = $1 AND r.enabled = TRUE
		  AND ($3 = 'active' OR EXISTS(SELECT 1 FROM user_skins us WHERE us.user_id = $2 AND us.skin_id = s.id))
		ORDER BY s.display_order, s.id, r.name, r.id
	`, eventID, user, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventSkinReward{}
	for rows.Next() {
		var (
			item                                     EventSkinReward
			ruleName                                 string
			minimumLevel, loginStreak, eventCheckIns sql.NullInt64
			achievementID, achievementName           sql.NullString
			conditions                               []byte
			achievementEarned                        bool
			xp                                       int64
			currentLoginStreak                       int
		)
		if err := rows.Scan(
			&item.Skin.ID, &item.Skin.SkinType, &item.Skin.Name, &item.Skin.Description,
			&item.Skin.AssetKey, &item.Skin.DisplayOrder, &item.Requirement.Type, &ruleName,
			&minimumLevel, &loginStreak, &eventCheckIns, &achievementID, &achievementName,
			&conditions, &item.Owned, &achievementEarned, &xp, &currentLoginStreak,
		); err != nil {
			return nil, err
		}
		item.Skin.UnlockRules = []SkinUnlockRule{{RuleType: item.Requirement.Type, Name: ruleName}}
		rule := &item.Skin.UnlockRules[0]
		if minimumLevel.Valid {
			value := int(minimumLevel.Int64)
			rule.MinimumLevel = &value
		}
		if loginStreak.Valid {
			value := int(loginStreak.Int64)
			rule.LoginStreakDays = &value
		}
		if eventCheckIns.Valid {
			value := int(eventCheckIns.Int64)
			rule.EventCheckInCount = &value
		}
		if achievementID.Valid && achievementName.Valid {
			rule.Achievement = &SkinAchievement{ID: achievementID.String, Name: achievementName.String}
		}
		if rule.RuleType == "game_condition" {
			if err := json.Unmarshal(conditions, &rule.Conditions); err != nil {
				return nil, fmt.Errorf("decode event skin conditions: %w", err)
			}
		}
		item.Requirement = eventRewardRequirement(
			item.Requirement.Type, ruleName, minimumLevel, loginStreak, eventCheckIns,
			achievementID, achievementName, authenticated, item.Owned, achievementEarned,
			LevelFromXP(xp), currentLoginStreak, checkInCount,
		)
		items = append(items, item)
	}
	return items, rows.Err()
}

func grantEventCheckInSkins(tx *sql.Tx, eventID string, eventRevision int, userID uuid.UUID, count int) ([]SkinGrant, error) {
	rows, err := tx.Query(`WITH inserted AS (
			INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
			SELECT $3, r.skin_id, r.id, $1, $2, sr.id FROM skin_unlock_rules r JOIN skins s ON s.id = r.skin_id
			JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
			WHERE r.event_id = $1 AND r.rule_type = 'event_check_in_count' AND r.event_check_in_count <= $4 AND r.enabled AND s.enabled
		ON CONFLICT DO NOTHING RETURNING skin_id, skin_unlock_rule_id)
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, 'event:' || e.slug
		FROM inserted i JOIN skins s ON s.id = i.skin_id JOIN skin_unlock_rules r ON r.id = i.skin_unlock_rule_id JOIN events e ON e.id = r.event_id`, eventID, eventRevision, userID, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []SkinGrant{}
	for rows.Next() {
		var g SkinGrant
		if err := rows.Scan(&g.ID, &g.SkinType, &g.Name, &g.Description, &g.AssetKey, &g.DisplayOrder, &g.Source); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func eventRewardRequirement(
	ruleType, ruleName string,
	minimumLevel, loginStreak, eventCheckIns sql.NullInt64,
	achievementID, achievementName sql.NullString,
	authenticated, owned, achievementEarned bool,
	level, currentLoginStreak, checkInCount int,
) EventRewardRequirement {
	requirement := EventRewardRequirement{Type: ruleType, Completed: owned}
	withProgress := func(progress, target int) {
		requirement.Progress = &progress
		requirement.Target = &target
		requirement.Completed = owned || progress >= target
	}
	switch ruleType {
	case "event_check_in_count":
		target := int(eventCheckIns.Int64)
		requirement.Description = eventCheckInRequirement(target)
		if authenticated {
			withProgress(checkInCount, target)
		}
	case "minimum_level":
		target := int(minimumLevel.Int64)
		requirement.Description = fmt.Sprintf("Reach player level %d during the event", target)
		if authenticated {
			withProgress(level, target)
		}
	case "login_streak":
		target := int(loginStreak.Int64)
		requirement.Description = fmt.Sprintf("Reach a %d-day login streak during the event", target)
		if authenticated {
			withProgress(currentLoginStreak, target)
		}
	case "achievement":
		name := achievementName.String
		if name == "" {
			name = achievementID.String
		}
		requirement.Description = fmt.Sprintf("Earn the %s achievement during the event", name)
		if authenticated {
			progress, target := 0, 1
			if achievementEarned {
				progress = 1
			}
			withProgress(progress, target)
		}
	case "game_condition":
		requirement.Description = fmt.Sprintf("Complete the %s challenge during the event", ruleName)
	}
	return requirement
}

func eventCheckInRequirement(target int) string {
	day := "days"
	if target == 1 {
		day = "day"
	}
	return fmt.Sprintf("Check in on %d event %s", target, day)
}

func eventDay(now time.Time, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	return now.In(location).Format("2006-01-02")
}

func eventStatus(now, startsAt, endsAt time.Time) string {
	if now.Before(startsAt) {
		return "upcoming"
	}
	if !now.Before(endsAt) {
		return "ended"
	}
	return "active"
}
