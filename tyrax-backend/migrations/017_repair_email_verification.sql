-- Migration 017: repair email verification schema if 016 was bootstrap-skipped.
-- Idempotent — safe on databases where 016 already applied correctly.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS email_verifications (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    code       TEXT NOT NULL,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_verifications_user  ON email_verifications (user_id);
CREATE INDEX IF NOT EXISTS idx_email_verifications_token ON email_verifications (token);
CREATE INDEX IF NOT EXISTS idx_email_verifications_email ON email_verifications (email);
