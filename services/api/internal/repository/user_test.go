package repository

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

// UpsertOAuthUser regenerates the username and retries when the insert collides
// with a concurrently-created row (the EXISTS check and INSERT aren't atomic).
// The failed insert is isolated by a savepoint so the surrounding transaction
// survives.
func TestUpsertOAuthUserRetriesOnUsernameCollision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	// New user: no provider link, and no email to fall back on.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id FROM user_providers WHERE provider = $1 AND provider_id = $2")).
		WithArgs("github", "123").
		WillReturnError(sql.ErrNoRows)

	existsRows := func(v bool) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"exists"}).AddRow(v)
	}
	userCols := []string{"id", "email", "password_hash", "display_name", "username", "created_at"}

	// Attempt 1: base "alice" is free per EXISTS, but the INSERT races and hits
	// the unique index -> rollback to savepoint.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice").WillReturnRows(existsRows(false))
	mock.ExpectExec(regexp.QuoteMeta("SAVEPOINT oauth_user_insert")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Alice", "alice").
		WillReturnError(&pq.Error{Code: "23505", Constraint: "idx_users_username"})
	mock.ExpectExec(regexp.QuoteMeta("ROLLBACK TO SAVEPOINT oauth_user_insert")).WillReturnResult(sqlmock.NewResult(0, 0))

	// Attempt 2: "alice" now reads as taken, so "alice_2" is generated and the
	// insert succeeds.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice").WillReturnRows(existsRows(true))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice_2").WillReturnRows(existsRows(false))
	mock.ExpectExec(regexp.QuoteMeta("SAVEPOINT oauth_user_insert")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Alice", "alice_2").
		WillReturnRows(sqlmock.NewRows(userCols).AddRow("11111111-1111-1111-1111-111111111111", nil, nil, "Alice", "alice_2", time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("RELEASE SAVEPOINT oauth_user_insert")).WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_providers")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	user, err := UpsertOAuthUser(db, OAuthProfile{Provider: "github", ProviderUserID: "123", DisplayName: "Alice"})
	if err != nil {
		t.Fatalf("UpsertOAuthUser: %v", err)
	}
	if user.Username != "alice_2" {
		t.Fatalf("username = %q, want alice_2", user.Username)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
