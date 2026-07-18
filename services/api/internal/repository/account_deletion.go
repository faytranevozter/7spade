package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ScheduleAccountDeletion sets deletion_scheduled_at = NOW() when not already
// pending. Returns the scheduled timestamp and whether this call newly scheduled
// (false when already pending — idempotent). Returns (zero, false, nil) when the
// user row does not exist.
func ScheduleAccountDeletion(db *sql.DB, id uuid.UUID) (scheduledAt time.Time, newlyScheduled bool, err error) {
	var existing sql.NullTime
	err = db.QueryRow(`SELECT deletion_scheduled_at FROM users WHERE id = $1`, id).Scan(&existing)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("schedule account deletion lookup: %w", err)
	}
	if existing.Valid {
		return existing.Time, false, nil
	}

	var at time.Time
	err = db.QueryRow(`
		UPDATE users SET deletion_scheduled_at = NOW()
		WHERE id = $1 AND deletion_scheduled_at IS NULL
		RETURNING deletion_scheduled_at
	`, id).Scan(&at)
	if err == sql.ErrNoRows {
		// Race: another request scheduled between SELECT and UPDATE.
		err = db.QueryRow(`SELECT deletion_scheduled_at FROM users WHERE id = $1`, id).Scan(&existing)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("schedule account deletion race lookup: %w", err)
		}
		if existing.Valid {
			return existing.Time, false, nil
		}
		return time.Time{}, false, fmt.Errorf("schedule account deletion: user vanished")
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("schedule account deletion: %w", err)
	}
	return at, true, nil
}

// CancelAccountDeletion clears deletion_scheduled_at when still pending.
// Returns false when the user is missing or not pending deletion.
func CancelAccountDeletion(db *sql.DB, id uuid.UUID) (bool, error) {
	res, err := db.Exec(`
		UPDATE users SET deletion_scheduled_at = NULL
		WHERE id = $1 AND deletion_scheduled_at IS NOT NULL
	`, id)
	if err != nil {
		return false, fmt.Errorf("cancel account deletion: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cancel account deletion rows: %w", err)
	}
	return n > 0, nil
}

// ListUsersDueForDeletion returns user ids whose grace period has elapsed
// (deletion_scheduled_at <= now - grace). Limited to keep each tick bounded.
func ListUsersDueForDeletion(db *sql.DB, grace time.Duration, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 50
	}
	cutoff := time.Now().Add(-grace)
	rows, err := db.Query(`
		SELECT id FROM users
		WHERE deletion_scheduled_at IS NOT NULL
		  AND deletion_scheduled_at <= $1
		ORDER BY deletion_scheduled_at ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list users due for deletion: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user due for deletion: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users due for deletion: %w", err)
	}
	return ids, nil
}

// FinalizeAccountDeletion anonymizes historical seats, clears soft identity
// refs, then hard-deletes the user (cascades personal tables). Safe to call
// when the user is already gone (returns finalized=false).
func FinalizeAccountDeletion(db *sql.DB, id uuid.UUID) (finalized bool, err error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("finalize account deletion begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Ensure the user still exists and is due (or at least still scheduled).
	var scheduled sql.NullTime
	err = tx.QueryRow(`SELECT deletion_scheduled_at FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&scheduled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("finalize account deletion lock: %w", err)
	}
	if !scheduled.Valid {
		return false, nil
	}

	if _, err = tx.Exec(`
		UPDATE game_players SET display_name = $1 WHERE user_id = $2
	`, DeletedUserDisplayName, id); err != nil {
		return false, fmt.Errorf("finalize anonymize game_players: %w", err)
	}

	if _, err = tx.Exec(`DELETE FROM room_players WHERE user_id = $1`, id); err != nil {
		return false, fmt.Errorf("finalize delete room_players: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM room_kicked_players WHERE user_id = $1`, id); err != nil {
		return false, fmt.Errorf("finalize delete room_kicked_players: %w", err)
	}
	if _, err = tx.Exec(`
		UPDATE game_result_details SET subject_id = NULL
		WHERE subject_id = $1
	`, id.String()); err != nil {
		return false, fmt.Errorf("finalize null game_result_details.subject_id: %w", err)
	}

	res, err := tx.Exec(`DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("finalize delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("finalize delete user rows: %w", err)
	}
	if n == 0 {
		return false, nil
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("finalize account deletion commit: %w", err)
	}
	committed = true
	return true, nil
}
