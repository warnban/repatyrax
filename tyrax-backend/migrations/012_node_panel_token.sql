-- 3x-ui >= 3.x wraps POST /login in CSRF middleware (403 for tokenless calls).
-- The backend now authenticates to the panel with a Bearer API token instead of
-- username/password: the token bypasses CSRF on all /panel/api/... routes.
-- Create the token in the panel UI (Settings -> Security -> API Token) and store
-- its plaintext value here. panel_user/panel_pass are kept for reference only.
ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS panel_token TEXT NOT NULL DEFAULT '';
