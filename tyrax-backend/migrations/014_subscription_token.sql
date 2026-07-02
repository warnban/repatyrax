-- Migration 014: opaque token for Happ / external client subscription URLs
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS subscription_token TEXT UNIQUE;

CREATE INDEX IF NOT EXISTS idx_users_subscription_token
    ON users (subscription_token)
    WHERE subscription_token IS NOT NULL;
