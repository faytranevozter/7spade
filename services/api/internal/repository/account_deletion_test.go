package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestScheduleAccountDeletionNew(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	at := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NOW()
		WHERE id = $1 AND deletion_scheduled_at IS NULL
		RETURNING deletion_scheduled_at
	`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(at))

	got, newly, err := ScheduleAccountDeletion(db, id)
	if err != nil {
		t.Fatalf("ScheduleAccountDeletion: %v", err)
	}
	if !newly || !got.Equal(at) {
		t.Fatalf("got=%v newly=%v want=%v", got, newly, at)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestScheduleAccountDeletionIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	at := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(at))

	got, newly, err := ScheduleAccountDeletion(db, id)
	if err != nil {
		t.Fatalf("ScheduleAccountDeletion: %v", err)
	}
	if newly || !got.Equal(at) {
		t.Fatalf("got=%v newly=%v", got, newly)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestCancelAccountDeletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE users SET deletion_scheduled_at = NULL
		WHERE id = $1 AND deletion_scheduled_at IS NOT NULL
	`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ok, err := CancelAccountDeletion(db, id)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestListUsersDueForDeletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	a := uuid.New()
	b := uuid.New()
	mock.ExpectQuery("SELECT id FROM users").
		WithArgs(sqlmock.AnyArg(), 50).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(a).AddRow(b))

	ids, err := ListUsersDueForDeletion(db, AccountDeletionGracePeriod, 50)
	if err != nil {
		t.Fatalf("ListUsersDueForDeletion: %v", err)
	}
	if len(ids) != 2 || ids[0] != a || ids[1] != b {
		t.Fatalf("ids = %v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestFinalizeAccountDeletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	scheduled := time.Now().Add(-8 * 24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1 FOR UPDATE`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(scheduled))
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE game_players SET display_name = $1 WHERE user_id = $2
	`)).
		WithArgs(DeletedUserDisplayName, id).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM room_players WHERE user_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM room_kicked_players WHERE user_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE game_result_details SET subject_id = NULL
		WHERE subject_id = $1
	`)).
		WithArgs(id.String()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM users WHERE id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ok, err := FinalizeAccountDeletion(db, id)
	if err != nil {
		t.Fatalf("FinalizeAccountDeletion: %v", err)
	}
	if !ok {
		t.Fatal("expected finalized=true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestFinalizeAccountDeletionNotPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT deletion_scheduled_at FROM users WHERE id = $1 FOR UPDATE`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"deletion_scheduled_at"}).AddRow(nil))
	mock.ExpectRollback()

	ok, err := FinalizeAccountDeletion(db, id)
	if err != nil {
		t.Fatalf("FinalizeAccountDeletion: %v", err)
	}
	if ok {
		t.Fatal("expected finalized=false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
