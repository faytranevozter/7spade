package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type LoginProgress struct {
	CurrentStreak int        `json:"current_streak"`
	BestStreak    int        `json:"best_streak"`
	LastLoginDate *time.Time `json:"last_claim_date"`
	ClaimedToday  bool       `json:"claimed_today"`
}

func GetLoginProgress(db *sql.DB, userID uuid.UUID, now time.Time) (LoginProgress, error) {
	var progress LoginProgress
	var lastLogin sql.NullTime
	err := db.QueryRow(`
		SELECT current_streak, best_streak, last_login_date
		FROM user_login_progress
		WHERE user_id = $1
	`, userID).Scan(&progress.CurrentStreak, &progress.BestStreak, &lastLogin)
	if err == sql.ErrNoRows {
		return progress, nil
	}
	if err != nil {
		return LoginProgress{}, fmt.Errorf("get login progression: %w", err)
	}
	setLoginProgressDate(&progress, lastLogin, now)
	if progress.LastLoginDate != nil {
		previousDay := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
		lastDay := progress.LastLoginDate.UTC().Format("2006-01-02")
		if !progress.ClaimedToday && lastDay != previousDay {
			progress.CurrentStreak = 0
		}
	}
	return progress, nil
}

func ClaimDailyLogin(db *sql.DB, userID uuid.UUID, now time.Time) (LoginProgress, []SkinGrant, error) {
	tx, err := db.Begin()
	if err != nil {
		return LoginProgress{}, nil, fmt.Errorf("begin login progression: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	progress, grants, err := claimDailyLogin(tx, userID, now)
	if err != nil {
		return LoginProgress{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return LoginProgress{}, nil, fmt.Errorf("commit login progression: %w", err)
	}
	return progress, grants, nil
}

// RecordInteractiveLogin counts one login per UTC calendar date and grants all
// newly eligible streak rewards in the same transaction as refresh-token issue.
func RecordInteractiveLogin(db *sql.DB, userID uuid.UUID, authenticatedAt time.Time, refreshTokenHash string, refreshExpiresAt time.Time) (LoginProgress, []SkinGrant, error) {
	tx, err := db.Begin()
	if err != nil {
		return LoginProgress{}, nil, fmt.Errorf("begin login progression: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	progress, grants, err := claimDailyLogin(tx, userID, authenticatedAt)
	if err != nil {
		return LoginProgress{}, nil, err
	}
	if _, err := tx.Exec(`
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), userID, refreshTokenHash, refreshExpiresAt, authenticatedAt); err != nil {
		return LoginProgress{}, nil, fmt.Errorf("store login refresh token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return LoginProgress{}, nil, fmt.Errorf("commit login progression: %w", err)
	}
	return progress, grants, nil
}

func claimDailyLogin(tx *sql.Tx, userID uuid.UUID, now time.Time) (LoginProgress, []SkinGrant, error) {
	day := now.UTC().Format("2006-01-02")
	if _, err := tx.Exec(`
		INSERT INTO user_login_progress (user_id, current_streak, best_streak, last_login_date, updated_at)
		VALUES ($1, 0, 0, NULL, NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return LoginProgress{}, nil, fmt.Errorf("create login progression: %w", err)
	}

	var progress LoginProgress
	var lastLogin sql.NullTime
	err := tx.QueryRow(`
		SELECT current_streak, best_streak, last_login_date
		FROM user_login_progress
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&progress.CurrentStreak, &progress.BestStreak, &lastLogin)
	if err != nil {
		return LoginProgress{}, nil, fmt.Errorf("lock login progression: %w", err)
	}

	sameDay := lastLogin.Valid && lastLogin.Time.UTC().Format("2006-01-02") == day
	if !sameDay {
		previousDay := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
		if lastLogin.Valid && lastLogin.Time.UTC().Format("2006-01-02") == previousDay {
			progress.CurrentStreak++
		} else {
			progress.CurrentStreak = 1
		}
		if progress.CurrentStreak > progress.BestStreak {
			progress.BestStreak = progress.CurrentStreak
		}
		if _, err := tx.Exec(`
			UPDATE user_login_progress
			SET current_streak = $1, best_streak = $2, last_login_date = $3, updated_at = NOW()
			WHERE user_id = $4
		`, progress.CurrentStreak, progress.BestStreak, day, userID); err != nil {
			return LoginProgress{}, nil, fmt.Errorf("update login progression: %w", err)
		}
	}
	claimedDate := now.UTC()
	progress.LastLoginDate = &claimedDate
	progress.ClaimedToday = true

	grants, err := grantLoginStreakSkins(tx, userID, progress.CurrentStreak)
	if err != nil {
		return LoginProgress{}, nil, err
	}
	return progress, grants, nil
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
