-- TYRAX Database Schema
-- Migration 007: per-device tunnel IP, allocated from the 10.0.0.0/8 pool.

ALTER TABLE devices ADD COLUMN IF NOT EXISTS client_ip TEXT NOT NULL DEFAULT '10.0.0.2';
