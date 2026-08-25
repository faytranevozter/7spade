package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DailyLoginXPConfig struct {
	Base int
	Step int
	Max  int
}

type LoginProgress struct {
	CurrentStreak int        `json:"current_streak"`
	BestStreak    int        `json:"best_streak"`
	LastLoginDate *time.Time `json:"last_claim_date"`
	ClaimedToday  bool       `json:"claimed_today"`
}

type DailyLoginResult struct {
	Progress             LoginProgress
	NewlyClaimed         bool
	XPDelta              int
	XPAfter              int64
	Level                int
	HasLoginStreakReward bool
	NextRewardDay        *int
	SkinGrants           []SkinGrant
}

func DailyLoginXP(streakDay int, cfg DailyLoginXPConfig) int {
	if streakDay < 1 || cfg.Base <= 0 || cfg.Step <= 0 || cfg.Max < cfg.Base {
		return 0
	}
	stepsUntilCap := (cfg.Max - cfg.Base) / cfg.Step
	if streakDay-1 > stepsUntilCap {
		return cfg.Max
	}
	return cfg.Base + (streakDay-1)*cfg.Step
}

func GetLoginProgress(db *sql.DB, userID uuid.UUID, now time.Time) (DailyLoginResult, error) {
	var result DailyLoginResult
	var lastLogin sql.NullTime
	err := db.QueryRow(`
		SELECT current_streak, best_streak, last_login_date
		FROM user_login_progress
		WHERE user_id = $1
	`, userID).Scan(&result.Progress.CurrentStreak, &result.Progress.BestStreak, &lastLogin)
	if err != nil && err != sql.ErrNoRows {
		return DailyLoginResult{}, fmt.Errorf("get login progression: %w", err)
	}
	if err == nil {
		setLoginProgressDate(&result.Progress, lastLogin, now)
		if result.Progress.LastLoginDate != nil {
			previousDay := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
			lastDay := result.Progress.LastLoginDate.UTC().Format("2006-01-02")
			if !result.Progress.ClaimedToday && lastDay != previousDay {
				result.Progress.CurrentStreak = 0
			}
		}
	}
	if err := db.QueryRow(`SELECT COALESCE((SELECT xp FROM user_stats WHERE user_id = $1), 0)`, userID).Scan(&result.XPAfter); err != nil {
		return DailyLoginResult{}, fmt.Errorf("get login xp: %w", err)
	}
	result.Level = LevelFromXP(result.XPAfter)
	if err := fillLoginRewardMetadata(db, userID, result.Progress.CurrentStreak, &result); err != nil {
		return DailyLoginResult{}, err
	}
	return result, nil
}

func ClaimDailyLogin(db *sql.DB, userID uuid.UUID, now time.Time, cfg DailyLoginXPConfig) (DailyLoginResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return DailyLoginResult{}, fmt.Errorf("begin login progression: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := claimDailyLogin(tx, userID, now, cfg)
	if err != nil {
		return DailyLoginResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DailyLoginResult{}, fmt.Errorf("commit login progression: %w", err)
	}
	return result, nil
}

// RecordInteractiveLogin counts one login per UTC calendar date and grants all
// newly eligible rewards in the same transaction as refresh-token issue.
func RecordInteractiveLogin(db *sql.DB, userID uuid.UUID, authenticatedAt time.Time, refreshTokenHash string, refreshExpiresAt time.Time, cfg DailyLoginXPConfig) (DailyLoginResult, error) {
	tx, err := db.Begin()
	if err != nil {
		return DailyLoginResult{}, fmt.Errorf("begin login progression: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := claimDailyLogin(tx, userID, authenticatedAt, cfg)
	if err != nil {
		return DailyLoginResult{}, err
	}
	if _, err := tx.Exec(`
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), userID, refreshTokenHash, refreshExpiresAt, authenticatedAt); err != nil {
		return DailyLoginResult{}, fmt.Errorf("store login refresh token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DailyLoginResult{}, fmt.Errorf("commit login progression: %w", err)
	}
	return result, nil
}

func claimDailyLogin(tx *sql.Tx, userID uuid.UUID, now time.Time, cfg DailyLoginXPConfig) (DailyLoginResult, error) {
	day := now.UTC().Format("2006-01-02")
	if _, err := tx.Exec(`
		INSERT INTO user_login_progress (user_id, current_streak, best_streak, last_login_date, updated_at)
		VALUES ($1, 0, 0, NULL, NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return DailyLoginResult{}, fmt.Errorf("create login progression: %w", err)
	}

	var result DailyLoginResult
	var lastLogin sql.NullTime
	err := tx.QueryRow(`
		SELECT current_streak, best_streak, last_login_date
		FROM user_login_progress
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&result.Progress.CurrentStreak, &result.Progress.BestStreak, &lastLogin)
	if err != nil {
		return DailyLoginResult{}, fmt.Errorf("lock login progression: %w", err)
	}

	sameDay := lastLogin.Valid && lastLogin.Time.UTC().Format("2006-01-02") == day
	if !sameDay {
		previousDay := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
		if lastLogin.Valid && lastLogin.Time.UTC().Format("2006-01-02") == previousDay {
			result.Progress.CurrentStreak++
		} else {
			result.Progress.CurrentStreak = 1
		}
		if result.Progress.CurrentStreak > result.Progress.BestStreak {
			result.Progress.BestStreak = result.Progress.CurrentStreak
		}
		if _, err := tx.Exec(`
			UPDATE user_login_progress
			SET current_streak = $1, best_streak = $2, last_login_date = $3, updated_at = NOW()
			WHERE user_id = $4
		`, result.Progress.CurrentStreak, result.Progress.BestStreak, day, userID); err != nil {
			return DailyLoginResult{}, fmt.Errorf("update login progression: %w", err)
		}
		result.NewlyClaimed = true
	}
	claimedDate := now.UTC()
	result.Progress.LastLoginDate = &claimedDate
	result.Progress.ClaimedToday = true

	if _, err := tx.Exec(`INSERT INTO user_stats (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return DailyLoginResult{}, fmt.Errorf("create login xp stats: %w", err)
	}
	if result.NewlyClaimed {
		result.XPDelta = DailyLoginXP(result.Progress.CurrentStreak, cfg)
		if err := tx.QueryRow(`UPDATE user_stats SET xp = xp + $1, updated_at = NOW() WHERE user_id = $2 RETURNING xp`, result.XPDelta, userID).Scan(&result.XPAfter); err != nil {
			return DailyLoginResult{}, fmt.Errorf("award login xp: %w", err)
		}
		xpBefore := result.XPAfter - int64(result.XPDelta)
		if _, err := tx.Exec(`
			INSERT INTO daily_login_xp_events (user_id, claim_date, streak_day, xp_before, xp_after, xp_delta)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, claim_date) DO NOTHING
		`, userID, day, result.Progress.CurrentStreak, xpBefore, result.XPAfter, result.XPDelta); err != nil {
			return DailyLoginResult{}, fmt.Errorf("record login xp: %w", err)
		}
	} else if err := tx.QueryRow(`SELECT xp FROM user_stats WHERE user_id = $1`, userID).Scan(&result.XPAfter); err != nil {
		return DailyLoginResult{}, fmt.Errorf("read login xp: %w", err)
	}
	result.Level = LevelFromXP(result.XPAfter)

	grants, err := grantLoginStreakSkins(tx, userID, result.Progress.CurrentStreak)
	if err != nil {
		return DailyLoginResult{}, err
	}
	levelGrants, err := GrantMinimumLevelSkins(tx, userID, result.Level)
	if err != nil {
		return DailyLoginResult{}, err
	}
	result.SkinGrants = append(grants, levelGrants...)
	if err := fillLoginRewardMetadata(tx, userID, result.Progress.CurrentStreak, &result); err != nil {
		return DailyLoginResult{}, err
	}
	return result, nil
}

type loginRowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func fillLoginRewardMetadata(q loginRowQuerier, userID uuid.UUID, streak int, result *DailyLoginResult) error {
	var next sql.NullInt64
	if err := q.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM skin_unlock_rules r JOIN skins s ON s.id = r.skin_id
			WHERE r.rule_type = 'login_streak' AND r.enabled = TRUE AND s.enabled = TRUE
		), (
			SELECT MIN(r.login_streak_days) FROM skin_unlock_rules r JOIN skins s ON s.id = r.skin_id
			WHERE r.rule_type = 'login_streak' AND r.enabled = TRUE AND s.enabled = TRUE
			  AND r.login_streak_days > $2
			  AND NOT EXISTS (SELECT 1 FROM user_skins us WHERE us.user_id = $1 AND us.skin_id = r.skin_id)
		)
	`, userID, streak).Scan(&result.HasLoginStreakReward, &next); err != nil {
		return fmt.Errorf("get login reward metadata: %w", err)
	}
	if next.Valid {
		day := int(next.Int64)
		result.NextRewardDay = &day
	}
	return nil
}

func setLoginProgressDate(progress *LoginProgress, lastLogin sql.NullTime, now time.Time) {
	if !lastLogin.Valid {
		return
	}
	date := lastLogin.Time.UTC()
	progress.LastLoginDate = &date
	progress.ClaimedToday = date.Format("2006-01-02") == now.UTC().Format("2006-01-02")
}

func grantLoginStreakSkins(tx *sql.Tx, userID uuid.UUID, streak int) ([]SkinGrant, error) {
	rows, err := tx.Query(`
		WITH inserted AS (
			INSERT INTO user_skins (user_id, skin_id, source)
			SELECT $1, s.id, 'login_streak:' || r.login_streak_days
			FROM skin_unlock_rules r
			JOIN skins s ON s.id = r.skin_id
			WHERE r.rule_type = 'login_streak'
			  AND r.login_streak_days <= $2
			  AND r.enabled = TRUE
			  AND s.enabled = TRUE
			ON CONFLICT (user_id, skin_id) DO NOTHING
			RETURNING skin_id, source
		)
		SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order, i.source
		FROM inserted i
		JOIN skins s ON s.id = i.skin_id
		ORDER BY s.display_order, s.id
	`, userID, streak)
	if err != nil {
		return nil, fmt.Errorf("grant login streak skins: %w", err)
	}
	defer rows.Close()

	grants := []SkinGrant{}
	for rows.Next() {
		var grant SkinGrant
		if err := rows.Scan(&grant.ID, &grant.SkinType, &grant.Name, &grant.Description, &grant.AssetKey, &grant.DisplayOrder, &grant.Source); err != nil {
			return nil, fmt.Errorf("scan login streak skin grant: %w", err)
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate login streak skin grants: %w", err)
	}
	return grants, nil
}
