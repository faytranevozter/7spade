package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Username rules: lowercase letters, digits, and underscores, 3-32 chars. Stored
// normalized (lowercase) so friend lookups are a plain unique-key match.
const (
	usernameMinLen = 3
	usernameMaxLen = 32
)

var usernameRegex = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

// ErrUsernameInvalid is returned when a username doesn't satisfy the rules.
var ErrUsernameInvalid = errors.New("invalid username")

// NormalizeUsername lowercases and trims surrounding whitespace. It does not
// validate; pair it with ValidateUsername.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// ValidateUsername reports whether a normalized username satisfies the rules.
func ValidateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return ErrUsernameInvalid
	}
	return nil
}

// usernameFromCandidate turns an arbitrary string (display name, email local
// part, provider handle) into a valid username base: lowercased, non-[a-z0-9_]
// runs collapsed to "_", trimmed of leading/trailing "_", and truncated. Returns
// "player" when nothing usable remains so callers always get a valid base.
var nonUsernameChars = regexp.MustCompile(`[^a-z0-9_]+`)

func usernameFromCandidate(candidate string) string {
	s := nonUsernameChars.ReplaceAllString(strings.ToLower(candidate), "_")
	s = strings.Trim(s, "_")
	if len(s) > usernameMaxLen {
		s = s[:usernameMaxLen]
		s = strings.Trim(s, "_")
	}
	if len(s) < usernameMinLen {
		return "player"
	}
	return s
}

// GenerateUniqueUsername derives a unique username from one or more candidate
// strings (tried in order) using the given querier (a *sql.DB or *sql.Tx via
// rowQuerier). The first candidate that normalizes to a valid, unused username
// wins; on collision a numeric suffix is appended (alice, alice_2, alice_3, ...)
// with the base truncated so the result stays within usernameMaxLen.
func GenerateUniqueUsername(q rowQuerier, candidates ...string) (string, error) {
	base := "player"
	for _, c := range candidates {
		if cand := usernameFromCandidate(c); cand != "player" {
			base = cand
			break
		}
	}

	for attempt := 1; attempt <= 10000; attempt++ {
		candidate := base
		if attempt > 1 {
			suffix := fmt.Sprintf("_%d", attempt)
			maxBase := usernameMaxLen - len(suffix)
			b := base
			if len(b) > maxBase {
				b = strings.Trim(b[:maxBase], "_")
				if len(b) == 0 {
					b = "player"
				}
			}
			candidate = b + suffix
		}

		var exists bool
		if err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, candidate).Scan(&exists); err != nil {
			return "", fmt.Errorf("check username: %w", err)
		}
		if !exists {
			return candidate, nil
		}
	}
	// Astronomically unlikely; fall back to a random handle.
	return "player_" + strings.ReplaceAll(uuid.New().String()[:8], "-", ""), nil
}

// rowQuerier is satisfied by both *sql.DB and *sql.Tx, letting username
// generation run inside or outside a transaction.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}
