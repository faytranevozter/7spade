ALTER TABLE admin_mfa_methods
    ADD CONSTRAINT admin_mfa_methods_admin_user_unique UNIQUE (admin_user_id);

ALTER TABLE admin_sessions
    ADD COLUMN mfa_verified BOOLEAN NOT NULL DEFAULT FALSE;
