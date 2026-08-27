CREATE TABLE admin_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    invited_by_admin_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX admin_invitations_email_idx ON admin_invitations (LOWER(email));
