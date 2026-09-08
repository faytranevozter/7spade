CREATE TABLE feature_settings (
    key TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO feature_settings (key, enabled) VALUES
    ('daily_login', FALSE),
    ('new_registrations', TRUE),
    ('guest_access', TRUE),
    ('room_creation', TRUE),
    ('quick_play', TRUE),
    ('new_game_starts', TRUE),
    ('spectator_access', TRUE),
    ('emotes', TRUE);

INSERT INTO admin_permissions (name, description) VALUES
    ('settings.read', 'View application settings'),
    ('settings.write', 'Manage application settings')
ON CONFLICT (name) DO NOTHING;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name
FROM admin_roles r
CROSS JOIN admin_permissions p
WHERE r.name = 'super_admin' AND p.name IN ('settings.read', 'settings.write')
ON CONFLICT DO NOTHING;
