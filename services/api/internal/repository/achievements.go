package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Achievement IDs. Stable identifiers used by seeded DB rules and historical
// earned rows. Presentation and award rules live in the database.
const (
	AchievementFirstWin     = "first_win"
	AchievementGames10      = "games_10"
	AchievementGames50      = "games_50"
	AchievementGames100     = "games_100"
	AchievementStreak3      = "streak_3"
	AchievementStreak5      = "streak_5"
	AchievementPerfectRound = "perfect_round"
	AchievementSharedWin    = "shared_win"
)

// Achievement is the display catalog row returned to clients.
type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// EarnedAchievement is a single earned badge with its timestamp.
type EarnedAchievement struct {
	AchievementID string `json:"achievement_id"`
	EarnedAt      string `json:"earned_at"`
}

type achievementRule struct {
	AchievementID string
	Metric        string
	Operator      string
	Value         string
}

// achievementContext carries everything the evaluator needs about one player's
// just-saved game and updated counters, all available inside SaveGame's tx.
type achievementContext struct {
	IsWinner      bool
	SharedWin     bool // winner tied with at least one other player
	Penalty       int
	GamesPlayed   int
	Wins          int
	CurrentStreak int
}

// EvaluateAchievementIDs returns every enabled achievement whose DB-configured
// rules match the player's just-saved game context. Each achievement's rules are
// ANDed together; each achievement is evaluated independently, so all matching
// tiers are awarded.
func EvaluateAchievementIDs(tx *sql.Tx, ctx achievementContext) ([]string, error) {
	rows, err := tx.Query(`
		SELECT a.id, r.metric, r.operator, r.value
		FROM achievements a
		JOIN achievement_rules r ON r.achievement_id = a.id
		WHERE a.enabled = TRUE
		ORDER BY a.display_order ASC, a.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query achievement rules: %w", err)
	}
	defer rows.Close()

	rulesByAchievement := map[string][]achievementRule{}
	order := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var rule achievementRule
		if err := rows.Scan(&rule.AchievementID, &rule.Metric, &rule.Operator, &rule.Value); err != nil {
			return nil, fmt.Errorf("scan achievement rule: %w", err)
		}
		if !seen[rule.AchievementID] {
			seen[rule.AchievementID] = true
			order = append(order, rule.AchievementID)
		}
		rulesByAchievement[rule.AchievementID] = append(rulesByAchievement[rule.AchievementID], rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievement rules: %w", err)
	}

	ids := []string{}
	for _, achievementID := range order {
		matches := true
		for _, rule := range rulesByAchievement[achievementID] {
			ok, err := ruleMatches(ctx, rule)
			if err != nil {
				return nil, err
			}
			if !ok {
				matches = false
				break
			}
		}
		if matches {
			ids = append(ids, achievementID)
		}
	}
	return ids, nil
}

func ruleMatches(ctx achievementContext, rule achievementRule) (bool, error) {
	switch rule.Metric {
	case "is_winner":
		return compareBool(ctx.IsWinner, rule.Operator, rule.Value)
	case "shared_win":
		return compareBool(ctx.SharedWin, rule.Operator, rule.Value)
	case "penalty":
		return compareInt(ctx.Penalty, rule.Operator, rule.Value)
	case "games_played":
		return compareInt(ctx.GamesPlayed, rule.Operator, rule.Value)
	case "wins":
		return compareInt(ctx.Wins, rule.Operator, rule.Value)
	case "current_streak":
		return compareInt(ctx.CurrentStreak, rule.Operator, rule.Value)
	default:
		return false, fmt.Errorf("unknown achievement metric %q for %s", rule.Metric, rule.AchievementID)
	}
}

func compareBool(actual bool, operator string, value string) (bool, error) {
	if operator != "eq" {
		return false, fmt.Errorf("unsupported boolean achievement operator %q", operator)
	}
	expected, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse boolean achievement value %q: %w", value, err)
	}
	return actual == expected, nil
}

func compareInt(actual int, operator string, value string) (bool, error) {
	expected, err := strconv.Atoi(value)
	if err != nil {
		return false, fmt.Errorf("parse integer achievement value %q: %w", value, err)
	}
	switch operator {
	case "eq":
		return actual == expected, nil
	case "gte":
		return actual >= expected, nil
	case "lte":
		return actual <= expected, nil
	case "gt":
		return actual > expected, nil
	case "lt":
		return actual < expected, nil
	default:
		return false, fmt.Errorf("unsupported integer achievement operator %q", operator)
	}
}

// AwardAchievements inserts the given achievement IDs for a user inside the
// caller's transaction, ignoring already-earned ones (idempotent) and any ID
// not on the allowlist. A no-op for an empty list.
func AwardAchievements(tx *sql.Tx, userID uuid.UUID, ids []string) error {
	for _, id := range ids {
		if _, err := tx.Exec(`
			INSERT INTO user_achievements (user_id, achievement_id)
			SELECT $1, id FROM achievements WHERE id = $2 AND enabled = TRUE
			ON CONFLICT (user_id, achievement_id) DO NOTHING
		`, userID, id); err != nil {
			return fmt.Errorf("award achievement %s: %w", id, err)
		}
	}
	return nil
}

// GetAchievementCatalog returns enabled achievements in display order.
func GetAchievementCatalog(db *sql.DB) ([]Achievement, error) {
	rows, err := db.Query(`
		SELECT id, name, description, icon
		FROM achievements
		WHERE enabled = TRUE
		ORDER BY display_order ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query achievement catalog: %w", err)
	}
	defer rows.Close()

	catalog := []Achievement{}
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Icon); err != nil {
			return nil, fmt.Errorf("scan achievement catalog: %w", err)
		}
		catalog = append(catalog, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievement catalog: %w", err)
	}
	return catalog, nil
}

// GetUserAchievements returns a user's earned achievements, newest first. The
// list is empty (not an error) for a user who has earned none.
func GetUserAchievements(db *sql.DB, userID uuid.UUID) ([]EarnedAchievement, error) {
	rows, err := db.Query(`
		SELECT achievement_id, earned_at
		FROM user_achievements
		WHERE user_id = $1
		ORDER BY earned_at DESC, achievement_id ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query achievements: %w", err)
	}
	defer rows.Close()

	earned := []EarnedAchievement{}
	for rows.Next() {
		var a EarnedAchievement
		var earnedAt time.Time
		if err := rows.Scan(&a.AchievementID, &earnedAt); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}
		a.EarnedAt = earnedAt.UTC().Format(time.RFC3339)
		earned = append(earned, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievements: %w", err)
	}
	return earned, nil
}
