package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

const (
	SkinTypeProfileBackground    = "profile_background"
	SkinTypeAvatarFrame          = "avatar_frame"
	SkinTypeDisplayPicture       = "display_picture"
	SkinTypePlayerCardBackground = "player_card_background"
)

var (
	ErrSkinNotOwned = errors.New("skin is not owned by this user")
	ErrSkinType     = errors.New("invalid skin type")
)

type Skin struct {
	ID           string           `json:"id"`
	SkinType     string           `json:"skin_type"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	AssetKey     string           `json:"asset_key"`
	DisplayOrder int              `json:"display_order"`
	UnlockRules  []SkinUnlockRule `json:"unlock_rules"`
}

type SkinUnlockRule struct {
	RuleType          string                `json:"rule_type"`
	Name              string                `json:"name,omitempty"`
	Achievement       *SkinAchievement      `json:"achievement,omitempty"`
	MinimumLevel      *int                  `json:"minimum_level,omitempty"`
	LoginStreakDays   *int                  `json:"login_streak_days,omitempty"`
	EventCheckInCount *int                  `json:"event_check_in_count,omitempty"`
	Event             *SkinUnlockRuleEvent  `json:"event,omitempty"`
	Conditions        []SkinUnlockCondition `json:"conditions,omitempty"`
}

type SkinAchievement struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SkinUnlockRuleEvent struct {
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type SkinUnlockCondition struct {
	Metric   string `json:"metric"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type OwnedSkin struct {
	Skin
	Source   string `json:"source"`
	Equipped bool   `json:"equipped"`
}

type SkinGrant struct {
	Skin
	Source string `json:"source"`
}

type EquippedSkin struct {
	SkinType string `json:"skin_type"`
	SkinID   string `json:"skin_id"`
	AssetKey string `json:"asset_key"`
}

func IsSkinType(skinType string) bool {
	switch skinType {
	case SkinTypeProfileBackground, SkinTypeAvatarFrame, SkinTypeDisplayPicture, SkinTypePlayerCardBackground:
		return true
	default:
		return false
	}
}

func GrantAchievementSkins(tx *sql.Tx, userID uuid.UUID, achievementIDs []string) ([]SkinGrant, error) {
	grants := []SkinGrant{}
	for _, achievementID := range achievementIDs {
		rows, err := tx.Query(`
			WITH inserted AS (
				INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, skin_revision_id)
				SELECT $1, s.id, r.id, sr.id
				FROM skin_unlock_rules r
				JOIN skins s ON s.id = r.skin_id
				JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
				WHERE r.rule_type = 'achievement'
				  AND r.achievement_id = $2
				  AND r.enabled = TRUE
				  AND s.enabled = TRUE
				  AND (r.event_id IS NULL OR EXISTS (SELECT 1 FROM events e WHERE e.id = r.event_id AND e.enabled AND e.starts_at <= NOW() AND NOW() < e.ends_at))
				ON CONFLICT (user_id, skin_id) DO NOTHING
				RETURNING skin_id, skin_unlock_rule_id
			)
			SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
			       'achievement:' || r.achievement_id
			FROM inserted i
			JOIN skins s ON s.id = i.skin_id
			JOIN skin_unlock_rules r ON r.id = i.skin_unlock_rule_id
			ORDER BY s.display_order, s.id
		`, userID, achievementID)
		if err != nil {
			return nil, fmt.Errorf("grant skins for achievement %s: %w", achievementID, err)
		}
		for rows.Next() {
			var grant SkinGrant
			if err := rows.Scan(
				&grant.ID, &grant.SkinType, &grant.Name, &grant.Description,
				&grant.AssetKey, &grant.DisplayOrder, &grant.Source,
			); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan granted skin: %w", err)
			}
			grants = append(grants, grant)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate granted skins: %w", err)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("close granted skins: %w", err)
		}
	}
	return grants, nil
}

func GrantMinimumLevelSkins(tx *sql.Tx, userID uuid.UUID, level int) ([]SkinGrant, error) {
	rows, err := tx.Query(`
		WITH inserted AS (
			INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, skin_revision_id)
			SELECT $1, s.id, r.id, sr.id
			FROM skin_unlock_rules r
			JOIN skins s ON s.id = r.skin_id
			JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
			WHERE r.rule_type = 'minimum_level'
			  AND r.minimum_level <= $2
			  AND r.enabled = TRUE
			  AND s.enabled = TRUE
			  AND (r.event_id IS NULL OR EXISTS (SELECT 1 FROM events e WHERE e.id = r.event_id AND e.enabled AND e.starts_at <= NOW() AND NOW() < e.ends_at))
			ON CONFLICT (user_id, skin_id) DO NOTHING
			RETURNING skin_id, skin_unlock_rule_id
		)
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
		       'level:' || r.minimum_level
		FROM inserted i
		JOIN skins s ON s.id = i.skin_id
		JOIN skin_unlock_rules r ON r.id = i.skin_unlock_rule_id
		ORDER BY s.display_order, s.id
	`, userID, level)
	if err != nil {
		return nil, fmt.Errorf("grant skins for level %d: %w", level, err)
	}
	defer rows.Close()

	grants := []SkinGrant{}
	for rows.Next() {
		var grant SkinGrant
		if err := rows.Scan(
			&grant.ID, &grant.SkinType, &grant.Name, &grant.Description,
			&grant.AssetKey, &grant.DisplayOrder, &grant.Source,
		); err != nil {
			return nil, fmt.Errorf("scan level skin grant: %w", err)
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate level skin grants: %w", err)
	}
	return grants, nil
}

func GrantGameConditionSkins(tx *sql.Tx, userID uuid.UUID, ctx achievementContext, occurredAt time.Time) ([]SkinGrant, error) {
	rows, err := tx.Query(`
		SELECT r.id, r.name, r.skin_id, r.event_id, event_version.revision, c.metric, c.operator, c.value
		FROM skin_unlock_rules r
		JOIN skin_unlock_rule_conditions c ON c.skin_unlock_rule_id = r.id
		JOIN skins s ON s.id = r.skin_id
		LEFT JOIN event_versions event_version
		  ON event_version.event_id = r.event_id AND event_version.revision = r.event_revision
		WHERE r.rule_type = 'game_condition'
		  AND r.enabled = TRUE
		  AND s.enabled = TRUE
		  AND (r.event_id IS NULL OR (event_version.published_at <= $1 AND event_version.starts_at <= $1 AND $1 < event_version.ends_at))
		ORDER BY s.display_order, s.id, r.name, r.id, c.created_at, c.id
	`, occurredAt)
	if err != nil {
		return nil, fmt.Errorf("query game-condition skin rules: %w", err)
	}
	defer rows.Close()

	type gameConditionRule struct {
		id, name, skinID string
		eventRevision    sql.NullInt64
		conditions       []achievementRule
	}
	grouped := []gameConditionRule{}
	for rows.Next() {
		var ruleID, ruleName, skinID string
		var condition achievementRule
		var eventID sql.NullString
		var eventRevision sql.NullInt64
		if err := rows.Scan(&ruleID, &ruleName, &skinID, &eventID, &eventRevision, &condition.Metric, &condition.Operator, &condition.Value); err != nil {
			return nil, fmt.Errorf("scan game-condition skin rule: %w", err)
		}
		if len(grouped) == 0 || grouped[len(grouped)-1].id != ruleID {
			grouped = append(grouped, gameConditionRule{id: ruleID, name: ruleName, skinID: skinID, eventRevision: eventRevision})
		}
		grouped[len(grouped)-1].conditions = append(grouped[len(grouped)-1].conditions, condition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game-condition skin rules: %w", err)
	}

	matches := []gameConditionRule{}
	for _, rule := range grouped {
		matched := true
		for _, condition := range rule.conditions {
			ok, err := ruleMatches(ctx, condition)
			if err != nil {
				log.Printf("skins: ignoring invalid game-condition rule %s: %v", rule.name, err)
				matched = false
				break
			}
			if !ok {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, rule)
		}
	}

	grants := []SkinGrant{}
	for _, rule := range matches {
		var grant SkinGrant
		err := tx.QueryRow(`
			WITH inserted AS (
				INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
					SELECT $1, s.id, r.id, r.event_id, CASE WHEN r.event_id IS NULL THEN NULL ELSE $6::integer END, sr.id
					FROM skin_unlock_rules r
					JOIN skins s ON s.id = r.skin_id
					JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
					WHERE r.id = $3 AND s.id = $2 AND r.enabled = TRUE AND s.enabled = TRUE
					  AND (r.event_id IS NULL OR EXISTS (
						SELECT 1 FROM event_versions ev
						WHERE ev.event_id = r.event_id AND ev.revision = r.event_revision AND ev.revision = $6
						  AND ev.published_at <= $5 AND ev.starts_at <= $5 AND $5 < ev.ends_at
					  ))
				ON CONFLICT (user_id, skin_id) DO NOTHING
				RETURNING skin_id
			)
			SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, 'game_condition:' || $4
			FROM inserted i
			JOIN skins s ON s.id = i.skin_id
		`, userID, rule.skinID, rule.id, rule.name, occurredAt, rule.eventRevision).Scan(
			&grant.ID, &grant.SkinType, &grant.Name, &grant.Description,
			&grant.AssetKey, &grant.DisplayOrder, &grant.Source,
		)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("grant skin for game-condition rule %s: %w", rule.name, err)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func GetSkinCatalog(db *sql.DB) ([]Skin, error) {
	rows, err := db.Query(`
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
		       r.id, r.name, r.rule_type, r.achievement_id, a.name,
		       r.minimum_level, r.login_streak_days, r.event_check_in_count,
		       e.slug, e.name, e.starts_at, e.ends_at,
		       c.id, c.metric, c.operator, c.value
		FROM skins s
		LEFT JOIN skin_unlock_rules r ON r.skin_id = s.id AND r.enabled = TRUE
		  AND (r.event_id IS NULL OR EXISTS (SELECT 1 FROM events e WHERE e.id = r.event_id AND e.enabled AND e.starts_at <= NOW() AND NOW() < e.ends_at))
		LEFT JOIN achievements a ON a.id = r.achievement_id
		LEFT JOIN events e ON e.id = r.event_id
		LEFT JOIN skin_unlock_rule_conditions c ON c.skin_unlock_rule_id = r.id
		WHERE s.enabled = TRUE AND s.catalog_visible = TRUE
		  AND (
		    NOT EXISTS (SELECT 1 FROM skin_unlock_rules er WHERE er.skin_id = s.id AND er.enabled = TRUE)
		    OR EXISTS (SELECT 1 FROM skin_unlock_rules er WHERE er.skin_id = s.id AND er.enabled = TRUE AND er.event_id IS NULL)
		    OR EXISTS (SELECT 1 FROM skin_unlock_rules er JOIN events e ON e.id = er.event_id WHERE er.skin_id = s.id AND er.enabled AND e.enabled AND e.starts_at <= NOW() AND NOW() < e.ends_at)
		  )
		ORDER BY s.skin_type, s.display_order, s.id, r.name, r.id, c.created_at, c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query skin catalog: %w", err)
	}
	defer rows.Close()

	items := []Skin{}
	var currentRuleID string
	for rows.Next() {
		var item Skin
		var ruleID, ruleName, ruleType, achievementID, achievementName sql.NullString
		var minimumLevel, loginStreakDays, eventCheckInCount sql.NullInt64
		var eventSlug, eventName sql.NullString
		var eventStartsAt, eventEndsAt sql.NullTime
		var conditionID, conditionMetric, conditionOperator, conditionValue sql.NullString
		if err := rows.Scan(
			&item.ID, &item.SkinType, &item.Name, &item.Description, &item.AssetKey, &item.DisplayOrder,
			&ruleID, &ruleName, &ruleType, &achievementID, &achievementName,
			&minimumLevel, &loginStreakDays, &eventCheckInCount,
			&eventSlug, &eventName, &eventStartsAt, &eventEndsAt,
			&conditionID, &conditionMetric, &conditionOperator, &conditionValue,
		); err != nil {
			return nil, fmt.Errorf("scan skin catalog: %w", err)
		}
		if len(items) == 0 || items[len(items)-1].ID != item.ID {
			item.UnlockRules = []SkinUnlockRule{}
			items = append(items, item)
			currentRuleID = ""
		}
		if !ruleID.Valid {
			continue
		}
		if currentRuleID != ruleID.String {
			rule := SkinUnlockRule{RuleType: ruleType.String, Name: ruleName.String}
			if achievementID.Valid && achievementName.Valid {
				rule.Achievement = &SkinAchievement{ID: achievementID.String, Name: achievementName.String}
			}
			if minimumLevel.Valid {
				value := int(minimumLevel.Int64)
				rule.MinimumLevel = &value
			}
			if loginStreakDays.Valid {
				value := int(loginStreakDays.Int64)
				rule.LoginStreakDays = &value
			}
			if eventCheckInCount.Valid {
				value := int(eventCheckInCount.Int64)
				rule.EventCheckInCount = &value
			}
			if eventSlug.Valid && eventName.Valid && eventStartsAt.Valid && eventEndsAt.Valid {
				rule.Event = &SkinUnlockRuleEvent{Slug: eventSlug.String, Name: eventName.String, StartsAt: eventStartsAt.Time, EndsAt: eventEndsAt.Time}
			}
			if rule.RuleType == "game_condition" {
				rule.Conditions = []SkinUnlockCondition{}
			}
			items[len(items)-1].UnlockRules = append(items[len(items)-1].UnlockRules, rule)
			currentRuleID = ruleID.String
		}
		if conditionID.Valid {
			rules := items[len(items)-1].UnlockRules
			rule := &rules[len(rules)-1]
			rule.Conditions = append(rule.Conditions, SkinUnlockCondition{
				Metric: conditionMetric.String, Operator: conditionOperator.String, Value: conditionValue.String,
			})
			items[len(items)-1].UnlockRules = rules
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skin catalog: %w", err)
	}
	return items, nil
}

func GetUserSkins(db *sql.DB, userID uuid.UUID) ([]OwnedSkin, []EquippedSkin, error) {
	rows, err := db.Query(`
		SELECT s.id, s.skin_type, s.name, s.description, sr.asset_key, s.display_order,
		       COALESCE(
		           us.source,
			           CASE
			             WHEN r.event_id IS NOT NULL THEN 'event:' || e.slug
			             WHEN r.rule_type = 'achievement' THEN 'achievement:' || r.achievement_id
			             WHEN r.rule_type = 'minimum_level' THEN 'level:' || r.minimum_level
			             WHEN r.rule_type = 'login_streak' THEN 'login_streak:' || r.login_streak_days
			             WHEN r.rule_type = 'game_condition' THEN 'game_condition:' || r.name
			             WHEN r.rule_type = 'event_check_in_count' THEN 'event:' || e.slug
		           END,
		           'unknown'
		       ),
		       (ues.skin_id IS NOT NULL) AS equipped
		FROM user_skins us
		JOIN skins s ON s.id = us.skin_id AND s.enabled = TRUE
		JOIN skin_revisions sr ON sr.id = us.skin_revision_id AND sr.skin_id = us.skin_id AND sr.enabled = TRUE
		LEFT JOIN skin_unlock_rules r ON r.id = us.skin_unlock_rule_id
		LEFT JOIN events e ON e.id = r.event_id
		LEFT JOIN user_equipped_skins ues
		  ON ues.user_id = us.user_id AND ues.skin_id = us.skin_id AND ues.skin_type = s.skin_type
		WHERE us.user_id = $1
		ORDER BY s.skin_type, s.display_order, s.id
	`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("query user skins: %w", err)
	}
	defer rows.Close()

	owned := []OwnedSkin{}
	for rows.Next() {
		var item OwnedSkin
		if err := rows.Scan(&item.ID, &item.SkinType, &item.Name, &item.Description, &item.AssetKey, &item.DisplayOrder, &item.Source, &item.Equipped); err != nil {
			return nil, nil, fmt.Errorf("scan user skin: %w", err)
		}
		owned = append(owned, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate user skins: %w", err)
	}

	equipped, err := GetEquippedSkins(db, userID)
	if err != nil {
		return nil, nil, err
	}
	return owned, equipped, nil
}

func GetEquippedSkins(db *sql.DB, userID uuid.UUID) ([]EquippedSkin, error) {
	rows, err := db.Query(`
		SELECT ues.skin_type, ues.skin_id, sr.asset_key
		FROM user_equipped_skins ues
		JOIN user_skins us ON us.user_id = ues.user_id AND us.skin_id = ues.skin_id
		JOIN skins s ON s.id = us.skin_id AND s.enabled = TRUE
		JOIN skin_revisions sr ON sr.id = us.skin_revision_id AND sr.skin_id = us.skin_id AND sr.enabled = TRUE
		WHERE ues.user_id = $1
		ORDER BY ues.skin_type
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query equipped skins: %w", err)
	}
	defer rows.Close()

	equipped := []EquippedSkin{}
	for rows.Next() {
		var item EquippedSkin
		if err := rows.Scan(&item.SkinType, &item.SkinID, &item.AssetKey); err != nil {
			return nil, fmt.Errorf("scan equipped skin: %w", err)
		}
		equipped = append(equipped, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipped skins: %w", err)
	}
	return equipped, nil
}

func EquipSkin(db *sql.DB, userID uuid.UUID, skinType, skinID string) error {
	if !IsSkinType(skinType) {
		return ErrSkinType
	}

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM user_skins us
			JOIN skins s ON s.id = us.skin_id
			JOIN skin_revisions sr ON sr.id = us.skin_revision_id AND sr.skin_id = us.skin_id AND sr.enabled = TRUE
			WHERE us.user_id = $1 AND s.id = $2 AND s.skin_type = $3 AND s.enabled = TRUE
		)
	`, userID, skinID, skinType).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check skin ownership: %w", err)
	}
	if !exists {
		return ErrSkinNotOwned
	}

	if _, err := db.Exec(`
		INSERT INTO user_equipped_skins (user_id, skin_type, skin_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, skin_type)
		DO UPDATE SET skin_id = EXCLUDED.skin_id, equipped_at = NOW()
	`, userID, skinType, skinID); err != nil {
		return fmt.Errorf("equip skin: %w", err)
	}
	return nil
}

func UnequipSkin(db *sql.DB, userID uuid.UUID, skinType string) error {
	if !IsSkinType(skinType) {
		return ErrSkinType
	}
	if _, err := db.Exec(`DELETE FROM user_equipped_skins WHERE user_id = $1 AND skin_type = $2`, userID, skinType); err != nil {
		return fmt.Errorf("unequip skin: %w", err)
	}
	return nil
}
