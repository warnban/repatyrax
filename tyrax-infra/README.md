# TYRAX — Infrastructure & Deployment

Deployment playbook for the VLESS + Reality + XHTTP stack (RU 2026 anti-DPI).
See `../VLESS_XHTTP_PLAN.md` for the full rationale and decisions.

> Control plane (backend + Postgres) and data plane (VPN nodes) live on
> SEPARATE servers. A blocked node IP must never take down auth/DB.

## Layout
```
tyrax-infra/
├── website/
│   ├── deploy-website.sh           # manual rsync (fallback)
│   └── deploy-website.ps1
├── node/
│   ├── deploy-node.sh              # harden + BBR + 3x-ui + Reality keygen
│   └── inbound-xhttp-reality.json  # 3x-ui inbound template (Profile A)
├── backend/
│   ├── deploy-backend.sh           # Docker + compose + Caddy auto-TLS
│   └── Caddyfile                   # reference reverse-proxy config
└── db/
    └── insert_node.sql             # add/update a node row in Postgres
```

## Servers to rent (Aeza)
| Role | Plan | Location | Notes |
|---|---|---|---|
| Test node (phase 0) | HELs-1 (1vCPU/2GB) hourly | Helsinki | destroy after proving bypass |
| Backend (phase 1) | DEs-1/HELs-1 (1vCPU/2GB) | FI/DE | needs a domain + TLS |
| Node(s) (phase 1–2) | HELs-2 (2vCPU/4GB) | FI(+DE+NL) | unlimited traffic, port ≥1Gbps |

## Order of operations

### Phase 0 — prove DPI bypass (no backend)
1. Rent 1 Aeza Helsinki VPS, Ubuntu 24.04.
2. `ADMIN_IP=<your-ip> bash node/deploy-node.sh`
3. `x-ui` → set panel creds + base path. Open panel via SSH tunnel:
   `ssh -L 2053:127.0.0.1:2053 root@<node-ip>`
4. Add Inbound from `node/inbound-xhttp-reality.json` (paste the PRIVATE key +
   shortId printed by the script; generate a bootstrap client UUID).
5. Export the `vless://` link → import into HAPP/v2rayNG → test on RU mobile+home.
   Run the checklist in `../VLESS_XHTTP_PLAN.md` §4 (esp. 3-min throttling test).
6. Stable? proceed. Throttled? set Profile B (Vision) and/or change SNI donor.

### Phase 1 — backend + node
7. Rent backend VPS. Create DNS A record `api.<domain>` → backend IP.
8. `git clone <repo> /opt/tyrax` then
   `API_DOMAIN=api.<domain> ADMIN_IP=<your-ip> bash backend/deploy-backend.sh`
9. Edit `/opt/tyrax/tyrax-backend/.env` (Telegram/payment secrets), then
   `docker compose up -d` again.
10. Fill `db/insert_node.sql` with the node's PUBLIC key + shortId + IP and run it.
11. Re-run `node/deploy-node.sh` with `BACKEND_IP=<backend-ip>` (or just
    `ufw allow from <backend-ip> to any port 2053`) so the backend can reach the
    3x-ui API for per-device UUID sync (see UUID-sync task below).
12. Build the Android app with `BASE_URL=https://api.<domain>/`, install, connect.

## Deploy via GitHub (`repatyrax`)

Production servers pull from **https://github.com/warnban/repatyrax.git**:

```bash
# Local — push changes
git push repatyrax master

# Backend (api.tyrax.tech — 5.129.195.144)
ssh root@5.129.195.144
cd /opt/tyrax && git pull
cd tyrax-backend && docker compose up -d --build

# Website (tyrax.tech — 147.45.245.80)
ssh root@147.45.245.80
cd /opt/tyrax && git pull
# nginx root → /opt/tyrax/tyrax-website (or rsync to /var/www/tyrax.tech)
```

Windows installer lives in `tyrax-website/download/windows/TYRAX-Setup.exe` —
stage locally with `tyrax-windows/build/stage-website-download.ps1` before push.

### Phase 2 — scale
13. Repeat node steps in DE/NL; add a DB row per node. Auto-select picks best ping.

## Profiles (per-node `security` + `network`)
| Profile | DB `security` | DB `network` | Use | IP exposure |
|---|---|---|---|---|
| A (default) | `reality` | `xhttp` | direct, steal-from-self TLS | origin IP visible |
| A-CDN | `tls` | `xhttp` (mode `packet-up`) | **fronted by Cloudflare** | origin IP HIDDEN |
| B (max stealth) | `reality` | `xhttp` + `flow=xtls-rprx-vision` | direct, Vision (stream-one) | origin IP visible |
| C (legacy) | `reality` | `tcp` | fallback | origin IP visible |

A-CDN is the most block-resistant in hostile conditions: because traffic goes to
Cloudflare's IPs, the hoster subnet reputation is almost irrelevant. Reality
CANNOT be used over a CDN (it terminates TLS via a direct steal), so A-CDN uses
real TLS on a proxied domain.

## Cloudflare fronting (Profile A-CDN)
1. Add your domain to Cloudflare (free plan is enough).
2. DNS: `A`/`AAAA` record for `cdn.<domain>` → ORIGIN IP, **Proxied (orange cloud)**.
3. SSL/TLS mode = **Full (strict)**.
4. Origin cert: Cloudflare dashboard → SSL/TLS → Origin Server → Create
   Certificate. Save the cert+key on the node (e.g. `/root/cert/cdn.<domain>.crt`
   and `.key`) and reference them in the 3x-ui inbound.
5. In 3x-ui, Add Inbound from `node/inbound-xhttp-tls-cdn.json` (VLESS + XHTTP +
   TLS). Port must be a CF-proxied HTTPS port: 443/2053/2083/2087/2096/8443.
   `path` + `host` (= the proxied domain) must match the DB row.
6. DB: use `db/insert_node_cdn.sql` (`security='tls'`, `host=cdn.<domain>`,
   `xhttp_mode='packet-up'`). The backend still reaches the 3x-ui panel API on the
   ORIGIN IP directly (open that port to BACKEND_IP), NOT through Cloudflare.
7. Verify: the client connects to `cdn.<domain>` (Cloudflare edge) and the origin
   IP never appears in the handshake.

## Security notes
- `tyrax-backend/docker-compose.yml` publishes Postgres as `5432:5432`. For a
  public VPS, change it to `127.0.0.1:5432:5432` so the DB is not world-exposed.
- Never expose the 3x-ui panel port to `0.0.0.0`; the firewall allows only your
  ADMIN_IP and the BACKEND_IP.
- The Reality PRIVATE key stays on the node (3x-ui) only. The DB/clients get the
  PUBLIC key.

## Pending coding task — 3x-ui UUID sync (chosen: API integration)
The backend issues a per-device VLESS UUID; the node's Xray only accepts UUIDs in
its inbound client list. Backend must register each device UUID via the 3x-ui
panel API (`POST /login` → `POST /panel/api/inbounds/addClient`) on AddDevice/
Connect, and remove it on DeleteDevice. This requires node-side panel credentials
+ base path + inbound id stored per node (extend the `nodes` table / config).
Tracked in `../VLESS_XHTTP_PLAN.md` §8.
