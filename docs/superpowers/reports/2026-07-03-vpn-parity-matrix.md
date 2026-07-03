# TYRAX VPN — Cross-Client Parity Matrix (Android / Windows / HAPP)

**Date:** 2026-07-03  **Branch:** `fix/vpn-audit`
**Sources:** `pkg/vpnconfig/vless.go` (`GenerateVlessConfig`), `pkg/vpnconfig/vless_uri.go`
(`GenerateVlessURI`), Android `XrayConfigPatcher.kt` / `GeoAssets.kt` / `SplitTunnel.kt`,
Windows `XrayWindowsConfigAdapter.cs` / `SplitTunnelDefaults.cs`.

The backend `/vpn/connect` emits a SOCKS-inbound profile (`GenerateVlessConfig`) that the
**native clients rewrite** before running Xray: Android via `XrayConfigPatcher`, Windows via
`XrayWindowsConfigAdapter`. HAPP consumes the raw `vless://` share link (`GenerateVlessURI`)
and applies its own inbound/routing. So the columns below are the **effective client config**.

## Matrix

| Parameter | Backend base (`GenerateVlessConfig`) | Android (`XrayConfigPatcher`) | Windows (`XrayWindowsConfigAdapter`) | HAPP (`GenerateVlessURI`) | Status |
|---|---|---|---|---|---|
| VLESS `id` (UUID) | node/device UUID | inherited | inherited | in link | OK — must be registered on node (Task 2/3) |
| `encryption` | none | none | none | none | OK |
| Reality `pbk`/`sid`/`sni` | from node row | inherited | inherited | pbk/sid/sni in link | OK — `sid` MUST be non-empty (Task 1 §4) |
| `flow` | node.Flow | inherited | inherited | in link if set | OK |
| fingerprint | chrome | inherited | inherited | chrome | OK |
| network `type` | xhttp | xhttp | xhttp | xhttp | OK |
| xhttp `path`/`mode` | `/api/v1/data`/`auto` | inherited | inherited | in link (Vision→stream-one) | OK |
| xhttp `xPaddingBytes` | `100-1000` | preserved | preserved | not in link | Minor — HAPP link omits padding |
| `packetEncoding` | not set | **xudp** | **xudp** | not in link (client default) | Minor — HAPP relies on client default |
| **xmux** | none (client adds) | maxConn=1, maxConc=0, reusableSecs **1800-3000** | maxConn=1, maxConc=0, reusableSecs **7200-10800** | **now** maxConn=1, maxConc=0, reusableSecs 1800-3000 (Task 4) | OK — HAPP fixed (Task 4). Windows reusableSecs longer = intentional desktop tuning |
| DNS | fakedns + 1.1.1.1:53 via proxy | DoH 1.1.1.1/8.8.8.8 | DoH 1.1.1.1/8.8.8.8 | HAPP client default | OK (native override); intentional |
| sniffing destOverride | http,tls,quic,fakedns | http,tls | http,tls | client default | Minor — native drops quic (fakedns removed) |
| `domainStrategy` | IPIfNonMatch | AsIs (split off) / IPIfNonMatch (split on) | AsIs / IPIfNonMatch (split on) — Task 5 | client default | OK |
| Private ranges → direct | yes (excl. 10/8 semantics) | yes (excl. 10/8) | yes (excl. 10/8; TUN 10.7.0.x) | n/a | OK |
| **RU split-tunnel** | n/a (base) | `geoip:ru` + `domain:`(RU_SPLIT_DOMAINS) + per-app disallow | **now** `geosite:category-ru` + `domain:` + `geoip:ru` (Task 5) | **none** (external client) | See divergences #1/#2 |
| geo assets (`geoip.dat`/`geosite.dat`) | n/a | bundled in `assets/` | shipped in `engines/` (Task 5 §1) | client-bundled | OK |
| Xray core major | n/a | v26.x (`libv2ray.aar`) | `xray.exe` (verify) | Happ-bundled (verify) | ⚠️ version-lock vs node — Task 1 §5 |

## Divergences & resolution

1. **HAPP has no RU split-tunnel (inherent).** A `vless://` link cannot express per-app
   exclusion or client routing; HAPP routes everything through the node. RU-geoblocked apps
   will NOT bypass on HAPP. This is a HAPP-client limitation, not a backend bug. Recommendation:
   document for HAPP users that RU apps may need the native TYRAX app; or ship a second
   "RU-direct" HAPP profile/routing if HAPP supports client-side rules (out of current scope).

2. **Android vs Windows RU mechanism differ slightly.** Android bypasses via `geoip:ru` +
   an explicit `domain:` list (`SplitTunnel.RU_SPLIT_DOMAINS`) + per-app `addDisallowedApplication`;
   Windows (Task 5) uses `geosite:category-ru` + `domain:` + `geoip:ru` (no per-app layer — the
   native TUN routes by domain/IP). Both achieve RU-direct; Windows' `geosite:category-ru` is
   broader. **Follow-up (Minor):** consider adding `geosite:category-ru` to the Android bypass
   too for parity/coverage (Android already bundles `geosite.dat`). Not blocking.

3. **xmux `hMaxReusableSecs`: Android 1800-3000 vs Windows 7200-10800.** Intentional
   per-platform tuning (desktop sessions run longer; documented in `XrayWindowsConfigAdapter`).
   HAPP mirrors Android's mobile values. No action.

4. **Minor link omissions on HAPP:** `xPaddingBytes`, `packetEncoding` are not in the share
   link (HAPP/Xray apply client defaults). Low impact. No action unless field testing shows a gap.

## Correctness gate (the actual outage)

The "connected/no-traffic in both split modes" outage is **not** a parity issue — it is the
node silently rejecting an unregistered UUID (H1) masked by `1e3a7c7`. Task 2 (committed
`0e2ec9c`) makes connect/HAPP fail honestly; Task 1 confirms the branch and Task 3 applies the
node/panel fix. Verify per Task 6 Steps 2-5 (USER).

## Verification status (Task 6 Steps 2-5 — pending USER)

- [ ] Android native (split ON): ACCESS GRANTED + real traffic; RU app bypasses.
- [ ] HAPP (phone + PC): nodes ping + load; sustained speed (xmux) on RU LTE.
- [ ] Windows native: traffic loads; RU apps route direct (Task 5); no ~30-min mux drop.
- [ ] No false-connected: broken panel sync → `NODE UNAVAILABLE` (native) / node omitted (HAPP).
