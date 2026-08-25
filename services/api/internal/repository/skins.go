package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

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
	ID                string `json:"id"`
	SkinType          string `json:"skin_type"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AssetKey          string `json:"asset_key"`
	DisplayOrder      int    `json:"display_order"`
	UnlockRequirement string `json:"unlock_requirement,omitempty"`
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
				INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id)
				SELECT $1, s.id, r.id
				FROM skin_unlock_rules r
				JOIN skins s ON s.id = r.skin_id
				WHERE r.rule_type = 'achievement'
				  AND r.achievement_id = $2
				  AND r.enabled = TRUE
				  AND s.enabled = TRUE
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
			INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id)
			SELECT $1, s.id, r.id
			FROM skin_unlock_rules r
			JOIN skins s ON s.id = r.skin_id
			WHERE r.rule_type = 'minimum_level'
			  AND r.minimum_level <= $2
			  AND r.enabled = TRUE
			  AND s.enabled = TRUE
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

func GrantGameConditionSkins(tx *sql.Tx, userID uuid.UUID, ctx achievementContext) ([]SkinGrant, error) {
	rows, err := tx.Query(`
		SELECT r.id, r.name, r.skin_id, c.metric, c.operator, c.value
		FROM skin_unlock_rules r
		JOIN skin_unlock_rule_conditions c ON c.skin_unlock_rule_id = r.id
		JOIN skins s ON s.id = r.skin_id
		WHERE r.rule_type = 'game_condition'
		  AND r.enabled = TRUE
		  AND s.enabled = TRUE
		ORDER BY s.display_order, s.id, r.name, r.id, c.created_at, c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query game-condition skin rules: %w", err)
	}
	defer rows.Close()

	type gameConditionRule struct {
		id, name, skinID string
		conditions       []achievementRule
	}
	grouped := []gameConditionRule{}
	for rows.Next() {
		var ruleID, ruleName, skinID string
		var condition achievementRule
		if err := rows.Scan(&ruleID, &ruleName, &skinID, &condition.Metric, &condition.Operator, &condition.Value); err != nil {
			return nil, fmt.Errorf("scan game-condition skin rule: %w", err)
		}
		if len(grouped) == 0 || grouped[len(grouped)-1].id != ruleID {
			grouped = append(grouped, gameConditionRule{id: ruleID, name: ruleName, skinID: skinID})
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
				INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id)
				SELECT $1, s.id, $3
				FROM skins s
				WHERE s.id = $2 AND s.enabled = TRUE
				ON CONFLICT (user_id, skin_id) DO NOTHING
				RETURNING skin_id
			)
			SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, 'game_condition:' || $4
			FROM inserted i
			JOIN skins s ON s.id = i.skin_id
		`, userID, rule.skinID, rule.id, rule.name).Scan(
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
		       COALESCE(string_agg(
		           CASE r.rule_type
		             WHEN 'achievement' THEN 'Earn the ' || a.name || ' achievement'
		             WHEN 'minimum_level' THEN 'Reach player level ' || r.minimum_level
		             WHEN 'login_streak' THEN 'Log in on ' || r.login_streak_days || ' consecutive days'
		             WHEN 'game_condition' THEN COALESCE((
		               SELECT string_agg(
		                 CASE c.metric
		                   WHEN 'is_winner' THEN CASE c.value WHEN 'true' THEN 'Win a completed game' ELSE 'Finish a completed game without winning' END
		                   WHEN 'shared_win_count' THEN 'Share a win with ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' players'
		                   WHEN 'penalty' THEN 'Finish with ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' penalty points'
		                   WHEN 'games_played' THEN 'Play ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' games'
		                   WHEN 'wins' THEN 'Win ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' games'
		                   WHEN 'current_streak' THEN 'Reach a win streak of ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END)
		                   WHEN 'current_top2_streak' THEN 'Reach a top-two streak of ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END)
		                   WHEN 'first_place_count' THEN 'Finish first in ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' games'
		                   WHEN 'zero_penalty_games' THEN 'Complete ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' zero-penalty games'
		                   WHEN 'human_only_games' THEN 'Complete ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' human-only games'
		                   WHEN 'all_zero_penalty' THEN CASE c.value WHEN 'true' THEN 'Complete a game where every player has zero penalty' ELSE 'Complete a game where not every player has zero penalty' END
		                   WHEN 'ace_closed' THEN CASE c.value WHEN 'true' THEN 'Close an Ace during the game' ELSE 'Complete a game without closing an Ace' END
		                   WHEN 'game_duration_seconds' THEN 'Finish a game in ' || (CASE c.operator WHEN 'eq' THEN 'exactly ' || c.value WHEN 'gte' THEN 'at least ' || c.value WHEN 'lte' THEN 'at most ' || c.value WHEN 'gt' THEN 'more than ' || c.value WHEN 'lt' THEN 'fewer than ' || c.value END) || ' seconds'
		                 END,
		                 ' and ' ORDER BY c.created_at, c.id
		               )
		               FROM skin_unlock_rule_conditions c
		               WHERE c.skin_unlock_rule_id = r.id
		             ), 'Complete the ' || r.name || ' challenge')
		           END,
		           ' or ' ORDER BY r.name
		       ) FILTER (WHERE r.id IS NOT NULL), '') AS unlock_requirement
		FROM skins s
		LEFT JOIN skin_unlock_rules r ON r.skin_id = s.id AND r.enabled = TRUE
		LEFT JOIN achievements a ON a.id = r.achievement_id
		WHERE s.enabled = TRUE
		GROUP BY s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order
		ORDER BY s.skin_type, s.display_order, s.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query skin catalog: %w", err)
	}
	defer rows.Close()

	items := []Skin{}
	for rows.Next() {
		var item Skin
		if err := rows.Scan(&item.ID, &item.SkinType, &item.Name, &item.Description, &item.AssetKey, &item.DisplayOrder, &item.UnlockRequirement); err != nil {
			return nil, fmt.Errorf("scan skin catalog: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skin catalog: %w", err)
	}
	return items, nil
}

func GetUserSkins(db *sql.DB, userID uuid.UUID) ([]OwnedSkin, []EquippedSkin, error) {
	rows, err := db.Query(`
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order,
		       COALESCE(
		           us.source,
		           CASE r.rule_type
		             WHEN 'achievement' THEN 'achievement:' || r.achievement_id
		             WHEN 'minimum_level' THEN 'level:' || r.minimum_level
		             WHEN 'login_streak' THEN 'login_streak:' || r.login_streak_days
		             WHEN 'game_condition' THEN 'game_condition:' || r.name
		           END,
		           'unknown'
		       ),
		       (ues.skin_id IS NOT NULL) AS equipped
		FROM user_skins us
		JOIN skins s ON s.id = us.skin_id AND s.enabled = TRUE
		LEFT JOIN skin_unlock_rules r ON r.id = us.skin_unlock_rule_id
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
		SELECT ues.skin_type, ues.skin_id, s.asset_key
		FROM user_equipped_skins ues
		JOIN skins s ON s.id = ues.skin_id AND s.enabled = TRUE
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
