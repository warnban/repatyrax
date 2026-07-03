#!/usr/bin/env bash
#
# TYRAX backend bootstrap — Ubuntu 22.04/24.04 (separate from VPN nodes).
# Installs Docker + Caddy, brings up the Go API + Postgres via docker-compose,
# and terminates TLS for the public API domain (api.<your-domain>).
#
# Run as root on a CLEAN server:
#   API_DOMAIN=api.tyrax.app ADMIN_IP=1.2.3.4 REPO_DIR=/opt/tyrax bash deploy-backend.sh
#
# API_DOMAIN  public FQDN for the API (A record must point here first). REQUIRED.
# ADMIN_IP    your IP for SSH (firewall allowlist). REQUIRED.
# REPO_DIR    where tyrax-backend is checked out (default /opt/tyrax/tyrax-backend).
#
set -euo pipefail

API_DOMAIN="${API_DOMAIN:-}"
ADMIN_IP="${ADMIN_IP:-}"
REPO_DIR="${REPO_DIR:-/opt/tyrax/tyrax-backend}"

if [[ $EUID -ne 0 ]]; then echo "Run as root."; exit 1; fi
if [[ -z "$API_DOMAIN" ]]; then echo "Set API_DOMAIN=api.<domain> (DNS must resolve to this host)."; exit 1; fi
if [[ -z "$ADMIN_IP" ]]; then echo "Set ADMIN_IP=<your ip>."; exit 1; fi

echo "==> [1/6] System update + firewall"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y && apt-get upgrade -y
apt-get install -y ca-certificates curl gnupg ufw debian-keyring debian-archive-keyring apt-transport-https openssl git
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow from "$ADMIN_IP" to any port 22 proto tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "==> [2/6] Install Docker Engine + compose plugin"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" >/etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

echo "==> [3/6] Install Caddy (auto-HTTPS reverse proxy)"
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' >/etc/apt/sources.list.d/caddy-stable.list
apt-get update -y && apt-get install -y caddy

echo "==> [4/6] Fetch backend source"
if [[ ! -d "$REPO_DIR" ]]; then
  echo "NOTE: clone your repo to $REPO_DIR first (git clone ... $REPO_DIR), then re-run." 
  echo "      Or copy tyrax-backend/ there manually."
  exit 1
fi
cd "$REPO_DIR"

echo "==> [5/6] Configure .env (edit secrets before going live!)"
if [[ ! -f .env ]]; then
  cp .env.example .env
  sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$(openssl rand -hex 32)|" .env
  sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(openssl rand -hex 16)|" .env
  echo "  -> .env created with random JWT_SECRET + POSTGRES_PASSWORD."
  echo "  -> Fill TELEGRAM_BOT_TOKEN / FREEKASSA_* / CRYPTO_PAY_TOKEN, then re-run compose."
fi
# Bind Postgres to localhost only (do not expose 5432 publicly).
docker compose up -d
echo "  -> Backend + Postgres up. Migrations 001..009 auto-applied on first boot."

echo "==> [6/6] Caddy reverse proxy -> :8080 with auto TLS"
ADMIN_DOMAIN="${ADMIN_DOMAIN:-admin.tyrax.tech}"
PARTNER_DOMAIN="${PARTNER_DOMAIN:-partner.tyrax.tech}"
cat >/etc/caddy/Caddyfile <<EOF
${API_DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}

${ADMIN_DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}

${PARTNER_DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}
EOF
systemctl reload caddy

echo "DONE. Verify: curl -s https://${API_DOMAIN}/health  (expect {\"status\":\"ok\"})"
echo "Admin panel: https://${ADMIN_DOMAIN}/"
echo "Point the Android app BASE_URL at https://${API_DOMAIN}/"
echo "If fronting with Cloudflare: set the DNS A record to PROXIED and use Full(strict) TLS."
