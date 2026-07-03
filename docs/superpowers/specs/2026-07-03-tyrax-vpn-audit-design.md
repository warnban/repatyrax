# TYRAX — Full VPN Connectivity & Speed Audit (Android / Windows / HAPP) — Design

**Date:** 2026-07-03
**Status:** Design approved (verbal), pending written-spec review.

## Problem

On the latest Android build (**v1.0.2**, installed from the site / in-app update) the
native TYRAX client shows **ACCESS GRANTED but loads no internet at all**, and this
happens **with RU split-tunnel both ON and OFF**. The regression appeared right after a
batch of automated ("Composer 2.5 Fast") commits. The user also wants a **thorough,
detailed audit of every client path** — Android native, Windows native, and HAPP
(external client on phones and PCs) — with two explicit quality bars:

1. **Correctness:** every client actually passes traffic (no false "connected").
2. **High network speed:** the tunnel must be fast on RU mobile carriers, not just "up".

## Hard Requirements / Constraints

- **RU split-tunnel MUST stay working.** With the VPN on, RU-geoblocked apps
  (Wildberries, Ozon, banks, Gosuslugi, VK, Yandex, etc.) must exit **directly over the
  real Russian network**, bypassing the foreign node. This is a feature, never removed.
- Do **not** swap the working Android engine `app/libs/libv2ray.aar` (Xray core v26.x).
- **Version lock:** client Xray core major version MUST match the node's Xray core.
- **Reality:** `reality_short_id` MUST be non-empty (empty sid → decoy site → looks dead).
- Server inbound params MUST equal emitted client params exactly: `port`, `pbk`↔privateKey,
  `sid`, `sni`, `dest`, `type=xhttp`, `xhttp path`, `xhttp mode`, `flow`, `packetEncoding`.
- TYRAX brand copy rules: cold, uppercase, no soft/apologetic UI strings.

## Root-Cause Analysis (hypotheses, evidence-first)

Because traffic is dead in **both** split modes, split-tunnel routing is **excluded** as the
cause. With split OFF the Android routing is only: `dns-out`(port 53) → `direct`(private
CIDRs) → `proxy`(catch-all tcp/udp), `domainStrategy=AsIs`. Everything real goes to the
`proxy` (node) outbound. Dead in that mode ⇒ **the node is silently rejecting the proxied
stream**.

**H1 — UUID not registered on the node inbound (primary).** VLESS+Reality rejects an
unknown UUID by serving the decoy site; the TLS handshake still completes, so the client
reports "connected" while no real traffic flows. The recent commit
`1e3a7c7 fix(vpn): unblock connect when panel sync fails` changed `vpn_service.go` Connect
from hard-fail (`return ErrNodeUnavailable`) to `slog.Warn` + hand out config anyway. If
`panel.AddClient` is failing (bad `panel_url`/`panel_token`/`panel_inbound_id`, node/panel
down), the client now gets a config whose UUID the node doesn't know → **connected, no
traffic**. The same best-effort `AddClient` (with `_ = err`) is in the HAPP feed
(`happ_subscription_service.go:139`) → identical failure for HAPP, and the shared node row
means Windows is affected too.

**H2 — Node-row / Reality param mismatch (secondary).** A wrong/empty `reality_short_id`,
wrong `reality_public_key`, or mismatched `xhttp_path`/`mode`/`flow` in the `nodes` DB row
makes the node reject/decoy every client uniformly (Android, Windows, HAPP) — again
"connected, no traffic" in any split mode.

**H3 — Client-side transport/DNS regression (tertiary).** e.g. DoH bootstrap loop, MTU, or
xmux params the node's core version doesn't accept. Lower likelihood because the same
Android transport worked on Jul 1, but it is checked with on-device logs.

### Evidence to collect before any fix (systematic-debugging)

1. **On device** (`Android/data/com.tyrax/files/`): `xray_error.log`, `xray_access.log`,
   `xray_config.json` — reveals decoy/handshake reject vs DNS vs routing.
2. **Backend logs:** the new `slog.Warn("panel addClient (connect)" ...)` — shows whether
   panel sync is failing and the real error.
3. **3x-ui panel:** is the device UUID present on the inbound? Xray status + version.

First failing signal selects the fix branch; we do not fix blind.

## Audit Scope (cross-client parity + speed)

A single source-of-truth matrix `client × parameter × value/status`, walking the whole path
`nodes (DB) → panel AddClient → generator → client transform → live handshake`.

**Config generators / transforms in scope:**
- Backend: `pkg/vpnconfig/vless.go` (`GenerateVlessConfig`), `pkg/vpnconfig/vless_uri.go`
  (`GenerateVlessURI`), `internal/service/vpn_service.go` (Connect), HAPP feed.
- Android: `XrayConfigPatcher.kt`, `GeoAssets.kt`, `TyraxXrayVpnService.kt`, `SplitTunnel.kt`.
- Windows: `XrayWindowsConfigAdapter.cs`, `XrayConfigBuilder.cs`, `SplitTunnelDefaults.cs`,
  engine/version files under `Tyrax.Service/Engines/`.

**Audit axes:**
1. **UUID / panel sync integrity** — silent-failure sites (`_ = err`, downgraded `Warn`);
   ensure a client never gets a live "connected" state for a UUID the node rejects.
2. **Reality/XHTTP parity** — `pbk`, `sid` (non-empty), `sni`, `port`, `type`, `path`,
   `mode`, `flow`, `packetEncoding`, fingerprint identical across all three clients.
3. **Version lock** — Android `libv2ray.aar` (v26.x) vs Windows `xray.exe` vs node core;
   HAPP client-core expectations.
4. **Speed / xmux parity (HIGH priority)** — HAPP `vless://` currently carries **no xmux**,
   so on RU LTE it opens many H2 connections → carrier throttles → slow. Android and Windows
   also disagree on `hMaxReusableSecs` (1800–3000 vs 7200–10800). Normalise the single-mux
   tuning across clients (verify HAPP-supported query keys before shipping).
5. **Split-tunnel parity** — Android per-app + `geoip:ru`; Windows `SplitTunnelDefaults`;
   confirm RU-bypass exists/behaves on Windows too. Must remain fully functional.
6. **DNS strategy** — DoH bootstrap and `domainStrategy` consistency; no resolution loops.

## Deliverables

1. **Root-cause confirmation** for the Android "connected/no-traffic" regression (via the 3
   evidence sources) and the **minimal fix** on the failing branch.
2. **Silent-failure hardening:** connect/HAPP distinguish "UUID already on node (transient
   panel error → proceed)" from "UUID not on inbound (→ honest `NODE UNAVAILABLE`, no false
   ACCESS GRANTED)"; replace `_ = err` with real logging.
3. **Cross-client parity + speed report** (the matrix) with each divergence and its fix.
4. **HAPP + Windows verification** that they ping the node and load traffic.
5. RU split-tunnel preserved and verified end-to-end.

## Success Criteria

- Android native (split ON): ACCESS GRANTED **and** real traffic (YouTube/Google load).
- Wildberries / Ozon / banks work with the VPN on (they bypass directly over RU network).
- HAPP (phone + PC) and Windows native: node pings and traffic loads; speed is not
  throttled on RU LTE (single-mux tuning in effect).
- No client ever shows a live "connected" state for a UUID the node rejects.

## Out of Scope

- New node provisioning / new locations.
- Payment/subscription, auth, and website changes unrelated to the tunnel data plane.
- Replacing the Xray engine binaries or changing the transport profile (XHTTP+Reality stays).
