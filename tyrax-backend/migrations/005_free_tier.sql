-- TYRAX Database Schema
-- Migration 005: FREE tier + Telegram confirmation flag
--
-- NOTE: PostgreSQL forbids using a freshly-added enum value inside the SAME
-- transaction that added it ("unsafe use of new value"). Run this file in
-- autocommit mode (e.g. `psql -f`, which commits each statement separately),
-- NOT wrapped in a single BEGIN/COMMIT block.

-- 1) Allow FREE as a valid subscription tier.
ALTER TYPE subscription_tier ADD VALUE IF NOT EXISTS 'FREE';

-- 2) New identities default to FREE (column is named subscription_tier).
ALTER TABLE users ALTER COLUMN subscription_tier SET DEFAULT 'FREE';

-- 3) Telegram deep-link tokens need a confirmation flag the bot flips to TRUE.
ALTER TABLE telegram_auth_tokens
    ADD COLUMN IF NOT EXISTS confirmed BOOLEAN NOT NULL DEFAULT FALSE;
