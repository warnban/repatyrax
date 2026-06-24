-- TYRAX Database Schema
-- Migration 002: VLESS XTLS-Reality node parameters

ALTER TABLE nodes
    ADD COLUMN reality_public_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN reality_short_id   TEXT NOT NULL DEFAULT '',
    ADD COLUMN reality_sni        TEXT NOT NULL DEFAULT '';

-- Point the VLESS node at a masquerade SNI; keys are provisioned per node.
UPDATE nodes
SET reality_sni = 'www.microsoft.com'
WHERE protocol = 'vless';
