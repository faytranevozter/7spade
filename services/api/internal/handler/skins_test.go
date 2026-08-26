package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestSkinCatalogReturnsStructuredRulesAndEmptyRuleArrays(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	columns := []string{
		"id", "skin_type", "name", "description", "asset_key", "display_order",
		"rule_id", "rule_name", "rule_type", "achievement_id", "achievement_name",
		"minimum_level", "login_streak_days", "event_check_in_count",
		"event_slug", "event_name", "event_starts_at", "event_ends_at",
		"condition_id", "condition_metric", "condition_operator", "condition_value",
	}
	mock.ExpectQuery("SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow("skin-1", "avatar_frame", "Starter", "Starter frame", "starter.svg", 1,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	SkinHandler{DB: db}.Catalog(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Skins []struct {
			UnlockRules []json.RawMessage `json:"unlock_rules"`
		} `json:"skins"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Skins) != 1 || response.Skins[0].UnlockRules == nil || len(response.Skins[0].UnlockRules) != 0 {
		t.Fatalf("response = %s, want non-null empty unlock_rules", recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSkinCatalogReturnsEmptySkinArray(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "skin_type", "name", "description", "asset_key", "display_order",
			"rule_id", "rule_name", "rule_type", "achievement_id", "achievement_name",
			"minimum_level", "login_streak_days", "event_check_in_count", "event_slug", "event_name",
			"event_starts_at", "event_ends_at", "condition_id", "condition_metric", "condition_operator", "condition_value",
		}))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	SkinHandler{DB: db}.Catalog(context)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "{\"skins\":[]}" {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSkinCatalogReturnsInternalServerError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT s.id, s.skin_type, s.name, s.description, s.asset_key, s.display_order").
		WillReturnError(sql.ErrConnDone)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	SkinHandler{DB: db}.Catalog(context)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
