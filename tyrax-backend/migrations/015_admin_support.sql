-- TYRAX Database Schema
-- Migration 015: admin panel fields + support tickets

ALTER TABLE users ADD COLUMN IF NOT EXISTS registration_ip INET;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

DO $$ BEGIN
    CREATE TYPE support_ticket_status AS ENUM ('open', 'closed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS support_tickets (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID REFERENCES users(id) ON DELETE SET NULL,
    telegram_id         BIGINT NOT NULL,
    telegram_username   TEXT,
    subscription_tier   subscription_tier NOT NULL DEFAULT 'FREE',
    status              support_ticket_status NOT NULL DEFAULT 'open',
    subject             TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at           TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS support_messages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id   UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender      TEXT NOT NULL CHECK (sender IN ('user', 'admin')),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_support_tickets_updated ON support_tickets(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_messages_ticket ON support_messages(ticket_id, created_at);
CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen_at DESC NULLS LAST);
