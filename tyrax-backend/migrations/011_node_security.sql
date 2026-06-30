-- TYRAX Database Schema
-- Migration 011: per-node stream security selector (Reality vs TLS-over-CDN).
--
-- 'reality' (default)  -> direct connection, XTLS-Reality steal-from-self.
-- 'tls'                -> VLESS + XHTTP + real TLS, fronted by a CDN (Cloudflare).
--                         Reality CANNOT traverse a CDN (it terminates TLS via a
--                         direct TCP steal), so the CDN profile uses real TLS on a
--                         proxied domain. This hides the origin IP/subnet entirely,
--                         making hoster IP reputation almost irrelevant.

ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS security TEXT NOT NULL DEFAULT 'reality';
