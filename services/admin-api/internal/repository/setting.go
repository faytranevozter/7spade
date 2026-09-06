package repository

import (
	"context"
	"fmt"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
)

func (s *PostgresStore) GetFeatureSetting(ctx context.Context, key string) (model.FeatureSetting, error) {
	var setting model.FeatureSetting
	err := s.db.QueryRowContext(ctx, `SELECT key, enabled FROM feature_settings WHERE key = $1`, key).Scan(&setting.Key, &setting.Enabled)
	if err != nil {
		return model.FeatureSetting{}, fmt.Errorf("get feature setting: %w", err)
	}
	return setting, nil
}

func (s *PostgresStore) UpdateFeatureSetting(ctx context.Context, key string, enabled bool, audit model.AuditEvent) (model.FeatureSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.FeatureSetting{}, err
	}
	defer tx.Rollback()

	var before bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled FROM feature_settings WHERE key = $1 FOR UPDATE`, key).Scan(&before); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("lock feature setting: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE feature_settings SET enabled = $1, updated_at = NOW() WHERE key = $2`, enabled, key); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("update feature setting: %w", err)
	}
	audit.BeforeState = featureSettingState(before)
	audit.AfterState = featureSettingState(enabled)
	if err := appendAudit(ctx, tx, audit); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("append feature setting audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.FeatureSetting{}, fmt.Errorf("commit feature setting: %w", err)
	}
	return model.FeatureSetting{Key: key, Enabled: enabled}, nil
}

func featureSettingState(enabled bool) []byte {
	if enabled {
		return []byte(`{"enabled":true}`)
	}
	return []byte(`{"enabled":false}`)
}
