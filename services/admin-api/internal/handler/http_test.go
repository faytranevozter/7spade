package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminAuthenticationAndAuthorization(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", SecureCookies: true}, store)

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body=%s", login.Code, login.Body.String())
	}
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken == "" || auth.Admin.Email != "ops@example.com" {
		t.Fatalf("unexpected login response: %+v", auth)
	}
	cookies := login.Result().Cookies()
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/auth" {
		t.Fatalf("insecure refresh cookie: %+v", cookie)
	}
	csrf := cookies[1]
	if csrf.HttpOnly || !csrf.Secure || csrf.SameSite != http.SameSiteStrictMode || csrf.Path != "/" {
		t.Fatalf("invalid CSRF cookie: %+v", csrf)
	}

	dashboard := request(t, router, http.MethodGet, "/dashboard", "", auth.AccessToken)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body=%s", dashboard.Code, dashboard.Body.String())
	}
	var result Dashboard
	if err := json.Unmarshal(dashboard.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Environment != "development" || result.Windows.Day.From.IsZero() || result.Windows.Month.To.IsZero() {
		t.Fatalf("dashboard did not expose explicit activity windows: %+v", result)
	}
	if result.Services.API.Status != "ok" || result.Services.WS.Status != "not_configured" {
		t.Fatalf("unexpected service health: %+v", result.Services)
	}

	store.SetPermissions("admin-1", nil)
	forbidden := request(t, router, http.MethodGet, "/dashboard", "", auth.AccessToken)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("dashboard without permission = %d", forbidden.Code)
	}
}

func TestDailyLoginSettingCanBeReadAndUpdatedWithAudit(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "operator", Email: "operator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"settings.read", "settings.write"}}
	store := NewMemoryStore(admin)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"operator@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodGet, "/settings/daily-login", "", auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":true`) {
		t.Fatalf("default daily login setting = %d %s", response.Code, response.Body.String())
	}

	response = request(t, router, http.MethodPut, "/settings/daily-login", `{"enabled":false,"reason":"Pause rewards during maintenance"}`, auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) {
		t.Fatalf("updated daily login setting = %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodGet, "/settings/daily-login", "", auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) {
		t.Fatalf("persisted daily login setting = %d %s", response.Code, response.Body.String())
	}

	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.Action != "setting.daily_login.update" || last.ResourceType != "feature_setting" || last.ResourceID != "daily_login" || last.Reason != "Pause rewards during maintenance" || string(last.BeforeState) != `{"enabled":true}` || string(last.AfterState) != `{"enabled":false}` {
		t.Fatalf("daily login setting audit = %+v", last)
	}
}

func TestApplicationSettingsCanBeListedAndUpdated(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "operator", Email: "operator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"settings.read", "settings.write"}}
	store := NewMemoryStore(admin)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"operator@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodGet, "/settings", "", auth.AccessToken)
	var settings []model.FeatureSetting
	if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"daily_login", "emotes", "guest_access", "new_game_starts", "new_registrations", "quick_play", "room_creation", "spectator_access"}
	gotKeys := make([]string, 0, len(settings))
	for _, setting := range settings {
		gotKeys = append(gotKeys, setting.Key)
		if !setting.Enabled {
			t.Fatalf("setting %q should be enabled by default", setting.Key)
		}
	}
	if response.Code != http.StatusOK || fmt.Sprint(gotKeys) != fmt.Sprint(wantKeys) {
		t.Fatalf("settings list = %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPut, "/settings/new_game_starts", `{"enabled":false,"reason":"Maintenance"}`, auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) {
		t.Fatalf("settings update = %d %s", response.Code, response.Body.String())
	}
	response = request(t, router, http.MethodPut, "/settings/unknown", `{"enabled":false,"reason":"Maintenance"}`, auth.AccessToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown settings update = %d %s", response.Code, response.Body.String())
	}
}

func TestDailyLoginSettingUsesSeparateReadAndWritePermissions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"settings.read"}}
	writer := Admin{ID: "writer", Email: "writer@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"settings.write"}}
	store := NewMemoryStore(reader, writer)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := func(email string) string {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth.AccessToken
	}

	if response := request(t, router, http.MethodPut, "/settings/daily-login", `{"enabled":false,"reason":"test"}`, login(reader.Email)); response.Code != http.StatusForbidden {
		t.Fatalf("reader update status = %d", response.Code)
	}
	if response := request(t, router, http.MethodGet, "/settings/daily-login", "", login(writer.Email)); response.Code != http.StatusForbidden {
		t.Fatalf("writer read status = %d", response.Code)
	}
}

func TestUserInvestigationSearchesRedactsAndPaginates(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"users.read"}}
	sensitiveReader := Admin{ID: "sensitive", Email: "sensitive@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"users.read", "users.sensitive.read"}}
	store := NewMemoryStore(reader, sensitiveReader)
	firstID := "00000000-0000-0000-0000-000000000001"
	secondID := "00000000-0000-0000-0000-000000000002"
	store.SetUsers(
		UserDetail{User: User{ID: firstID, Username: "ace", DisplayName: "Ace Player", Email: "ace@example.com", CreatedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Online: true}, Providers: []string{"google"}, Stats: map[string]any{"xp": int64(250)}},
		UserDetail{User: User{ID: secondID, Username: "king", DisplayName: "King Player", Email: "king@example.com", CreatedAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}},
	)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := func(email string) AuthResponse {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth
	}

	plain := login("reader@example.com")
	response := request(t, router, http.MethodGet, "/users", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated users = %d", response.Code)
	}
	response = request(t, router, http.MethodGet, "/users?query=ace&limit=1", "", plain.AccessToken)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "ace@example.com") || !strings.Contains(response.Body.String(), `"online":true`) {
		t.Fatalf("redacted search = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users?limit=1", "", plain.AccessToken); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), secondID) {
		t.Fatalf("stable first page = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users?limit=1&offset=1", "", plain.AccessToken); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), firstID) {
		t.Fatalf("stable second page = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users/"+firstID, "", plain.AccessToken); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "ace@example.com") || !strings.Contains(response.Body.String(), `"xp":250`) {
		t.Fatalf("redacted detail = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users?query=ace@example.com", "", plain.AccessToken); response.Code != http.StatusOK || strings.Contains(response.Body.String(), firstID) {
		t.Fatalf("email search without permission = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users?limit=0", "", plain.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid pagination = %d", response.Code)
	}

	sensitive := login("sensitive@example.com")
	if response = request(t, router, http.MethodGet, "/users?query=ace@example.com", "", sensitive.AccessToken); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ace@example.com") {
		t.Fatalf("sensitive search = %d %s", response.Code, response.Body.String())
	}
	if response = request(t, router, http.MethodGet, "/users/00000000-0000-0000-0000-000000000099", "", sensitive.AccessToken); response.Code != http.StatusNotFound {
		t.Fatalf("missing user = %d", response.Code)
	}
	if response = request(t, router, http.MethodGet, "/users/not-a-uuid", "", sensitive.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid user ID = %d", response.Code)
	}
	if store.AuditEvents()[len(store.AuditEvents())-1].Action != "users.search.sensitive" {
		t.Fatalf("sensitive search was not audited")
	}
}

func TestGameInvestigationSearchDetailAndAnnotations(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"games.read", "games.annotate"}}
	store := NewMemoryStore(reader)
	gameID := "10000000-0000-0000-0000-000000000001"
	roomID := "20000000-0000-0000-0000-000000000001"
	playerID := "30000000-0000-0000-0000-000000000001"
	finished := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store.SetGames(GameDetail{Game: Game{ID: gameID, RoomID: roomID, RoomName: "Practice table", Mode: "classic", SeasonID: "2026-06", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: &finished, ReplayAvailable: true}, Players: []GamePlayer{{UserID: playerID, DisplayName: "Ace", PenaltyPoints: 0, Rank: 1, IsWinner: true, Team: intPtr(0), FaceDownCards: []GameCard{{Suit: "spades", Rank: 7, Points: 1}}}}, Moves: []GameMove{{Index: 0, PlayerIndex: 0, Suit: "spades", Rank: 7, Type: "play"}}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	path := "/games?id=" + gameID + "&room_id=" + roomID + "&player_id=" + playerID + "&mode=classic&season_id=2026-06&completion=completed&finished_from=2026-08-20T00:00:00Z&finished_to=2026-08-21T00:00:00Z&limit=1&offset=0"
	response := request(t, router, http.MethodGet, path, "", auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"game_id":"`+gameID+`"`) {
		t.Fatalf("game search = %d %s", response.Code, response.Body.String())
	}
	detail := request(t, router, http.MethodGet, "/games/"+gameID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"display_name":"Ace"`) || !strings.Contains(detail.Body.String(), `"replay_available":true`) || !strings.Contains(detail.Body.String(), `"facedown_cards"`) || !strings.Contains(detail.Body.String(), `"type":"play"`) {
		t.Fatalf("game detail = %d %s", detail.Code, detail.Body.String())
	}
	flag := request(t, router, http.MethodPost, "/games/"+gameID+"/flags", `{"reason":"reported result"}`, auth.AccessToken)
	if flag.Code != http.StatusCreated || !strings.Contains(flag.Body.String(), `"reason":"reported result"`) {
		t.Fatalf("game flag = %d %s", flag.Code, flag.Body.String())
	}
	note := request(t, router, http.MethodPost, "/games/"+gameID+"/notes", `{"reason":"reviewed replay","body":"No issue found."}`, auth.AccessToken)
	if note.Code != http.StatusCreated || !strings.Contains(note.Body.String(), `"body":"No issue found."`) {
		t.Fatalf("game note = %d %s", note.Code, note.Body.String())
	}
	detail = request(t, router, http.MethodGet, "/games/"+gameID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"flags":[`) || !strings.Contains(detail.Body.String(), `"notes":[`) || !strings.Contains(detail.Body.String(), `"rank":1`) || !strings.Contains(detail.Body.String(), `"penalty_points":0`) {
		t.Fatalf("annotated game detail = %d %s", detail.Code, detail.Body.String())
	}
	missingReplayID := "10000000-0000-0000-0000-000000000002"
	store.SetGames(GameDetail{Game: Game{ID: missingReplayID, RoomID: roomID, Mode: "classic", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: &finished}, Players: []GamePlayer{{DisplayName: "Robo", PenaltyPoints: 8, Rank: 2, IsWinner: false, IsBot: true}}})
	missing := request(t, router, http.MethodGet, "/games/"+missingReplayID, "", auth.AccessToken)
	if missing.Code != http.StatusOK || !strings.Contains(missing.Body.String(), `"replay_available":false`) {
		t.Fatalf("missing replay detail = %d %s", missing.Code, missing.Body.String())
	}
	if strings.Contains(missing.Body.String(), `"user_id"`) {
		t.Fatalf("bot account identity leaked in detail: %s", missing.Body.String())
	}
	search := request(t, router, http.MethodGet, "/games?limit=1", "", auth.AccessToken)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"total":2`) || strings.Contains(search.Body.String(), `"user_id"`) {
		t.Fatalf("redacted search = %d %s", search.Code, search.Body.String())
	}
	dateRange := request(t, router, http.MethodGet, "/games?finished_from=2026-08-20&finished_to=2026-08-21", "", auth.AccessToken)
	if dateRange.Code != http.StatusOK || !strings.Contains(dateRange.Body.String(), `"game_id":"`+gameID+`"`) {
		t.Fatalf("date-range search = %d %s", dateRange.Code, dateRange.Body.String())
	}
	season := request(t, router, http.MethodGet, "/games?season_id=2026-06", "", auth.AccessToken)
	if season.Code != http.StatusOK {
		t.Fatalf("season search = %d %s", season.Code, season.Body.String())
	}
	for _, invalid := range []string{"/games?completion=incomplete", "/games?completion=abandoned", "/games?limit=0", "/games?id=bad", "/games?season_id=bad", "/games?finished_from=bad", "/games?finished_from=2026-08-22T00:00:00Z&finished_to=2026-08-21T00:00:00Z"} {
		if got := request(t, router, http.MethodGet, invalid, "", auth.AccessToken); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid game filter %s = %d %s", invalid, got.Code, got.Body.String())
		}
	}
}

type failingGameFlagStore struct{ Store }

func (s failingGameFlagStore) FlagGame(context.Context, string, string, AuditEvent) (GameFlag, error) {
	return GameFlag{}, errors.New("database unavailable")
}

func TestGameFlagFailsClosedWhenAnnotationCannotPersist(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "operator", Email: "operator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"games.annotate"}}
	store := NewMemoryStore(admin)
	gameID := "10000000-0000-0000-0000-000000000001"
	finished := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store.SetGames(GameDetail{Game: Game{ID: gameID, RoomID: "room-1", Mode: "classic", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: &finished}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, failingGameFlagStore{Store: store})
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"operator@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)
	response := request(t, router, http.MethodPost, "/games/"+gameID+"/flags", `{"reason":"reported result"}`, auth.AccessToken)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("annotation failure status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGameInvestigationRequiresAnnotationPermission(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"games.read"}}
	store := NewMemoryStore(reader)
	gameID := "10000000-0000-0000-0000-000000000001"
	finished := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store.SetGames(GameDetail{Game: Game{ID: gameID, RoomID: "room-1", Mode: "classic", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: &finished}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)
	flag := request(t, router, http.MethodPost, "/games/"+gameID+"/flags", `{"reason":"reported result"}`, auth.AccessToken)
	if flag.Code != http.StatusForbidden {
		t.Fatalf("flag without permission = %d %s", flag.Code, flag.Body.String())
	}
	detail := request(t, router, http.MethodGet, "/games/"+gameID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"flags":[]`) || !strings.Contains(detail.Body.String(), `"notes":[]`) {
		t.Fatalf("game detail unchanged after denied flag = %d %s", detail.Code, detail.Body.String())
	}
	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.Action != "permission.denied" || last.Outcome != "rejected" || !strings.Contains(string(last.Metadata), `"permission":"games.annotate"`) {
		t.Fatalf("denied annotation audit = %+v", last)
	}
}

func TestGameConcurrentAnnotationsPreserveOriginalGameAndResult(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "operator", Email: "operator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"games.annotate", "games.read"}}
	store := NewMemoryStore(admin)
	gameID := "10000000-0000-0000-0000-000000000001"
	finished := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store.SetGames(GameDetail{Game: Game{ID: gameID, RoomID: "room-1", RoomName: "Practice table", Mode: "classic", StartedAt: finished.Add(-5 * time.Minute), FinishedAt: &finished, ReplayAvailable: true}, Players: []GamePlayer{{UserID: "player-1", DisplayName: "Ace", PenaltyPoints: 0, Rank: 1, IsWinner: true}}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"operator@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	var wait sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, 4)
	for index := 0; index < 4; index++ {
		wait.Add(1)
		go func(position int) {
			defer wait.Done()
			body := fmt.Sprintf(`{"reason":"review %d","body":"note %d"}`, position, position)
			results[position] = request(t, router, http.MethodPost, "/games/"+gameID+"/notes", body, auth.AccessToken)
		}(index)
	}
	wait.Wait()
	for position, result := range results {
		if result.Code != http.StatusCreated {
			t.Fatalf("concurrent note %d status=%d body=%s", position, result.Code, result.Body.String())
		}
	}
	detail := request(t, router, http.MethodGet, "/games/"+gameID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"display_name":"Ace"`) || !strings.Contains(detail.Body.String(), `"rank":1`) || !strings.Contains(detail.Body.String(), `"penalty_points":0`) || !strings.Contains(detail.Body.String(), `"replay_available":true`) || !strings.Contains(detail.Body.String(), `"is_winner":true`) || strings.Count(detail.Body.String(), `"body":"note `) != 4 {
		t.Fatalf("concurrent annotations lost detail = %d %s", detail.Code, detail.Body.String())
	}
}

func TestGameSearchStablePaginationAndCountAgree(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "operator", Email: "operator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"games.read"}}
	store := NewMemoryStore(admin)
	finished := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for index := 0; index < 3; index++ {
		id := fmt.Sprintf("10000000-0000-0000-0000-%012d", index+1)
		store.SetGames(GameDetail{Game: Game{ID: id, RoomID: "room-1", Mode: "classic", StartedAt: finished.Add(time.Duration(-index) * time.Minute), FinishedAt: &finished}})
	}
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"operator@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)
	first := request(t, router, http.MethodGet, "/games?limit=2&offset=0", "", auth.AccessToken)
	second := request(t, router, http.MethodGet, "/games?limit=2&offset=2", "", auth.AccessToken)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"total":3`) || !strings.Contains(first.Body.String(), "000000000003") || !strings.Contains(first.Body.String(), "000000000002") {
		t.Fatalf("first page = %d %s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), "000000000001") || strings.Contains(second.Body.String(), "000000000003") {
		t.Fatalf("second page = %d %s", second.Code, second.Body.String())
	}
}

func intPtr(value int) *int { return &value }

func TestRoomInvestigationSearchDetailAndLiveAvailability(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.read"}}
	store := NewMemoryStore(reader)
	roomID := "10000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store.SetRooms(RoomDetail{Room: Room{ID: roomID, InviteCode: "ACE123", Name: "Practice table", Status: "waiting", Visibility: "private", GameMode: "classic", PracticeMode: true, MaxPlayers: 4, TurnTimerSeconds: 60, CreatedBy: "20000000-0000-0000-0000-000000000001", CreatedAt: created}, Players: []RoomPlayer{{UserID: "30000000-0000-0000-0000-000000000001", DisplayName: "Ace", JoinedAt: created}}})
	live := &stubLiveRoomClient{summary: LiveRoomSummary{Phase: "lobby", Players: []LiveRoomPlayer{{UserID: "30000000-0000-0000-0000-000000000001", DisplayName: "Ace", Connected: true}}, StateVersion: 7, OwnerID: "ws-2", FenceToken: 9}}
	router := newTestRouterWithLive(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, live)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	if response := request(t, router, http.MethodGet, "/rooms", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated rooms = %d", response.Code)
	}
	path := "/rooms?id=" + roomID + "&invite_code=ace123&status=waiting&visibility=private&mode=classic&created_from=2026-08-20T00:00:00Z&created_to=2026-08-21T00:00:00Z&limit=1&offset=0"
	response := request(t, router, http.MethodGet, path, "", auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"invite_code":"ACE123"`) || !strings.Contains(response.Body.String(), `"player_count":1`) {
		t.Fatalf("room search = %d %s", response.Code, response.Body.String())
	}
	for _, invalid := range []string{"/rooms?limit=101", "/rooms?id=bad", "/rooms?created_from=bad", "/rooms?created_from=2026-08-22T00:00:00Z&created_to=2026-08-21T00:00:00Z"} {
		if got := request(t, router, http.MethodGet, invalid, "", auth.AccessToken); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid filter %s = %d %s", invalid, got.Code, got.Body.String())
		}
	}

	detail := request(t, router, http.MethodGet, "/rooms/"+roomID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"display_name":"Ace"`) || !strings.Contains(detail.Body.String(), `"state_version":7`) || !strings.Contains(detail.Body.String(), `"live":{"available":true`) {
		t.Fatalf("room detail = %d %s", detail.Code, detail.Body.String())
	}
	if live.roomID != roomID {
		t.Fatalf("live room ID = %q", live.roomID)
	}

	live.err = errors.New("ws unavailable")
	detail = request(t, router, http.MethodGet, "/rooms/"+roomID, "", auth.AccessToken)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"live":{"available":false`) || !strings.Contains(detail.Body.String(), `"reason":"unavailable"`) {
		t.Fatalf("unavailable live detail = %d %s", detail.Code, detail.Body.String())
	}
}

type stubLiveRoomClient struct {
	summary      LiveRoomSummary
	err          error
	roomID       string
	hidden       json.RawMessage
	hiddenErr    error
	hiddenRoomID string
}

func (s *stubLiveRoomClient) RoomSummary(_ context.Context, roomID string) (LiveRoomSummary, error) {
	s.roomID = roomID
	return s.summary, s.err
}

func (s *stubLiveRoomClient) HiddenRoomState(_ context.Context, roomID string) (json.RawMessage, error) {
	s.hiddenRoomID = roomID
	return s.hidden, s.hiddenErr
}

func TestWSAdminClientUsesMachineCredentialAndRedactedContract(t *testing.T) {
	seenSecret := ""
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/rooms/10000000-0000-0000-0000-000000000001" {
			t.Errorf("path = %s", r.URL.Path)
		}
		seenSecret = r.Header.Get("X-WS-Inspection-Secret")
		_, _ = w.Write([]byte(`{"phase":"playing","players":[{"user_id":"player-1","display_name":"Ace","connected":true},{"user_id":"bot-1","display_name":"Robo","is_bot":true,"connected":false}],"turn_deadline":"2026-08-20T12:01:00Z","snapshot_age_ms":3000,"state_version":8,"owner":{"role":"owner","replica_id":"ws-2","fencing_token":10}}`))
	}))
	defer ws.Close()
	summary, err := NewWSAdminClient(ws.URL, "machine-secret").RoomSummary(context.Background(), "10000000-0000-0000-0000-000000000001")
	if err != nil || seenSecret != "machine-secret" || summary.Phase != "playing" || summary.Role != "owner" || summary.StateVersion != 8 || summary.OwnerID != "ws-2" || summary.FenceToken != 10 || len(summary.Players) != 2 {
		t.Fatalf("summary=%+v secret=%q err=%v", summary, seenSecret, err)
	}
	if !summary.Players[1].IsBot {
		t.Fatalf("bot player not decoded: %+v", summary.Players[1])
	}
}

func TestWSAdminClientSurfacesEdgeResponseWithoutLiveSummary(t *testing.T) {
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"owner":{"role":"edge","replica_id":"ws-1","fencing_token":5}}`))
	}))
	defer ws.Close()
	summary, err := NewWSAdminClient(ws.URL, "machine-secret").RoomSummary(context.Background(), "10000000-0000-0000-0000-000000000001")
	if err != nil || summary.Role != "edge" || summary.OwnerID != "ws-1" || summary.FenceToken != 5 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}

type failingAppendAuditStore struct{ Store }

func (s failingAppendAuditStore) AppendAudit(ctx context.Context, event AuditEvent) error {
	if event.Action == "rooms.hidden_state.read" {
		return errors.New("audit unavailable")
	}
	return s.Store.AppendAudit(ctx, event)
}

func TestHiddenRoomStateRejectsWhenAuditCannotRecordDeniedAttempt(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.read"}}
	memory := NewMemoryStore(admin)
	router := newTestRouterWithLive(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, failingAppendAuditStore{Store: memory}, nil)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodPost, "/rooms/10000000-0000-0000-0000-000000000001/hidden-state", `{"reason":"investigating report"}`, auth.AccessToken)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "Permission denied") {
		t.Fatalf("denied audit failure status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHiddenRoomStateFailsClosedWhenAuditCannotBeRecorded(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "inspector", Email: "inspector@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.inspect_hidden"}}
	memory := NewMemoryStore(admin)
	live := &stubLiveRoomClient{hidden: json.RawMessage(`{"hands":[[{"suit":"spades","rank":7}]]}`)}
	router := newTestRouterWithLive(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, failingAppendAuditStore{Store: memory}, live)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"inspector@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodPost, "/rooms/10000000-0000-0000-0000-000000000001/hidden-state", `{"reason":"investigating report"}`, auth.AccessToken)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "hands") {
		t.Fatalf("audit failure status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHiddenRoomStateMapsWSOutcomesAndAuditsThem(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	roomID := "10000000-0000-0000-0000-000000000001"
	admin := Admin{ID: "inspector", Email: "inspector@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.inspect_hidden"}}
	store := NewMemoryStore(admin)
	live := &stubLiveRoomClient{hiddenErr: wsAdminStatusError{status: http.StatusNotFound}}
	router := newTestRouterWithLive(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, live)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"inspector@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodPost, "/rooms/"+roomID+"/hidden-state", `{"reason":"investigating report"}`, auth.AccessToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d body=%s", response.Code, response.Body.String())
	}
	if audit := store.AuditEvents()[len(store.AuditEvents())-1]; audit.Outcome != "not_found" {
		t.Fatalf("not-found audit=%+v", audit)
	}
	live.hiddenErr = wsAdminStatusError{status: http.StatusConflict}
	response = request(t, router, http.MethodPost, "/rooms/"+roomID+"/hidden-state", `{"reason":"investigating report"}`, auth.AccessToken)
	if response.Code != http.StatusConflict {
		t.Fatalf("lease loss status=%d body=%s", response.Code, response.Body.String())
	}
	if audit := store.AuditEvents()[len(store.AuditEvents())-1]; audit.Outcome != "lease_lost" {
		t.Fatalf("lease-loss audit=%+v", audit)
	}
}

func TestWSAdminClientGetsHiddenStateWithMachineCredential(t *testing.T) {
	seenSecret := ""
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/rooms/10000000-0000-0000-0000-000000000001/hidden-state" {
			t.Errorf("path=%s", r.URL.Path)
		}
		seenSecret = r.Header.Get("X-WS-Inspection-Secret")
		_, _ = w.Write([]byte(`{"Hands":[[{"Suit":"spades","Rank":7}] ]}`))
	}))
	defer ws.Close()
	state, err := NewWSAdminClient(ws.URL, "machine-secret").HiddenRoomState(context.Background(), "10000000-0000-0000-0000-000000000001")
	if err != nil || seenSecret != "machine-secret" || !strings.Contains(string(state), "spades") {
		t.Fatalf("state=%s secret=%q err=%v", state, seenSecret, err)
	}
}

func TestHiddenRoomStateRequiresExceptionalPermissionAndAuditsOutcome(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	roomID := "10000000-0000-0000-0000-000000000001"
	reader := Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.read"}}
	inspector := Admin{ID: "inspector", Email: "inspector@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"rooms.inspect_hidden"}}
	store := NewMemoryStore(reader, inspector)
	live := &stubLiveRoomClient{hidden: json.RawMessage(`{"hands":[[{"suit":"spades","rank":7}]],"face_down":[[]]}`)}
	router := newTestRouterWithLive(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, live)

	login := func(email string) AuthResponse {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		if err := json.Unmarshal(response.Body.Bytes(), &auth); err != nil {
			t.Fatal(err)
		}
		return auth
	}
	if response := request(t, router, http.MethodPost, "/rooms/"+roomID+"/hidden-state", `{"reason":"investigating report"}`, login(reader.Email).AccessToken); response.Code != http.StatusForbidden {
		t.Fatalf("routine reader hidden-state status=%d body=%s", response.Code, response.Body.String())
	}
	denied := store.AuditEvents()[len(store.AuditEvents())-1]
	if denied.Action != "rooms.hidden_state.read" || denied.AdminID != reader.ID || denied.SessionID == "" || denied.RequestID != "test-request" || denied.ResourceID != roomID || denied.Reason != "investigating report" || denied.Outcome != "rejected" {
		t.Fatalf("denied hidden-state audit = %+v", denied)
	}

	response := request(t, router, http.MethodPost, "/rooms/"+roomID+"/hidden-state", `{"reason":"  investigating report  "}`, login(inspector.Email).AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"hands"`) || live.hiddenRoomID != roomID {
		t.Fatalf("hidden state status=%d body=%s room=%q", response.Code, response.Body.String(), live.hiddenRoomID)
	}
	last := store.AuditEvents()[len(store.AuditEvents())-1]
	if last.Action != "rooms.hidden_state.read" || last.ResourceType != "room" || last.ResourceID != roomID || last.AdminID != inspector.ID || last.SessionID == "" || last.RequestID != "test-request" || last.Reason != "  investigating report  " || last.Outcome != "success" {
		t.Fatalf("hidden-state audit = %+v", last)
	}
}

func TestUserSuspensionAndReinstatementAreAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	moderator := Admin{ID: "moderator", Email: "moderator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"users.moderate"}}
	store := NewMemoryStore(moderator)
	userID := "00000000-0000-0000-0000-000000000001"
	store.SetUsers(UserDetail{User: User{ID: userID, Username: "ace", DisplayName: "Ace Player"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"moderator@example.com","password":"password"}`, "")
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}

	missingReason := request(t, router, http.MethodPost, "/users/"+userID+"/suspension", `{}`, auth.AccessToken)
	if missingReason.Code != http.StatusBadRequest {
		t.Fatalf("missing suspension reason status = %d", missingReason.Code)
	}
	pastExpiry := request(t, router, http.MethodPost, "/users/"+userID+"/suspension", `{"reason":"Abuse","expires_at":"2020-01-01T00:00:00Z"}`, auth.AccessToken)
	if pastExpiry.Code != http.StatusBadRequest {
		t.Fatalf("past suspension expiry status = %d", pastExpiry.Code)
	}

	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	suspended := request(t, router, http.MethodPost, "/users/"+userID+"/suspension", `{"reason":"Repeated abuse","expires_at":"`+expiresAt+`"}`, auth.AccessToken)
	if suspended.Code != http.StatusOK || !strings.Contains(suspended.Body.String(), `"audit_action":"user.suspend"`) {
		t.Fatalf("suspend response = %d %s", suspended.Code, suspended.Body.String())
	}
	detail, err := store.GetUser(context.Background(), userID, false)
	if err != nil || detail.User.Suspension == nil || detail.User.Suspension.Reason != "Repeated abuse" || detail.User.Suspension.ExpiresAt == nil {
		t.Fatalf("suspension was not stored: %+v err=%v", detail.User.Suspension, err)
	}

	reinstated := request(t, router, http.MethodDelete, "/users/"+userID+"/suspension", "", auth.AccessToken)
	if reinstated.Code != http.StatusOK || !strings.Contains(reinstated.Body.String(), `"audit_action":"user.reinstate"`) {
		t.Fatalf("reinstate response = %d %s", reinstated.Code, reinstated.Body.String())
	}
	detail, err = store.GetUser(context.Background(), userID, false)
	if err != nil || detail.User.Suspension != nil {
		t.Fatalf("user remained suspended: %+v err=%v", detail.User.Suspension, err)
	}
	audits := store.AuditEvents()
	if len(audits) < 2 || audits[len(audits)-2].Action != "user.suspend" || audits[len(audits)-2].Reason != "Repeated abuse" || audits[len(audits)-1].Action != "user.reinstate" {
		t.Fatalf("moderation audit history = %+v", audits)
	}
}

func TestUserDisplayNameModerationIsVersionedAndAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	moderator := Admin{ID: "moderator", Email: "moderator@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"users.moderate"}}
	store := NewMemoryStore(moderator)
	userID := "00000000-0000-0000-0000-000000000001"
	store.SetUsers(
		UserDetail{User: User{ID: userID, Username: "ace", DisplayName: "Bad Name", Version: 3}},
		UserDetail{User: User{ID: "00000000-0000-0000-0000-000000000002", Username: "other", DisplayName: "Clean Name", Version: 1}},
	)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"moderator@example.com","password":"password"}`, "")
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{`{}`, `{"display_name":"Clean Name","version":3}`, `{"display_name":"Clean Name","reason":"Policy"}`} {
		response := request(t, router, http.MethodPatch, "/users/"+userID+"/display-name", body, auth.AccessToken)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid request %s status = %d", body, response.Code)
		}
	}
	updated := request(t, router, http.MethodPatch, "/users/"+userID+"/display-name", `{"display_name":"Clean Name","reason":"Inappropriate name","version":3}`, auth.AccessToken)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"version":4`) {
		t.Fatalf("update = %d %s", updated.Code, updated.Body.String())
	}
	conflict := request(t, router, http.MethodPatch, "/users/"+userID+"/display-name", `{"display_name":"Other Name","reason":"Retry","version":3}`, auth.AccessToken)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict = %d %s", conflict.Code, conflict.Body.String())
	}
	detail, _ := store.GetUser(context.Background(), userID, false)
	if detail.User.DisplayName != "Clean Name" || detail.User.Version != 4 {
		t.Fatalf("user = %+v", detail.User)
	}
	audits := store.AuditEvents()
	audit := audits[len(audits)-1]
	if audit.Action != "user.display_name.update" || audit.Reason != "Inappropriate name" || string(audit.BeforeState) != `{"display_name":"Bad Name","version":3}` || string(audit.AfterState) != `{"display_name":"Clean Name","version":4}` {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestMFAEnrollmentChallengeAndSingleUseRecovery(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", MFAEncryptionKey: "test-mfa-key-at-least-32-bytes!!"}, store)

	initial := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var initialAuth AuthResponse
	if err := json.Unmarshal(initial.Body.Bytes(), &initialAuth); err != nil {
		t.Fatal(err)
	}
	enroll := request(t, router, http.MethodPost, "/auth/mfa/enroll", `{}`, initialAuth.AccessToken)
	if enroll.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, body=%s", enroll.Code, enroll.Body.String())
	}
	var enrollment MFAEnrollmentResponse
	if err := json.Unmarshal(enroll.Body.Bytes(), &enrollment); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirm := request(t, router, http.MethodPost, "/auth/mfa/confirm", `{"code":"`+code+`"}`, initialAuth.AccessToken)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body=%s", confirm.Code, confirm.Body.String())
	}
	var confirmed MFAConfirmationResponse
	if err := json.Unmarshal(confirm.Body.Bytes(), &confirmed); err != nil {
		t.Fatal(err)
	}
	if len(confirmed.RecoveryCodes) != recoveryCodeCount {
		t.Fatalf("recovery code count = %d", len(confirmed.RecoveryCodes))
	}
	audits := store.AuditEvents()
	if len(audits) < 2 || audits[len(audits)-2].Action != "admin.mfa.enroll" || audits[len(audits)-1].Action != "admin.mfa.confirm" {
		t.Fatalf("MFA credential changes were not audited: %+v", audits)
	}
	for _, audit := range audits[len(audits)-2:] {
		payload := string(audit.BeforeState) + string(audit.AfterState) + string(audit.Metadata)
		if payload != "" || strings.Contains(payload, enrollment.Secret) {
			t.Fatalf("MFA audit contains credential material: %+v", audit)
		}
	}

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if login.Code != http.StatusAccepted {
		t.Fatalf("MFA login status = %d, body=%s", login.Code, login.Body.String())
	}
	var challenge MFAChallengeResponse
	if err := json.Unmarshal(login.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.ChallengeToken == "" {
		t.Fatal("missing challenge token")
	}

	completeCode, _ := totp.GenerateCode(enrollment.Secret, time.Now())
	complete := request(t, router, http.MethodPost, "/auth/mfa/challenge", `{"challenge_token":"`+challenge.ChallengeToken+`","code":"`+completeCode+`"}`, "")
	if complete.Code != http.StatusOK {
		t.Fatalf("TOTP challenge status = %d, body=%s", complete.Code, complete.Body.String())
	}
	var verifiedAuth AuthResponse
	if err := json.Unmarshal(complete.Body.Bytes(), &verifiedAuth); err != nil {
		t.Fatal(err)
	}
	reenroll := request(t, router, http.MethodPost, "/auth/mfa/enroll", `{}`, verifiedAuth.AccessToken)
	if reenroll.Code != http.StatusConflict {
		t.Fatalf("MFA re-enrollment status = %d, body=%s", reenroll.Code, reenroll.Body.String())
	}

	recoveryLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if err := json.Unmarshal(recoveryLogin.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	recoveryBody := `{"challenge_token":"` + challenge.ChallengeToken + `","recovery_code":"` + confirmed.RecoveryCodes[0] + `"}`
	recovery := request(t, router, http.MethodPost, "/auth/mfa/challenge", recoveryBody, "")
	if recovery.Code != http.StatusOK {
		t.Fatalf("recovery challenge status = %d, body=%s", recovery.Code, recovery.Body.String())
	}

	reusedLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if err := json.Unmarshal(reusedLogin.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	reused := request(t, router, http.MethodPost, "/auth/mfa/challenge", `{"challenge_token":"`+challenge.ChallengeToken+`","recovery_code":"`+confirmed.RecoveryCodes[0]+`"}`, "")
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("reused recovery status = %d", reused.Code)
	}
}

func TestInvitationAcceptanceIsRateLimited(t *testing.T) {
	store := NewMemoryStore()
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	body := `{"token":"invalid","display_name":"Operator","password":"correct horse battery staple"}`
	for i := 0; i < 5; i++ {
		response := request(t, router, http.MethodPost, "/auth/invitations/accept", body, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("attempt %d status = %d, body=%s", i+1, response.Code, response.Body.String())
		}
	}
	response := request(t, router, http.MethodPost, "/auth/invitations/accept", body, "")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status = %d, body=%s", response.Code, response.Body.String())
	}
}

func TestProductionWritesRequireMFAVerifiedSession(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long", MFAEncryptionKey: "test-mfa-key-at-least-32-bytes!!", Environment: "production"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var auth AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	enroll := request(t, router, http.MethodPost, "/auth/mfa/enroll", `{}`, auth.AccessToken)
	if enroll.Code != http.StatusOK {
		t.Fatalf("production MFA enrollment status = %d", enroll.Code)
	}
	response := request(t, router, http.MethodPost, "/auth/mfa/confirm", `{"code":"invalid"}`, auth.AccessToken)
	if response.Code == http.StatusForbidden {
		t.Fatalf("production MFA confirmation was blocked by policy")
	}
}

func TestRefreshRotatesSessionAndLogoutRevokesIt(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", DisplayName: "Operator", PasswordHash: string(hash), Status: "active", Permissions: []string{"dashboard.read"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	oldCookie := login.Result().Cookies()[0]

	refreshReq := csrfRequest(http.MethodPost, "/auth/refresh", oldCookie, login.Result().Cookies()[1])
	refresh := httptest.NewRecorder()
	router.ServeHTTP(refresh, refreshReq)
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body=%s", refresh.Code, refresh.Body.String())
	}
	newCookie := refresh.Result().Cookies()[0]
	if newCookie.Value == oldCookie.Value {
		t.Fatal("refresh token was not rotated")
	}

	replayReq := csrfRequest(http.MethodPost, "/auth/refresh", oldCookie, login.Result().Cookies()[1])
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, replayReq)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh status = %d", replay.Code)
	}
	familyReq := csrfRequest(http.MethodPost, "/auth/refresh", newCookie, refresh.Result().Cookies()[1])
	family := httptest.NewRecorder()
	router.ServeHTTP(family, familyReq)
	if family.Code != http.StatusUnauthorized {
		t.Fatalf("token family after replay status = %d", family.Code)
	}

	logoutReq := csrfRequest(http.MethodDelete, "/auth/logout", newCookie, refresh.Result().Cookies()[1])
	logout := httptest.NewRecorder()
	router.ServeHTTP(logout, logoutReq)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.Code)
	}

	revokedReq := csrfRequest(http.MethodPost, "/auth/refresh", newCookie, refresh.Result().Cookies()[1])
	revoked := httptest.NewRecorder()
	router.ServeHTTP(revoked, revokedReq)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked refresh status = %d", revoked.Code)
	}
	var refreshed AuthResponse
	if err := json.Unmarshal(refresh.Body.Bytes(), &refreshed); err != nil {
		t.Fatal(err)
	}
	afterLogout := request(t, router, http.MethodGet, "/dashboard", "", refreshed.AccessToken)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("access token after logout status = %d", afterLogout.Code)
	}
	if len(store.AuditEvents()) < 3 {
		t.Fatalf("expected authentication audit events, got %d", len(store.AuditEvents()))
	}
}

func TestRefreshAndLogoutRequireCSRFToken(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	refreshCookie := login.Result().Cookies()[0]

	for _, testCase := range []struct{ method, path string }{{http.MethodPost, "/auth/refresh"}, {http.MethodDelete, "/auth/logout"}} {
		req := httptest.NewRequest(testCase.method, testCase.path, nil)
		req.AddCookie(refreshCookie)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s without CSRF status = %d", testCase.path, response.Code)
		}
	}
}

func TestLoginValidationAndThrottling(t *testing.T) {
	store := NewMemoryStore()
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	invalid := request(t, router, http.MethodPost, "/auth/login", `{"email":"not-an-email","password":""}`, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid login status = %d", invalid.Code)
	}
	for attempt := 0; attempt < 4; attempt++ {
		request(t, router, http.MethodPost, "/auth/login", `{"email":"none@example.com","password":"wrong"}`, "")
	}
	throttled := request(t, router, http.MethodPost, "/auth/login", `{"email":"none@example.com","password":"wrong"}`, "")
	if throttled.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled login status = %d", throttled.Code)
	}
}

func TestAdminAPIRejectsWrongIssuerAndAudience(t *testing.T) {
	store := NewMemoryStore(Admin{ID: "admin-1", Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	for _, claims := range []jwt.RegisteredClaims{
		{Subject: "admin-1", Issuer: "seven-spade-player", Audience: jwt.ClaimStrings{"admin-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Subject: "admin-1", Issuer: "seven-spade-admin", Audience: jwt.ClaimStrings{"player-api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	} {
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims{SessionID: "session", RegisteredClaims: claims}).SignedString([]byte("test-secret-at-least-32-bytes-long"))
		response := request(t, router, http.MethodGet, "/me", "", token)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("wrong token status = %d", response.Code)
		}
	}
}

func TestDisabledAdminCannotLogin(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "disabled"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	response := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status = %d", response.Code)
	}
}

func TestAdministratorListsAndRevokesSessions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	first := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	second := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
	var firstAuth, secondAuth AuthResponse
	_ = json.Unmarshal(first.Body.Bytes(), &firstAuth)
	_ = json.Unmarshal(second.Body.Bytes(), &secondAuth)

	list := request(t, router, http.MethodGet, "/sessions", "", firstAuth.AccessToken)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "token_hash") || strings.Contains(list.Body.String(), "refresh") {
		t.Fatalf("unsafe sessions response: status=%d body=%s", list.Code, list.Body.String())
	}
	var sessions []SessionResponse
	if err := json.Unmarshal(list.Body.Bytes(), &sessions); err != nil || len(sessions) != 2 {
		t.Fatalf("sessions = %+v, err=%v", sessions, err)
	}
	var current, other SessionResponse
	for _, session := range sessions {
		if session.Current {
			current = session
		} else {
			other = session
		}
	}
	if current.ID == "" || other.ID == "" {
		t.Fatalf("current/other session not identified: %+v", sessions)
	}

	revokeOther := request(t, router, http.MethodDelete, "/sessions/"+other.ID, "", firstAuth.AccessToken)
	if revokeOther.Code != http.StatusNoContent {
		t.Fatalf("revoke other status=%d body=%s", revokeOther.Code, revokeOther.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", secondAuth.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("revoked access status=%d", got.Code)
	}
	if got := request(t, router, http.MethodDelete, "/sessions/"+current.ID, "", firstAuth.AccessToken); got.Code != http.StatusNoContent {
		t.Fatalf("revoke current status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", firstAuth.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("current access after revoke status=%d", got.Code)
	}

	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.AdminID != "admin-1" || last.SessionID != current.ID || last.RequestID != "test-request" || last.ResourceType != "admin_session" || last.ResourceID != current.ID {
		t.Fatalf("incomplete revoke audit: %+v", last)
	}
}

func TestAdministratorRevokesAllOtherSessions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "ops@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := func() AuthResponse {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"ops@example.com","password":"correct horse battery staple"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth
	}
	current, other := login(), login()

	revoked := request(t, router, http.MethodDelete, "/sessions/others", "", current.AccessToken)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke others status=%d body=%s", revoked.Code, revoked.Body.String())
	}
	if got := request(t, router, http.MethodGet, "/me", "", current.AccessToken); got.Code != http.StatusOK {
		t.Fatalf("current session status=%d", got.Code)
	}
	if got := request(t, router, http.MethodGet, "/me", "", other.AccessToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("other session status=%d", got.Code)
	}
}

func TestAdministratorManagementAndRolePermissions(t *testing.T) {
	superHash, _ := bcrypt.GenerateFromPassword([]byte("super-secret-password"), bcrypt.MinCost)
	viewerHash, _ := bcrypt.GenerateFromPassword([]byte("viewer-secret-password"), bcrypt.MinCost)

	superAdmin := Admin{
		ID:           "admin-super",
		Email:        "super@example.com",
		DisplayName:  "Super Admin",
		PasswordHash: string(superHash),
		Status:       "active",
		Roles:        []Role{{ID: "role-super", Name: "super_admin", Permissions: []string{"admins.read", "admins.manage", "dashboard.read"}}},
		Permissions:  []string{"admins.read", "admins.manage", "dashboard.read"},
	}
	viewerAdmin := Admin{
		ID:           "admin-viewer",
		Email:        "viewer@example.com",
		DisplayName:  "Viewer Admin",
		PasswordHash: string(viewerHash),
		Status:       "active",
		Roles:        []Role{{ID: "role-viewer", Name: "viewer", Permissions: []string{"dashboard.read"}}},
		Permissions:  []string{"dashboard.read"},
	}

	store := NewMemoryStore(superAdmin, viewerAdmin)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	// Super admin logs in
	superLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"super@example.com","password":"super-secret-password"}`, "")
	var superAuth AuthResponse
	_ = json.Unmarshal(superLogin.Body.Bytes(), &superAuth)

	// Viewer logs in
	viewerLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"viewer@example.com","password":"viewer-secret-password"}`, "")
	var viewerAuth AuthResponse
	_ = json.Unmarshal(viewerLogin.Body.Bytes(), &viewerAuth)

	// 1. Viewer cannot list admins or invite admins (Forbidden)
	forbiddenList := request(t, router, http.MethodGet, "/admins", "", viewerAuth.AccessToken)
	if forbiddenList.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer listing admins, got %d", forbiddenList.Code)
	}
	forbiddenInvite := request(t, router, http.MethodPost, "/admins/invite", `{"email":"newadmin@example.com","role_id":"role-moderator"}`, viewerAuth.AccessToken)
	if forbiddenInvite.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer inviting admin, got %d", forbiddenInvite.Code)
	}

	// 2. Unauthenticated request rejected (Unauthorized)
	unauthedList := request(t, router, http.MethodGet, "/admins", "", "")
	if unauthedList.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", unauthedList.Code)
	}

	// 3. Super admin invites a new moderator admin
	inviteRes := request(t, router, http.MethodPost, "/admins/invite", `{"email":"newmod@example.com","role_id":"role-moderator"}`, superAuth.AccessToken)
	if inviteRes.Code != http.StatusCreated {
		t.Fatalf("invite failed: status=%d body=%s", inviteRes.Code, inviteRes.Body.String())
	}
	var inviteBody InviteAdminResponse
	if err := json.Unmarshal(inviteRes.Body.Bytes(), &inviteBody); err != nil || inviteBody.Token == "" {
		t.Fatalf("invalid invite response: %+v", inviteBody)
	}
	inspectRes := request(t, router, http.MethodPost, "/auth/invitations/inspect", `{"token":"`+inviteBody.Token+`"}`, "")
	if inspectRes.Code != http.StatusOK || !strings.Contains(inspectRes.Body.String(), `"email":"newmod@example.com"`) || strings.Contains(inspectRes.Body.String(), "token_hash") {
		t.Fatalf("inspect invitation failed: status=%d body=%s", inspectRes.Code, inspectRes.Body.String())
	}
	listInvites := request(t, router, http.MethodGet, "/admin-invitations", "", superAuth.AccessToken)
	if listInvites.Code != http.StatusOK || !strings.Contains(listInvites.Body.String(), `"email":"newmod@example.com"`) {
		t.Fatalf("list invitations failed: status=%d body=%s", listInvites.Code, listInvites.Body.String())
	}

	// Duplicate invite conflict
	dupInvite := request(t, router, http.MethodPost, "/admins/invite", `{"email":"newmod@example.com","role_id":"role-moderator"}`, superAuth.AccessToken)
	if dupInvite.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate invite, got %d", dupInvite.Code)
	}

	// 4. Accept invitation
	acceptRes := request(t, router, http.MethodPost, "/auth/invitations/accept", `{"token":"`+inviteBody.Token+`","display_name":"Moderator Bob","password":"bob-secure-password"}`, "")
	if acceptRes.Code != http.StatusOK {
		t.Fatalf("accept invite failed: status=%d body=%s", acceptRes.Code, acceptRes.Body.String())
	}
	var newAdmin Admin
	_ = json.Unmarshal(acceptRes.Body.Bytes(), &newAdmin)
	if newAdmin.Email != "newmod@example.com" || newAdmin.DisplayName != "Moderator Bob" {
		t.Fatalf("unexpected new admin: %+v", newAdmin)
	}

	// 5. Newly invited admin can login and has moderator permissions
	modLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"newmod@example.com","password":"bob-secure-password"}`, "")
	if modLogin.Code != http.StatusOK {
		t.Fatalf("moderator login failed: status=%d body=%s", modLogin.Code, modLogin.Body.String())
	}
	var modAuth AuthResponse
	_ = json.Unmarshal(modLogin.Body.Bytes(), &modAuth)
	if !contains(modAuth.Admin.Permissions, "users.moderate") {
		t.Fatalf("expected users.moderate permission in moderator: %+v", modAuth.Admin.Permissions)
	}

	// 6. Super admin lists admins
	listRes := request(t, router, http.MethodGet, "/admins", "", superAuth.AccessToken)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list admins failed: status=%d", listRes.Code)
	}
	var adminList []Admin
	_ = json.Unmarshal(listRes.Body.Bytes(), &adminList)
	if len(adminList) != 3 {
		t.Fatalf("expected 3 admins, got %d", len(adminList))
	}

	// 7. Super admin updates role permissions (e.g. add skin management to moderator role)
	updateRolePerms := request(t, router, http.MethodPut, "/roles/role-moderator/permissions", `{"permissions":["dashboard.read","users.read","users.moderate","skins.manage"]}`, superAuth.AccessToken)
	if updateRolePerms.Code != http.StatusNoContent {
		t.Fatalf("update role permissions failed: status=%d body=%s", updateRolePerms.Code, updateRolePerms.Body.String())
	}

	// Permission mapping changes invalidate sessions for administrators assigned to the role.
	stalePermissionCheck := request(t, router, http.MethodGet, "/me", "", modAuth.AccessToken)
	if stalePermissionCheck.Code != http.StatusUnauthorized {
		t.Fatalf("expected stale session to be unauthorized after permission change, got %d", stalePermissionCheck.Code)
	}
	modLogin = request(t, router, http.MethodPost, "/auth/login", `{"email":"newmod@example.com","password":"bob-secure-password"}`, "")
	_ = json.Unmarshal(modLogin.Body.Bytes(), &modAuth)

	// 8. Super admin updates administrator roles
	setRolesRes := request(t, router, http.MethodPut, "/admins/"+newAdmin.ID+"/roles", `{"role_ids":["role-operator"]}`, superAuth.AccessToken)
	if setRolesRes.Code != http.StatusNoContent {
		t.Fatalf("set admin roles failed: status=%d body=%s", setRolesRes.Code, setRolesRes.Body.String())
	}

	// Sessions of modified admin are revoked promptly
	staleCheck := request(t, router, http.MethodGet, "/me", "", modAuth.AccessToken)
	if staleCheck.Code != http.StatusUnauthorized {
		t.Fatalf("expected stale session to be unauthorized after role change, got %d", staleCheck.Code)
	}

	// 9. Super admin disables the administrator
	disableRes := request(t, router, http.MethodPatch, "/admins/"+newAdmin.ID+"/status", `{"status":"disabled"}`, superAuth.AccessToken)
	if disableRes.Code != http.StatusNoContent {
		t.Fatalf("disable admin failed: status=%d body=%s", disableRes.Code, disableRes.Body.String())
	}

	// Disabled administrator cannot login or use existing tokens
	disabledLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"newmod@example.com","password":"bob-secure-password"}`, "")
	if disabledLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for disabled admin login, got %d", disabledLogin.Code)
	}

	// Verify audit events were written for invite, accept, role update, status update
	audits := store.AuditEvents()
	var actions []string
	for _, a := range audits {
		actions = append(actions, a.Action)
	}
	for _, expectedAction := range []string{"admin.invite", "admin.invite.accept", "admin.role_permissions.update", "admin.roles.update", "admin.status.update"} {
		if !contains(actions, expectedAction) {
			t.Fatalf("missing expected audit action %q in %+v", expectedAction, actions)
		}
	}
}

func TestInvitationReissueAndRevoke(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "admin-super", Email: "super@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"admins.read", "admins.manage"}}
	store := NewMemoryStore(admin)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	loginRes := request(t, router, http.MethodPost, "/auth/login", `{"email":"super@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(loginRes.Body.Bytes(), &auth)

	inviteRes := request(t, router, http.MethodPost, "/admins/invite", `{"email":"rotate@example.com","role_id":"role-viewer"}`, auth.AccessToken)
	var original InviteAdminResponse
	_ = json.Unmarshal(inviteRes.Body.Bytes(), &original)
	reissueRes := request(t, router, http.MethodPost, "/admin-invitations/"+original.Invitation.ID+"/reissue", "", auth.AccessToken)
	if reissueRes.Code != http.StatusOK {
		t.Fatalf("reissue failed: status=%d body=%s", reissueRes.Code, reissueRes.Body.String())
	}
	var replacement ReissueInviteResponse
	_ = json.Unmarshal(reissueRes.Body.Bytes(), &replacement)
	if replacement.Token == "" || replacement.Token == original.Token {
		t.Fatalf("expected rotated token: %+v", replacement)
	}
	oldInspect := request(t, router, http.MethodPost, "/auth/invitations/inspect", `{"token":"`+original.Token+`"}`, "")
	if oldInspect.Code != http.StatusNotFound {
		t.Fatalf("expected old token to be invalid, got %d", oldInspect.Code)
	}
	revokeRes := request(t, router, http.MethodDelete, "/admin-invitations/"+original.Invitation.ID, "", auth.AccessToken)
	if revokeRes.Code != http.StatusNoContent {
		t.Fatalf("revoke failed: status=%d body=%s", revokeRes.Code, revokeRes.Body.String())
	}
	newInspect := request(t, router, http.MethodPost, "/auth/invitations/inspect", `{"token":"`+replacement.Token+`"}`, "")
	if newInspect.Code != http.StatusNotFound {
		t.Fatalf("expected revoked token to be invalid, got %d", newInspect.Code)
	}
}

func TestRouteAuthorizationMatrix(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	superAdmin := Admin{
		ID:           "admin-super",
		Email:        "super@example.com",
		PasswordHash: string(hash),
		Status:       "active",
		Permissions:  []string{"dashboard.read", "admins.read", "admins.manage"},
	}
	viewerAdmin := Admin{
		ID:           "admin-viewer",
		Email:        "viewer@example.com",
		PasswordHash: string(hash),
		Status:       "active",
		Permissions:  []string{"dashboard.read"},
	}
	disabledAdmin := Admin{
		ID:           "admin-disabled",
		Email:        "disabled@example.com",
		PasswordHash: string(hash),
		Status:       "disabled",
		Permissions:  []string{"dashboard.read", "admins.read", "admins.manage"},
	}

	store := NewMemoryStore(superAdmin, viewerAdmin, disabledAdmin)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	superLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"super@example.com","password":"password"}`, "")
	var superAuth AuthResponse
	_ = json.Unmarshal(superLogin.Body.Bytes(), &superAuth)

	viewerLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"viewer@example.com","password":"password"}`, "")
	var viewerAuth AuthResponse
	_ = json.Unmarshal(viewerLogin.Body.Bytes(), &viewerAuth)

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard", ""},
		{http.MethodGet, "/admins", ""},
		{http.MethodPost, "/admins/invite", `{"email":"invitee@example.com","role_id":"role-viewer"}`},
		{http.MethodPatch, "/admins/admin-viewer/status", `{"status":"active"}`},
		{http.MethodPut, "/admins/admin-viewer/roles", `{"role_ids":["role-viewer"]}`},
		{http.MethodGet, "/roles", ""},
		{http.MethodPut, "/roles/role-viewer/permissions", `{"permissions":["dashboard.read"]}`},
		{http.MethodGet, "/permissions", ""},
		{http.MethodGet, "/sessions", ""},
	}

	// 1. Unauthenticated test for every route
	for _, r := range routes {
		res := request(t, router, r.method, r.path, r.body, "")
		if res.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s %s expected 401, got %d", r.method, r.path, res.Code)
		}
	}

	// 2. Allowed test with super admin for every route
	for _, r := range routes {
		res := request(t, router, r.method, r.path, r.body, superAuth.AccessToken)
		if res.Code >= 400 {
			t.Errorf("super admin %s %s expected success, got %d body=%s", r.method, r.path, res.Code, res.Body.String())
		}
	}

	// Re-login viewer since session was modified/tested during previous calls
	viewerLogin2 := request(t, router, http.MethodPost, "/auth/login", `{"email":"viewer@example.com","password":"password"}`, "")
	var viewerAuth2 AuthResponse
	_ = json.Unmarshal(viewerLogin2.Body.Bytes(), &viewerAuth2)

	// 3. Forbidden test with viewer admin for manage routes
	manageRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/admins", ""},
		{http.MethodPost, "/admins/invite", `{"email":"invitee2@example.com","role_id":"role-viewer"}`},
		{http.MethodPatch, "/admins/admin-super/status", `{"status":"disabled"}`},
		{http.MethodPut, "/admins/admin-super/roles", `{"role_ids":["role-viewer"]}`},
		{http.MethodGet, "/roles", ""},
		{http.MethodPut, "/roles/role-viewer/permissions", `{"permissions":["dashboard.read"]}`},
		{http.MethodGet, "/permissions", ""},
	}
	for _, r := range manageRoutes {
		res := request(t, router, r.method, r.path, r.body, viewerAuth2.AccessToken)
		if res.Code != http.StatusForbidden {
			t.Errorf("viewer %s %s expected 403, got %d body=%s", r.method, r.path, res.Code, res.Body.String())
		}
	}
}

func TestAuditEventsAreSearchableAndSensitiveReadsAreAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "auditor@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"audit.read"}})
	store.audits = append(store.audits,
		AuditEvent{ID: "event-1", AdminID: "00000000-0000-0000-0000-000000000001", Action: "admin.status.update", ResourceType: "admin_user", ResourceID: "target-1", Outcome: "success", OccurredAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)},
		AuditEvent{ID: "event-2", AdminID: "admin-2", Action: "admin.roles.update", ResourceType: "admin_user", ResourceID: "target-2", Outcome: "rejected", OccurredAt: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)},
	)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"auditor@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	if invalid := request(t, router, http.MethodGet, "/audit-events?id=not-a-uuid", "", auth.AccessToken); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid audit event ID status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	response := request(t, router, http.MethodGet, "/audit-events?actor_id=00000000-0000-0000-0000-000000000001&action=admin.status.update&resource_type=admin_user&resource_id=target-1&outcome=success&from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", auth.AccessToken)
	if response.Code != http.StatusOK {
		t.Fatalf("audit list status=%d body=%s", response.Code, response.Body.String())
	}
	var result AuditEventPage
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Events) != 1 || result.Events[0].ID != "event-1" {
		t.Fatalf("unexpected audit result: %+v err=%v", result, err)
	}
	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.Action != "audit.events.read" || last.ResourceType != "admin_audit_event" || last.Outcome != "success" {
		t.Fatalf("sensitive read was not audited: %+v", last)
	}
}

func TestAuditListAndExportAcceptEveryPersistedOutcome(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "auditor", Email: "auditor@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"audit.read", "audit.export"}})
	when := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	outcomes := []string{"success", "rejected", "failed", "denied", "invalid_request"}
	for _, outcome := range outcomes {
		store.audits = append(store.audits, AuditEvent{ID: uuid.NewString(), Action: "test." + outcome, Outcome: outcome, OccurredAt: when})
	}
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"auditor@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	for _, outcome := range outcomes {
		list := request(t, router, http.MethodGet, "/audit-events?outcome="+outcome, "", auth.AccessToken)
		if list.Code != http.StatusOK {
			t.Errorf("list outcome %q status=%d body=%s", outcome, list.Code, list.Body.String())
		}
		export := request(t, router, http.MethodGet, "/audit-events/export?outcome="+outcome+"&from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", auth.AccessToken)
		if export.Code != http.StatusOK {
			t.Errorf("export outcome %q status=%d body=%s", outcome, export.Code, export.Body.String())
		}
	}
}

func TestAuditExportIsAuthorizedFilteredRedactedAndAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(
		Admin{ID: "exporter", Email: "exporter@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"audit.export"}},
		Admin{ID: "reader", Email: "reader@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"audit.read"}},
	)
	store.audits = append(store.audits, AuditEvent{ID: "event-1", AdminID: "00000000-0000-0000-0000-000000000001", SessionID: "secret-session", RequestID: "request-1", Action: "admin.status.update", ResourceType: "admin_user", ResourceID: "target-1", Reason: "policy violation", Outcome: "success", BeforeState: []byte(`{"email":"private@example.com"}`), AfterState: []byte(`{"status":"disabled"}`), Metadata: []byte(`{"token":"secret"}`), IPAddress: "192.0.2.1", UserAgent: "secret-agent", OccurredAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)

	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"exporter@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)
	response := request(t, router, http.MethodGet, "/audit-events/export?actor_id=00000000-0000-0000-0000-000000000001&from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", auth.AccessToken)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("export status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	body := response.Body.String()
	for _, secret := range []string{"secret-session", "private@example.com", "secret-agent", "192.0.2.1", "secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("export leaked %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, "event-1") || strings.Contains(body, "policy violation") {
		t.Fatalf("export did not apply the field allowlist: %s", body)
	}
	last := store.AuditEvents()[len(store.AuditEvents())-1]
	if last.Action != "audit.events.export" || last.Outcome != "success" || !strings.Contains(string(last.Metadata), `"exported_rows":1`) {
		t.Fatalf("export audit = %+v", last)
	}

	readerLogin := request(t, router, http.MethodPost, "/auth/login", `{"email":"reader@example.com","password":"password"}`, "")
	var readerAuth AuthResponse
	_ = json.Unmarshal(readerLogin.Body.Bytes(), &readerAuth)
	forbidden := request(t, router, http.MethodGet, "/audit-events/export?from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", readerAuth.AccessToken)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("reader export status=%d", forbidden.Code)
	}

	invalid := request(t, router, http.MethodGet, "/audit-events/export", "", auth.AccessToken)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unbounded export status=%d", invalid.Code)
	}
	last = store.AuditEvents()[len(store.AuditEvents())-1]
	if last.Action != "audit.events.export" || last.Outcome != "rejected" {
		t.Fatalf("rejected export audit = %+v", last)
	}

	empty := request(t, router, http.MethodGet, "/audit-events/export?action=missing&from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", auth.AccessToken)
	if empty.Code != http.StatusOK || strings.Count(strings.TrimSpace(empty.Body.String()), "\n") != 0 {
		t.Fatalf("empty export status=%d body=%q", empty.Code, empty.Body.String())
	}
}

type failingAuditListStore struct{ Store }

func (s failingAuditListStore) ListAuditEvents(context.Context, AuditFilter) (AuditEventPage, error) {
	return AuditEventPage{}, errors.New("database unavailable")
}

func TestAuditExportFailureIsAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	memory := NewMemoryStore(Admin{ID: "exporter", Email: "exporter@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"audit.export"}})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, failingAuditListStore{Store: memory})
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"exporter@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodGet, "/audit-events/export?from=2026-08-20T00:00:00Z&to=2026-08-21T00:00:00Z", "", auth.AccessToken)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed export status=%d body=%s", response.Code, response.Body.String())
	}
	last := memory.AuditEvents()[len(memory.AuditEvents())-1]
	if last.Action != "audit.events.export" || last.Outcome != "failed" {
		t.Fatalf("failed export audit = %+v", last)
	}
}

func TestRejectedPolicyControlledActionIsAudited(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "admin-1", Email: "viewer@example.com", PasswordHash: string(hash), Status: "active"})
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := request(t, router, http.MethodPost, "/auth/login", `{"email":"viewer@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)

	response := request(t, router, http.MethodPatch, "/admins/target/status", `{"status":"disabled","reason":"security review"}`, auth.AccessToken)
	if response.Code != http.StatusForbidden {
		t.Fatalf("rejected mutation status=%d", response.Code)
	}
	audits := store.AuditEvents()
	last := audits[len(audits)-1]
	if last.Action != "admin.status.update" || last.ResourceType != "admin_user" || last.ResourceID != "target" || last.Outcome != "rejected" {
		t.Fatalf("rejected action audit = %+v", last)
	}
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func newTestRouter(cfg Config, store Store) *gin.Engine {
	return newTestRouterWithLive(cfg, store, nil)
}

func newTestRouterWithLive(cfg Config, store Store, live LiveRoomClient) *gin.Engine {
	h := NewAdminHandler(cfg, store, Dependencies{LiveRooms: live})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request")
		c.Next()
	})
	router.POST("/auth/login", h.Login)
	router.POST("/auth/mfa/challenge", h.MFAChallenge)
	router.POST("/auth/refresh", h.Refresh)
	router.DELETE("/auth/logout", h.Logout)
	router.POST("/auth/invitations/accept", h.AcceptInvite)
	router.POST("/auth/invitations/inspect", h.GetInvitation)

	authed := router.Group("")
	authed.Use(h.RequireAuth)
	authed.GET("/me", h.Me)
	authed.GET("/sessions", h.ListSessions)
	authed.DELETE("/sessions/others", h.RevokeOtherSessions)
	authed.DELETE("/sessions/:id", h.RevokeSession)
	authed.POST("/auth/mfa/enroll", h.EnrollMFA)
	authed.POST("/auth/mfa/confirm", h.ConfirmMFA)
	authed.GET("/dashboard", h.RequirePermission("dashboard.read"), h.Dashboard)
	authed.GET("/users", h.RequirePermission("users.read"), h.SearchUsers)
	authed.GET("/users/:id", h.RequirePermission("users.read"), h.GetUser)
	authed.GET("/rooms", h.RequirePermission("rooms.read"), h.SearchRooms)
	authed.GET("/rooms/:id", h.RequirePermission("rooms.read"), h.GetRoom)
	authed.POST("/rooms/:id/hidden-state", h.HiddenRoomState)
	authed.GET("/games", h.RequirePermission("games.read"), h.SearchGames)
	authed.GET("/games/:id", h.RequirePermission("games.read"), h.GetGame)
	authed.POST("/games/:id/flags", h.RequirePermission("games.annotate"), h.FlagGame)
	authed.POST("/games/:id/notes", h.RequirePermission("games.annotate"), h.AddGameNote)
	authed.POST("/users/:id/suspension", h.RequirePermission("users.moderate"), h.SuspendUser)
	authed.DELETE("/users/:id/suspension", h.RequirePermission("users.moderate"), h.ReinstateUser)
	authed.PATCH("/users/:id/display-name", h.RequirePermission("users.moderate"), h.UpdateUserDisplayName)

	authed.GET("/admins", h.RequirePermission("admins.read"), h.ListAdmins)
	authed.POST("/admins/invite", h.RequirePermission("admins.manage"), h.InviteAdmin)
	authed.GET("/admin-invitations", h.RequirePermission("admins.read"), h.ListInvitations)
	authed.DELETE("/admin-invitations/:id", h.RequirePermission("admins.manage"), h.RevokeInvitation)
	authed.POST("/admin-invitations/:id/reissue", h.RequirePermission("admins.manage"), h.ReissueInvitation)
	authed.PATCH("/admins/:id/status", h.RequirePermission("admins.manage"), h.SetAdminStatus)
	authed.PUT("/admins/:id/roles", h.RequirePermission("admins.manage"), h.SetAdminRoles)

	authed.GET("/roles", h.RequirePermission("admins.read"), h.ListRoles)
	authed.PUT("/roles/:id/permissions", h.RequirePermission("admins.manage"), h.UpdateRolePermissions)
	authed.GET("/permissions", h.RequirePermission("admins.read"), h.ListPermissions)
	authed.GET("/audit-events", h.RequirePermission("audit.read"), h.ListAuditEvents)
	authed.GET("/audit-events/export", h.RequirePermission("audit.export"), h.ExportAuditEvents)
	authed.GET("/settings/daily-login", h.RequirePermission("settings.read"), h.GetDailyLoginSetting)
	authed.PUT("/settings/daily-login", h.RequirePermission("settings.write"), h.UpdateDailyLoginSetting)
	authed.GET("/settings", h.RequirePermission("settings.read"), h.ListApplicationSettings)
	authed.PUT("/settings/:key", h.RequirePermission("settings.write"), h.UpdateApplicationSetting)
	authed.GET("/skins", h.RequirePermission("skins.read"), h.ListSkins)
	authed.GET("/skins/:id", h.RequirePermission("skins.read"), h.GetSkin)
	authed.POST("/skins", h.RequirePermission("skins.manage"), h.CreateSkin)
	authed.PUT("/skins/:id", h.RequirePermission("skins.manage"), h.UpdateSkin)
	authed.POST("/skins/:id/uploads", h.RequirePermission("skins.manage"), h.PresignSkinUpload)
	authed.POST("/skins/:id/revisions", h.RequirePermission("skins.manage"), h.PublishSkin)
	authed.POST("/skins/:id/revisions/:revisionId/disable", h.RequirePermission("skins.manage"), h.DisableSkinRevision)
	authed.POST("/users/:id/skins/:skinId/grant", h.RequirePermission("skins.entitlements"), h.ChangeSkinEntitlement("grant"))
	authed.POST("/users/:id/skins/:skinId/revoke", h.RequirePermission("skins.entitlements"), h.ChangeSkinEntitlement("revoke"))
	return router
}

func request(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func csrfRequest(method, path string, refresh, csrf *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(refresh)
	req.AddCookie(csrf)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	return req
}

type stubSkinSigner struct {
	err         error
	key         string
	contentType string
	size        int64
	body        []byte
}

func (s *stubSkinSigner) PresignPut(_ context.Context, key, contentType string, size int64, _ time.Duration) (string, error) {
	s.key, s.contentType, s.size = key, contentType, size
	return "https://upload.example/put", s.err
}
func (s *stubSkinSigner) PutObject(_ context.Context, key, contentType string, size int64, body io.Reader) error {
	if s.err != nil {
		return s.err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.key, s.contentType, s.size, s.body = key, contentType, size, data
	return nil
}
func (s *stubSkinSigner) HeadObject(_ context.Context, key string) (string, int64, error) {
	if key != s.key {
		return "", 0, ErrNotFound
	}
	return s.contentType, s.size, s.err
}
func (s *stubSkinSigner) PublicURL(key string) string { return "https://cdn.example/" + key }

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestSkinAssetPrefix(t *testing.T) {
	for skinType, want := range map[string]string{
		"profile_background":     "skins/backgrounds/",
		"player_card_background": "skins/player-card-backgrounds/",
		"avatar_frame":           "skins/frames/",
		"display_picture":        "skins/display-pictures/",
	} {
		got, ok := skinAssetPrefix(skinType)
		if !ok || got != want {
			t.Errorf("skinAssetPrefix(%q) = %q, %t; want %q, true", skinType, got, ok, want)
		}
	}
	if prefix, ok := skinAssetPrefix("unknown"); ok || prefix != "" {
		t.Errorf("skinAssetPrefix(unknown) = %q, %t; want empty, false", prefix, ok)
	}
}

func uploadSkinAssetRequest(t *testing.T, router http.Handler, token, skinID, filename, contentType string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="` + filename + `"`}, "Content-Type": {contentType}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/skins/"+skinID+"/assets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestSkinLifecycleThroughAdminHTTP(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	admin := Admin{ID: "manager", Email: "skins@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"skins.read", "skins.manage", "skins.entitlements"}}
	store := NewMemoryStore(admin)
	skinID := "42395ffa-fc5f-4700-bdb7-713a501f7305"
	store.SetSkins(Skin{ID: skinID, Name: "Gold", SkinType: "profile_background", AssetKey: "skins/gold.png", Enabled: true, CatalogVisible: true, Revisions: []SkinRevision{{ID: "revision-1", SkinID: skinID, Version: 1, AssetKey: "skins/gold.png", Enabled: true}}})
	draftSkinID := "52395ffa-fc5f-4700-bdb7-713a501f7305"
	store.SetSkins(Skin{ID: draftSkinID, Name: "Draft", SkinType: "profile_background"})
	store.SetEvents(model.Event{ID: "00000000-0000-0000-0000-000000000002", Name: "Summer", State: model.EventPublished, Revision: 1})
	store.SetUsers(UserDetail{User: User{ID: "00000000-0000-0000-0000-000000000001", Username: "ace", DisplayName: "Ace"}})
	signer := &stubSkinSigner{}
	h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{Storage: signer})
	r := gin.New()
	r.POST("/auth/login", h.Login)
	g := r.Group("")
	g.Use(h.RequireAuth)
	g.GET("/skins", h.RequirePermission("skins.read"), h.ListSkins)
	g.POST("/skins", h.RequirePermission("skins.manage"), h.CreateSkin)
	g.PUT("/skins/:id", h.RequirePermission("skins.manage"), h.UpdateSkin)
	g.POST("/skins/:id/uploads", h.RequirePermission("skins.manage"), h.PresignSkinUpload)
	g.POST("/skins/:id/assets", h.RequirePermission("skins.manage"), h.UploadSkinAsset)
	g.POST("/skins/:id/revisions", h.RequirePermission("skins.manage"), h.PublishSkin)
	g.POST("/skins/:id/revisions/:revisionId/disable", h.RequirePermission("skins.manage"), h.DisableSkinRevision)
	g.POST("/users/:id/skins/:skinId/grant", h.RequirePermission("skins.entitlements"), h.ChangeSkinEntitlement("grant"))
	g.POST("/users/:id/skins/:skinId/revoke", h.RequirePermission("skins.entitlements"), h.ChangeSkinEntitlement("revoke"))
	login := request(t, r, "POST", "/auth/login", `{"email":"skins@example.com","password":"password"}`, "")
	var auth AuthResponse
	_ = json.Unmarshal(login.Body.Bytes(), &auth)
	token := auth.AccessToken
	if got := request(t, r, "PUT", "/skins/"+draftSkinID, `{"name":"Draft","enabled":true,"catalog_visible":true,"reason":"publish too early","unlock_rules":[{"name":"Level two","rule_type":"minimum_level","minimum_level":2,"enabled":true}]}`, token); got.Code != http.StatusConflict {
		t.Fatalf("enable unpublished skin=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "GET", "/skins", "", token); got.Code != 200 || !strings.Contains(got.Body.String(), `"asset_url":"https://cdn.example/skins/gold.png"`) {
		t.Fatalf("list=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Platinum","enabled":true,"catalog_visible":true,"reason":"catalog correction","unlock_rules":[{"name":"Summer check-ins","rule_type":"event_check_in_count","event_id":"00000000-0000-0000-0000-000000000002","event_check_in_count":3,"enabled":true}]}`, token); got.Code != 200 || !strings.Contains(got.Body.String(), `"event_id":"00000000-0000-0000-0000-000000000002"`) || !strings.Contains(got.Body.String(), `"event_check_in_count":3`) {
		t.Fatalf("event rule edit=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Platinum","enabled":true,"catalog_visible":true,"reason":"invalid rule","unlock_rules":[{"name":"Bad","rule_type":"game_condition","conditions":[{"metric":"client_claim","operator":"eq","value":"true"}]}]}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid unlock rule=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Platinum","enabled":true,"catalog_visible":true,"reason":"invalid event","unlock_rules":[{"name":"Bad event","rule_type":"game_condition","event_id":"not-a-uuid","conditions":[{"metric":"is_winner","operator":"eq","value":"true"}]}]}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid event game condition=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Platinum","enabled":true,"catalog_visible":true,"reason":"invalid value","unlock_rules":[{"name":"Bad value","rule_type":"game_condition","conditions":[{"metric":"wins","operator":"gte","value":"many"}]}]}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid game condition value=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Platinum","enabled":true,"catalog_visible":true,"reason":"invalid boolean","unlock_rules":[{"name":"Bad boolean","rule_type":"game_condition","conditions":[{"metric":"is_winner","operator":"eq","value":"1"}]}]}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid game condition boolean=%d %s", got.Code, got.Body.String())
	}
	upload := request(t, r, "POST", "/skins/"+skinID+"/uploads", `{"filename":"x.png","content_type":"image/png","size":100}`, token)
	var uploadTicket struct {
		AssetKey string `json:"asset_key"`
	}
	_ = json.Unmarshal(upload.Body.Bytes(), &uploadTicket)
	if upload.Code != http.StatusCreated || !strings.HasPrefix(uploadTicket.AssetKey, "skins/backgrounds/") || signer.key != uploadTicket.AssetKey {
		t.Fatalf("upload=%d %s", upload.Code, upload.Body.String())
	}
	issuedKey := signer.key
	if got := request(t, r, "POST", "/skins/missing/uploads", `{"filename":"x.png","content_type":"image/png","size":100}`, token); got.Code != http.StatusNotFound || signer.key != issuedKey {
		t.Fatalf("missing upload=%d key=%q", got.Code, signer.key)
	}
	if got := request(t, r, "POST", "/skins/"+skinID+"/uploads", `{"filename":"x.exe","content_type":"application/octet-stream","size":1}`, token); got.Code != 400 {
		t.Fatalf("constraint=%d", got.Code)
	}
	if got := request(t, r, "POST", "/skins/"+skinID+"/uploads", `{"filename":"asset.svg","content_type":"image/svg+xml","size":100}`, token); got.Code != 201 {
		t.Fatalf("svg upload=%d", got.Code)
	}
	validPNG := testPNG(t, 10, 7)
	assetResponse := uploadSkinAssetRequest(t, r, token, skinID, "replacement.png", "image/png", validPNG)
	var assetTicket struct {
		AssetKey string `json:"asset_key"`
	}
	_ = json.Unmarshal(assetResponse.Body.Bytes(), &assetTicket)
	if assetResponse.Code != http.StatusCreated || !strings.HasPrefix(assetTicket.AssetKey, "skins/backgrounds/") || signer.key != assetTicket.AssetKey || signer.size != int64(len(validPNG)) || !bytes.Equal(signer.body, validPNG) {
		t.Fatalf("asset upload=%d key=%q body=%q response=%s", assetResponse.Code, signer.key, signer.body, assetResponse.Body.String())
	}
	validSVG := []byte(`<svg viewBox="0 0 10 7" xmlns="http://www.w3.org/2000/svg"></svg>`)
	if response := uploadSkinAssetRequest(t, r, token, skinID, "replacement.svg", "image/svg+xml", validSVG); response.Code != http.StatusCreated {
		t.Fatalf("svg asset upload=%d %s", response.Code, response.Body.String())
	}
	if response := uploadSkinAssetRequest(t, r, token, skinID, "wrong.png", "image/png", testPNG(t, 7, 10)); response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "10:7") {
		t.Fatalf("wrong-ratio png upload=%d %s", response.Code, response.Body.String())
	}
	var invalidAssetBody bytes.Buffer
	invalidWriter := multipart.NewWriter(&invalidAssetBody)
	invalidPart, err := invalidWriter.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="replacement.png"`}, "Content-Type": {"image/jpeg"}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = invalidPart.Write([]byte("image bytes"))
	if err := invalidWriter.Close(); err != nil {
		t.Fatal(err)
	}
	invalidAssetRequest := httptest.NewRequest(http.MethodPost, "/skins/"+skinID+"/assets", &invalidAssetBody)
	invalidAssetRequest.Header.Set("Content-Type", invalidWriter.FormDataContentType())
	invalidAssetRequest.Header.Set("Authorization", "Bearer "+token)
	invalidAssetResponse := httptest.NewRecorder()
	r.ServeHTTP(invalidAssetResponse, invalidAssetRequest)
	if invalidAssetResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid asset upload=%d %s", invalidAssetResponse.Code, invalidAssetResponse.Body.String())
	}
	assetResponse = uploadSkinAssetRequest(t, r, token, skinID, "approved.png", "image/png", validPNG)
	if assetResponse.Code != http.StatusCreated {
		t.Fatalf("approved asset upload=%d %s", assetResponse.Code, assetResponse.Body.String())
	}
	approvedKey := signer.key
	publish := func(key string) SkinRevision {
		got := request(t, r, "POST", "/skins/"+skinID+"/revisions", `{"asset_key":"`+key+`","content_type":"image/png","reason":"approved artwork"}`, token)
		var rev SkinRevision
		_ = json.Unmarshal(got.Body.Bytes(), &rev)
		return rev
	}
	one := publish(approvedKey)
	if one.Version != 2 {
		t.Fatalf("version=%+v", one)
	}
	if got := request(t, r, "POST", "/skins/"+skinID+"/revisions", `{"asset_key":"skins/`+skinID+`/not-issued.png","content_type":"image/png"}`, token); got.Code != http.StatusBadRequest {
		t.Fatalf("unissued publish=%d %s", got.Code, got.Body.String())
	}
	upload = request(t, r, "POST", "/skins/"+skinID+"/uploads", `{"filename":"two.png","content_type":"image/png","size":100}`, token)
	var ticket struct {
		AssetKey string `json:"asset_key"`
	}
	_ = json.Unmarshal(upload.Body.Bytes(), &ticket)
	two := publish(ticket.AssetKey)
	if two.Version != 3 {
		t.Fatalf("second version=%+v", two)
	}
	if got := request(t, r, "POST", "/skins/"+skinID+"/revisions/"+one.ID+"/disable", "", token); got.Code != http.StatusBadRequest {
		t.Fatalf("disable without reason=%d", got.Code)
	}
	if got := request(t, r, "POST", "/skins/"+skinID+"/revisions/"+one.ID+"/disable", `{"reason":"superseded artwork"}`, token); got.Code != 204 {
		t.Fatalf("disable=%d", got.Code)
	}
	base := "/users/00000000-0000-0000-0000-000000000001/skins/" + skinID
	if got := request(t, r, "POST", base+"/grant", `{"revision_id":"`+one.ID+`","reason":"support correction"}`, token); got.Code != http.StatusNotFound {
		t.Fatalf("disabled grant=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "POST", base+"/grant", `{"revision_id":"`+two.ID+`","reason":"support correction"}`, token); got.Code != 201 {
		t.Fatalf("grant=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Metadata only","enabled":true,"catalog_visible":true,"reason":"safe edit"}`, token); got.Code != http.StatusOK {
		t.Fatalf("metadata after grant=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "PUT", "/skins/"+skinID, `{"name":"Rule edit","enabled":true,"catalog_visible":true,"reason":"unsafe edit","unlock_rules":[]}`, token); got.Code != http.StatusConflict {
		t.Fatalf("rules after grant=%d %s", got.Code, got.Body.String())
	}
	listed := request(t, r, "GET", "/skins", "", token)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"unlock_rules_locked":true`) {
		t.Fatalf("locked list=%d %s", listed.Code, listed.Body.String())
	}
	if got := request(t, r, "POST", base+"/revoke", `{"revision_id":"`+one.ID+`","reason":"mistake"}`, token); got.Code != http.StatusConflict {
		t.Fatalf("unrelated revoke=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, "POST", base+"/revoke", `{"revision_id":"`+two.ID+`","reason":"mistake"}`, token); got.Code != 201 || !strings.Contains(got.Body.String(), two.ID) {
		t.Fatalf("revoke=%d %s", got.Code, got.Body.String())
	}
	events := store.EntitlementEvents()
	if len(events) != 2 || events[1].RevisionID != two.ID {
		t.Fatalf("history=%+v", events)
	}
	if store.AuditEvents()[len(store.AuditEvents())-1].Action != "skin.entitlement.revoke" {
		t.Fatal("missing audit")
	}
}
func TestCreateSkinRequiresPermissionAndCreatesDraftWithAudit(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	manager := Admin{ID: "manager", Email: "manager@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"skins.manage"}}
	viewer := Admin{ID: "viewer", Email: "viewer@example.com", PasswordHash: string(hash), Status: "active"}
	store := NewMemoryStore(manager, viewer)
	router := newTestRouter(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store)
	login := func(email string) string {
		response := request(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth.AccessToken
	}
	body := `{"name":"  Night table  ","skin_type":"profile_background","description":"","display_order":4,"unlock_rules":[{"name":"First win","rule_type":"achievement","achievement_id":"first_win","retroactive":false,"enabled":true}],"reason":"new seasonal draft"}`
	if got := request(t, router, http.MethodPost, "/skins", body, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create=%d", got.Code)
	}
	if got := request(t, router, http.MethodPost, "/skins", body, login(viewer.Email)); got.Code != http.StatusForbidden {
		t.Fatalf("unpermitted create=%d", got.Code)
	}
	managerToken := login(manager.Email)
	created := request(t, router, http.MethodPost, "/skins", body, managerToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var skin Skin
	if err := json.Unmarshal(created.Body.Bytes(), &skin); err != nil {
		t.Fatal(err)
	}
	if skin.ID == "" || skin.Name != "Night table" || skin.SkinType != "profile_background" || skin.AssetKey != "" || skin.Enabled || skin.CatalogVisible || skin.IsStarter || skin.DisplayOrder != 4 {
		t.Fatalf("created skin=%+v", skin)
	}
	audit := store.AuditEvents()[len(store.AuditEvents())-1]
	if audit.Action != "skin.create" || audit.ResourceID != skin.ID || audit.Reason != "new seasonal draft" || audit.AdminID != manager.ID {
		t.Fatalf("create audit=%+v", audit)
	}
	for _, invalid := range []string{`{"name":"","skin_type":"avatar_frame","reason":"x"}`, `{"name":"Valid","skin_type":"unknown","reason":"x"}`, `{"name":"Valid","skin_type":"avatar_frame","display_order":-1,"reason":"x"}`, `{"name":"Valid","skin_type":"avatar_frame","reason":""}`, `{"name":"Valid","skin_type":"avatar_frame","reason":"x"}`} {
		if got := request(t, router, http.MethodPost, "/skins", invalid, managerToken); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid create %s = %d", invalid, got.Code)
		}
	}
}

func TestSkinUploadFailureAndPermission(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := NewMemoryStore(Admin{ID: "x", Email: "x@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"skins.manage"}}, Admin{ID: "y", Email: "y@example.com", PasswordHash: string(hash), Status: "active"})
	store.SetSkins(Skin{ID: "s", SkinType: "profile_background"})
	signer := &stubSkinSigner{err: errors.New("down")}
	h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{Storage: signer})
	r := gin.New()
	r.POST("/auth/login", h.Login)
	g := r.Group("")
	g.Use(h.RequireAuth)
	g.POST("/skins/:id/uploads", h.RequirePermission("skins.manage"), h.PresignSkinUpload)
	login := func(email string) string {
		got := request(t, r, "POST", "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var a AuthResponse
		_ = json.Unmarshal(got.Body.Bytes(), &a)
		return a.AccessToken
	}
	body := `{"filename":"x.png","content_type":"image/png","size":10}`
	if got := request(t, r, "POST", "/skins/s/uploads", body, login("x@example.com")); got.Code != 503 {
		t.Fatalf("failure=%d", got.Code)
	}
	if got := request(t, r, "POST", "/skins/s/uploads", body, login("y@example.com")); got.Code != 403 {
		t.Fatalf("permission=%d", got.Code)
	}
}

func TestAchievementManagementThroughAdminHTTP(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	manager := Admin{ID: "manager", Email: "achievements@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"achievements.read", "achievements.manage", "achievements.entitlements"}}
	viewer := Admin{ID: "viewer", Email: "viewer@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"achievements.read"}}
	store := NewMemoryStore(manager, viewer)
	userID := "00000000-0000-0000-0000-000000000001"
	store.SetUsers(UserDetail{User: User{ID: userID, Username: "ace", DisplayName: "Ace"}})
	store.SetAchievements(Achievement{ID: "first_win", Name: "First Blood", Description: "Win once", Icon: "trophy", DisplayOrder: 10, Enabled: true, Rules: []AchievementRule{{Metric: "is_winner", Operator: "eq", Value: "true"}}})
	h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{})
	r := gin.New()
	r.POST("/auth/login", h.Login)
	g := r.Group("")
	g.Use(h.RequireAuth)
	g.GET("/achievements", h.RequirePermission("achievements.read"), h.ListAchievements)
	g.POST("/achievements", h.RequirePermission("achievements.manage"), h.CreateAchievement)
	g.PUT("/achievements/:id", h.RequirePermission("achievements.manage"), h.UpdateAchievement)
	g.POST("/users/:id/achievements/:achievementID/grant", h.RequirePermission("achievements.entitlements"), h.ChangeAchievementEntitlement("grant"))
	g.POST("/users/:id/achievements/:achievementID/revoke", h.RequirePermission("achievements.entitlements"), h.ChangeAchievementEntitlement("revoke"))
	login := func(email string) string {
		response := request(t, r, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth.AccessToken
	}
	managerToken := login(manager.Email)
	create := request(t, r, http.MethodPost, "/achievements", `{"id":"season_winner","name":"Season Winner","description":"Win a season game","icon":"crown","display_order":30,"enabled":true,"rules":[{"metric":" wins ","operator":" gte ","value":" 1 "}],"reason":"new reward"}`, managerToken)
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"metric":"wins","operator":"gte","value":"1"`) {
		t.Fatalf("create normalized achievement=%d %s", create.Code, create.Body.String())
	}
	if got := request(t, r, http.MethodGet, "/achievements", "", managerToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"enabled":true`) {
		t.Fatalf("list=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPut, "/achievements/first_win", `{"name":"First Victory","description":"Win a game","icon":"medal","display_order":20,"enabled":false,"rules":[{"metric":" is_winner ","operator":" eq ","value":" true "}],"reason":"copy update"}`, managerToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"metric":"is_winner","operator":"eq","value":"true"`) {
		t.Fatalf("update=%d %s", got.Code, got.Body.String())
	}
	base := "/users/" + userID + "/achievements/first_win"
	body := `{"reason":"support correction","idempotency_key":"grant-1"}`
	if got := request(t, r, http.MethodPost, base+"/grant", body, managerToken); got.Code != http.StatusCreated {
		t.Fatalf("grant=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, base+"/grant", body, managerToken); got.Code != http.StatusOK {
		t.Fatalf("idempotent grant=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, base+"/revoke", `{"reason":"wrong operation","idempotency_key":"grant-1"}`, managerToken); got.Code != http.StatusConflict {
		t.Fatalf("reused idempotency key=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPut, "/achievements/first_win", `{"name":"First Victory","description":"Win a game","icon":"medal","display_order":20,"enabled":false,"rules":[{"metric":" is_winner ","operator":" eq ","value":" true "}],"reason":"copy update after grant"}`, managerToken); got.Code != http.StatusOK {
		t.Fatalf("normalized unchanged rules after grant=%d %s", got.Code, got.Body.String())
	}
	listed := request(t, r, http.MethodGet, "/achievements", "", managerToken)
	for _, rule := range []string{`"metric":"wins","operator":"gte","value":"1"`, `"metric":"is_winner","operator":"eq","value":"true"`} {
		if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), rule) {
			t.Fatalf("persisted normalized rules missing %s: %d %s", rule, listed.Code, listed.Body.String())
		}
	}
	if got := request(t, r, http.MethodPut, "/achievements/first_win", `{"name":"First Victory","description":"Win a game","icon":"medal","display_order":20,"enabled":false,"rules":[{"metric":"wins","operator":"gte","value":"1"}],"reason":"rule rewrite"}`, managerToken); got.Code != http.StatusConflict {
		t.Fatalf("rule rewrite=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, base+"/revoke", `{"reason":"grant was mistaken","idempotency_key":"revoke-1"}`, managerToken); got.Code != http.StatusCreated {
		t.Fatalf("revoke=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, base+"/grant", `{"reason":"missing key"}`, managerToken); got.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency key=%d", got.Code)
	}
	if got := request(t, r, http.MethodPost, base+"/grant", body, login(viewer.Email)); got.Code != http.StatusForbidden {
		t.Fatalf("unpermitted grant=%d", got.Code)
	}
	events := store.AchievementEntitlementEvents()
	if len(events) != 2 || events[0].Action != "grant" || events[1].Action != "revoke" {
		t.Fatalf("entitlement history=%+v", events)
	}
	foundAudit := false
	for _, audit := range store.AuditEvents() {
		if audit.Action == "achievement.entitlement.revoke" && audit.Reason == "grant was mistaken" && string(audit.BeforeState) == `{"entitled":true}` && string(audit.AfterState) == `{"entitled":false}` {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("missing revoke audit: %+v", store.AuditEvents())
	}
}

func TestEventLifecycleThroughAdminHTTP(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	manager := Admin{ID: "manager", Email: "events@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"events.read", "events.manage"}}
	viewer := Admin{ID: "viewer", Email: "event-viewer@example.com", PasswordHash: string(hash), Status: "active", Permissions: []string{"events.read"}}
	store := NewMemoryStore(manager, viewer)
	h := NewAdminHandler(Config{JWTSecret: "test-secret-at-least-32-bytes-long"}, store, Dependencies{})
	r := gin.New()
	r.POST("/auth/login", h.Login)
	g := r.Group("")
	g.Use(h.RequireAuth)
	g.GET("/events", h.RequirePermission("events.read"), h.ListEvents)
	g.GET("/events/:id", h.RequirePermission("events.read"), h.GetEvent)
	g.POST("/events", h.RequirePermission("events.manage"), h.CreateEvent)
	g.PUT("/events/:id", h.RequirePermission("events.manage"), h.UpdateEvent)
	g.POST("/events/:id/schedule", h.RequirePermission("events.manage"), h.ScheduleEvent)
	g.POST("/events/:id/publish", h.RequirePermission("events.manage"), h.PublishEvent)
	g.POST("/events/:id/archive", h.RequirePermission("events.manage"), h.ArchiveEvent)
	login := func(email string) string {
		response := request(t, r, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password"}`, "")
		var auth AuthResponse
		_ = json.Unmarshal(response.Body.Bytes(), &auth)
		return auth.AccessToken
	}
	managerToken := login(manager.Email)
	createBody := `{"slug":"harvest-week","name":"Harvest Week","summary":"Gather rewards","description":"Play daily.","starts_at":"2026-09-10T00:00:00Z","ends_at":"2026-09-17T00:00:00Z","reward_config":{"xp":100},"reason":"prepare campaign"}`
	created := request(t, r, http.MethodPost, "/events", createBody, managerToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var event model.Event
	if err := json.Unmarshal(created.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.State != "draft" || event.Version != 1 || event.Revision != 1 || event.ID == "" {
		t.Fatalf("created event=%+v", event)
	}
	if got := request(t, r, http.MethodGet, "/events/"+event.ID+"?preview=true", "", managerToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"event":{"id"`) || !strings.Contains(got.Body.String(), `"description":"Play daily."`) || !strings.Contains(got.Body.String(), `"skin_rewards":[]`) {
		t.Fatalf("preview=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, "/events/"+event.ID+"/schedule", `{"version":1,"reason":"dates approved"}`, managerToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"state":"scheduled"`) {
		t.Fatalf("schedule=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, "/events/"+event.ID+"/publish", `{"version":2,"reason":"launch approved"}`, managerToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"state":"published"`) {
		t.Fatalf("publish=%d %s", got.Code, got.Body.String())
	}
	update := `{"slug":"harvest-week","name":"Harvest Week Plus","summary":"Gather more rewards","description":"Play daily.","starts_at":"2026-09-10T00:00:00Z","ends_at":"2026-09-18T00:00:00Z","reward_config":{"xp":200},"version":3,"reason":"expand rewards"}`
	updated := request(t, r, http.MethodPut, "/events/"+event.ID, update, managerToken)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"revision":2`) || !strings.Contains(updated.Body.String(), `"state":"draft"`) {
		t.Fatalf("versioned update=%d %s", updated.Code, updated.Body.String())
	}
	if got := request(t, r, http.MethodPut, "/events/"+event.ID, update, managerToken); got.Code != http.StatusConflict {
		t.Fatalf("stale update=%d %s", got.Code, got.Body.String())
	}
	if got := request(t, r, http.MethodPost, "/events", createBody, login(viewer.Email)); got.Code != http.StatusForbidden {
		t.Fatalf("unpermitted create=%d", got.Code)
	}
	if got := request(t, r, http.MethodPost, "/events", `{"slug":"bad","name":"Bad","summary":"x","description":"x","starts_at":"2026-09-17T00:00:00Z","ends_at":"2026-09-10T00:00:00Z","reward_config":{},"reason":"invalid"}`, managerToken); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid dates=%d", got.Code)
	}
	invalidReward := `{"slug":"bad-reward","name":"Bad Reward","summary":"x","description":"x","starts_at":"2026-09-10T00:00:00Z","ends_at":"2026-09-17T00:00:00Z","reward_config":{"daily_login":{"enabled":true,"xp_per_claim":0}},"reason":"invalid"}`
	if got := request(t, r, http.MethodPost, "/events", invalidReward, managerToken); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid daily login reward=%d %s", got.Code, got.Body.String())
	}
	foundPublishAudit := false
	for _, audit := range store.AuditEvents() {
		if audit.Action == "event.publish" && audit.ResourceID == event.ID && audit.Reason == "launch approved" && audit.ID != "" {
			foundPublishAudit = true
		}
	}
	if !foundPublishAudit {
		t.Fatalf("missing publish audit: %+v", store.AuditEvents())
	}
}
