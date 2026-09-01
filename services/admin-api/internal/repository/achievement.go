package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
)

func (s *PostgresStore) ListAchievements(ctx context.Context) ([]model.Achievement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.name,a.description,a.icon,a.display_order,a.enabled,EXISTS(SELECT 1 FROM user_achievements ua WHERE ua.achievement_id=a.id) FROM achievements a ORDER BY a.display_order,a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	achievements := []model.Achievement{}
	for rows.Next() {
		var achievement model.Achievement
		if err := rows.Scan(&achievement.ID, &achievement.Name, &achievement.Description, &achievement.Icon, &achievement.DisplayOrder, &achievement.Enabled, &achievement.RulesLocked); err != nil {
			return nil, err
		}
		if err := loadAchievementRules(ctx, s.db, &achievement); err != nil {
			return nil, err
		}
		achievements = append(achievements, achievement)
	}
	return achievements, rows.Err()
}

func loadAchievementRules(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, achievement *model.Achievement) error {
	rows, err := db.QueryContext(ctx, `SELECT metric,operator,value FROM achievement_rules WHERE achievement_id=$1 ORDER BY id`, achievement.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rule model.AchievementRule
		if err := rows.Scan(&rule.Metric, &rule.Operator, &rule.Value); err != nil {
			return err
		}
		achievement.Rules = append(achievement.Rules, rule)
	}
	return rows.Err()
}

func sameAchievementRules(left, right []model.AchievementRule) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (s *PostgresStore) CreateAchievement(ctx context.Context, achievement model.Achievement, event AuditEvent) (model.Achievement, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Achievement{}, err
	}
	defer tx.Rollback()
	var createdID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO achievements(id,name,description,icon,display_order,enabled) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING RETURNING id`, achievement.ID, achievement.Name, achievement.Description, achievement.Icon, achievement.DisplayOrder, achievement.Enabled).Scan(&createdID); errors.Is(err, sql.ErrNoRows) {
		return model.Achievement{}, ErrConflict
	} else if err != nil {
		return model.Achievement{}, err
	}
	for _, rule := range achievement.Rules {
		if _, err := tx.ExecContext(ctx, `INSERT INTO achievement_rules(achievement_id,metric,operator,value) VALUES($1,$2,$3,$4)`, achievement.ID, rule.Metric, rule.Operator, rule.Value); err != nil {
			return model.Achievement{}, err
		}
	}
	event.AfterState, _ = json.Marshal(achievement)
	if err := appendAudit(ctx, tx, event); err != nil {
		return model.Achievement{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Achievement{}, err
	}
	return achievement, nil
}

func (s *PostgresStore) UpdateAchievement(ctx context.Context, id string, next model.Achievement, event AuditEvent) (model.Achievement, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Achievement{}, err
	}
	defer tx.Rollback()
	var current model.Achievement
	if err := tx.QueryRowContext(ctx, `SELECT id,name,description,icon,display_order,enabled,EXISTS(SELECT 1 FROM user_achievements ua WHERE ua.achievement_id=achievements.id) FROM achievements WHERE id=$1 FOR UPDATE`, id).Scan(&current.ID, &current.Name, &current.Description, &current.Icon, &current.DisplayOrder, &current.Enabled, &current.RulesLocked); errors.Is(err, sql.ErrNoRows) {
		return model.Achievement{}, ErrNotFound
	} else if err != nil {
		return model.Achievement{}, err
	}
	if err := loadAchievementRules(ctx, tx, &current); err != nil {
		return model.Achievement{}, err
	}
	if current.RulesLocked && !sameAchievementRules(current.Rules, next.Rules) {
		return model.Achievement{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE achievements SET name=$2,description=$3,icon=$4,display_order=$5,enabled=$6,updated_at=NOW() WHERE id=$1`, id, next.Name, next.Description, next.Icon, next.DisplayOrder, next.Enabled); err != nil {
		return model.Achievement{}, err
	}
	if !current.RulesLocked {
		if _, err := tx.ExecContext(ctx, `DELETE FROM achievement_rules WHERE achievement_id=$1`, id); err != nil {
			return model.Achievement{}, err
		}
		for _, rule := range next.Rules {
			if _, err := tx.ExecContext(ctx, `INSERT INTO achievement_rules(achievement_id,metric,operator,value) VALUES($1,$2,$3,$4)`, id, rule.Metric, rule.Operator, rule.Value); err != nil {
				return model.Achievement{}, err
			}
		}
	}
	next.ID, next.RulesLocked = id, current.RulesLocked
	event.BeforeState, _ = json.Marshal(current)
	event.AfterState, _ = json.Marshal(next)
	if err := appendAudit(ctx, tx, event); err != nil {
		return model.Achievement{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Achievement{}, err
	}
	return next, nil
}

func (s *PostgresStore) ChangeAchievementEntitlement(ctx context.Context, userID, achievementID, action, reason, key string, event AuditEvent) (model.AchievementEntitlementEvent, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	defer tx.Rollback()
	var existing model.AchievementEntitlementEvent
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,achievement_id,action,reason,idempotency_key,COALESCE(admin_user_id::text,''),occurred_at FROM user_achievement_entitlement_events WHERE idempotency_key=$1`, key).Scan(&existing.ID, &existing.UserID, &existing.AchievementID, &existing.Action, &existing.Reason, &existing.IdempotencyKey, &existing.AdminID, &existing.OccurredAt)
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.AchievementEntitlementEvent{}, false, err
	}
	var userExists, achievementExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1), EXISTS(SELECT 1 FROM achievements WHERE id=$2)`, userID, achievementID).Scan(&userExists, &achievementExists); err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	if !userExists || !achievementExists {
		return model.AchievementEntitlementEvent{}, false, ErrNotFound
	}
	if action == "grant" {
		_, err = tx.ExecContext(ctx, `INSERT INTO user_achievements(user_id,achievement_id) VALUES($1,$2)`, userID, achievementID)
	} else {
		var removed string
		err = tx.QueryRowContext(ctx, `DELETE FROM user_achievements WHERE user_id=$1 AND achievement_id=$2 RETURNING achievement_id`, userID, achievementID).Scan(&removed)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.AchievementEntitlementEvent{}, false, ErrConflict
	}
	if err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	result := model.AchievementEntitlementEvent{ID: uuid.NewString(), UserID: userID, AchievementID: achievementID, Action: action, Reason: reason, IdempotencyKey: key, AdminID: event.AdminID, OccurredAt: time.Now().UTC()}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_achievement_entitlement_events(id,user_id,achievement_id,action,reason,idempotency_key,admin_user_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,$8)`, result.ID, result.UserID, result.AchievementID, result.Action, result.Reason, result.IdempotencyKey, result.AdminID, result.OccurredAt); err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	event.BeforeState = []byte(`{"entitled":false}`)
	if action == "revoke" {
		event.BeforeState = []byte(`{"entitled":true}`)
	}
	event.AfterState = []byte(`{"entitled":true}`)
	if action == "revoke" {
		event.AfterState = []byte(`{"entitled":false}`)
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.AchievementEntitlementEvent{}, false, err
	}
	return result, false, nil
}
