-- Email verification: nullable timestamp set when a user verifies their email.
-- NULL means unverified. Email/password and OAuth users are unverified until
-- they complete the verification flow; verification is soft (no gameplay gate).

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMP NULL;
