package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrEventNotActive = errors.New("event is not active")

type Event struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Summary      string    `json:"summary"`
	Description  string    `json:"description"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	Timezone     string    `json:"timezone"`
	HeroAssetKey *string   `json:"hero_asset_key,omitempty"`
	AccentColor  *string   `json:"accent_color,omitempty"`
	Status       string    `json:"status"`
	ServerTime   time.Time `json:"server_time"`
}

type EventCheckIn struct {
	Authenticated bool       `json:"authenticated"`
	Count         int        `json:"count"`
	ClaimedToday  bool       `json:"claimed_today"`
	NextClaimAt   *time.Time `json:"next_claim_at,omitempty"`
}

type EventSkinReward struct {
	Skin        Skin   `json:"skin"`
	Requirement string `json:"requirement"`
	Target      int    `json:"target,omitempty"`
	Progress    int    `json:"progress"`
	Completed   bool   `json:"completed"`
	Owned       bool   `json:"owned"`
}

type EventDetail struct {
	Event       Event             `json:"event"`
	CheckIn     EventCheckIn      `json:"check_in"`
	SkinRewards []EventSkinReward `json:"skin_rewards"`
}

type EventClaimResult struct {
	NewlyClaimed bool         `json:"newly_claimed"`
	CheckIn      EventCheckIn `json:"check_in"`
	SkinGrants   []SkinGrant  `json:"skin_grants"`
}

func GetEventDetail(db *sql.DB, slug string, userID *uuid.UUID, now time.Time) (*EventDetail, error) {
	var event Event
	err := db.QueryRow(`
		SELECT id, slug, name, summary, description, starts_at, ends_at, timezone,
		       hero_asset_key, accent_color
		FROM events WHERE slug = $1 AND enabled = TRUE
	`, slug).Scan(&event.ID, &event.Slug, &event.Name, &event.Summary, &event.Description,
		&event.StartsAt, &event.EndsAt, &event.Timezone, &event.HeroAssetKey, &event.AccentColor)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	event.ServerTime = now.UTC()
	event.Status = eventStatus(now, event.StartsAt, event.EndsAt)

	detail := &EventDetail{Event: event, SkinRewards: []EventSkinReward{}}
	detail.CheckIn.Authenticated = userID != nil
	if userID != nil {
		progress, err := getEventCheckIn(db, event.ID, *userID, event.Timezone, now)
		if err != nil {
			return nil, err
		}
		detail.CheckIn = progress
	}
	rewards, err := getEventSkinRewards(db, event.ID, userID, detail.CheckIn.Count, event.Status)
	if err != nil {
		return nil, err
	}
	detail.SkinRewards = rewards
	return detail, nil
}

func ClaimEventCheckIn(db *sql.DB, slug string, userID uuid.UUID, now time.Time) (EventClaimResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return EventClaimResult{}, fmt.Errorf("begin event check-in: %w", err)
	}
	defer tx.Rollback()
	var eventID, timezone string
	var startsAt, endsAt time.Time
	if err := tx.QueryRow(`SELECT id, timezone, starts_at, ends_at FROM events WHERE slug = $1 AND enabled = TRUE`, slug).Scan(&eventID, &timezone, &startsAt, &endsAt); err != nil {
		return EventClaimResult{}, err
	}
	if eventStatus(now, startsAt, endsAt) != "active" {
		return EventClaimResult{}, ErrEventNotActive
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return EventClaimResult{}, fmt.Errorf("load event timezone: %w", err)
	}
	day := now.In(location).Format("2006-01-02")
	result, err := tx.Exec(`INSERT INTO event_check_ins (event_id, user_id, event_day) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, eventID, userID, day)
	if err != nil {
		return EventClaimResult{}, fmt.Errorf("claim event check-in: %w", err)
	}
	affected, _ := result.RowsAffected()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM event_check_ins WHERE event_id = $1 AND user_id = $2`, eventID, userID).Scan(&count); err != nil {
		return EventClaimResult{}, err
	}
	grants, err := grantEventCheckInSkins(tx, eventID, userID, count)
	if err != nil {
		return EventClaimResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return EventClaimResult{}, err
	}
	return EventClaimResult{NewlyClaimed: affected == 1, CheckIn: EventCheckIn{Authenticated: true, Count: count, ClaimedToday: true}, SkinGrants: grants}, nil
}

func getEventCheckIn(db *sql.DB, eventID string, userID uuid.UUID, timezone string, now time.Time) (EventCheckIn, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return EventCheckIn{}, err
	}
	day := now.In(location).Format("2006-01-02")
	var progress EventCheckIn
	progress.Authenticated = true
	if err := db.QueryRow(`SELECT COUNT(*), EXISTS(SELECT 1 FROM event_check_ins WHERE event_id = $1 AND user_id = $2 AND event_day = $3) FROM event_check_ins WHERE event_id = $1 AND user_id = $2`, eventID, userID, day).Scan(&progress.Count, &progress.ClaimedToday); err != nil {
		return progress, err
	}
	return progress, nil
}

func getEventSkinRewards(db *sql.DB, eventID string, userID *uuid.UUID, progress int, status string) ([]EventSkinReward, error) {
	user := uuid.Nil
	if userID != nil {
		user = *userID
	}
	rows, err := db.Query(`
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
		       r.event_check_in_count, EXISTS(SELECT 1 FROM user_skins us WHERE us.user_id = $2 AND us.skin_id = s.id)
		FROM skin_unlock_rules r JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
		WHERE r.event_id = $1 AND r.enabled = TRUE AND r.rule_type = 'event_check_in_count'
		  AND ($3 = 'active' OR EXISTS(SELECT 1 FROM user_skins us WHERE us.user_id = $2 AND us.skin_id = s.id))
		ORDER BY r.event_check_in_count, s.display_order
	`, eventID, user, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventSkinReward{}
	for rows.Next() {
		var item EventSkinReward
		if err := rows.Scan(&item.Skin.ID, &item.Skin.SkinType, &item.Skin.Name, &item.Skin.Description, &item.Skin.AssetKey, &item.Skin.DisplayOrder, &item.Target, &item.Owned); err != nil {
			return nil, err
		}
		item.Progress = progress
		item.Completed = item.Owned || progress >= item.Target
		item.Requirement = fmt.Sprintf("Check in on %d event days", item.Target)
		items = append(items, item)
	}
	return items, rows.Err()
}

func grantEventCheckInSkins(tx *sql.Tx, eventID string, userID uuid.UUID, count int) ([]SkinGrant, error) {
	rows, err := tx.Query(`WITH inserted AS (
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id)
		SELECT $2, r.skin_id, r.id FROM skin_unlock_rules r JOIN skins s ON s.id = r.skin_id
		WHERE r.event_id = $1 AND r.rule_type = 'event_check_in_count' AND r.event_check_in_count <= $3 AND r.enabled AND s.enabled
		ON CONFLICT DO NOTHING RETURNING skin_id, skin_unlock_rule_id)
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, 'event:' || e.slug
		FROM inserted i JOIN skins s ON s.id = i.skin_id JOIN skin_unlock_rules r ON r.id = i.skin_unlock_rule_id JOIN events e ON e.id = r.event_id`, eventID, userID, count)
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

func eventStatus(now, startsAt, endsAt time.Time) string {
	if now.Before(startsAt) {
		return "upcoming"
	}
	if !now.Before(endsAt) {
		return "ended"
	}
	return "active"
}
