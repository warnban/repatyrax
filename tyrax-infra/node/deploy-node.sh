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
apt-get install -y curl ufw fail2ban ca-certificates openssl iptables-persistent

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

echo "==> [3b/6] TCP MSS clamp on ${NODE_PORT} (fixes mobile-carrier MTU black holes)"
# RU mobile carriers (GTP encapsulation + blocked ICMP 'frag needed') silently drop
# full-size 1460-MSS segments to foreign datacenter IPs, so a client's tunnel to the
# node establishes but then stalls (cwnd collapses to 1, endless retransmits, pages
# never load) while a plain browser to the same IP works. Advertising a small MSS
# forces both directions to use segments that fit through the radio path.
TYRAX_MSS="${TYRAX_MSS:-1280}"
iptables -t mangle -C PREROUTING -p tcp --dport "$NODE_PORT" --tcp-flags SYN,RST SYN -j TCPMSS --set-mss "$TYRAX_MSS" 2>/dev/null \
  || iptables -t mangle -A PREROUTING -p tcp --dport "$NODE_PORT" --tcp-flags SYN,RST SYN -j TCPMSS --set-mss "$TYRAX_MSS"
iptables -t mangle -C OUTPUT -p tcp --sport "$NODE_PORT" --tcp-flags SYN,RST SYN -j TCPMSS --set-mss "$TYRAX_MSS" 2>/dev/null \
  || iptables -t mangle -A OUTPUT -p tcp --sport "$NODE_PORT" --tcp-flags SYN,RST SYN -j TCPMSS --set-mss "$TYRAX_MSS"
mkdir -p /etc/iptables
iptables-save > /etc/iptables/rules.v4
systemctl enable netfilter-persistent >/dev/null 2>&1 || true

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
 SNI donor (use)  : www.apple.com   (dest www.apple.com:443)  # microsoft cert chain too big for Reality buffer
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
