package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

func TestNilFeatureLookupUsesDatabase(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enabled   bool
		lookupErr error
		status    int
	}{
		{"enabled", true, nil, 204}, {"disabled", false, nil, 503},
		{"missing", false, sql.ErrNoRows, 500}, {"failure", false, errors.New("offline"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			q := mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs(repository.SettingGuestAccess)
			if tc.lookupErr != nil {
				q.WillReturnError(tc.lookupErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(tc.enabled))
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if requireFeatureEnabled(c, db, nil, repository.SettingGuestAccess, "disabled") {
				c.Status(204)
			}
			c.Writer.WriteHeaderNow()
			if w.Code != tc.status {
				t.Fatalf("response = %d %s", w.Code, w.Body.String())
			}
			if tc.status == 503 && !strings.Contains(w.Body.String(), `"code":"feature_disabled"`) {
				t.Fatal(w.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNilFeatureLookupWithoutDatabaseFailsClosed(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if requireFeatureEnabled(c, nil, nil, repository.SettingGuestAccess, "disabled") || w.Code != http.StatusInternalServerError {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}

func TestOAuthRegistrationControl(t *testing.T) {
	for _, flow := range []string{"web", "google", "telegram"} {
		for _, account := range []string{"new", "existing", "email"} {
			t.Run(flow+"/"+account, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				id := uuid.New()
				provider := "google"
				if flow == "telegram" {
					provider = "telegram"
				}
				profile := repository.OAuthProfile{Provider: provider, ProviderUserID: "identity", Email: "alice@example.com", DisplayName: "Alice"}
				mock.ExpectBegin()
				mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(provider + ":identity").WillReturnResult(sqlmock.NewResult(0, 1))
				q := mock.ExpectQuery("SELECT user_id FROM user_providers").WithArgs(provider, "identity")
				if account == "existing" {
					q.WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(id))
				} else {
					q.WillReturnError(sql.ErrNoRows)
					q = mock.ExpectQuery("SELECT id FROM users WHERE email").WithArgs(profile.Email)
					if account == "email" {
						q.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
					} else {
						q.WillReturnError(sql.ErrNoRows)
					}
				}
				status := http.StatusOK
				if account == "new" {
					mock.ExpectQuery("SELECT enabled FROM feature_settings.*FOR SHARE").WithArgs(repository.SettingNewRegistrations).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
					mock.ExpectRollback()
					status = 503
				} else if account == "email" && flow != "web" {
					mock.ExpectRollback()
					status = 409
				} else {
					mock.ExpectQuery("UPDATE users SET display_name").WithArgs("Alice", id).WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "username", "created_at"}).AddRow(id, profile.Email, nil, "Alice", "alice", time.Now()))
					mock.ExpectExec("INSERT INTO user_providers").WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit()
					mock.ExpectQuery("SELECT suspended_at IS NOT NULL").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"suspended"}).AddRow(false))
					mock.ExpectExec("INSERT INTO refresh_tokens").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				h := OAuthHandler{DB: db, Redis: newTestRedisClient(t), JWTSecret: "test-secret"}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				switch flow {
				case "web":
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"access_token":"provider-token"}`))
					}))
					defer srv.Close()
					h.Providers = map[string]OAuthProviderConfig{provider: {TokenURL: srv.URL, FetchUser: func(context.Context, *http.Client, tokenResponse) (repository.OAuthProfile, error) {
						return profile, nil
					}}}
					if err := h.Redis.StoreOAuthState(context.Background(), "state", "verifier", provider, "", time.Minute); err != nil {
						t.Fatal(err)
					}
					c.Params = gin.Params{{Key: "provider", Value: provider}}
					c.Request = httptest.NewRequest("POST", "/callback", strings.NewReader(`{"code":"code","state":"state"}`))
					h.Callback(c)
				case "google":
					h.Providers = map[string]OAuthProviderConfig{"google": {ClientID: "client"}}
					h.VerifyIDToken = func(context.Context, string, string, string, string) (map[string]any, error) {
						return map[string]any{"sub": "identity", "email": profile.Email, "email_verified": true, "name": "Alice"}, nil
					}
					c.Request = httptest.NewRequest("POST", "/google", strings.NewReader(`{"id_token":"token"}`))
					h.MobileGoogle(c)
				case "telegram":
					encoded, err := json.Marshal(profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := h.Redis.StoreMobileOAuthHandoff(context.Background(), "handoff", codeChallenge("verifier"), encoded, time.Minute); err != nil {
						t.Fatal(err)
					}
					c.Request = httptest.NewRequest("POST", "/exchange", strings.NewReader(`{"code":"handoff","code_verifier":"verifier"}`))
					h.MobileTelegramExchange(c)
				}
				if w.Code != status {
					t.Fatalf("response = %d %s, want %d", w.Code, w.Body.String(), status)
				}
				if status == 503 && !strings.Contains(w.Body.String(), `"code":"feature_disabled"`) {
					t.Fatal(w.Body.String())
				}
				if status == 200 && !strings.Contains(w.Body.String(), `"access_token":`) {
					t.Fatal(w.Body.String())
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestQuickPlayCreationControl(t *testing.T) {
	for _, matching := range []bool{true, false} {
		t.Run(fmt.Sprint("matching=", matching), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			id, roomID := uuid.New(), uuid.New()
			mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs(repository.SettingQuickPlay).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
			mock.ExpectBegin()
			mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(id.String()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("FROM room_players rp").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"id", "invite_code", "status", "practice_mode"}))
			rows := sqlmock.NewRows([]string{"id", "invite_code", "name", "visibility", "turn_timer_seconds", "bot_difficulty", "practice_mode", "min_elo", "max_elo", "status", "created_by", "created_at", "player_count"})
			if matching {
				rows.AddRow(roomID, "ABC123", "Room", "public", 60, "medium", false, nil, nil, "waiting", uuid.New(), time.Now(), 1)
			}
			mock.ExpectQuery("WITH candidate AS").WithArgs(60, "medium", id, false, nil).WillReturnRows(rows)
			status := 200
			if matching {
				// Existing-room joins must not depend on the creation setting at all.
				mock.ExpectExec("INSERT INTO room_players").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectQuery("SELECT enabled FROM feature_settings.*FOR SHARE").WithArgs(repository.SettingRoomCreation).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
				mock.ExpectRollback()
				status = 503
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(middleware.ClaimsKey, &auth.Claims{Sub: id.String(), DisplayName: "Alice", IsGuest: true})
			c.Request = httptest.NewRequest("POST", "/quick-play", nil)
			RoomHandler{DB: db}.QuickPlay(c)
			if w.Code != status {
				t.Fatalf("response = %d %s", w.Code, w.Body.String())
			}
			if !matching && !strings.Contains(w.Body.String(), `"code":"feature_disabled"`) {
				t.Fatal(w.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDisabledApplicationControlsRejectNewActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabled := func(_ *sql.DB, _ string) (bool, error) { return false, nil }
	tests := []struct {
		name    string
		path    string
		body    string
		handler gin.HandlerFunc
		message string
	}{
		{"guest access", "/guest", `{"display_name":"Guest"}`, AuthHandler{FeatureEnabled: disabled}.Guest, "Guest access is temporarily unavailable"},
		{"registration", "/register", `{}`, AuthHandler{FeatureEnabled: disabled}.Register, "New registrations are temporarily unavailable"},
		{"room creation", "/rooms", `{}`, RoomHandler{FeatureEnabled: disabled}.Create, "Room creation is temporarily unavailable"},
		{"quick play", "/rooms/quick-play", ``, RoomHandler{FeatureEnabled: disabled}.QuickPlay, "Quick Play is temporarily unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.POST(tt.path, tt.handler)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body)))
			if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), tt.message) || !strings.Contains(response.Body.String(), `"code":"feature_disabled"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestApplicationControlLookupFailureFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	failing := func(_ *sql.DB, _ string) (bool, error) { return false, errors.New("database unavailable") }
	r := gin.New()
	r.POST("/guest", AuthHandler{FeatureEnabled: failing}.Guest)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/guest", strings.NewReader(`{"display_name":"Guest"}`)))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}
