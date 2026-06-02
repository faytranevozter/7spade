CREATE TABLE IF NOT EXISTS achievements (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL,
    icon          TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS achievement_rules (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    achievement_id TEXT NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    metric         TEXT NOT NULL,
    operator       TEXT NOT NULL,
    value          TEXT NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_achievement_rules_achievement_id ON achievement_rules(achievement_id);

INSERT INTO achievements (id, name, description, icon, display_order, enabled)
VALUES
    ('first_win', 'First Blood', 'Win your first game', '🏆', 10, TRUE),
    ('games_10', 'Regular', 'Play 10 games', '🎴', 20, TRUE),
    ('games_50', 'Veteran', 'Play 50 games', '🎖️', 30, TRUE),
    ('games_100', 'Centurion', 'Play 100 games', '💯', 40, TRUE),
    ('streak_3', 'On a Roll', 'Win 3 games in a row', '🔥', 50, TRUE),
    ('streak_5', 'Unstoppable', 'Win 5 games in a row', '⚡', 60, TRUE),
    ('perfect_round', 'Flawless', 'Finish a game with zero penalty', '✨', 70, TRUE),
    ('shared_win', 'Good Company', 'Share a win in a tie', '🤝', 80, TRUE)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon,
    display_order = EXCLUDED.display_order,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

DELETE FROM achievement_rules
WHERE achievement_id IN ('first_win', 'games_10', 'games_50', 'games_100', 'streak_3', 'streak_5', 'perfect_round', 'shared_win');

INSERT INTO achievement_rules (achievement_id, metric, operator, value)
VALUES
    ('first_win', 'is_winner', 'eq', 'true'),
    ('games_10', 'games_played', 'gte', '10'),
    ('games_50', 'games_played', 'gte', '50'),
    ('games_100', 'games_played', 'gte', '100'),
    ('streak_3', 'current_streak', 'gte', '3'),
    ('streak_5', 'current_streak', 'gte', '5'),
    ('perfect_round', 'penalty', 'eq', '0'),
    ('shared_win', 'shared_win', 'eq', 'true');
