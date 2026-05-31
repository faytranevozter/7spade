package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeUsername(t *testing.T) {
	cases := map[string]string{
		"  Alice ":   "alice",
		"CardShark":  "cardshark",
		"BOB_99":     "bob_99",
		"":           "",
		"   ":        "",
		"MixedCase_": "mixedcase_",
	}
	for in, want := range cases {
		if got := NormalizeUsername(in); got != want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"abc", "alice", "bob_99", "a_b_c", "x__y", "user_name_123", "aaa", "abcdefghijklmnopqrstuvwxyz012345"}
	for _, u := range valid {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", u, err)
		}
	}
	invalid := []string{"", "ab", "Alice", "has space", "emoji😀", "with-dash", "dot.dot", "ab", "thisusernameiswaytoolongtobevalid_xx"}
	for _, u := range invalid {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want error", u)
		}
	}
}

func TestUsernameFromCandidate(t *testing.T) {
	cases := map[string]string{
		"Alice":            "alice",
		"Card Shark":       "card_shark",
		"  spaced  ":       "spaced",
		"@@@":              "player",
		"a":                "player",
		"José Ramírez":     "jos_ram_rez",
		"user.name@x.com":  "user_name_x_com",
	}
	for in, want := range cases {
		if got := usernameFromCandidate(in); got != want {
			t.Errorf("usernameFromCandidate(%q) = %q, want %q", in, got, want)
		}
	}
}

// GenerateUniqueUsername returns the base when it's free.
func TestGenerateUniqueUsernameFree(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	got, err := GenerateUniqueUsername(db, "Alice")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got != "alice" {
		t.Fatalf("got %q, want alice", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GenerateUniqueUsername appends a numeric suffix on collision.
func TestGenerateUniqueUsernameCollision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("alice_2").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	got, err := GenerateUniqueUsername(db, "Alice")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got != "alice_2" {
		t.Fatalf("got %q, want alice_2", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// GenerateUniqueUsername falls through invalid candidates to the next usable one.
func TestGenerateUniqueUsernameSkipsInvalidCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// First candidate "@@" -> "player" (skipped as a base preference); second
	// candidate "Bob" -> "bob" is used.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)")).
		WithArgs("bob").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	got, err := GenerateUniqueUsername(db, "@@", "Bob")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got != "bob" {
		t.Fatalf("got %q, want bob", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
