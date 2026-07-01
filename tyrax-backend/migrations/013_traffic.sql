-- TYRAX Database Schema
-- Migration 013: Traffic metering + FREE-tier quota enforcement
--
-- FREE users get a rolling 3 GB / 30-day quota. When exhausted, the tunnel is
-- blocked for 30 days from the moment traffic ran out (blocked_until). Paid tiers
-- are unlimited and never blocked.
--
-- Idempotent (IF NOT EXISTS) so it is safe to apply manually with `psql -f` on an
-- already-provisioned database.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS traffic_used_bytes   BIGINT      NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS traffic_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS blocked_until        TIMESTAMPTZ;

-- Per-device cumulative counter last read from the node panel. Used to compute
-- traffic deltas (handles panel-side resets: a drop below last means reset).
ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS last_traffic_bytes BIGINT NOT NULL DEFAULT 0;
