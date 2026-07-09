package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// SendFriendRequest auto-accepts when the addressee already has a pending
// request to the caller (both intentions present).
func TestSendFriendRequestAutoAcceptsReverse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	other := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtext($1))")).
		WithArgs(friendshipPairKey(me, other)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// No block in either direction.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM friendships")).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// Reverse pending row exists -> UPDATE affects 1 row.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE friendships SET status = 'accepted'")).
		WithArgs(other, me).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	status, err := SendFriendRequest(db, me, other)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if status != FriendshipAccepted {
		t.Fatalf("status = %q, want accepted", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// SendFriendRequest is idempotent when the two are already friends and the
// caller was the original addressee: the reverse accepted row is re-settled
// rather than creating a duplicate forward row (regression for review finding).
func TestSendFriendRequestIdempotentWhenAlreadyFriends(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	other := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtext($1))")).
		WithArgs(friendshipPairKey(me, other)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM friendships")).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// A reverse ACCEPTED row (other -> me) exists; the widened UPDATE matches it
	// and returns 1, so no forward row is inserted.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE friendships SET status = 'accepted'")).
		WithArgs(other, me).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	status, err := SendFriendRequest(db, me, other)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if status != FriendshipAccepted {
		t.Fatalf("status = %q, want accepted", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// SendFriendRequest creates a pending row when there's no reverse request.
func TestSendFriendRequestCreatesPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	other := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtext($1))")).
		WithArgs(friendshipPairKey(me, other)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM friendships")).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// No reverse pending row -> UPDATE affects 0 rows.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE friendships SET status = 'accepted'")).
		WithArgs(other, me).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO friendships")).
		WithArgs(me, other).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM friendships")).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))
	mock.ExpectCommit()

	status, err := SendFriendRequest(db, me, other)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if status != FriendshipPending {
		t.Fatalf("status = %q, want pending", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// SendFriendRequest rejects a self-request before touching the DB.
func TestSendFriendRequestSelfRejected(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	if _, err := SendFriendRequest(db, me, me); err != ErrFriendshipSelf {
		t.Fatalf("err = %v, want ErrFriendshipSelf", err)
	}
}

// SendFriendRequest rejects when a block exists in either direction.
func TestSendFriendRequestBlocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	other := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtext($1))")).
		WithArgs(friendshipPairKey(me, other)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM friendships")).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	if _, err := SendFriendRequest(db, me, other); err != ErrFriendshipBlocked {
		t.Fatalf("err = %v, want ErrFriendshipBlocked", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// AcceptFriendRequest reports whether a pending incoming row was updated.
func TestAcceptFriendRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	other := uuid.New()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE friendships SET status = 'accepted'")).
		WithArgs(other, me).
		WillReturnResult(sqlmock.NewResult(0, 1))

	accepted, err := AcceptFriendRequest(db, me, other)
	if err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	if !accepted {
		t.Fatal("accepted = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// ListFriends maps rows to entries with avatar + direction.
func TestListFriends(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	me := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("FROM friendships f")).
		WithArgs(me).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "username", "avatar_url", "direction"}).
			AddRow("u1", "Alice", "alice", "https://cdn/a.png", "accepted").
			AddRow("u2", "Bob", "bob", nil, "incoming"))

	entries, err := ListFriends(db, me)
	if err != nil {
		t.Fatalf("ListFriends: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].DisplayName != "Alice" || entries[0].Username != "alice" || entries[0].Status != "accepted" {
		t.Fatalf("entry[0] = %+v", entries[0])
	}
	if entries[0].AvatarURL == nil || *entries[0].AvatarURL != "https://cdn/a.png" {
		t.Fatalf("entry[0].AvatarURL = %v", entries[0].AvatarURL)
	}
	if entries[1].AvatarURL != nil || entries[1].Username != "bob" || entries[1].Status != "incoming" {
		t.Fatalf("entry[1] = %+v", entries[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
