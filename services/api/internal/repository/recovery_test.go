package repository

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestUpdatePasswordHash(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET password_hash = $1 WHERE id = $2")).
		WithArgs("new-hash", id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := UpdatePasswordHash(db, id, "new-hash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdatePasswordHashNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET password_hash = $1 WHERE id = $2")).
		WithArgs("new-hash", id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := UpdatePasswordHash(db, id, "new-hash"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMarkEmailVerified(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET email_verified_at = NOW() WHERE id = $1")).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := MarkEmailVerified(db, id); err != nil {
		t.Fatalf("MarkEmailVerified: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRevokeAllRefreshTokensForUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM refresh_tokens WHERE user_id = $1")).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 3))

	if err := RevokeAllRefreshTokensForUser(db, id); err != nil {
		t.Fatalf("RevokeAllRefreshTokensForUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
