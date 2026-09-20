package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
)

type Skin = model.Skin
type SkinRevision = model.SkinRevision
type SkinEntitlementEvent = model.SkinEntitlementEvent

func (s *PostgresStore) ListSkins(ctx context.Context) ([]Skin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,skin_type,name,description,asset_key,is_starter,display_order,enabled,catalog_visible,EXISTS(SELECT 1 FROM user_skin_entitlement_events e WHERE e.skin_id=skins.id) OR EXISTS(SELECT 1 FROM user_skins us WHERE us.skin_id=skins.id AND us.skin_unlock_rule_id IS NOT NULL) FROM skins ORDER BY display_order,id`)
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
func (s *PostgresStore) GetSkin(ctx context.Context, id string) (Skin, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Skin{}, ErrNotFound
	}
	var skin Skin
	err := s.db.QueryRowContext(ctx, `SELECT id,skin_type,name,description,asset_key,is_starter,display_order,enabled,catalog_visible,EXISTS(SELECT 1 FROM user_skin_entitlement_events e WHERE e.skin_id=skins.id) OR EXISTS(SELECT 1 FROM user_skins us WHERE us.skin_id=skins.id AND us.skin_unlock_rule_id IS NOT NULL) FROM skins WHERE id=$1`, id).Scan(&skin.ID, &skin.SkinType, &skin.Name, &skin.Description, &skin.AssetKey, &skin.IsStarter, &skin.DisplayOrder, &skin.Enabled, &skin.CatalogVisible, &skin.UnlockRulesLocked)
	if errors.Is(err, sql.ErrNoRows) {
		return Skin{}, ErrNotFound
	}
	if err != nil {
		return Skin{}, err
	}
	if err = loadSkinContent(ctx, s.db, &skin); err != nil {
		return Skin{}, err
	}
	return skin, nil
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
	ruleRows, err := db.QueryContext(ctx, `SELECT id,name,rule_type,COALESCE(achievement_id,''),minimum_level,login_streak_days,COALESCE(event_id::text,''),event_revision,event_check_in_count,retroactive,enabled,COALESCE((SELECT jsonb_agg(jsonb_build_object('metric',metric,'operator',operator,'value',value) ORDER BY id) FROM skin_unlock_rule_conditions WHERE skin_unlock_rule_id=r.id),'[]') FROM skin_unlock_rules r WHERE skin_id=$1 ORDER BY created_at,id`, skin.ID)
	if err != nil {
		return err
	}
	defer ruleRows.Close()
	for ruleRows.Next() {
		var r model.SkinUnlockRule
		if err = ruleRows.Scan(&r.ID, &r.Name, &r.RuleType, &r.AchievementID, &r.MinimumLevel, &r.LoginStreakDays, &r.EventID, &r.EventRevision, &r.EventCheckInCount, &r.Retroactive, &r.Enabled, &r.Conditions); err != nil {
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
	if skin.UnlockRules, err = lockSkinRuleEvents(ctx, tx, skin.UnlockRules); err != nil {
		return Skin{}, err
	}
	skin.ID = uuid.NewString()
	skin.AssetKey = ""
	skin.IsStarter = false
	skin.Enabled = false
	skin.CatalogVisible = false
	if _, err = tx.ExecContext(ctx, `INSERT INTO skins(id,skin_type,name,description,asset_key,is_starter,display_order,enabled,catalog_visible) VALUES($1,$2,$3,$4,$5,FALSE,$6,FALSE,FALSE)`, skin.ID, skin.SkinType, skin.Name, skin.Description, skin.AssetKey, skin.DisplayOrder); err != nil {
		return Skin{}, err
	}
	if skin.UnlockRules, err = insertSkinUnlockRules(ctx, tx, skin.ID, skin.UnlockRules); err != nil {
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
	if next.UnlockRules, err = lockSkinRuleEvents(ctx, tx, next.UnlockRules); err != nil {
		return Skin{}, err
	}
	var lockedID string
	var wasEnabled bool
	if err = tx.QueryRowContext(ctx, `SELECT id,enabled FROM skins WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID, &wasEnabled); errors.Is(err, sql.ErrNoRows) {
		return Skin{}, ErrNotFound
	} else if err != nil {
		return Skin{}, err
	}
	// Read revisions in a fresh statement snapshot after any concurrent writer releases the skin lock.
	var currentRevisionEnabled bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM skins s JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled WHERE s.id=$1)`, id).Scan(&currentRevisionEnabled); err != nil {
		return Skin{}, err
	}
	if (next.Enabled || next.CatalogVisible) && !currentRevisionEnabled {
		return Skin{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE skins SET name=$2,description=$3,display_order=$4,enabled=$5,catalog_visible=$6,updated_at=NOW() WHERE id=$1`, id, next.Name, next.Description, next.DisplayOrder, next.Enabled, next.CatalogVisible)
	if err != nil {
		return Skin{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return Skin{}, ErrNotFound
	}
	var granted bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_skin_entitlement_events WHERE skin_id=$1) OR EXISTS(SELECT 1 FROM user_skins WHERE skin_id=$1 AND skin_unlock_rule_id IS NOT NULL)`, id).Scan(&granted); err != nil {
		return Skin{}, err
	}
	if granted && next.UnlockRules != nil {
		return Skin{}, ErrConflict
	}
	if next.UnlockRules != nil && !granted {
		if _, err = tx.ExecContext(ctx, `DELETE FROM skin_unlock_rules WHERE skin_id=$1`, id); err != nil {
			return Skin{}, err
		}
	}
	if next.UnlockRules, err = insertSkinUnlockRules(ctx, tx, id, next.UnlockRules); err != nil {
		return Skin{}, err
	}
	if next.Enabled && (!wasEnabled || next.UnlockRules != nil) {
		if err = reconcileRetroactiveSkinGrants(ctx, tx, id); err != nil {
			return Skin{}, err
		}
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_skin_entitlement_events WHERE skin_id=$1) OR EXISTS(SELECT 1 FROM user_skins WHERE skin_id=$1 AND skin_unlock_rule_id IS NOT NULL)`, id).Scan(&granted); err != nil {
		return Skin{}, err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return Skin{}, err
	}
	if err = tx.Commit(); err != nil {
		return Skin{}, err
	}
	next.ID = id
	next.UnlockRulesLocked = granted
	return next, nil
}

type skinRuleExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func lockSkinRuleEvents(ctx context.Context, tx skinRuleExecer, rules []model.SkinUnlockRule) ([]model.SkinUnlockRule, error) {
	eventIDs := make([]string, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		if rule.EventID == "" {
			continue
		}
		if _, ok := seen[rule.EventID]; !ok {
			seen[rule.EventID] = struct{}{}
			eventIDs = append(eventIDs, rule.EventID)
		}
	}
	sort.Strings(eventIDs)

	revisions := make(map[string]*int, len(eventIDs))
	for _, eventID := range eventIDs {
		var state string
		var revision int
		err := tx.QueryRowContext(ctx, `SELECT lifecycle_state,revision FROM events WHERE id=$1 FOR UPDATE`, eventID).Scan(&state, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if state == model.EventArchived {
			return nil, ErrConflict
		}
		if state == model.EventPublished {
			revisionCopy := revision
			revisions[eventID] = &revisionCopy
		} else {
			revisions[eventID] = nil
		}
	}
	for i := range rules {
		if rules[i].EventID != "" {
			rules[i].EventRevision = revisions[rules[i].EventID]
		}
	}
	return rules, nil
}

func insertSkinUnlockRules(ctx context.Context, tx skinRuleExecer, skinID string, rules []model.SkinUnlockRule) ([]model.SkinUnlockRule, error) {
	for i := range rules {
		r := &rules[i]
		rid := uuid.NewString()
		if _, err := tx.ExecContext(ctx, `INSERT INTO skin_unlock_rules(id,name,skin_id,rule_type,achievement_id,minimum_level,login_streak_days,event_id,event_revision,event_check_in_count,retroactive,enabled) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,'')::uuid,$9,$10,$11,$12)`, rid, r.Name, skinID, r.RuleType, r.AchievementID, r.MinimumLevel, r.LoginStreakDays, r.EventID, r.EventRevision, r.EventCheckInCount, r.Retroactive, r.Enabled); err != nil {
			return nil, err
		}
		r.ID = rid
		if r.EventID != "" && r.EventRevision != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO skin_unlock_rule_event_versions(skin_unlock_rule_id,event_id,event_revision) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, rid, r.EventID, r.EventRevision); err != nil {
				return nil, err
			}
		}
		var conditions []struct{ Metric, Operator, Value string }
		if len(r.Conditions) > 0 && string(r.Conditions) != "null" {
			if err := json.Unmarshal(r.Conditions, &conditions); err != nil {
				return nil, err
			}
		}
		for _, c := range conditions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO skin_unlock_rule_conditions(skin_unlock_rule_id,metric,operator,value) VALUES($1,$2,$3,$4)`, rid, c.Metric, c.Operator, c.Value); err != nil {
				return nil, err
			}
		}
	}
	return rules, nil
}

func reconcileRetroactiveSkinGrants(ctx context.Context, tx skinRuleExecer, skinID string) error {
	queries := []string{
		// Minimum-level progress is reconstructed from the durable XP total.
		`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
		SELECT us.user_id, s.id, r.id, r.event_id, r.event_revision, sr.id
		FROM user_stats us
		JOIN users u ON u.id=us.user_id AND u.deletion_scheduled_at IS NULL
		JOIN skins s ON s.id=$1 AND s.enabled
		JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
		JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='minimum_level' AND r.retroactive AND r.enabled
		LEFT JOIN event_versions ev ON ev.event_id=r.event_id AND ev.revision=r.event_revision
		WHERE us.xp >= ((r.minimum_level - 1)::BIGINT * (r.minimum_level - 1) * 100)
		  AND (r.event_id IS NULL OR (ev.published_at <= NOW() AND ev.starts_at <= NOW() AND NOW() < ev.ends_at))
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`,
		// Achievement ownership and its earned timestamp are durable.
		`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
		SELECT ua.user_id, s.id, r.id, r.event_id, r.event_revision, sr.id
		FROM user_achievements ua
		JOIN users u ON u.id=ua.user_id AND u.deletion_scheduled_at IS NULL
		JOIN skins s ON s.id=$1 AND s.enabled
		JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
		JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='achievement' AND r.achievement_id=ua.achievement_id AND r.retroactive AND r.enabled
		LEFT JOIN event_versions ev ON ev.event_id=r.event_id AND ev.revision=r.event_revision
		WHERE r.event_id IS NULL OR (ev.published_at <= ua.earned_at AND ev.starts_at <= ua.earned_at AND ua.earned_at < ev.ends_at)
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`,
		// best_streak retains previously reached login milestones after a reset.
		`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
		SELECT lp.user_id, s.id, r.id, r.event_id, r.event_revision, sr.id
		FROM user_login_progress lp
		JOIN users u ON u.id=lp.user_id AND u.deletion_scheduled_at IS NULL
		JOIN skins s ON s.id=$1 AND s.enabled
		JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
		JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='login_streak' AND r.retroactive AND r.enabled
		LEFT JOIN event_versions ev ON ev.event_id=r.event_id AND ev.revision=r.event_revision
		WHERE lp.best_streak >= r.login_streak_days
		  AND (r.event_id IS NULL OR (ev.published_at <= NOW() AND ev.starts_at <= NOW() AND NOW() < ev.ends_at))
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`,
		// Check-ins retain exact event revision provenance.
		`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, event_id, event_revision, skin_revision_id)
		SELECT ci.user_id, s.id, r.id, r.event_id, r.event_revision, sr.id
		FROM (
			SELECT event_id,event_revision,user_id,COUNT(*) AS check_ins
			FROM event_check_ins GROUP BY event_id,event_revision,user_id
		) ci
		JOIN users u ON u.id=ci.user_id AND u.deletion_scheduled_at IS NULL
		JOIN skins s ON s.id=$1 AND s.enabled
		JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
		JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='event_check_in_count' AND r.event_id=ci.event_id AND r.event_revision=ci.event_revision AND r.retroactive AND r.enabled
		WHERE ci.check_ins >= r.event_check_in_count
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`,
		// Permanent aggregate-only conditions remain reconstructable even when
		// detailed games predate retention or were imported as summary stats.
		`
		INSERT INTO user_skins (user_id,skin_id,skin_unlock_rule_id,event_id,event_revision,skin_revision_id)
		SELECT us.user_id,s.id,r.id,NULL,NULL,sr.id
		FROM user_stats us
		JOIN users u ON u.id=us.user_id AND u.deletion_scheduled_at IS NULL
		JOIN skins s ON s.id=$1 AND s.enabled
		JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
		JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='game_condition' AND r.event_id IS NULL AND r.retroactive AND r.enabled
		WHERE EXISTS(SELECT 1 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id=r.id)
		  AND NOT EXISTS(SELECT 1 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id=r.id AND c.metric NOT IN ('games_played','wins','current_streak','current_top2_streak','first_place_count','zero_penalty_games','human_only_games'))
		  AND NOT EXISTS(
			SELECT 1 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id=r.id AND NOT CASE c.operator
			WHEN 'eq' THEN CASE c.metric WHEN 'games_played' THEN us.games_played WHEN 'wins' THEN us.wins WHEN 'current_streak' THEN us.current_streak WHEN 'current_top2_streak' THEN us.current_top2_streak WHEN 'first_place_count' THEN us.first_place_count WHEN 'zero_penalty_games' THEN us.zero_penalty_games WHEN 'human_only_games' THEN us.human_only_games END = c.value::INTEGER
			WHEN 'gte' THEN CASE c.metric WHEN 'games_played' THEN us.games_played WHEN 'wins' THEN us.wins WHEN 'current_streak' THEN us.current_streak WHEN 'current_top2_streak' THEN us.current_top2_streak WHEN 'first_place_count' THEN us.first_place_count WHEN 'zero_penalty_games' THEN us.zero_penalty_games WHEN 'human_only_games' THEN us.human_only_games END >= c.value::INTEGER
			WHEN 'lte' THEN CASE c.metric WHEN 'games_played' THEN us.games_played WHEN 'wins' THEN us.wins WHEN 'current_streak' THEN us.current_streak WHEN 'current_top2_streak' THEN us.current_top2_streak WHEN 'first_place_count' THEN us.first_place_count WHEN 'zero_penalty_games' THEN us.zero_penalty_games WHEN 'human_only_games' THEN us.human_only_games END <= c.value::INTEGER
			WHEN 'gt' THEN CASE c.metric WHEN 'games_played' THEN us.games_played WHEN 'wins' THEN us.wins WHEN 'current_streak' THEN us.current_streak WHEN 'current_top2_streak' THEN us.current_top2_streak WHEN 'first_place_count' THEN us.first_place_count WHEN 'zero_penalty_games' THEN us.zero_penalty_games WHEN 'human_only_games' THEN us.human_only_games END > c.value::INTEGER
			WHEN 'lt' THEN CASE c.metric WHEN 'games_played' THEN us.games_played WHEN 'wins' THEN us.wins WHEN 'current_streak' THEN us.current_streak WHEN 'current_top2_streak' THEN us.current_top2_streak WHEN 'first_place_count' THEN us.first_place_count WHEN 'zero_penalty_games' THEN us.zero_penalty_games WHEN 'human_only_games' THEN us.human_only_games END < c.value::INTEGER
			ELSE FALSE END)
		ON CONFLICT (user_id,skin_id) DO NOTHING
	`,
		// Replay every retained game context so mixed per-game, cumulative, and streak
		// conditions preserve the runtime evaluator's AND semantics.
		`
		WITH player_games AS (
			SELECT gp.user_id,gp.game_id,g.finished_at,gp.is_winner,gp.penalty_points,gp.rank,
				CASE WHEN gp.is_winner THEN (SELECT COUNT(*) FROM game_players w WHERE w.game_id=gp.game_id AND w.user_id IS NOT NULL AND w.is_winner) ELSE 0 END AS shared_win_count,
				ROW_NUMBER() OVER (PARTITION BY gp.user_id ORDER BY g.finished_at,g.id)::INTEGER AS games_played,
				SUM(gp.is_winner::INTEGER) OVER (PARTITION BY gp.user_id ORDER BY g.finished_at,g.id)::INTEGER AS wins,
				SUM((gp.rank=1)::INTEGER) OVER (PARTITION BY gp.user_id ORDER BY g.finished_at,g.id)::INTEGER AS first_place_count,
				SUM((gp.penalty_points=0)::INTEGER) OVER (PARTITION BY gp.user_id ORDER BY g.finished_at,g.id)::INTEGER AS zero_penalty_games,
				SUM((NOT EXISTS(SELECT 1 FROM game_players b WHERE b.game_id=gp.game_id AND b.is_bot))::INTEGER) OVER (PARTITION BY gp.user_id ORDER BY g.finished_at,g.id)::INTEGER AS human_only_games,
				(NOT EXISTS(SELECT 1 FROM game_players p WHERE p.game_id=gp.game_id AND NOT p.is_bot AND p.penalty_points>0)) AS all_zero_penalty,
				EXISTS(SELECT 1 FROM game_moves m WHERE m.game_id=gp.game_id AND m.move_type='ace_close') AS ace_closed,
				EXISTS(SELECT 1 FROM game_initial_hands h WHERE h.game_id=gp.game_id) AS replay_retained,
				GREATEST(0,EXTRACT(EPOCH FROM (g.finished_at-g.started_at))::INTEGER) AS game_duration_seconds,
				(SELECT COUNT(*) FROM game_players streak JOIN games sg ON sg.id=streak.game_id WHERE streak.user_id=gp.user_id AND streak.is_winner AND (sg.finished_at,sg.id) <= (g.finished_at,g.id) AND NOT EXISTS(SELECT 1 FROM game_players loss JOIN games lg ON lg.id=loss.game_id WHERE loss.user_id=gp.user_id AND NOT loss.is_winner AND (lg.finished_at,lg.id) > (sg.finished_at,sg.id) AND (lg.finished_at,lg.id) <= (g.finished_at,g.id)))::INTEGER AS current_streak,
				(SELECT COUNT(*) FROM game_players streak JOIN games sg ON sg.id=streak.game_id WHERE streak.user_id=gp.user_id AND streak.rank<=2 AND (sg.finished_at,sg.id) <= (g.finished_at,g.id) AND NOT EXISTS(SELECT 1 FROM game_players loss JOIN games lg ON lg.id=loss.game_id WHERE loss.user_id=gp.user_id AND loss.rank>2 AND (lg.finished_at,lg.id) > (sg.finished_at,sg.id) AND (lg.finished_at,lg.id) <= (g.finished_at,g.id)))::INTEGER AS current_top2_streak
			FROM game_players gp JOIN games g ON g.id=gp.game_id
			WHERE gp.user_id IS NOT NULL AND NOT gp.is_bot
		), candidates AS (
			SELECT pg.user_id,s.id AS skin_id,r.id AS rule_id,r.event_id,r.event_revision,sr.id AS revision_id
			FROM player_games pg
			JOIN users u ON u.id=pg.user_id AND u.deletion_scheduled_at IS NULL
			JOIN skins s ON s.id=$1 AND s.enabled
			JOIN skin_revisions sr ON sr.skin_id=s.id AND sr.asset_key=s.asset_key AND sr.enabled
			JOIN skin_unlock_rules r ON r.skin_id=s.id AND r.rule_type='game_condition' AND r.retroactive AND r.enabled
			LEFT JOIN event_versions ev ON ev.event_id=r.event_id AND ev.revision=r.event_revision
			WHERE (r.event_id IS NULL OR (ev.published_at<=pg.finished_at AND ev.starts_at<=pg.finished_at AND pg.finished_at<ev.ends_at))
			  AND EXISTS(SELECT 1 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id=r.id)
			  AND NOT EXISTS (
				SELECT 1 FROM skin_unlock_rule_conditions c WHERE c.skin_unlock_rule_id=r.id AND NOT (
					CASE c.metric
					WHEN 'is_winner' THEN c.operator='eq' AND pg.is_winner=c.value::BOOLEAN
					WHEN 'all_zero_penalty' THEN c.operator='eq' AND pg.all_zero_penalty=c.value::BOOLEAN
					WHEN 'ace_closed' THEN pg.replay_retained AND c.operator='eq' AND pg.ace_closed=c.value::BOOLEAN
					ELSE CASE c.operator
						WHEN 'eq' THEN CASE c.metric WHEN 'shared_win_count' THEN pg.shared_win_count WHEN 'penalty' THEN pg.penalty_points WHEN 'games_played' THEN pg.games_played WHEN 'wins' THEN pg.wins WHEN 'current_streak' THEN pg.current_streak WHEN 'current_top2_streak' THEN pg.current_top2_streak WHEN 'first_place_count' THEN pg.first_place_count WHEN 'zero_penalty_games' THEN pg.zero_penalty_games WHEN 'human_only_games' THEN pg.human_only_games WHEN 'game_duration_seconds' THEN pg.game_duration_seconds END = c.value::INTEGER
						WHEN 'gte' THEN CASE c.metric WHEN 'shared_win_count' THEN pg.shared_win_count WHEN 'penalty' THEN pg.penalty_points WHEN 'games_played' THEN pg.games_played WHEN 'wins' THEN pg.wins WHEN 'current_streak' THEN pg.current_streak WHEN 'current_top2_streak' THEN pg.current_top2_streak WHEN 'first_place_count' THEN pg.first_place_count WHEN 'zero_penalty_games' THEN pg.zero_penalty_games WHEN 'human_only_games' THEN pg.human_only_games WHEN 'game_duration_seconds' THEN pg.game_duration_seconds END >= c.value::INTEGER
						WHEN 'lte' THEN CASE c.metric WHEN 'shared_win_count' THEN pg.shared_win_count WHEN 'penalty' THEN pg.penalty_points WHEN 'games_played' THEN pg.games_played WHEN 'wins' THEN pg.wins WHEN 'current_streak' THEN pg.current_streak WHEN 'current_top2_streak' THEN pg.current_top2_streak WHEN 'first_place_count' THEN pg.first_place_count WHEN 'zero_penalty_games' THEN pg.zero_penalty_games WHEN 'human_only_games' THEN pg.human_only_games WHEN 'game_duration_seconds' THEN pg.game_duration_seconds END <= c.value::INTEGER
						WHEN 'gt' THEN CASE c.metric WHEN 'shared_win_count' THEN pg.shared_win_count WHEN 'penalty' THEN pg.penalty_points WHEN 'games_played' THEN pg.games_played WHEN 'wins' THEN pg.wins WHEN 'current_streak' THEN pg.current_streak WHEN 'current_top2_streak' THEN pg.current_top2_streak WHEN 'first_place_count' THEN pg.first_place_count WHEN 'zero_penalty_games' THEN pg.zero_penalty_games WHEN 'human_only_games' THEN pg.human_only_games WHEN 'game_duration_seconds' THEN pg.game_duration_seconds END > c.value::INTEGER
						WHEN 'lt' THEN CASE c.metric WHEN 'shared_win_count' THEN pg.shared_win_count WHEN 'penalty' THEN pg.penalty_points WHEN 'games_played' THEN pg.games_played WHEN 'wins' THEN pg.wins WHEN 'current_streak' THEN pg.current_streak WHEN 'current_top2_streak' THEN pg.current_top2_streak WHEN 'first_place_count' THEN pg.first_place_count WHEN 'zero_penalty_games' THEN pg.zero_penalty_games WHEN 'human_only_games' THEN pg.human_only_games WHEN 'game_duration_seconds' THEN pg.game_duration_seconds END < c.value::INTEGER
						ELSE FALSE END END))
		)
		INSERT INTO user_skins (user_id,skin_id,skin_unlock_rule_id,event_id,event_revision,skin_revision_id)
		SELECT DISTINCT ON (user_id,skin_id) user_id,skin_id,rule_id,event_id,event_revision,revision_id FROM candidates ORDER BY user_id,skin_id,rule_id
		ON CONFLICT (user_id,skin_id) DO NOTHING
	`,
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query, skinID); err != nil {
			return err
		}
	}
	return nil
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
	var lockedID string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM skins WHERE id=$1 FOR UPDATE`, skinID).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
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
