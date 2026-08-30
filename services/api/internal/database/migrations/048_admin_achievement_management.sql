CREATE TABLE user_achievement_entitlement_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id TEXT NOT NULL REFERENCES achievements(id) ON DELETE RESTRICT,
    action TEXT NOT NULL CHECK (action IN ('grant', 'revoke')),
    reason TEXT NOT NULL CHECK (length(btrim(reason)) > 0),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (length(btrim(idempotency_key)) > 0),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX user_achievement_entitlement_events_user_idx ON user_achievement_entitlement_events(user_id, occurred_at DESC);

INSERT INTO admin_permissions(name,description) VALUES
    ('achievements.entitlements','Grant and revoke exceptional achievement entitlements')
ON CONFLICT DO NOTHING;
INSERT INTO admin_role_permissions(role_id,permission_name)
SELECT id,'achievements.entitlements' FROM admin_roles WHERE name='super_admin'
ON CONFLICT DO NOTHING;
