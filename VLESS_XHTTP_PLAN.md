# TYRAX — VLESS + REALITY + XHTTP MIGRATION PLAN

> WORKING/PROGRESS DOCUMENT. Read this file at the start of every task in this
> project. It is the single source of truth for the anti-DPI (RU 2026) rework.
> Update the "PROGRESS LOG" section whenever code changes are made.

---

## 0. GOAL

Make the TYRAX VPN client reliably bypass Russian DPI (ТСПУ / РКН) in 2026.
The current VLESS + Reality over raw TCP gets throttled/RST'd by behavioural
analysis. Target stack: **VLESS + REALITY + XHTTP** (golden standard 2026),
with XTLS-Vision as an optional per-node profile.

There are **NO servers yet** (no backend host, no nodes). Servers will be rented
only AFTER the client+config pipeline is proven correct. So all work now is:
code + config generation + ops documentation, verifiable offline (compile +
JSON sanity + manual import into v2rayNG/HAPP once a test node exists).

---

## 1. ROOT-CAUSE DIAGNOSIS (why it's blocked now)

Effective runtime path:
`POST /vpn/connect` → backend `GenerateVlessConfig` (pkg/vpnconfig/vless.go)
→ full Xray JSON in `VpnConfig.config` → `XrayManager.startVpn`
→ `TyraxXrayVpnService` runs `XrayConfigPatcher.enhance()` → libv2ray core.

(Note: `XrayConfigBuilder.kt` + `VlessConfig.kt` are currently DEAD CODE — nothing
calls `.build()` / constructs `VlessConfig`. Kept in lockstep for future use.)

Problems with the current config:
| Field | Now | Why it fails in 2026 |
|---|---|---|
| `network` | `tcp` | Long-lived TCP to foreign IP, no request/response structure → behavioural detection → throttling to 2–5 Kbit/s after 30–60s. |
| `flow` | `""` | No `xtls-rprx-vision` → TLS-in-TLS pattern visible on first packets. |
| `shortId` | `""` | Empty short id weakens Reality auth / makes node generic. |
| SNI donor | per-node, uncontrolled | ASN mismatch + active probing risk. |

Reality still passes signature/JA3/active-probing. The **behavioural layer** is
what breaks → must be closed by **XHTTP** (wraps tunnel into normal HTTP/2-3
request/response transactions + `xPaddingBytes` to normalise packet sizes).

Reality and XHTTP are complementary (different detection layers), not rivals.

---

## 2. KEY DECISIONS

- **Panel = 3x-ui** (NOT Marzban). Rationale: single binary + web UI, native
  XHTTP+Reality inbound editor, lightweight, ideal for self-host. The Go backend
  already owns users/devices/nodes/subscriptions, so Marzban's multi-node DB
  orchestration is redundant. 3x-ui just runs the Xray inbound on each node; the
  backend stores the matching Reality params and generates client configs.
- **Engine: no rebuild needed.** Bundled `libgojni.so` (in `app/libs/libv2ray.aar`)
  already contains `XHTTP`, `splithttp`, `xtls-rprx-vision`, `xPaddingBytes`,
  `packet-up/stream-up/stream-one`. Verified via `strings`.
- **Transport is per-node configurable** (DB columns) → gradual rollout + fallback.
- **Default profile (Profile A):** VLESS + Reality + XHTTP, `mode=auto`,
  `xPaddingBytes=100-1000`, `fingerprint=chrome`, real `shortId`, good SNI donor,
  `flow=""` (no Vision). XHTTP padding makes Vision largely redundant and `auto`
  keeps CDN/relay compatibility.
- **Profile B (max stealth, opt-in per node):** XHTTP `mode=stream-one` +
  `flow=xtls-rprx-vision`. Best TLS-in-TLS hiding, direct only, slightly slower.
- **Profile C (legacy fallback):** TCP + Reality but WITH Vision + shortId.
- **Version lock:** server Xray core MUST match the client's bundled core major
  version. XHTTP is actively developed; mismatch = silent breakage.
- **SNI donor rules:** real high-traffic service in the node's region, correct
  ASN, no RU PoP. GOOD: `www.microsoft.com`, `github.com`, `www.twitch.tv`.
  BAD: Apple (own ASN → mismatch), small sites, any `.ru`. Port **443**.

---

## 3. IMPLEMENTATION (by layer)

### 3.1 Backend (Go)
- `internal/model/user.go` `Node` struct: add `Network`, `Flow`, `XhttpPath`,
  `XhttpMode`, `Fingerprint`, `XPaddingBytes`, `RealityDest`.
- `migrations/009_node_xhttp.sql`: ALTER TABLE nodes ADD COLUMN ... with safe
  defaults (existing nodes keep working). Default vless nodes to XHTTP profile.
- `repository/node_repository_impl.go`: extend `nodeColumns` + `scanNode`.
- `pkg/vpnconfig/vless.go` `GenerateVlessConfig`: emit `network`, optional
  `xhttpSettings{path,mode,extra.xPaddingBytes}`, `flow`, real `shortId`,
  `fingerprint` from node.
- `internal/service/vpn_service.go` `DeviceConfig` + handler DTO: pass through
  new structured fields (no `omitempty` on strings).

### 3.2 Android (Kotlin)
- `data/remote/Dtos.kt`: `DeviceConfigDto` + `NodeDto` new fields.
- `domain/model/Device.kt`: `DeviceConfig` new fields.
- `data/repository/VpnRepositoryImpl.kt`: map new fields.
- `data/vpn/VlessConfig.kt` + `XrayConfigBuilder.kt`: parity (network/flow/xhttp).
- `data/vpn/XrayConfigPatcher.kt`: keep XHTTP `streamSettings` + `flow` intact;
  verify routing/packetEncoding coexist with Vision.

### 3.3 Node setup (3x-ui) — ops doc (do when server is rented)
Inbound template (server side, must MATCH backend DB row for the node):
```
protocol: vless
port: 443
network: xhttp
xhttp: { path: "/api/v1/data", mode: "auto", extra: { xPaddingBytes: "100-1000" } }
security: reality
reality: {
  dest: "www.microsoft.com:443",
  serverNames: ["www.microsoft.com"],
  privateKey/publicKey: x25519 pair,
  shortIds: ["<non-empty>"],
  fingerprint: chrome
}
client flow: "" (Profile A) or "xtls-rprx-vision" (Profile B, mode=stream-one)
```

---

## 4. VERIFICATION CHECKLIST (when a test node exists)
1. JA3 via proxy on scrapfly.io ja3 == clean Chrome.
2. Active probing: `curl -v https://<node> --resolve <node>:443:<IP>` → donor HTTP, not TLS error.
3. **Throttling test (main):** sustained download >2–3 min on RU mobile — speed must NOT collapse.
4. Server/client Xray versions match.
5. Regression: WireGuard path + legacy TCP profile still connect.
6. Sanity: generated client JSON imports cleanly into v2rayNG/HAPP and connects.

---

## 5. ROLLOUT ORDER
1. Backend migration + fields + generator (default nodes already improved).
2. Stand up 1 test node on 3x-ui (Profile A); verify manually.
3. Ship Android changes built against current `libgojni.so`.
4. Flip nodes to xhttp, enable auto-fallback chain, monitor.

---

## 6. PROGRESS LOG
- 2026-07-01: Profile A-CDN IMPLEMENTED — per-node `security` (reality|tls).
  TLS/CDN path: backend `migrations/011_node_security.sql`, Node.Security + repo
  scan, generator `buildStreamSettings` TLS branch (tlsSettings + xhttp host
  header) + xrayTLSSettings type; service DeviceConfig.Security; Android DTO/
  domain/repo + VlessConfig.security + XrayConfigBuilder TLS branch. Infra:
  `node/inbound-xhttp-tls-cdn.json`, `db/insert_node_cdn.sql`, README Cloudflare
  section + profile table. go build/vet + android compile PASS; CDN JSON verified
  (security=tls, tlsSettings, xhttp host, packet-up, no realitySettings).
  Reality can't traverse a CDN → CDN profile uses real TLS on a proxied domain;
  origin IP hidden so hoster subnet reputation becomes near-irrelevant.
- 2026-07-01: 3x-ui UUID sync (variant a) IMPLEMENTED — see §8. Backend builds +
  vets clean. This closes the last code gap before renting servers.
- 2026-07-01: Plan created. Research done (Habr/2026 DPI). Engine confirmed
  XHTTP-capable. Panel decision: 3x-ui. Starting code implementation.
- 2026-07-01: Deployment scaffolding added under `tyrax-infra/`:
  `node/deploy-node.sh` (+ `inbound-xhttp-reality.json`),
  `backend/deploy-backend.sh` (+ `Caddyfile`), `db/insert_node.sql`, `README.md`.
  Server plan (Aeza): phase0 1 node, phase1 backend+1 node, phase2 +2-3 nodes.
  UUID-sync decision = variant (a) 3x-ui API (code task still pending).
- 2026-07-01: Implementation DONE (code only; no servers yet).
  - Backend:
    - `internal/model/user.go` Node: added Network, Flow, XhttpPath, XhttpMode,
      XPaddingBytes, Fingerprint, RealityDest.
    - `migrations/009_node_xhttp.sql`: new columns + defaults; vless nodes
      flipped to XHTTP profile.
    - `repository/node_repository_impl.go`: nodeColumns + scanNode extended.
    - `pkg/vpnconfig/vless.go`: `buildStreamSettings` emits per-node transport;
      XHTTP `xhttpSettings{path,mode,extra.xPaddingBytes}`, user `flow`,
      fingerprint + real shortId from node; Vision auto-forces `stream-one`.
    - `internal/service/vpn_service.go`: DeviceConfig carries new fields.
    - `go build ./...` PASS. Verified generated JSON for Profile A / B / blank.
  - Android:
    - `data/remote/Dtos.kt` DeviceConfigDto + `domain/model/Device.kt` +
      `data/repository/VpnRepositoryImpl.kt` mapping: new fields plumbed.
    - `data/vpn/VlessConfig.kt` + `XrayConfigBuilder.kt`: parity (network/flow/
      xhttp/fingerprint; Vision→stream-one guard).
    - `data/vpn/XrayConfigPatcher.kt`: confirmed transport-agnostic (leaves
      streamSettings + flow intact); comment updated.
    - `:app:compileDebugKotlin` PASS.
  - Runtime path reminder: `/vpn/connect` → backend `GenerateVlessConfig` →
    `XrayConfigPatcher.enhance()` → libv2ray. The backend generator is the
    effective source of the transport; client builder is parity/fallback.

## 7. SERVER / DEPLOYMENT PLAN (Aeza)

Provider: **Aeza** (RU-card friendly, unlimited traffic, 1–25 Gbps, DDoS incl.).
Best latency for RU users: **Finland/Helsinki 30–50 ms** > Germany 40–60 ms >
Netherlands 35–70 ms. Use Finland as the primary node location.
Caveat — provider reputation matters MORE in 2026 (RKN now blocks whole
SUBNETS, not just single IPs). This is a SEPARATE layer from our XHTTP+Reality
work: a dirty subnet can block you regardless of a perfect config. Both layers
must pass.
- AEZA: OFAC-sanctioned (2025-07-01) bulletproof host; migrated its ranges to a
  new ASN AS211522 (Hypercore) to evade → its ASNs are dirty / blocklist-prone.
  DO NOT run production nodes on Aeza, and don't even Phase-0 test on it (a
  pre-blocked Aeza subnet would make a good config look broken). At most a
  throwaway experiment.
- PREFER a clean, less-saturated host for nodes: HostKey (legit since 2008,
  NL/Amsterdam, KVM, RU cards, free IP change), or similar. Keep free IP-change.
- STRONGEST mitigation (makes provider choice almost irrelevant): front the node
  with Cloudflare CDN via XHTTP `packet-up` → origin IP/subnet is hidden behind
  Cloudflare IPs. Also: put a domain in front so you can repoint to a fresh IP.
- 2026 note: `fingerprint: chrome` reportedly under more suspicion; Firefox more
  lenient. Already per-node configurable in our schema (`fingerprint` column).
  Consider a "Profile A-CDN" variant: XHTTP packet-up + Cloudflare fronting.

Control plane (backend + Postgres) and data plane (VPN nodes) MUST be separate
servers: a blocked/flagged node IP must never take down auth/DB.

| Phase | Servers | Aeza plan | Location | ~Price |
|---|---|---|---|---|
| 0 Test | 1 (node only, hourly, destroy after) | HELs-1 (1vCPU/2GB/30GB) | Helsinki | €4.94/mo or €0.02/h |
| 1 MVP | Backend ×1 + Node ×1 | Backend DEs-1/HELs-1 (1vCPU/2GB); Node HELs-2 (2vCPU/4GB/60GB) | FI/DE | ~€15/mo total |
| 2 Scale | Backend ×1 + Nodes ×2–3 | Nodes HELs-2 in FI+DE(+NL) | FI/DE/NL | ~€25–35/mo |

Sizing: Xray is light (1vCPU/2GB ≈ 100–300 concurrent users; bottleneck is
network, which is unlimited on Aeza). Backend (Go+Postgres) on 2GB serves
thousands — it's low-QPS auth + config-gen. Domain: 1 needed for backend API
(api.<domain>) with TLS (Caddy/Let's Encrypt) + optional Cloudflare proxy.
Nodes can run by IP (Reality uses the donor SNI, not your domain).

### 8. RESOLVED — 3x-ui client UUID sync (variant a IMPLEMENTED 2026-07-01)
Implemented:
- `migrations/010_node_panel.sql`: nodes.panel_url/panel_user/panel_pass/
  panel_inbound_id (secrets, json:"-").
- `internal/model/user.go`: Node panel fields; repo scan extended.
- `pkg/threexui/client.go`: panel API Client (login → addClient/delClient, cookie
  jar, self-signed TLS skip, re-login retry, idempotent dup/not-found) + Syncer
  (per-node client cache; empty panel_url = no-op).
- `internal/service/vpn_service.go`: PanelSyncer interface + wired:
  - AddDevice (vless): register on best node, best-effort (logs, won't fail).
  - Connect (vless): register on target node; HARD fail (ErrNodeUnavailable) if
    sync fails — without it Xray serves the decoy site.
  - GetConfig (vless): best-effort register.
  - DeleteDevice: remove UUID from all vless nodes, best-effort, then delete.
- `cmd/server/main.go`: threexui.NewSyncer() injected.
- `tyrax-infra/db/insert_node.sql`: panel_* columns added.
- `go build ./...` + `go vet ./...` PASS.
Operator must fill panel_url/user/pass/inbound_id in the node DB row (see
insert_node.sql) and open the panel port to BACKEND_IP (deploy-node.sh).

--- historical note (original gap description) ---
### 8b. ORIGINAL GAP — 3x-ui client UUID sync (must resolve for app↔backend↔node)
Backend issues a per-device `vless_uuid` (migration 008). Xray on the node only
authenticates UUIDs present in the inbound's client list. So:
- PHASE 0 proof bypasses this entirely: import 3x-ui's own vless:// link into
  HAPP/v2rayNG and test — no backend involved.
- PHASE 1+ options:
  (a) RECOMMENDED: backend calls the 3x-ui panel API (login →
      /panel/api/inbounds/addClient) to register each device UUID on AddDevice/
      Connect. New coding task (3x-ui API client in tyrax-backend).
  (b) SIMPLE INTERIM: one shared UUID per node; backend returns that node UUID
      instead of a per-device one. Loses per-device Xray-level identity (limits
      are enforced by the backend anyway). Needs a small generator/service tweak.
  DECISION: variant (a) — 3x-ui panel API integration. To implement in
  tyrax-backend: a 3x-ui API client (login + cookie/session, addClient,
  delClient), per-node panel creds/base-path/inbound-id stored in the nodes
  table (or a secrets table), wired into AddDevice/Connect/DeleteDevice.
  Deployment scaffolding for this lives in tyrax-infra/ (firewall opens the
  panel port to BACKEND_IP).

### 9. RESOLVED — client auto-fallback & silent reconnect (IMPLEMENTED 2026-07-01)
Goal: "works in any conditions" — when a node dies or DPI throttles it, switch
to another node/profile with no user action.

Key insight: [TyraxXrayVpnService] excludes our own package from the TUN, so an
in-app request does NOT traverse the tunnel. Health must be probed THROUGH the
Xray SOCKS inbound (127.0.0.1:10808) to exercise the real VLESS → node → net path.

Implemented (Android):
- `data/vpn/TunnelHealth.kt`: OkHttp client bound to the SOCKS5 inbound, GETs a
  204 endpoint (gstatic/generate_204) with a 6s timeout. Returns {ok, elapsedMs}.
- `domain/usecase/ConnectionSupervisor.kt` (@Singleton, app-scoped coroutine):
  - loadCandidates(): GET /nodes → OPEN, sorted by ping (falls back to bestNode).
    Different security profiles (Reality / TLS-CDN / Vision) are separate node
    rows, so profile fallback == node iteration. No protocol negotiation needed.
  - attemptNode(): starts engine via ConnectToNodeUseCase, awaits Connected.
    NeedsPermission is treated as a PAUSE (waits for consent), not a failure.
  - monitorUntilUnhealthy(): every 15s probes SOCKS; 2 consecutive failures OR
    "ok but > 4.5s" (throttle) → declare dead. WG path (no SOCKS) just waits for
    the state to leave Connected.
  - runSupervision(): on death → Reconnecting → teardown → next candidate;
    after 2 full passes with no luck → 8s backoff + refresh node list.
  - stop(): cancels the loop and tears the tunnel down.
- `MainViewModel`: connect()/disconnect() now delegate to the supervisor;
  removed the single-shot connectJob path. onPermissionGranted() unchanged.
- `./gradlew :app:compileDebugKotlin` PASS (incl. Hilt kapt).

Tuning constants (ConnectionSupervisor.companion): CONNECT_TIMEOUT 25s,
PERMISSION_WAIT 120s, INITIAL_GRACE 6s, PROBE_INTERVAL 15s, THROTTLE 4.5s,
MAX_FAILS 2, SWITCH_DELAY 1.2s, BACKOFF 8s. Adjust after RU-mobile field testing.

### 10. RESOLVED — tunnel dead on RU mobile: XHTTP connection explosion (2026-07-01)
Symptom: on a real RU phone (mobile/LTE) the tunnel connected but NOTHING loaded —
pages hung, "reconnecting". The SAME config on a Windows PC in RU loaded real pages
(google/youtube/cloudflare, MBs) in <1s. So config, node, and Xray core were proven
fine; the failure was phone-specific.

Diagnosis (via adb + `ss -tin` on the device):
- Node reachable from the phone's physical network: 0% ping loss, 40–70ms, and a
  single direct download from the node IP (reality-fallback to apple.com) ran fine
  at ~45 KB/s. So the phone↔node path was healthy for ONE connection.
- With the tunnel UP, Xray opened **31–76 parallel TCP connections** to the node
  (XHTTP xmux spawns a new client whenever `maxConcurrency` is hit). On the mobile
  radio those connections collapsed: `cwnd:1`, `retrans:5–7`, `rto:9s`, data stuck
  in Send-Q. Two compounding causes:
    1. Many simultaneous TLS connections to one foreign datacenter IP fingerprint as
       a VPN → carrier throttles them to a crawl.
    2. Dozens of parallel flows self-congest the radio link.
- MSS/MTU black hole was ALSO present (rmnet advertises 1500 but GTP path is smaller,
  ICMP frag-needed blocked) — addressed defensively but was NOT the primary cause.

Fix (TWO parts, both shipped):
- CLIENT (primary): `XrayConfigPatcher.tuneXhttpMux()` injects into the proxy
  outbound's `xhttpSettings.extra.xmux`:
    `maxConcurrency:0, maxConnections:1, cMaxReuseTimes:0,
     hMaxRequestTimes:"1000-5000", hMaxReusableSecs:"1800-3000", hKeepAlivePeriod:0`
  → ONE persistent multiplexed H2 connection carrying all streams (looks like a
  browser talking to apple.com). Result: 1 connection, `cwnd:10`, 0 retransmit;
  google 200 in 0.28s, youtube/cloudflare MBs through the tunnel. CONFIRMED WORKING
  on the user's phone.
- NODE (defensive): TCP MSS clamp to 1280 on the VLESS port, made persistent and
  baked into `deploy-node.sh` ([3b/6], `iptables-persistent`, `TYRAX_MSS=1280`):
    `iptables -t mangle -A PREROUTING -p tcp --dport 443 --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1280`
    `iptables -t mangle -A OUTPUT     -p tcp --sport 443 --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1280`

Also in this session: reverted Xray `loglevel` to `warning` (debug logging at RU
mobile traffic volume was itself I/O-thrashing the tunnel); relaxed the §9 supervisor
so a busy-but-live tunnel is not torn down (INITIAL_GRACE 12s, PROBE_INTERVAL 30s,
THROTTLE 9s, MAX_FAILS 4) and TunnelHealth timeout 6s→10s. libv2ray.aar aligned to a
current Xray core (v26.x) matching the node.

OPS NOTE: every node needs the MSS clamp. `deploy-node.sh` now does it automatically;
for the already-live FI-01 node it was applied by hand + saved via `iptables-save >
/etc/iptables/rules.v4`. Apply the same on PL-01 and any future node.

### NEXT (requires a rented server — NOT done yet)
- Rent VPS, install 3x-ui, create XHTTP+Reality inbound per §3.3 template.
- Insert matching node row into `nodes` (host/port/reality keys/shortId/sni/dest/
  network/flow/xhttp_*). shortId + privateKey/publicKey from `xray x25519`.
- Provision ≥2 nodes/profiles (e.g. Reality-direct + TLS-CDN) so the client
  auto-fallback (§9) has somewhere to switch to.
- Run VERIFICATION CHECKLIST §4 (esp. throttling test on RU mobile) and tune the
  §9 supervisor constants against real degradation behaviour.
