-- TYRAX Database Schema
-- Migration 004: Orders + subscription invites

CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id),
    tier              TEXT NOT NULL,
    months            INT NOT NULL DEFAULT 1,
    amount_rub        NUMERIC(10,2) NOT NULL,
    payment_method    TEXT NOT NULL,
    external_order_id TEXT,
    status            TEXT NOT NULL DEFAULT 'NEW',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at           TIMESTAMPTZ
);

CREATE INDEX idx_orders_user_id          ON orders (user_id);
CREATE INDEX idx_orders_external_order_id ON orders (external_order_id);

CREATE TABLE subscription_invites (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id   UUID NOT NULL REFERENCES users(id),
    invitee_id UUID NOT NULL REFERENCES users(id),
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add parent_subscription_id for DOMINION invite flow (if not present)
ALTER TABLE users ADD COLUMN IF NOT EXISTS parent_subscription_id UUID REFERENCES users(id);
