package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func TestUpdateFeatureSettingCommitsSettingAndAuditTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewPostgresStore(db, "test")
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT type, value, updated_at FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"type", "value", "updated_at"}).AddRow("boolean", []byte(`true`), now))
	mock.ExpectQuery("UPDATE feature_settings").WithArgs("false", "daily_login").WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(now.Add(time.Second)))
	mock.ExpectExec("INSERT INTO admin_audit_events").WithArgs(sqlmock.AnyArg(), nil, nil, "", "setting.daily_login.update", "feature_setting", "daily_login", "maintenance", "success", []byte(`{"key":"daily_login","type":"boolean","value":true}`), []byte(`{"key":"daily_login","type":"boolean","value":false}`), nil, "", "", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	setting, err := store.UpdateFeatureSetting(context.Background(), "daily_login", json.RawMessage(`false`), model.AuditEvent{Action: "setting.daily_login.update", ResourceType: "feature_setting", ResourceID: "daily_login", Reason: "maintenance", Outcome: "success"})
	if err != nil || string(setting.Value) != "false" {
		t.Fatalf("UpdateFeatureSetting = %+v, %v", setting, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateFeatureSettingRollsBackWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewPostgresStore(db, "test")
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT type, value, updated_at FROM feature_settings").WithArgs("daily_login").WillReturnRows(sqlmock.NewRows([]string{"type", "value", "updated_at"}).AddRow("boolean", []byte(`true`), now))
	mock.ExpectQuery("UPDATE feature_settings").WithArgs("false", "daily_login").WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(now.Add(time.Second)))
	mock.ExpectExec("INSERT INTO admin_audit_events").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	if _, err := store.UpdateFeatureSetting(context.Background(), "daily_login", json.RawMessage(`false`), model.AuditEvent{Action: "setting.daily_login.update"}); err == nil {
		t.Fatal("UpdateFeatureSetting succeeded when audit failed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
