package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Me rejects guests and tokenless requests before touching the database, so
// those paths can be exercised with DB: nil.
func TestStatsMeRejectsNonRegisteredUsers(t *testing.T) {
	cases := []struct {
		name    string
		claims  *auth.Claims
		setNil  bool
		wantMsg string
	}{
		{name: "no claims", setNil: true, wantMsg: "Authentication required"},
		{name: "guest", claims: &auth.Claims{Sub: uuid.NewString(), IsGuest: true}, wantMsg: "Logged-in user required"},
		{name: "bad sub", claims: &auth.Claims{Sub: "not-a-uuid"}, wantMsg: "Logged-in user required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := StatsHandler{DB: nil, MinGames: 5}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/stats", nil)
			if !tc.setNil {
				c.Set(middleware.ClaimsKey, tc.claims)
			}

			h.Me(c)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			assertErrorBody(t, w, tc.wantMsg)
		})
	}
}

// User validates the path param before touching the database.
func TestStatsUserRejectsInvalidUUID(t *testing.T) {
	h := StatsHandler{DB: nil, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/users/not-a-uuid/stats", nil)

	h.User(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertErrorBody(t, w, "Invalid user ID")
}

// User returns 404 when the user has no user_stats row.
func TestStatsUserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	h := StatsHandler{DB: db, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/users/"+id.String()+"/stats", nil)

	h.User(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	assertErrorBody(t, w, "Player not found")
}

// Me caps per_page at 50 and returns zeroed stats when the user has no row.
func TestStatsMeZeroedWhenNoRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_stats").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	h := StatsHandler{DB: db, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String(), DisplayName: "Alice"})
	c.Request = httptest.NewRequest(http.MethodGet, "/stats", nil)

	h.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body struct {
		UserID      string      `json:"user_id"`
		DisplayName string      `json:"display_name"`
		GamesPlayed int         `json:"games_played"`
		BestPenalty *int        `json:"best_penalty"`
		Rank        *int        `json:"rank"`
		Qualified   bool        `json:"qualified"`
		Extra       interface{} `json:"-"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.UserID != id.String() || body.DisplayName != "Alice" {
		t.Fatalf("identity = %q/%q", body.UserID, body.DisplayName)
	}
	if body.GamesPlayed != 0 || body.BestPenalty != nil || body.Rank != nil || body.Qualified {
		t.Fatalf("expected zeroed stats, got games=%d best=%v rank=%v qualified=%v", body.GamesPlayed, body.BestPenalty, body.Rank, body.Qualified)
	}
}

func assertErrorBody(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != want {
		t.Fatalf("error = %q, want %q", body.Error, want)
	}
}
