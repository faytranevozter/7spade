package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSavePendingMFAAtomicallyProtectsVerifiedMethod(t *testing.T) {
	dependencyErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name    string
		rows    int64
		execErr error
		want    error
	}{
		{"insert or replace pending", 1, nil, nil},
		{"verified conflict", 0, nil, ErrConflict},
		{"dependency error", 0, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			query := `INSERT INTO admin_mfa_methods (admin_user_id, method_type, secret_ciphertext) VALUES ($1, 'totp', $2) ON CONFLICT (admin_user_id) DO UPDATE SET secret_ciphertext = EXCLUDED.secret_ciphertext WHERE admin_mfa_methods.verified_at IS NULL`
			expect := mock.ExpectExec(regexp.QuoteMeta(query)).WithArgs("admin", []byte("secret"))
			if tc.execErr != nil {
				expect.WillReturnError(tc.execErr)
			} else {
				expect.WillReturnResult(sqlmock.NewResult(0, tc.rows))
			}
			if tc.want == nil {
				mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = NewPostgresStore(db, "test").SavePendingMFA(context.Background(), "admin", []byte("secret"), AuditEvent{Action: "admin.mfa.enroll", Outcome: "success"})
			if !errors.Is(err, tc.want) {
				t.Errorf("SavePendingMFA = %v, want %v", err, tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSavePendingMFARollsBackWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO admin_mfa_methods").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	err = NewPostgresStore(db, "test").SavePendingMFA(context.Background(), "admin", []byte("secret"), AuditEvent{Action: "admin.mfa.enroll", Outcome: "success"})
	if err == nil {
		t.Fatal("SavePendingMFA succeeded without its audit event")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmMFARollsBackWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE admin_mfa_methods").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM admin_recovery_codes").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO admin_recovery_codes").WithArgs("admin", "recovery-hash").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	err = NewPostgresStore(db, "test").ConfirmMFA(context.Background(), "admin", []string{"recovery-hash"}, AuditEvent{Action: "admin.mfa.confirm", Outcome: "success"})
	if err == nil {
		t.Fatal("ConfirmMFA succeeded without its audit event")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
