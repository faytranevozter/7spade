package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Achievement IDs. Stable identifiers; presentation (name/description/icon)
// lives in the frontend catalog (web/src/game/achievements.ts), kept in sync.
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

// AllAchievementIDs is the server-side allowlist of awardable achievements,
// used to validate awards and (optionally) expose the catalog.
var AllAchievementIDs = []string{
	AchievementFirstWin,
	AchievementGames10,
	AchievementGames50,
	AchievementGames100,
	AchievementStreak3,
	AchievementStreak5,
	AchievementPerfectRound,
	AchievementSharedWin,
}

var allowedAchievements = func() map[string]bool {
	m := make(map[string]bool, len(AllAchievementIDs))
	for _, id := range AllAchievementIDs {
		m[id] = true
	}
	return m
}()

// EarnedAchievement is a single earned badge with its timestamp.
type EarnedAchievement struct {
	AchievementID string `json:"achievement_id"`
	EarnedAt      string `json:"earned_at"`
}

// achievementContext carries everything the evaluator needs about one player's
// just-saved game and updated counters, all available inside SaveGame's tx.
type achievementContext struct {
	IsWinner    bool
	SharedWin   bool // winner tied with at least one other player
	Penalty     int
	GamesPlayed int
	Streak      int
}

// evaluateAchievementIDs returns the achievement IDs the player qualifies for
// given this game. Awarding is idempotent downstream, so emitting an already-
// earned ID is harmless — we don't need prior-earned state here.
func evaluateAchievementIDs(ctx achievementContext) []string {
	ids := make([]string, 0, 5)
	if ctx.IsWinner {
		ids = append(ids, AchievementFirstWin)
	}
	if ctx.SharedWin {
		ids = append(ids, AchievementSharedWin)
	}
	if ctx.Penalty == 0 {
		ids = append(ids, AchievementPerfectRound)
	}
	if ctx.GamesPlayed >= 100 {
		ids = append(ids, AchievementGames100)
	} else if ctx.GamesPlayed >= 50 {
		ids = append(ids, AchievementGames50)
	} else if ctx.GamesPlayed >= 10 {
		ids = append(ids, AchievementGames10)
	}
	if ctx.Streak >= 5 {
		ids = append(ids, AchievementStreak5)
	} else if ctx.Streak >= 3 {
		ids = append(ids, AchievementStreak3)
	}
	return ids
}

// AwardAchievements inserts the given achievement IDs for a user inside the
// caller's transaction, ignoring already-earned ones (idempotent) and any ID
// not on the allowlist. A no-op for an empty list.
func AwardAchievements(tx *sql.Tx, userID uuid.UUID, ids []string) error {
	for _, id := range ids {
		if !allowedAchievements[id] {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO user_achievements (user_id, achievement_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, achievement_id) DO NOTHING
		`, userID, id); err != nil {
			return fmt.Errorf("award achievement %s: %w", id, err)
		}
	}
	return nil
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
