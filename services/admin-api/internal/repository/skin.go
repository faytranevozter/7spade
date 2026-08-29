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

type Skin = model.Skin
type SkinRevision = model.SkinRevision
type SkinEntitlementEvent = model.SkinEntitlementEvent

func (s *PostgresStore) ListSkins(ctx context.Context) ([]Skin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,skin_type,name,description,asset_key,is_starter,display_order,enabled,catalog_visible,EXISTS(SELECT 1 FROM user_skin_entitlement_events e WHERE e.skin_id=skins.id) FROM skins ORDER BY display_order,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skins []Skin
	for rows.Next() {
		var skin Skin
		if err = rows.Scan(&skin.ID, &skin.SkinType, &skin.Name, &skin.Description, &skin.AssetKey, &skin.IsStarter, &skin.DisplayOrder, &skin.Enabled, &skin.CatalogVisible, &skin.UnlockRulesLocked); err != nil {
			return nil, err
		}
		if err = loadSkinContent(ctx, s.db, &skin); err != nil {
			return nil, err
		}
		skins = append(skins, skin)
	}
	return skins, rows.Err()
}
func (s *PostgresStore) SkinExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM skins WHERE id=$1)`, id).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func loadSkinContent(ctx context.Context, db *sql.DB, skin *Skin) error {
	rows, err := db.QueryContext(ctx, `SELECT id,version,asset_key,content_type,enabled,published_at,disabled_at FROM skin_revisions WHERE skin_id=$1 ORDER BY version DESC`, skin.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r SkinRevision
		r.SkinID = skin.ID
		if err = rows.Scan(&r.ID, &r.Version, &r.AssetKey, &r.ContentType, &r.Enabled, &r.PublishedAt, &r.DisabledAt); err != nil {
			return err
		}
		skin.Revisions = append(skin.Revisions, r)
	}
	ruleRows, err := db.QueryContext(ctx, `SELECT id,name,rule_type,COALESCE(achievement_id,''),minimum_level,login_streak_days,COALESCE(event_id::text,''),event_check_in_count,retroactive,enabled,COALESCE((SELECT jsonb_agg(jsonb_build_object('metric',metric,'operator',operator,'value',value) ORDER BY id) FROM skin_unlock_rule_conditions WHERE skin_unlock_rule_id=r.id),'[]') FROM skin_unlock_rules r WHERE skin_id=$1 ORDER BY created_at,id`, skin.ID)
	if err != nil {
		return err
	}
	defer ruleRows.Close()
	for ruleRows.Next() {
		var r model.SkinUnlockRule
		if err = ruleRows.Scan(&r.ID, &r.Name, &r.RuleType, &r.AchievementID, &r.MinimumLevel, &r.LoginStreakDays, &r.EventID, &r.EventCheckInCount, &r.Retroactive, &r.Enabled, &r.Conditions); err != nil {
			return err
		}
		skin.UnlockRules = append(skin.UnlockRules, r)
	}
	return ruleRows.Err()
}
func (s *PostgresStore) CreateSkin(ctx context.Context, skin Skin, event AuditEvent) (Skin, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Skin{}, err
	}
	defer tx.Rollback()
	skin.ID = uuid.NewString()
	skin.AssetKey = ""
	skin.IsStarter = false
	skin.Enabled = false
	skin.CatalogVisible = false
	if _, err = tx.ExecContext(ctx, `INSERT INTO skins(id,skin_type,name,description,asset_key,is_starter,display_order,enabled,catalog_visible) VALUES($1,$2,$3,$4,$5,FALSE,$6,FALSE,FALSE)`, skin.ID, skin.SkinType, skin.Name, skin.Description, skin.AssetKey, skin.DisplayOrder); err != nil {
		return Skin{}, err
	}
	event.ResourceID = skin.ID
	if err = appendAudit(ctx, tx, event); err != nil {
		return Skin{}, err
	}
	return skin, tx.Commit()
}

func (s *PostgresStore) UpdateSkin(ctx context.Context, id string, next Skin, event AuditEvent) (Skin, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Skin{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE skins SET name=$2,description=$3,display_order=$4,enabled=$5,catalog_visible=$6,updated_at=NOW() WHERE id=$1`, id, next.Name, next.Description, next.DisplayOrder, next.Enabled, next.CatalogVisible)
	if err != nil {
		return Skin{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return Skin{}, ErrNotFound
	}
	var granted bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_skin_entitlement_events WHERE skin_id=$1)`, id).Scan(&granted); err != nil {
		return Skin{}, err
	}
	if granted && next.UnlockRules != nil {
		return Skin{}, ErrConflict
	}
	if !granted {
		if _, err = tx.ExecContext(ctx, `DELETE FROM skin_unlock_rules WHERE skin_id=$1`, id); err != nil {
			return Skin{}, err
		}
	}
	for _, r := range next.UnlockRules {
		rid := uuid.NewString()
		if _, err = tx.ExecContext(ctx, `INSERT INTO skin_unlock_rules(id,name,skin_id,rule_type,achievement_id,minimum_level,login_streak_days,event_id,event_check_in_count,retroactive,enabled) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,''),$9,$10,$11)`, rid, r.Name, id, r.RuleType, r.AchievementID, r.MinimumLevel, r.LoginStreakDays, r.EventID, r.EventCheckInCount, r.Retroactive, r.Enabled); err != nil {
			return Skin{}, err
		}
		var conditions []struct{ Metric, Operator, Value string }
		if len(r.Conditions) > 0 && string(r.Conditions) != "null" {
			if err = json.Unmarshal(r.Conditions, &conditions); err != nil {
				return Skin{}, err
			}
		}
		for _, c := range conditions {
			if _, err = tx.ExecContext(ctx, `INSERT INTO skin_unlock_rule_conditions(skin_unlock_rule_id,metric,operator,value) VALUES($1,$2,$3,$4)`, rid, c.Metric, c.Operator, c.Value); err != nil {
				return Skin{}, err
			}
		}
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return Skin{}, err
	}
	if err = tx.Commit(); err != nil {
		return Skin{}, err
	}
	next.ID = id
	return next, nil
}
func (s *PostgresStore) PublishSkinRevision(ctx context.Context, skinID, key, contentType string, event AuditEvent) (SkinRevision, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SkinRevision{}, err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT MAX(version) FROM skin_revisions WHERE skin_id=$1),0)+1 FROM skins WHERE id=$1 FOR UPDATE`, skinID).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return SkinRevision{}, ErrNotFound
	} else if err != nil {
		return SkinRevision{}, err
	}
	r := SkinRevision{ID: uuid.NewString(), SkinID: skinID, Version: version, AssetKey: key, ContentType: contentType, Enabled: true, PublishedAt: time.Now()}
	result, err := tx.ExecContext(ctx, `UPDATE skins SET asset_key=$2,updated_at=NOW() WHERE id=$1`, skinID, key)
	if err != nil {
		return SkinRevision{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return SkinRevision{}, ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO skin_revisions(id,skin_id,version,asset_key,content_type,enabled,published_by,published_at)VALUES($1,$2,$3,$4,$5,TRUE,$6,$7)`, r.ID, skinID, version, key, contentType, event.AdminID, r.PublishedAt); err != nil {
		return SkinRevision{}, err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return SkinRevision{}, err
	}
	return r, tx.Commit()
}
func (s *PostgresStore) DisableSkinRevision(ctx context.Context, skinID, revisionID string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE skin_revisions SET enabled=FALSE,disabled_at=NOW(),disabled_by=$3 WHERE id=$1 AND skin_id=$2 AND disabled_at IS NULL`, revisionID, skinID, event.AdminID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE skins SET asset_key='',enabled=FALSE,catalog_visible=FALSE,updated_at=NOW() WHERE id=$1 AND asset_key=(SELECT asset_key FROM skin_revisions WHERE id=$2)`, skinID, revisionID); err != nil {
		return err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *PostgresStore) ChangeSkinEntitlement(ctx context.Context, userID, skinID, revisionID, action, reason string, event AuditEvent) (SkinEntitlementEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SkinEntitlementEvent{}, err
	}
	defer tx.Rollback()
	var userExists, revisionEnabled bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, userID).Scan(&userExists); err != nil || !userExists {
		if err == nil {
			err = ErrNotFound
		}
		return SkinEntitlementEvent{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM skin_revisions WHERE id=$1 AND skin_id=$2 AND enabled)`, revisionID, skinID).Scan(&revisionEnabled); err != nil || action == "grant" && !revisionEnabled {
		if err == nil {
			err = ErrNotFound
		}
		return SkinEntitlementEvent{}, err
	}
	if action == "grant" {
		_, err = tx.ExecContext(ctx, `INSERT INTO user_skins(user_id,skin_id,source,skin_revision_id)VALUES($1,$2,'admin_grant',$3)`, userID, skinID, revisionID)
	} else {
		var granted string
		err = tx.QueryRowContext(ctx, `DELETE FROM user_skins WHERE user_id=$1 AND skin_id=$2 AND source='admin_grant' AND skin_revision_id=$3 RETURNING skin_revision_id`, userID, skinID, revisionID).Scan(&granted)
		if err == nil {
			revisionID = granted
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return SkinEntitlementEvent{}, ErrConflict
	}
	if err != nil {
		return SkinEntitlementEvent{}, err
	}
	e := SkinEntitlementEvent{ID: uuid.NewString(), UserID: userID, SkinID: skinID, RevisionID: revisionID, Action: action, Reason: reason, AdminID: event.AdminID, OccurredAt: time.Now()}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_skin_entitlement_events(id,user_id,skin_id,skin_revision_id,action,reason,admin_user_id,occurred_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, userID, skinID, revisionID, action, reason, event.AdminID, e.OccurredAt)
	if err != nil {
		return SkinEntitlementEvent{}, err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return SkinEntitlementEvent{}, err
	}
	return e, tx.Commit()
}
