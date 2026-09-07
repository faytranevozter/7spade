package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
)

func TestInternalApplicationControlsRequiresSharedSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	router := NewRouter(&config.Config{InternalSecret: "shared-secret"}, db, nil)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/internal/application-controls", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	keys := []string{
		repository.SettingNewRegistrations,
		repository.SettingGuestAccess,
		repository.SettingRoomCreation,
		repository.SettingQuickPlay,
		repository.SettingNewGameStarts,
		repository.SettingSpectatorAccess,
		repository.SettingEmotes,
	}
	for _, key := range keys {
		mock.ExpectQuery("SELECT enabled FROM feature_settings").WithArgs(key).
			WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/application-controls", nil)
	req.Header.Set("X-Internal-Secret", "shared-secret")
	authorized := httptest.NewRecorder()
	router.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized response = %d %s", authorized.Code, authorized.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
