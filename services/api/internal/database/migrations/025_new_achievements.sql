-- New achievements and updated shared-win rule.
-- Good Company is re-scoped to 2-player shared wins only (shared_win_count eq 2);
-- Triumvirate and United Front cover 3- and 4-player ties.
-- Penalty thresholds, clean sweeps, no-Ace games, and speed runs round out the
-- per-game badge set.

-- ============================================================================
-- Update existing Good Company rule: shared_win_count eq 2 (was shared_win)
-- ============================================================================
DELETE FROM achievement_rules WHERE achievement_id = 'shared_win';

INSERT INTO achievement_rules (achievement_id, metric, operator, value)
VALUES ('shared_win', 'shared_win_count', 'eq', '2');

-- ============================================================================
-- New catalog entries
-- ============================================================================
INSERT INTO achievements (id, name, description, icon, display_order, enabled)
VALUES
    ('high_penalty',  'Chumbo',        'Finish a game with 100+ penalty',          '💣', 200, TRUE),
    ('penalty_80',    'Ouch',          'Finish a game with 80+ penalty',           '🤕', 210, TRUE),
    ('all_clean',     'Clean Sweep',   'All players finish with 0 penalty',        '🧹', 220, TRUE),
    ('no_ace_close',  'Old School',    'No Ace closes a suit all game',            '♠',  230, TRUE),
    ('shared_win_3',  'Triumvirate',   'Share a win with two other players',       '🤝', 240, TRUE),
    ('shared_win_4',  'United Front',  'All registered players share the win',     '🏛',  250, TRUE),
    ('short_game',    'Speed Demon',   'Finish a game in under 2 minutes',         '⚡', 260, TRUE)

ON CONFLICT (id) DO UPDATE SET
    name        = EXCLUDED.name,
    description = EXCLUDED.description,
    icon        = EXCLUDED.icon,
    display_order = EXCLUDED.display_order,
    enabled     = EXCLUDED.enabled,
    updated_at  = NOW();

-- ============================================================================
-- Rules for each new achievement
-- ============================================================================
DELETE FROM achievement_rules
WHERE achievement_id IN (
    'high_penalty', 'penalty_80', 'all_clean', 'no_ace_close',
    'shared_win_3', 'shared_win_4', 'short_game'
);

INSERT INTO achievement_rules (achievement_id, metric, operator, value)
VALUES
    ('high_penalty',  'penalty',                'gte',  '100'),
    ('penalty_80',    'penalty',                'gte',  '80'),
    ('all_clean',     'all_zero_penalty',       'eq',   'true'),
    ('no_ace_close',  'ace_closed',             'eq',   'false'),
    ('shared_win_3',  'shared_win_count',       'eq',   '3'),
    ('shared_win_4',  'shared_win_count',       'eq',   '4'),
    ('short_game',    'game_duration_seconds',  'lte',  '120');