-- TYRAX Database Schema
-- Migration 006: store the Telegram @username for bot-provisioned identities.

ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT;
