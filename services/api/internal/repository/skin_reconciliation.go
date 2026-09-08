package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type SkinReconciliationRuleOutcome struct {
	RuleName string `json:"rule_name"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	Granted  int64  `json:"granted"`
}

type SkinReconciliationReport struct {
	AchievementGrants   int64                           `json:"achievement_grants"`
	LevelGrants         int64                           `json:"level_grants"`
	GameConditionGrants int64                           `json:"game_condition_grants"`
	LoginRules          string                          `json:"login_rules"`
	GameRules           []SkinReconciliationRuleOutcome `json:"game_rules"`
}

func (r SkinReconciliationReport) TotalGrants() int64 {
	return r.AchievementGrants + r.LevelGrants + r.GameConditionGrants
}

// ReconcileProgressionSkins grants rewards provable from retained durable data.
// It only inserts ownership, so existing equipment is preserved. Ownership's
// unique key makes the operation safe to rerun.
func ReconcileProgressionSkins(db *sql.DB) (SkinReconciliationReport, error) {
	report := SkinReconciliationReport{
		LoginRules: "skipped: historical login activity is not retained",
		GameRules:  []SkinReconciliationRuleOutcome{},
	}
	tx, err := db.Begin()
	if err != nil {
		return report, fmt.Errorf("begin skin reconciliation: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, skin_revision_id)
		SELECT ua.user_id, s.id, r.id, sr.id
		FROM skin_unlock_rules r
		JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
		JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
		JOIN user_achievements ua ON ua.achievement_id = r.achievement_id
		JOIN users u ON u.id = ua.user_id AND u.deletion_scheduled_at IS NULL
		WHERE r.rule_type = 'achievement' AND r.enabled = TRUE AND r.event_id IS NULL
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`)
	if err != nil {
		return report, fmt.Errorf("reconcile achievement skins: %w", err)
	}
	report.AchievementGrants, _ = result.RowsAffected()

	result, err = tx.Exec(`
		INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, skin_revision_id)
		SELECT us.user_id, s.id, r.id, sr.id
		FROM skin_unlock_rules r
		JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
		JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
		JOIN user_stats us ON us.xp >= ((r.minimum_level - 1)::BIGINT * (r.minimum_level - 1) * 100)
		JOIN users u ON u.id = us.user_id AND u.deletion_scheduled_at IS NULL
		WHERE r.rule_type = 'minimum_level' AND r.enabled = TRUE AND r.event_id IS NULL
		  AND r.minimum_level IS NOT NULL AND r.minimum_level >= 1
		ON CONFLICT (user_id, skin_id) DO NOTHING
	`)
	if err != nil {
		return report, fmt.Errorf("reconcile level skins: %w", err)
	}
	report.LevelGrants, _ = result.RowsAffected()

	if err := reconcileGameConditionSkins(tx, &report); err != nil {
		return report, err
	}
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("commit skin reconciliation: %w", err)
	}
	return report, nil
}

func reconcileGameConditionSkins(tx *sql.Tx, report *SkinReconciliationReport) error {
	rows, err := tx.Query(`
		SELECT r.id, r.name, c.metric, c.operator, c.value, r.retroactive, s.enabled
		FROM skin_unlock_rules r
		JOIN skin_unlock_rule_conditions c ON c.skin_unlock_rule_id = r.id
		JOIN skins s ON s.id = r.skin_id
		WHERE r.rule_type = 'game_condition' AND r.enabled = TRUE AND r.event_id IS NULL
		ORDER BY r.name, r.id, c.created_at, c.id
	`)
	if err != nil {
		return fmt.Errorf("query game-condition reconciliation rules: %w", err)
	}
	type condition struct {
		metric, operator, value string
	}
	type rule struct {
		id, name                 string
		conditions               []condition
		retroactive, skinEnabled bool
	}
	rules := []rule{}
	for rows.Next() {
		var id, name string
		var item condition
		var retroactive, skinEnabled bool
		if err := rows.Scan(&id, &name, &item.metric, &item.operator, &item.value, &retroactive, &skinEnabled); err != nil {
			rows.Close()
			return fmt.Errorf("scan game-condition reconciliation rule: %w", err)
		}
		if len(rules) == 0 || rules[len(rules)-1].id != id {
			rules = append(rules, rule{id: id, name: name, retroactive: retroactive, skinEnabled: skinEnabled})
		}
		rules[len(rules)-1].conditions = append(rules[len(rules)-1].conditions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate game-condition reconciliation rules: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close game-condition reconciliation rules: %w", err)
	}

	for _, item := range rules {
		outcome := SkinReconciliationRuleOutcome{RuleName: item.name, Status: "skipped"}
		if !item.skinEnabled {
			outcome.Reason = "disabled_skin"
			report.GameRules = append(report.GameRules, outcome)
			continue
		}
		if !item.retroactive {
			outcome.Reason = "prospective_only"
			report.GameRules = append(report.GameRules, outcome)
			continue
		}
		gamePredicates := []string{}
		statsPredicates := []string{}
		queryArgs := []any{item.id}
		for _, condition := range item.conditions {
			scope, predicate, args, reason := historicalGamePredicate(condition.metric, condition.operator, condition.value, len(queryArgs)+1)
			if reason != "" {
				outcome.Reason = reason
				break
			}
			if scope == "game" {
				gamePredicates = append(gamePredicates, predicate)
			} else {
				statsPredicates = append(statsPredicates, predicate)
			}
			queryArgs = append(queryArgs, args...)
		}
		predicates := []string{}
		if len(gamePredicates) > 0 {
			predicates = append(predicates, "EXISTS (SELECT 1 FROM game_players gp WHERE gp.user_id = u.id AND "+strings.Join(gamePredicates, " AND ")+")")
		}
		if len(statsPredicates) > 0 {
			predicates = append(predicates, "EXISTS (SELECT 1 FROM user_stats us WHERE us.user_id = u.id AND "+strings.Join(statsPredicates, " AND ")+")")
		}
		if outcome.Reason != "" {
			report.GameRules = append(report.GameRules, outcome)
			continue
		}
		query := `
			INSERT INTO user_skins (user_id, skin_id, skin_unlock_rule_id, skin_revision_id)
			SELECT u.id, r.skin_id, r.id, sr.id
			FROM skin_unlock_rules r
			JOIN skins s ON s.id = r.skin_id AND s.enabled = TRUE
			JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
			JOIN users u ON u.deletion_scheduled_at IS NULL
			WHERE r.id = $1 AND r.enabled = TRUE AND (` + strings.Join(predicates, ") AND (") + `)
			ON CONFLICT (user_id, skin_id) DO NOTHING`
		result, err := tx.Exec(query, queryArgs...)
		if err != nil {
			return fmt.Errorf("reconcile game-condition rule %s: %w", item.name, err)
		}
		outcome.Status = "reconciled"
		outcome.Granted, _ = result.RowsAffected()
		report.GameConditionGrants += outcome.Granted
		report.GameRules = append(report.GameRules, outcome)
	}
	return nil
}

func historicalGamePredicate(metric, operator, value string, parameter int) (string, string, []any, string) {
	comparison := map[string]string{"eq": "=", "gte": ">=", "lte": "<=", "gt": ">", "lt": "<"}[operator]
	if comparison == "" {
		return "", "", nil, "unsupported_operator"
	}
	switch metric {
	case "is_winner":
		expected, err := strconv.ParseBool(value)
		if err != nil || operator != "eq" {
			return "", "", nil, "malformed_value"
		}
		return "game", fmt.Sprintf("gp.is_winner = $%d", parameter), []any{expected}, ""
	case "penalty":
		expected, err := strconv.Atoi(value)
		if err != nil {
			return "", "", nil, "malformed_value"
		}
		return "game", fmt.Sprintf("gp.penalty_points %s $%d", comparison, parameter), []any{expected}, ""
	case "games_played", "wins":
		expected, err := strconv.Atoi(value)
		if err != nil {
			return "", "", nil, "malformed_value"
		}
		return "stats", fmt.Sprintf("us.%s %s $%d", metric, comparison, parameter), []any{expected}, ""
	default:
		return "", "", nil, "unreconstructable_metric"
	}
}
