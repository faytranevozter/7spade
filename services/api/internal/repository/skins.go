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
	ID           string `json:"id"`
	SkinType     string `json:"skin_type"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AssetKey     string `json:"asset_key"`
	DisplayOrder int    `json:"display_order"`
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
				INSERT INTO user_skins (user_id, skin_id, source)
				SELECT $1, s.id, 'achievement:' || $2
				FROM skin_unlock_rules r
				JOIN skins s ON s.id = r.skin_id
				WHERE r.rule_type = 'achievement'
				  AND r.achievement_id = $2
				  AND r.enabled = TRUE
				  AND s.enabled = TRUE
				ON CONFLICT (user_id, skin_id) DO NOTHING
				RETURNING skin_id, source
			)
			SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, i.source
			FROM inserted i
			JOIN skins s ON s.id = i.skin_id
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

func GrantGameConditionSkins(tx *sql.Tx, userID uuid.UUID, ctx achievementContext) ([]SkinGrant, error) {
	rows, err := tx.Query(`
		SELECT r.id, r.skin_id, r.metric, r.operator, r.value
		FROM skin_unlock_rules r
		JOIN skins s ON s.id = r.skin_id
		WHERE r.rule_type = 'game_condition'
		  AND r.enabled = TRUE
		  AND s.enabled = TRUE
		ORDER BY s.display_order, s.id, r.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query game-condition skin rules: %w", err)
	}
	defer rows.Close()

	type matchingRule struct {
		id, skinID string
	}
	matches := []matchingRule{}
	for rows.Next() {
		var ruleID, skinID string
		var rule achievementRule
		if err := rows.Scan(&ruleID, &skinID, &rule.Metric, &rule.Operator, &rule.Value); err != nil {
			return nil, fmt.Errorf("scan game-condition skin rule: %w", err)
		}
		matched, err := ruleMatches(ctx, rule)
		if err != nil {
			log.Printf("skins: ignoring invalid game-condition rule %s: %v", ruleID, err)
			continue
		}
		if matched {
			matches = append(matches, matchingRule{id: ruleID, skinID: skinID})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game-condition skin rules: %w", err)
	}

	grants := []SkinGrant{}
	for _, rule := range matches {
		var grant SkinGrant
		err := tx.QueryRow(`
			WITH inserted AS (
				INSERT INTO user_skins (user_id, skin_id, source)
				SELECT $1, s.id, 'game_condition:' || $3
				FROM skins s
				WHERE s.id = $2 AND s.enabled = TRUE
				ON CONFLICT (user_id, skin_id) DO NOTHING
				RETURNING skin_id, source
			)
			SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, i.source
			FROM inserted i
			JOIN skins s ON s.id = i.skin_id
		`, userID, rule.skinID, rule.id).Scan(
			&grant.ID, &grant.SkinType, &grant.Name, &grant.Description,
			&grant.AssetKey, &grant.DisplayOrder, &grant.Source,
		)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("grant skin for game-condition rule %s: %w", rule.id, err)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func GetSkinCatalog(db *sql.DB) ([]Skin, error) {
	rows, err := db.Query(`
		SELECT id, skin_type, name, description, asset_key, display_order
		FROM skins
		WHERE enabled = TRUE
		ORDER BY skin_type, display_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("query skin catalog: %w", err)
	}
	defer rows.Close()

	items := []Skin{}
	for rows.Next() {
		var item Skin
		if err := rows.Scan(&item.ID, &item.SkinType, &item.Name, &item.Description, &item.AssetKey, &item.DisplayOrder); err != nil {
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
		       us.source, (ues.skin_id IS NOT NULL) AS equipped
		FROM user_skins us
		JOIN skins s ON s.id = us.skin_id AND s.enabled = TRUE
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
