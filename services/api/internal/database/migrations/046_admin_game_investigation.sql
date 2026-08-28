ALTER TABLE games ADD COLUMN IF NOT EXISTS season_id TEXT REFERENCES seasons(id);
CREATE INDEX IF NOT EXISTS idx_games_season_finished ON games(season_id, finished_at DESC, id DESC);

INSERT INTO seasons (id, label, started_at, ended_at)
SELECT DISTINCT
    to_char(g.finished_at AT TIME ZONE 'UTC', 'YYYY-MM'),
    to_char(g.finished_at AT TIME ZONE 'UTC', 'FMMonth YYYY'),
    date_trunc('month', g.finished_at AT TIME ZONE 'UTC'),
    date_trunc('month', g.finished_at AT TIME ZONE 'UTC') + INTERVAL '1 month'
FROM games g
ON CONFLICT (id) DO NOTHING;

UPDATE games g
SET season_id = s.id
FROM seasons s
WHERE g.season_id IS NULL
  AND g.finished_at >= s.started_at
  AND (s.ended_at IS NULL OR g.finished_at < s.ended_at);

INSERT INTO admin_permissions (name, description)
VALUES ('games.annotate', 'Flag games and add administrative notes')
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name
FROM admin_roles r
JOIN admin_permissions p ON p.name = 'games.annotate'
WHERE r.name IN ('operator', 'super_admin')
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS admin_game_flags (
    id UUID PRIMARY KEY,
    game_id UUID NOT NULL REFERENCES games(id),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_admin_game_flags_game ON admin_game_flags(game_id, created_at, id);

CREATE TABLE IF NOT EXISTS admin_game_notes (
    id UUID PRIMARY KEY,
    game_id UUID NOT NULL REFERENCES games(id),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id),
    reason TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_admin_game_notes_game ON admin_game_notes(game_id, created_at, id);
