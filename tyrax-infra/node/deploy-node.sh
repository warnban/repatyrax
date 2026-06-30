#!/usr/bin/env bash
#
# TYRAX VPN node bootstrap — Ubuntu 22.04/24.04 (Aeza Helsinki/Germany).
# Hardens the box, enables BBR, installs 3x-ui (Xray), opens only the ports we
# need, and generates a fresh Reality x25519 keypair + shortId for the inbound.
#
# Run as root on a CLEAN server:
#   ADMIN_IP=1.2.3.4 BACKEND_IP=5.6.7.8 PANEL_PORT=2053 bash deploy-node.sh
#
# ADMIN_IP    your home/office IP (SSH + panel access). REQUIRED.
# BACKEND_IP  the TYRAX backend server IP (it calls the 3x-ui API to sync UUIDs).
#             Optional now; add later once the backend exists.
# PANEL_PORT  3x-ui web panel port (default 2053). Never exposed to the world.
# NODE_PORT   VLESS listen port (default 443).
#
set -euo pipefail

PANEL_PORT="${PANEL_PORT:-2053}"
NODE_PORT="${NODE_PORT:-443}"
ADMIN_IP="${ADMIN_IP:-}"
BACKEND_IP="${BACKEND_IP:-}"

if [[ $EUID -ne 0 ]]; then echo "Run as root."; exit 1; fi
if [[ -z "$ADMIN_IP" ]]; then echo "Set ADMIN_IP=<your ip> (panel/SSH allowlist)."; exit 1; fi

echo "==> [1/6] System update"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y && apt-get upgrade -y
apt-get install -y curl ufw fail2ban ca-certificates openssl

echo "==> [2/6] Enable TCP BBR + network tuning (throughput)"
cat >/etc/sysctl.d/99-tyrax.conf <<'EOF'
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
net.ipv4.tcp_fastopen=3
net.core.rmem_max=26214400
net.core.wmem_max=26214400
net.ipv4.tcp_mtu_probing=1
EOF
sysctl --system >/dev/null

echo "==> [3/6] Firewall (ufw): SSH+panel from ADMIN/BACKEND only, 443 public"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow from "$ADMIN_IP" to any port 22 proto tcp
ufw allow "${NODE_PORT}/tcp"
ufw allow from "$ADMIN_IP" to any port "$PANEL_PORT" proto tcp
if [[ -n "$BACKEND_IP" ]]; then
  ufw allow from "$BACKEND_IP" to any port "$PANEL_PORT" proto tcp
fi
ufw --force enable
systemctl enable --now fail2ban

echo "==> [4/6] Install 3x-ui (bundles Xray-core)"
# Non-interactive install; you set login/port/path afterwards via `x-ui` menu.
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) <<EOF
n
EOF

echo "==> [5/6] Generate Reality x25519 keypair + shortId"
XRAY_BIN="$(find /usr/local/x-ui/bin -maxdepth 1 -name 'xray-*' -type f | head -n1 || true)"
if [[ -z "$XRAY_BIN" ]]; then echo "xray binary not found under /usr/local/x-ui/bin"; exit 1; fi
KEYS="$("$XRAY_BIN" x25519)"
PRIV="$(echo "$KEYS" | awk -F': ' '/Private/{print $2}')"
PUB="$(echo  "$KEYS" | awk -F': ' '/Public/{print $2}')"
SHORTID="$(openssl rand -hex 8)"

echo "==> [6/6] Done. Save these — they go into the 3x-ui inbound AND the DB row."
cat <<EOF

──────────────────────── TYRAX NODE PROVISIONED ────────────────────────
 Public IP        : $(curl -s4 https://api.ipify.org || echo '<run: curl -s4 ifconfig.me>')
 VLESS port       : ${NODE_PORT}
 Reality PRIVATE  : ${PRIV}        <- 3x-ui inbound only (NEVER to client/DB)
 Reality PUBLIC   : ${PUB}         <- DB nodes.reality_public_key
 Reality shortId  : ${SHORTID}     <- DB nodes.reality_short_id (NON-EMPTY!)
 SNI donor (set)  : www.microsoft.com   (dest www.microsoft.com:443)
 XHTTP path       : /api/v1/data
 Panel            : http://127.0.0.1:${PANEL_PORT}  (reach via: ssh -L ${PANEL_PORT}:127.0.0.1:${PANEL_PORT} root@<IP>)

 NEXT:
  1) Run \`x-ui\` -> set panel username/password + web base path. Record them
     (the backend needs them to sync client UUIDs via the panel API).
  2) In the panel, Add Inbound from tyrax-infra/node/inbound-xhttp-reality.json
     (paste PRIVATE key + shortId above; keep serverNames/dest/path identical).
  3) Fill tyrax-infra/db/insert_node.sql with PUBLIC key + shortId + this IP,
     then run it against the backend Postgres.
─────────────────────────────────────────────────────────────────────────
EOF
