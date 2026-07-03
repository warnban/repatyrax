# TYRAX — RU Auto Split-Tunnel + Android Update Banner (Design)

> Read `TYRAX_CONTEXT.md` and `.cursorrules` before touching code. Brand copy: uppercase, cold, RU/EN, no soft language, TYRAX palette only.

**Status:** Design approved (2026-07-03). Next step: writing-plans.

## Goal

Two independent features on the existing TYRAX stack:

1. **RU auto split-tunnel.** With the TYRAX protocol ON (foreign exit node), Russian services that geo-block foreign IPs (VK, Ozon, Wildberries, Sber, Tinkoff, VTB, Alfa, Gosuslugi, MAX messenger, Kinopoisk, etc.) must transparently reach the internet **as if from Russia** — i.e. their traffic bypasses the tunnel and exits over the phone's real RU network. Fully automatic, plus self-healing diagnosis that detects a RU service that is blocked-through-VPN and auto-adds it to the bypass set.
2. **Android in-app update banner.** When a newer signed APK is published, the running app detects it, shows a TYRAX-styled banner, downloads the APK, and launches the system installer.

## Scope decisions (confirmed with user)

- Split mechanism: **hybrid** — Xray domain routing + per-app exclusion + bundled full `geoip.dat`/`geosite.dat` (`geoip:ru`). Full `.dat` files accepted (~5–6 MB APK growth).
- UX: split-tunnel **always-on by default**, with a CONTROL/Settings toggle to disable.
- Diagnosis: **self-healing** (probe RU markers through proxy vs direct; auto-add blocked ones to bypass; show status in CONTROL).
- Update delivery: **self-hosted APK** on the TYRAX backend, banner downloads it and launches the system installer via `REQUEST_INSTALL_PACKAGES`.
- Engine scope: **VLESS/Xray only.** WireGuard is blocked in RU and is out of scope for split-tunnel work here.

## Why this works (foundation)

`TyraxXrayVpnService.establishTun()` already calls `addDisallowedApplication(packageName)`, so the TYRAX process is off the TUN. Xray's `direct` (freedom) outbound therefore dials over the **physical network (real RU IP)**, not through the tunnel. So routing a destination to `direct` in Xray is a genuine tunnel bypass with a Russian source IP. Domain-based split IS possible at the Xray routing layer (the old `SplitTunnel.kt` note only applies to `VpnService.Builder` route exclusion, not Xray routing). No new native code required.

## Current-state gaps (verified)

- `SplitTunnel.RU_SPLIT_DOMAINS` / `RU_BYPASS_APPS` exist but are **never applied**: `establishTun()` only excludes self; `XrayConfigPatcher.buildRoutingRules()` sends all `tcp,udp` → `proxy`.
- `VpnRepositoryImpl.getSplitDomains()` fetches the server list but the result never reaches the VpnService.
- `Libv2ray.initCoreEnv(filesDir, "")` — no asset dir → `geoip.dat`/`geosite.dat` not available, so `geoip:ru` rules can't be used yet.
- MAX messenger is absent from both the domain and app lists.
- Backend already has the Windows update pattern (`DownloadHandler.WindowsLatest`, `/download/windows/latest.json`, `cfg.WindowsAppVersion`) — Android just needs the mirror. No Android update surface exists client-side.
- `versionCode` is not exposed in `BuildConfig` (only `versionName` via `BuildConfig` default); update comparison needs the numeric code.

## Architecture

### A. Split-tunnel data model (Android)

- Extend `SplitTunnel.kt`:
  - Add MAX messenger to `RU_SPLIT_DOMAINS` (`max.ru`, `oneme.ru`) and `RU_BYPASS_APPS` (`ru.oneme.app` / `ru.vk.max` — verify actual package at implementation time; include known candidates, skip-if-not-installed is safe).
  - Keep the curated fallback list in lockstep with the backend seed list.
- New `SplitTunnelPrefs` (DataStore, in `data/local`): `splitTunnelEnabled: Boolean = true`, `dynamicBypassDomains: Set<String>` (self-healing additions).

### B. Xray routing (the domain + geoip layer)

`XrayConfigPatcher.enhance(rawConfigJson, logDir, splitConfig)` gains a `splitConfig` param carrying:
- `enabled: Boolean`
- `bypassDomains: List<String>` (server list ∪ local fallback ∪ dynamic self-heal additions)

`buildRoutingRules(splitConfig)` produces, in order:
1. `dns-out` for port 53 (unchanged).
2. `direct` for private CIDRs (unchanged).
3. **NEW** (only when `enabled`): `{ type: field, outboundTag: "direct", domain: [ "domain:ozon.ru", ... ] }` — use `domain:` prefix so subdomains match.
4. **NEW** (only when `enabled`): `{ type: field, outboundTag: "direct", ip: [ "geoip:ru" ] }`.
5. `proxy` for `tcp,udp` (catch-all, unchanged, stays last).

`domainStrategy` becomes `IPIfNonMatch` (needed so `geoip:ru` can resolve+match), keeping `AsIs` behavior otherwise acceptable — implementation verifies core accepts it with bundled data.

### C. geoip/geosite assets

- Ship full `geoip.dat` + `geosite.dat` in `app/src/main/assets/`.
- New `GeoAssets` helper: copy both from assets to `filesDir` on first run (idempotent, version-stamped), return the dir.
- `TyraxXrayVpnService.startTunnel()`: `Libv2ray.initCoreEnv(filesDir.absolutePath, geoAssetDir)` (second arg = asset path Xray reads `geoip.dat`/`geosite.dat` from).

### D. Per-app exclusion

- `TyraxXrayVpnService.establishTun()`: when split enabled, loop `RU_BYPASS_APPS` and `addDisallowedApplication(pkg)` guarded by try/catch (skip not-installed / already-excluded). Always keep the existing self-exclusion.
- The RU app list + `enabled` flag + bypass domains are passed to the service via new intent extras (`EXTRA_SPLIT_ENABLED`, `EXTRA_BYPASS_DOMAINS`, `EXTRA_BYPASS_APPS`) plumbed from `ConnectToNodeUseCase` → `XrayManager.startVpn(...)` → service. `ConnectToNodeUseCase` reads `SplitTunnelPrefs` + `vpnRepository.getSplitDomains()` before starting.

### E. Self-healing diagnosis

- New `SplitDiagnostics` (VPN layer). On tunnel UP and every `DIAG_INTERVAL` (e.g. 5 min):
  - For a small marker set (`vk.com`, `ozon.ru`, `sberbank.ru`, `max.ru`, `wildberries.ru`), issue a short HTTP(S) HEAD/GET **through the SOCKS proxy** (tunnel) and **direct**.
  - Classify "blocked-through-VPN" = proxy fails / times out / returns a block page while direct succeeds.
  - On such a marker not already bypassed: add its domain to `SplitTunnelPrefs.dynamicBypassDomains`, and trigger a lightweight reconnect/reload so the new routing takes effect (reuse existing connect path; acceptable to apply on next connect + immediately for domain layer if feasible).
  - Publish a `SplitStatus(bypassCount, lastCheckedAt, autoAdded)` to a `SplitStatusBus` (mirror `TunnelStatsBus`).
- Diagnosis is read-only w.r.t. the tunnel except for the DataStore write + routing refresh. Disabled when split-tunnel toggle is OFF.

### F. UI — CONTROL/Settings

- `SettingsScreen` + `SettingsViewModel`: `RU СПЛИТ-ТУННЕЛЬ` toggle (default ON), bound to `SplitTunnelPrefs`. Toggling while connected schedules apply on next connect (and shows a hint). Brand-styled row (no Material default switch look — custom or restyled).
- Status line under the toggle: `RU-BYPASS: <n> СЕРВИСОВ · АВТО` + last check age, fed by `SplitStatusBus`.
- All strings in `strings.xml`, UPPERCASE, cold tone.

### G. Android update banner

**Backend (Go):**
- `config.go`: add `AndroidAppVersion` (semver string), `AndroidAppVersionCode` (int), `AndroidAppURL` (defaults to `WebsiteURL + "/download/android/TYRAX.apk"`), env-overridable.
- `DownloadHandler`: add `AndroidLatest(c)` → `{ "version", "version_code", "url", "mandatory", "notes" }`. Add `mandatory` via config (`AndroidUpdateMandatory` bool, default false).
- `main.go`: route `app.Get("/download/android/latest.json", dlH.AndroidLatest)`; serve the APK file (static file route or `app.Static`).
- Unit test for `AndroidLatest` JSON shape.

**Android:**
- `build.gradle.kts`: enable `buildConfig`, expose `versionCode`/`versionName` in `BuildConfig` (add `buildConfigField` if needed).
- DTO `AndroidUpdateDto` (`version`, `versionCode`, `url`, `mandatory`, `notes`), API method `getAndroidLatest()` (public, no auth — separate call; base URL differs from `/api/v1/`, so use full URL via `@Url` or a dedicated endpoint).
- `CheckUpdateUseCase`: fetch manifest, compare `versionCode` > `BuildConfig.VERSION_CODE`; return `UpdateInfo?`. Respect a "dismissed version_code" stored in DataStore (`UpdatePrefs`) unless `mandatory`.
- `MainViewModel`: expose `updateInfo` in UI state; check on screen entry.
- `MainScreen`: top banner above status zone — `ДОСТУПНА НОВАЯ ВЕРСИЯ` + optional notes + `[ОБНОВИТЬ]` / `[ПОЗЖЕ]`. `mandatory` hides ПОЗЖЕ.
- `ApkInstaller`: OkHttp download to `cacheDir` with progress, then `FileProvider.getUriForFile` + `Intent(ACTION_VIEW, application/vnd.android.package-archive)` with `FLAG_GRANT_READ_URI_PERMISSION`. Handle `canRequestPackageInstalls()` → route to settings if needed.
- Manifest: `<uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES"/>`, `<provider>` FileProvider (`${applicationId}.fileprovider`), `res/xml/file_paths.xml`.

## Data flow (split-tunnel)

```
Settings toggle ─┐
                 ├─> SplitTunnelPrefs (DataStore)
Self-heal add ───┘            │
                              v
ConnectToNodeUseCase: reads prefs + getSplitDomains() ──> merged bypass set
   │                                                          │
   ├─ XrayManager.startVpn(config, codename, splitConfig) ────┤
   v                                                          v
TyraxXrayVpnService:                                   XrayConfigPatcher.enhance(..., splitConfig)
   - initCoreEnv(filesDir, geoAssetDir)                   - routing: domain[] -> direct
   - establishTun(): addDisallowedApplication(ru apps)    - routing: geoip:ru -> direct
   - SplitDiagnostics loop -> SplitStatusBus              - routing: tcp,udp  -> proxy (last)
```

## Error handling

- Missing/failed server split list → local `RU_SPLIT_DOMAINS` fallback (already the pattern).
- geoip asset copy failure → log, fall back to domain + per-app layers only (no `geoip:ru` rule); tunnel still works.
- Update manifest fetch failure → silent, no banner (never block the main flow).
- APK download/install failure → cold TYRAX-styled error, offer retry / open URL. Never crash.
- Diagnosis probe failures are non-fatal; never mutate the tunnel beyond adding a bypass domain.

## Testing

- Go: `AndroidLatest` returns expected JSON keys/values; split-domains seed contains MAX.
- Kotlin (unit): `XrayConfigPatcher` emits the `direct` domain rule + `geoip:ru` rule before the `proxy` catch-all, and omits them when disabled; `CheckUpdateUseCase` version-code comparison + dismissed-version logic; bypass-set merge (server ∪ local ∪ dynamic).
- Manual (device, in plan): connect, confirm VK/Ozon/Sber/MAX open; confirm foreign sites still tunnel; publish a higher `version_code` and confirm banner → install.

## Out of scope

- WireGuard split-tunnel (WG blocked in RU).
- iOS / desktop.
- Server-driven per-app list (apps stay client-side; only domains are server-driven, as today).
