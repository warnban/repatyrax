-- TYRAX Database Schema
-- Migration 008: per-device VLESS UUID (Xray-core / VLESS+Reality identity).
-- Generated locally (no Marzban dependency); stable across reconnects so the
-- same physical device keeps the same VLESS identity.

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS vless_uuid UUID NOT NULL DEFAULT uuid_generate_v4();
