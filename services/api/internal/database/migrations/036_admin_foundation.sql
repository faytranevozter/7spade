CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    mfa_required BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX admin_users_email_unique ON admin_users (LOWER(email));

CREATE TABLE admin_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE admin_permissions (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE admin_user_roles (
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    PRIMARY KEY (admin_user_id, role_id)
);

CREATE TABLE admin_role_permissions (
    role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    permission_name TEXT NOT NULL REFERENCES admin_permissions(name) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_name)
);

CREATE TABLE admin_mfa_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    method_type TEXT NOT NULL CHECK (method_type IN ('totp')),
    secret_ciphertext BYTEA NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE admin_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY,
    family_id UUID NOT NULL,
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX admin_sessions_admin_user_id_idx ON admin_sessions (admin_user_id);

CREATE TABLE admin_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    session_id UUID,
    request_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    reason TEXT,
    outcome TEXT NOT NULL,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB,
    ip_address TEXT,
    user_agent TEXT
);
CREATE INDEX admin_audit_events_actor_time_idx ON admin_audit_events (admin_user_id, occurred_at DESC);

CREATE OR REPLACE FUNCTION reject_admin_audit_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'admin audit events are append-only';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER admin_audit_events_append_only
BEFORE UPDATE OR DELETE ON admin_audit_events
FOR EACH ROW EXECUTE FUNCTION reject_admin_audit_mutation();

INSERT INTO admin_permissions (name, description) VALUES
    ('dashboard.read', 'View the admin operations dashboard'),
    ('users.read', 'View users'), ('users.moderate', 'Moderate users'), ('users.economy.adjust', 'Adjust rating and XP'),
    ('rooms.read', 'View rooms'), ('rooms.terminate', 'Terminate rooms'),
    ('games.read', 'View games'), ('games.invalidate', 'Invalidate games'),
    ('seasons.read', 'View seasons'), ('seasons.manage', 'Manage seasons'),
    ('events.read', 'View events'), ('events.manage', 'Manage events'),
    ('achievements.read', 'View achievements'), ('achievements.manage', 'Manage achievements'),
    ('skins.read', 'View skins'), ('skins.manage', 'Manage skins'),
    ('admins.read', 'View administrators'), ('admins.manage', 'Manage administrator identities and roles'),
    ('audit.read', 'View administrator audit events')
ON CONFLICT (name) DO NOTHING;

INSERT INTO admin_roles (name, description) VALUES
    ('viewer', 'Read-only operational access'),
    ('moderator', 'User and room moderation access'),
    ('operator', 'Content and operational management access'),
    ('super_admin', 'Full administrator access')
ON CONFLICT (name) DO NOTHING;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name
FROM admin_roles r
JOIN admin_permissions p ON r.name = 'super_admin'
ON CONFLICT DO NOTHING;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, 'dashboard.read'
FROM admin_roles r
WHERE r.name = 'viewer'
ON CONFLICT DO NOTHING;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name FROM admin_roles r JOIN admin_permissions p ON
    (r.name = 'moderator' AND p.name IN ('dashboard.read', 'users.read', 'users.moderate', 'rooms.read', 'rooms.terminate', 'games.read', 'audit.read')) OR
    (r.name = 'operator' AND p.name NOT IN ('admins.manage', 'users.economy.adjust', 'games.invalidate'))
ON CONFLICT DO NOTHING;
