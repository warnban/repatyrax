-- TYRAX Database Schema
-- Migration 001: Initial

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE subscription_tier AS ENUM ('CORE', 'SHADOW', 'DOMINION');
CREATE TYPE node_status AS ENUM ('OPEN', 'MONITORED', 'HEAVILY_RESTRICTED');

CREATE TABLE users (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email             TEXT UNIQUE,
    password_hash     TEXT,
    telegram_id       BIGINT UNIQUE,
    subscription_tier subscription_tier NOT NULL DEFAULT 'CORE',
    subscription_end  TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE nodes (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    codename  TEXT NOT NULL UNIQUE,   -- NL-01, DE-02, FI-01
    country   TEXT NOT NULL,
    host      TEXT NOT NULL,
    port      INT NOT NULL,
    protocol  TEXT NOT NULL,          -- wireguard | vless | shadowsocks
    status    node_status NOT NULL DEFAULT 'OPEN',
    public_key TEXT,                  -- WireGuard public key
    ping_ms   INT NOT NULL DEFAULT 0,
    min_tier  subscription_tier NOT NULL DEFAULT 'CORE'
);

CREATE TABLE connection_logs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id          UUID NOT NULL REFERENCES nodes(id),
    protocol         TEXT NOT NULL,
    connected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disconnected_at  TIMESTAMPTZ
);

CREATE TABLE telegram_auth_tokens (
    token      TEXT PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    used_at    TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes'
);

-- Seed nodes
INSERT INTO nodes (codename, country, host, port, protocol, status, min_tier) VALUES
    ('NL-01', 'Netherlands', 'nl-01.tyrax.app', 51820, 'wireguard', 'OPEN', 'CORE'),
    ('DE-01', 'Germany',     'de-01.tyrax.app', 51820, 'wireguard', 'OPEN', 'CORE'),
    ('FI-01', 'Finland',     'fi-01.tyrax.app', 443,   'vless',     'OPEN', 'SHADOW');
