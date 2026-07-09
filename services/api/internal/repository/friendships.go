package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Friendship status values.
const (
	FriendshipPending  = "pending"
	FriendshipAccepted = "accepted"
	FriendshipBlocked  = "blocked"
)

var (
	// ErrFriendshipSelf is returned when a user targets themselves.
	ErrFriendshipSelf = errors.New("cannot friend yourself")
	// ErrFriendshipBlocked is returned when a block prevents the action.
	ErrFriendshipBlocked = errors.New("blocked")
)

func friendshipPairKey(a, b uuid.UUID) string {
	as, bs := a.String(), b.String()
	if as > bs {
		as, bs = bs, as
	}
	return as + ":" + bs
}

// FriendEntry is one entry in a user's friends list. Direction distinguishes
// accepted friends from pending requests the caller sent vs. received.
type FriendEntry struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	Username    string  `json:"username"`
	AvatarURL   *string `json:"avatar_url"`
	// Status: "accepted" | "incoming" | "outgoing".
	Status string `json:"status"`
}

// SendFriendRequest creates a pending request from requester -> addressee. If a
// reverse pending request already exists (addressee previously requested
// requester), both intentions are present, so the friendship is auto-accepted.
// Returns the resulting status ("pending" or "accepted"). A blocked relation in
// either direction rejects the request.
func SendFriendRequest(db *sql.DB, requesterID, addresseeID uuid.UUID) (string, error) {
	if requesterID == addresseeID {
		return "", ErrFriendshipSelf
	}

	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin friend request: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Serialize all request mutations for this unordered pair. Without this,
	// crossed requests (A -> B and B -> A) can both miss the reverse pending row
	// and insert two directed pending rows before either commits.
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, friendshipPairKey(requesterID, addresseeID)); err != nil {
		return "", fmt.Errorf("lock friendship pair: %w", err)
	}

	// Reject if either side has a blocked relation.
	var blockedCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM friendships
		WHERE status = 'blocked'
		  AND ((requester_id = $1 AND addressee_id = $2) OR (requester_id = $2 AND addressee_id = $1))
	`, requesterID, addresseeID).Scan(&blockedCount); err != nil {
		return "", fmt.Errorf("check blocked: %w", err)
	}
	if blockedCount > 0 {
		return "", ErrFriendshipBlocked
	}

	// If a reverse row already exists from the addressee to the caller, settle
	// it as accepted instead of creating a competing forward row. This covers
	// both a reverse PENDING request (both intentions now present -> accept) and
	// a reverse ACCEPTED row (they're already friends; re-requesting is
	// idempotent and must not create a duplicate forward row).
	res, err := tx.Exec(`
		UPDATE friendships SET status = 'accepted', updated_at = NOW()
		WHERE requester_id = $1 AND addressee_id = $2 AND status IN ('pending', 'accepted')
	`, addresseeID, requesterID)
	if err != nil {
		return "", fmt.Errorf("auto-accept reverse: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("commit auto-accept: %w", err)
		}
		committed = true
		return FriendshipAccepted, nil
	}

	// Otherwise upsert a pending request in the caller's direction. If an
	// accepted row already exists, leave it accepted (idempotent).
	if _, err := tx.Exec(`
		INSERT INTO friendships (requester_id, addressee_id, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (requester_id, addressee_id) DO NOTHING
	`, requesterID, addresseeID); err != nil {
		return "", fmt.Errorf("insert friend request: %w", err)
	}

	var status string
	if err := tx.QueryRow(`
		SELECT status FROM friendships WHERE requester_id = $1 AND addressee_id = $2
	`, requesterID, addresseeID).Scan(&status); err != nil {
		return "", fmt.Errorf("read request status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit friend request: %w", err)
	}
	committed = true
	return status, nil
}

// AcceptFriendRequest marks an incoming pending request (other -> me) accepted.
// Returns false if there was no such pending request.
func AcceptFriendRequest(db *sql.DB, meID, otherID uuid.UUID) (bool, error) {
	res, err := db.Exec(`
		UPDATE friendships SET status = 'accepted', updated_at = NOW()
		WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'
	`, otherID, meID)
	if err != nil {
		return false, fmt.Errorf("accept friend request: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemoveFriendship deletes any pending/accepted relationship between the two
// users in either direction (decline, cancel, or unfriend). Blocked rows are
// left intact so a block survives an unfriend. Returns false if nothing was
// removed.
func RemoveFriendship(db *sql.DB, meID, otherID uuid.UUID) (bool, error) {
	res, err := db.Exec(`
		DELETE FROM friendships
		WHERE status <> 'blocked'
		  AND ((requester_id = $1 AND addressee_id = $2) OR (requester_id = $2 AND addressee_id = $1))
	`, meID, otherID)
	if err != nil {
		return false, fmt.Errorf("remove friendship: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// BlockUser removes any existing relationship and records a block owned by the
// caller (meID -> otherID). Idempotent.
func BlockUser(db *sql.DB, meID, otherID uuid.UUID) error {
	if meID == otherID {
		return ErrFriendshipSelf
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin block: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
		DELETE FROM friendships
		WHERE (requester_id = $1 AND addressee_id = $2) OR (requester_id = $2 AND addressee_id = $1)
	`, meID, otherID); err != nil {
		return fmt.Errorf("clear before block: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO friendships (requester_id, addressee_id, status)
		VALUES ($1, $2, 'blocked')
		ON CONFLICT (requester_id, addressee_id) DO UPDATE SET status = 'blocked', updated_at = NOW()
	`, meID, otherID); err != nil {
		return fmt.Errorf("insert block: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit block: %w", err)
	}
	committed = true
	return nil
}

// ListFriends returns the caller's accepted friends plus their pending
// incoming/outgoing requests, each with display name + avatar. Blocked rows are
// excluded.
func ListFriends(db *sql.DB, meID uuid.UUID) ([]FriendEntry, error) {
	rows, err := db.Query(`
		SELECT
			other.id,
			other.display_name,
			other.username,
			av.avatar_url,
			CASE
				WHEN f.status = 'accepted' THEN 'accepted'
				WHEN f.requester_id = $1 THEN 'outgoing'
				ELSE 'incoming'
			END AS direction
		FROM friendships f
		JOIN users other ON other.id = CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
		LEFT JOIN LATERAL (
			SELECT up.avatar_url
			FROM user_providers up
			WHERE up.user_id = other.id AND up.avatar_url IS NOT NULL
			ORDER BY CASE up.provider
			           WHEN 'google'   THEN 0
			           WHEN 'github'   THEN 1
			           WHEN 'telegram' THEN 2
			           ELSE 3
			         END,
			         up.created_at DESC
			LIMIT 1
		) av ON true
		WHERE (f.requester_id = $1 OR f.addressee_id = $1)
		  AND f.status IN ('pending', 'accepted')
		ORDER BY direction, other.display_name
	`, meID)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}
	defer rows.Close()

	entries := []FriendEntry{}
	for rows.Next() {
		var (
			e      FriendEntry
			avatar sql.NullString
		)
		if err := rows.Scan(&e.UserID, &e.DisplayName, &e.Username, &avatar, &e.Status); err != nil {
			return nil, fmt.Errorf("scan friend: %w", err)
		}
		if avatar.Valid {
			e.AvatarURL = &avatar.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate friends: %w", err)
	}
	return entries, nil
}
