package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// Achievements validates the path param before touching the database.
func TestStatsAchievementsRejectsInvalidUUID(t *testing.T) {
	h := StatsHandler{DB: nil, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/users/not-a-uuid/achievements", nil)

	h.Achievements(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertErrorBody(t, w, "Invalid user ID")
}

// Achievements returns earned badges plus the catalog for a valid user id.
func TestStatsAchievementsReturnsEarnedAndCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("FROM user_achievements").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"achievement_id", "earned_at"}))
	mock.ExpectQuery("FROM achievements").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "icon"}).
			AddRow("first_win", "First Blood", "Win your first game", "🏆"))

	h := StatsHandler{DB: db, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/users/"+id.String()+"/achievements", nil)

	h.Achievements(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body struct {
		Earned  []map[string]any `json:"earned"`
		Catalog []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Earned == nil {
		t.Fatal("expected earned to be a (possibly empty) array, got null")
	}
	if len(body.Catalog) != 1 || body.Catalog[0].ID != "first_win" || body.Catalog[0].Name == "" {
		t.Fatalf("expected catalog metadata, got %+v", body.Catalog)
	}
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

// Leaderboard resolves ?season=active to the open season id, scopes the query
// to season_user_stats, and echoes the season id back in the response.
func TestStatsLeaderboardSeasonActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// resolveSeason -> EnsureActiveSeason fast path (open season is current month).
	current := time.Now().UTC().Format("2006-01")
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow(current, "Current", "2026-06-01T00:00:00Z", nil))
	// Season-scoped leaderboard: count then page against season_user_stats.
	mock.ExpectQuery("FROM season_user_stats").
		WithArgs(current, 5).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("FROM season_user_stats").
		WithArgs(current, 5, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"rank", "user_id", "display_name", "avatar_url", "games_played", "wins", "win_rate", "avg_penalty", "best_penalty", "rating", "avg_rank", "top2_rate", "first_place_count", "human_only_games", "bot_mixed_games"}))

	h := StatsHandler{DB: db, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/leaderboard?season=active&sort=rating", nil)

	h.Leaderboard(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body struct {
		Season string `json:"season"`
		Sort   string `json:"sort"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Season != current {
		t.Fatalf("season = %q, want %q", body.Season, current)
	}
	if body.Sort != "rating" {
		t.Fatalf("sort = %q, want rating", body.Sort)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// Seasons ensures the current season exists then lists seasons newest-first.
func TestStatsSeasons(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	current := time.Now().UTC().Format("2006-01")
	// EnsureActiveSeason fast path.
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow(current, "Current", "2026-06-01T00:00:00Z", nil))
	// ListSeasons.
	mock.ExpectQuery("FROM seasons").
		WillReturnRows(sqlmock.NewRows([]string{"id", "label", "started_at", "ended_at"}).
			AddRow(current, "June 2026", "2026-06-01T00:00:00Z", nil).
			AddRow("2026-05", "May 2026", "2026-05-01T00:00:00Z", "2026-06-01T00:00:00Z"))

	h := StatsHandler{DB: db, MinGames: 5}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/seasons", nil)

	h.Seasons(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body struct {
		Seasons []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"seasons"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Seasons) != 2 || !body.Seasons[0].Active || body.Seasons[1].Active {
		t.Fatalf("seasons = %+v, want current active first", body.Seasons)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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
