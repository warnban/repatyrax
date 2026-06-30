-- TYRAX Database Schema
-- Migration 009: per-node transport / anti-DPI parameters (RU 2026).
-- Adds XHTTP transport support alongside the legacy raw-TCP Reality path so the
-- behavioural-detection layer of ТСПУ can be defeated. Existing rows get safe
-- defaults; vless nodes are flipped to the XHTTP profile (Profile A).

ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS reality_dest    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS network         TEXT NOT NULL DEFAULT 'tcp',
    ADD COLUMN IF NOT EXISTS flow            TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS xhttp_path      TEXT NOT NULL DEFAULT '/api/v1/data',
    ADD COLUMN IF NOT EXISTS xhttp_mode      TEXT NOT NULL DEFAULT 'auto',
    ADD COLUMN IF NOT EXISTS x_padding_bytes TEXT NOT NULL DEFAULT '100-1000',
    ADD COLUMN IF NOT EXISTS fingerprint     TEXT NOT NULL DEFAULT 'chrome';

-- Default VLESS nodes onto the XHTTP profile (Profile A: no Vision, mode=auto).
-- The masquerade dest mirrors the existing SNI so a freshly provisioned node is
-- internally consistent; the operator overrides per node in 3x-ui as needed.
UPDATE nodes
SET network      = 'xhttp',
    reality_dest = CASE
        WHEN reality_sni <> '' THEN reality_sni || ':443'
        ELSE 'www.microsoft.com:443'
    END,
    reality_sni  = CASE WHEN reality_sni = '' THEN 'www.microsoft.com' ELSE reality_sni END
WHERE protocol = 'vless';
