-- TYRAX Database Schema
-- Migration 010: per-node 3x-ui panel credentials for client-UUID sync.
-- The backend issues a per-device VLESS UUID; the node's Xray only authenticates
-- UUIDs present in its inbound client list. These columns let the backend call
-- the 3x-ui panel API (login -> addClient/delClient) to register/remove devices.
-- Secrets: never exposed in JSON (model fields are json:"-").

ALTER TABLE nodes
    -- Full panel base URL incl. scheme, host, port and the web base path, no
    -- trailing slash. Example: https://1.2.3.4:2053/Xk29fbqQ
    ADD COLUMN IF NOT EXISTS panel_url        TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS panel_user       TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS panel_pass       TEXT NOT NULL DEFAULT '',
    -- Numeric id of the VLESS+Reality+XHTTP inbound inside 3x-ui (usually 1).
    ADD COLUMN IF NOT EXISTS panel_inbound_id INT  NOT NULL DEFAULT 0;
