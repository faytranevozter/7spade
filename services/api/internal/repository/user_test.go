package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

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
