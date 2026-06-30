-- TYRAX — insert/update a CDN-fronted node: VLESS + XHTTP + TLS via Cloudflare.
-- Profile A-CDN. The origin IP is hidden behind Cloudflare, so hoster subnet
-- reputation is almost irrelevant. Reality is NOT used (can't traverse a CDN).
--
-- Prereqs (see tyrax-infra/README.md "Cloudflare fronting"):
--   - Domain DNS for `host` is PROXIED through Cloudflare (orange cloud).
--   - Cloudflare SSL/TLS mode = Full (strict); origin has a CF Origin Certificate.
--   - 3x-ui inbound from node/inbound-xhttp-tls-cdn.json (path/host must match).

INSERT INTO nodes (
    codename, country, host, port, protocol, status, min_tier,
    reality_public_key, reality_short_id, reality_sni, reality_dest,
    security, network, flow, xhttp_path, xhttp_mode, x_padding_bytes, fingerprint,
    panel_url, panel_user, panel_pass, panel_inbound_id
) VALUES (
    'FI-CDN-01',                   -- codename (unique)
    'Finland',                     -- country
    'cdn.tyrax.app',               -- host = the CLOUDFLARE-PROXIED domain (NOT the IP)
    443,                           -- port (CF-proxied HTTPS: 443/2053/2083/2087/2096/8443)
    'vless',                       -- protocol
    'OPEN',                        -- status
    'FREE',                        -- min_tier
    '',                            -- reality_public_key (unused for tls)
    '',                            -- reality_short_id   (unused for tls)
    'cdn.tyrax.app',               -- reality_sni -> reused as TLS serverName (= domain)
    '',                            -- reality_dest (unused for tls)
    'tls',                         -- security = tls  (CDN profile)
    'xhttp',                       -- network
    '',                            -- flow (Vision is reality-only; keep empty)
    '/api/v1/data',                -- xhttp_path (must match inbound + Host)
    'packet-up',                   -- xhttp_mode (packet-up traverses CDNs)
    '100-1000',                    -- x_padding_bytes
    'chrome',                      -- fingerprint
    -- 3x-ui panel creds for per-device UUID sync. The backend reaches the panel
    -- on the ORIGIN IP/port directly (not via CF) — open that port to BACKEND_IP.
    'https://<ORIGIN_IP>:2053/<BASE_PATH>',
    '<PANEL_USER>',
    '<PANEL_PASS>',
    1
)
ON CONFLICT (codename) DO UPDATE SET
    country            = EXCLUDED.country,
    host               = EXCLUDED.host,
    port               = EXCLUDED.port,
    protocol           = EXCLUDED.protocol,
    status             = EXCLUDED.status,
    min_tier           = EXCLUDED.min_tier,
    reality_public_key = EXCLUDED.reality_public_key,
    reality_short_id   = EXCLUDED.reality_short_id,
    reality_sni        = EXCLUDED.reality_sni,
    reality_dest       = EXCLUDED.reality_dest,
    security           = EXCLUDED.security,
    network            = EXCLUDED.network,
    flow               = EXCLUDED.flow,
    xhttp_path         = EXCLUDED.xhttp_path,
    xhttp_mode         = EXCLUDED.xhttp_mode,
    x_padding_bytes    = EXCLUDED.x_padding_bytes,
    fingerprint        = EXCLUDED.fingerprint,
    panel_url          = EXCLUDED.panel_url,
    panel_user         = EXCLUDED.panel_user,
    panel_pass         = EXCLUDED.panel_pass,
    panel_inbound_id   = EXCLUDED.panel_inbound_id;
