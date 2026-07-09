package repository

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestCreateUserMapsDuplicateEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(sqlmock.AnyArg(), "alice@example.com", "hash", "Alice", "alice", sqlmock.AnyArg()).
		WillReturnError(errors.New(`pq: duplicate key value violates unique constraint "users_email_key"`))

	_, err = CreateUser(db, "alice@example.com", "hash", "Alice", "alice")
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("CreateUser err = %v, want ErrEmailTaken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// SearchUsers runs the ILIKE search with relevance ordering, excludes the caller
// and blocked relationships, maps rows (incl. null avatar), and caps at limit.
func TestSearchUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	caller := uuid.New()
	a := uuid.New()
	b := uuid.New()
	mock.ExpectQuery("FROM users u").
		WithArgs(caller, "%ali%", "ali%", "ali", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "avatar_url"}).
			AddRow(a.String(), "alice", "Alice", "https://cdn/a.png").
			AddRow(b.String(), "alicia", "Alicia", nil))

	results, err := SearchUsers(db, "ali", caller, 20)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].UserID != a.String() || results[0].Username != "alice" || results[0].DisplayName != "Alice" {
		t.Fatalf("result[0] = %+v", results[0])
	}
	if results[0].AvatarURL == nil || *results[0].AvatarURL != "https://cdn/a.png" {
		t.Fatalf("result[0].AvatarURL = %v", results[0].AvatarURL)
	}
	if results[1].AvatarURL != nil {
		t.Fatalf("result[1].AvatarURL = %v, want nil", results[1].AvatarURL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// SearchUsers escapes LIKE wildcards in the query so they match literally; the
// bound contains/prefix args carry the escaped text.
func TestSearchUsersEscapesWildcards(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	caller := uuid.New()
	// Input "50%_off\x" -> escaped "50\%\_off\\x".
	escaped := `50\%\_off\\x`
	mock.ExpectQuery("FROM users u").
		WithArgs(caller, "%"+escaped+"%", escaped+"%", `50%_off\x`, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "avatar_url"}))

	if _, err := SearchUsers(db, `50%_off\x`, caller, 20); err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// The query filters out blocked relationships in both directions.
func TestSearchUsersExcludesBlocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	caller := uuid.New()
	// Assert the blocked-exclusion clause is present in the SQL.
	mock.ExpectQuery(regexp.QuoteMeta("f.status = 'blocked'")).
		WithArgs(caller, "%bob%", "bob%", "bob", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "avatar_url"}))

	if _, err := SearchUsers(db, "bob", caller, 20); err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// UpsertOAuthUser stamps email_verified_at when linking an existing OAuth account
// so social logins are treated as verified. The UPDATE uses COALESCE so an
// already-set verification timestamp is preserved.
func TestUpsertOAuthUserExistingMarksVerified(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	userID := uuid.New()
	profile := OAuthProfile{
		Provider:       "google",
		ProviderUserID: "g-123",
		Email:          "Alice@Example.com",
		DisplayName:    "Alice",
		AvatarURL:      "https://cdn/a.png",
	}

	mock.ExpectBegin()
	// Provider link already exists -> existing-user path.
	mock.ExpectQuery("SELECT user_id FROM user_providers").
		WithArgs("google", "g-123").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))
	// The UPDATE backfills email_verified_at via COALESCE.
	mock.ExpectQuery(regexp.QuoteMeta("email_verified_at = COALESCE(email_verified_at, NOW())")).
		WithArgs("Alice", userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "username", "created_at"}).
			AddRow(userID, "alice@example.com", nil, "Alice", "alice", time.Now()))
	mock.ExpectExec("INSERT INTO user_providers").
		WithArgs(userID, "google", "g-123", "alice@example.com", "https://cdn/a.png").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	user, err := UpsertOAuthUser(db, profile)
	if err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("user.ID = %v, want %v", user.ID, userID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// A new OAuth user is inserted with email_verified_at set, so the verify-email
// banner never shows for social signups.
func TestUpsertOAuthUserNewMarksVerified(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	profile := OAuthProfile{
		Provider:       "github",
		ProviderUserID: "gh-7",
		Email:          "bob@example.com",
		DisplayName:    "Bob",
		Username:       "bob",
	}

	mock.ExpectBegin()
	// No provider link...
	mock.ExpectQuery("SELECT user_id FROM user_providers").
		WithArgs("github", "gh-7").
		WillReturnError(sql.ErrNoRows)
	// ...and no user with that email -> new-user path.
	mock.ExpectQuery("SELECT id FROM users WHERE email").
		WithArgs("bob@example.com").
		WillReturnError(sql.ErrNoRows)
	// Username uniqueness probe (GenerateUniqueUsername).
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("bob").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("SAVEPOINT oauth_user_insert").
		WillReturnResult(sqlmock.NewResult(0, 0))
	// The INSERT includes email_verified_at.
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users (id, email, display_name, username, email_verified_at, created_at)")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "username", "created_at"}).
			AddRow(uuid.New(), "bob@example.com", nil, "Bob", "bob", time.Now()))
	mock.ExpectExec("RELEASE SAVEPOINT oauth_user_insert").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_providers").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := UpsertOAuthUser(db, profile); err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
