ALTER TABLE admin_sessions
    ADD COLUMN ip_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';
