-- Migration 016: email confirmation for email/password registrations.
-- Telegram-provisioned identities are trusted via Telegram and never require this.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- Grandfather every existing identity as verified so nobody is locked out by the
-- new gate. Only registrations created after this migration start unverified.
UPDATE users SET email_verified = TRUE WHERE email_verified = FALSE;

CREATE TABLE IF NOT EXISTS email_verifications (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    code       TEXT NOT NULL,          -- 6-digit numeric, entered in-app
    token      TEXT NOT NULL UNIQUE,   -- opaque secret embedded in the email link
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_verifications_user  ON email_verifications (user_id);
CREATE INDEX IF NOT EXISTS idx_email_verifications_token ON email_verifications (token);
CREATE INDEX IF NOT EXISTS idx_email_verifications_email ON email_verifications (email);
