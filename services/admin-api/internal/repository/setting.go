package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func (s *PostgresStore) GetFeatureSetting(ctx context.Context, key string) (model.FeatureSetting, error) {
	var setting model.FeatureSetting
	err := s.db.QueryRowContext(ctx, `SELECT key, type, value, updated_at FROM feature_settings WHERE key = $1`, key).Scan(&setting.Key, &setting.Type, &setting.Value, &setting.UpdatedAt)
	if err != nil {
		return model.FeatureSetting{}, fmt.Errorf("get feature setting: %w", err)
	}
	return setting, nil
}

func (s *PostgresStore) ListFeatureSettings(ctx context.Context) ([]model.FeatureSetting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, type, value, updated_at FROM feature_settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list feature settings: %w", err)
	}
	defer rows.Close()
	settings := make([]model.FeatureSetting, 0)
	for rows.Next() {
		var setting model.FeatureSetting
		if err := rows.Scan(&setting.Key, &setting.Type, &setting.Value, &setting.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan feature setting: %w", err)
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feature settings: %w", err)
	}
	return settings, nil
}

func (s *PostgresStore) UpdateFeatureSetting(ctx context.Context, key string, value json.RawMessage, audit model.AuditEvent) (model.FeatureSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.FeatureSetting{}, err
	}
	defer tx.Rollback()

	before := model.FeatureSetting{Key: key}
	if err := tx.QueryRowContext(ctx, `SELECT type, value, updated_at FROM feature_settings WHERE key = $1 FOR UPDATE`, key).Scan(&before.Type, &before.Value, &before.UpdatedAt); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("lock feature setting: %w", err)
	}
	after := before
	after.Value = value
	if err := tx.QueryRowContext(ctx, `UPDATE feature_settings SET value = $1::jsonb, updated_at = NOW() WHERE key = $2 RETURNING updated_at`, string(value), key).Scan(&after.UpdatedAt); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("update feature setting: %w", err)
	}
	audit.BeforeState = featureSettingState(before)
	audit.AfterState = featureSettingState(after)
	if err := appendAudit(ctx, tx, audit); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("append feature setting audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("commit feature setting: %w", err)
	}
	return after, nil
}

func featureSettingState(setting model.FeatureSetting) []byte {
	state, _ := json.Marshal(struct {
		Key   string          `json:"key"`
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}{Key: setting.Key, Type: setting.Type, Value: setting.Value})
	return state
}
