-- TYRAX — insert/update a VLESS + Reality + XHTTP node into the backend DB.
-- Run AFTER migrations 001..009 are applied (docker-compose does this on first
-- boot). Fill the <PLACEHOLDERS> with values from tyrax-infra/node/deploy-node.sh.
--
--   psql "$DATABASE_URL" -f insert_node.sql
--   (or: docker compose exec -T postgres psql -U tyrax -d tyrax -f - < insert_node.sql)
--
-- Profile A (default): XHTTP, no Vision, mode=auto.
-- For Profile B (max stealth): set flow='xtls-rprx-vision' and xhttp_mode is
-- auto-forced to 'stream-one' by the generator; mirror flow on the 3x-ui client.

INSERT INTO nodes (
    codename, country, host, port, protocol, status, min_tier,
    reality_public_key, reality_short_id, reality_sni, reality_dest,
    network, flow, xhttp_path, xhttp_mode, x_padding_bytes, fingerprint,
    panel_url, panel_user, panel_pass, panel_inbound_id
) VALUES (
    'FI-01',                       -- codename (unique)
    'Finland',                     -- country
    '<NODE_PUBLIC_IP_OR_DOMAIN>',  -- host  (the node IP from deploy-node.sh)
    443,                           -- port
    'vless',                       -- protocol
    'OPEN',                        -- status
    'FREE',                        -- min_tier (FREE = reachable by everyone)
    '<REALITY_PUBLIC_KEY>',        -- reality_public_key (NOT the private key!)
    '<REALITY_SHORT_ID>',          -- reality_short_id (must be NON-EMPTY)
    'www.microsoft.com',           -- reality_sni  (donor serverName)
    'www.microsoft.com:443',       -- reality_dest (server-side masquerade target)
    'xhttp',                       -- network
    '',                            -- flow ('' = Profile A; 'xtls-rprx-vision' = B)
    '/api/v1/data',                -- xhttp_path  (must match the inbound)
    'auto',                        -- xhttp_mode
    '100-1000',                    -- x_padding_bytes
    'chrome',                      -- fingerprint
    -- 3x-ui panel creds for per-device UUID sync (variant a). Leave panel_url
    -- empty to disable sync (manual / shared-UUID node).
    'https://<NODE_IP>:2053/<BASE_PATH>', -- panel_url (scheme+host+port+basePath, no trailing /)
    '<PANEL_USER>',                -- panel_user
    '<PANEL_PASS>',                -- panel_pass
    1                              -- panel_inbound_id (id of the inbound in 3x-ui, usually 1)
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

-- Optional: drop the placeholder WireGuard seed nodes from 001_init that point
-- at non-existent hosts, so auto-select never hands a client a dead node.
DELETE FROM nodes WHERE codename IN ('NL-01', 'DE-01') AND host LIKE '%.tyrax.app';
