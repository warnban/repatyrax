# RU Auto Split-Tunnel + Android Update Banner — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** With the TYRAX protocol ON, Russian services (VK, Ozon, WB, Sber, MAX, Gosuslugi, banks, …) transparently bypass the tunnel and exit over the phone's real RU IP — proactively (curated list + `geoip:ru` + per-app) and reactively (self-healing detection). Plus an in-app Android update banner that self-installs a newer APK.

**Architecture:** VLESS/Xray only. Because `TyraxXrayVpnService` excludes its own process from the TUN, Xray's `direct`(freedom) outbound exits over the physical network (real RU IP); routing RU destinations to `direct` is a genuine bypass. Split is applied at three layers: Xray domain routing, `geoip:ru` (bundled `.dat`), and per-app `addDisallowedApplication`. A `SplitDiagnostics` loop probes RU markers through proxy vs direct and auto-adds blocked ones to a dynamic bypass set. The update banner mirrors the existing Windows `DownloadHandler` pattern.

**Tech Stack:** Kotlin/Android (Jetpack Compose, Hilt, DataStore, OkHttp/Retrofit, libv2ray/Xray-core, hev-socks5-tunnel), Go/Fiber backend.

## Global Constraints

- Engine: VLESS/Xray only. WireGuard is out of scope (blocked in RU).
- Do NOT swap `app/libs/libv2ray.aar` (working Xray core v26.x build).
- The `proxy` catch-all routing rule (`network: tcp,udp`) MUST remain LAST so bypass rules take precedence.
- Xray `direct`/`freedom` outbound tag is `"direct"` (already built by backend + `XrayConfigBuilder`).
- Brand: UPPERCASE, cold tone, TYRAX palette (`#000000`/`#FFFFFF`/`#FF1E1E`), no Material3 defaults, no soft/apologetic copy. All user-facing strings in `strings.xml`.
- Split-tunnel default = ON; toggle lives in Settings/CONTROL.
- Update banner: self-hosted APK, install via `REQUEST_INSTALL_PACKAGES`; "ПОЗЖЕ" hidden only when `mandatory=true`.
- Windows shell is PowerShell: no `&&`/heredoc; commit via `git commit -F <file>`.

---

### Task 1: Backend — Android update manifest + expanded split-domains seed

**Files:**
- Modify: `tyrax-backend/internal/config/config.go` (add Android update config fields)
- Modify: `tyrax-backend/internal/handler/download_handler.go` (add `AndroidLatest`)
- Modify: `tyrax-backend/cmd/server/main.go` (route + APK static serve)
- Modify: `tyrax-backend/internal/service/vpn_service.go` (`GetSplitDomains` — add MAX + `.com` mirrors)
- Test: `tyrax-backend/internal/handler/download_handler_test.go`

**Interfaces:**
- Produces: `GET /download/android/latest.json` → `{"version":string,"version_code":int,"url":string,"mandatory":bool,"notes":string}`
- Produces: `GET /download/android/TYRAX.apk` (static file)

- [ ] **Step 1:** In `config.go` add fields to the config struct and loader: `AndroidAppVersion string`, `AndroidAppVersionCode int`, `AndroidAppURL string`, `AndroidUpdateMandatory bool`, `AndroidUpdateNotes string`. Defaults:

```go
AndroidAppVersion:      getEnv("ANDROID_APP_VERSION", "1.0.1"),
AndroidAppVersionCode:  getEnvInt("ANDROID_APP_VERSION_CODE", 2),
AndroidAppURL:          getEnv("ANDROID_APP_URL", getEnv("WEBSITE_URL", "https://tyrax.tech")+"/download/android/TYRAX.apk"),
AndroidUpdateMandatory: getEnvBool("ANDROID_UPDATE_MANDATORY", false),
AndroidUpdateNotes:     getEnv("ANDROID_UPDATE_NOTES", ""),
```
(If `getEnvBool` doesn't exist, add it mirroring `getEnvInt`.)

- [ ] **Step 2:** In `download_handler.go` extend the struct + constructor to carry the Android fields, and add:

```go
func (h *DownloadHandler) AndroidLatest(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version":      h.androidAppVersion,
		"version_code": h.androidAppVersionCode,
		"url":          h.androidAppURL,
		"mandatory":    h.androidUpdateMandatory,
		"notes":        h.androidUpdateNotes,
	})
}
```
Update `NewDownloadHandler` signature and its call site in `main.go`.

- [ ] **Step 3:** In `main.go`, next to the Windows manifest route add:

```go
app.Get("/download/android/latest.json", dlH.AndroidLatest)
app.Static("/download/android", "./download/android")
```
Create dir `tyrax-backend/download/android/` with a `.gitkeep` (APK dropped there at release time).

- [ ] **Step 4:** In `vpn_service.go` `GetSplitDomains`, append MAX + `.com` mirrors to the returned slice: `"max.ru", "oneme.ru", "sberbank.com", "sber.ru", "gosuslugi.ru", "tbank.ru", "ozon.com", "wildberries.com"` (dedupe; keep existing entries).

- [ ] **Step 5:** Write `download_handler_test.go`: build a `DownloadHandler` with known Android values, invoke `AndroidLatest` via a Fiber test app, assert status 200 and JSON keys `version`, `version_code`, `url`, `mandatory`.

- [ ] **Step 6:** Run `go test ./internal/handler/... ./internal/service/...` (from `tyrax-backend`). Expected: PASS.

- [ ] **Step 7:** Commit: `git add tyrax-backend; git commit -F <msg>` — `feat(backend): android update manifest + expand RU split-domains seed`.

---

### Task 2: Android — expand SplitTunnel lists + SplitTunnelPrefs

**Files:**
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/SplitTunnel.kt`
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/local/SplitTunnelPrefs.kt`
- Test: `tyrax-android/app/src/test/java/com/tyrax/data/vpn/SplitTunnelTest.kt`

**Interfaces:**
- Produces: `SplitTunnel.RU_SPLIT_DOMAINS: List<String>`, `SplitTunnel.RU_BYPASS_APPS: List<String>`
- Produces: `SplitTunnelPrefs` with `enabled: Flow<Boolean>` (default true), `setEnabled(Boolean)`, `dynamicBypassDomains: Flow<Set<String>>`, `addDynamicBypass(String)`.

- [ ] **Step 1:** Expand `RU_SPLIT_DOMAINS` — add: `"max.ru","oneme.ru","sberbank.com","sber.ru","tbank.ru","ozon.com","wildberries.com","aliexpress.ru","dzen.ru","gismeteo.ru","gosuslugi.ru","nalog.gov.ru","gostech.ru","mvideo.ru","dns-shop.ru","citilink.ru","megamarket.ru","yandex.net"`. Expand `RU_BYPASS_APPS` — add: `"ru.oneme.app","ru.vk.max","com.sberbankmobile","ru.sberbankmobile.push","ru.megafon.mlk","ru.beeline.services","com.aliexpress.ru","ru.mts.mtstv","ru.yandex.taxi","ru.yandex.market","ru.megamarket.mobile","ru.wildberries.wbservices"` (dedupe; safe to over-list — not-installed apps are skipped at apply time).

- [ ] **Step 2:** Create `SplitTunnelPrefs` (own DataStore instance `tyrax_split_prefs`, same pattern as `TokenDataStore`), Hilt `@Singleton @Inject constructor(@ApplicationContext context)`. Keys: `booleanPreferencesKey("split_enabled")`, `stringSetPreferencesKey("dynamic_bypass")`. `enabled` defaults `true` when unset.

- [ ] **Step 3:** Write `SplitTunnelTest`: assert `RU_SPLIT_DOMAINS` contains `"max.ru"`, `"sberbank.com"`, `"ozon.ru"`; assert no duplicates in either list.

- [ ] **Step 4:** Run `./gradlew.bat :app:testDebugUnitTest --tests "*SplitTunnelTest"`. Expected: PASS.

- [ ] **Step 5:** Commit — `feat(android): expand RU split lists + SplitTunnelPrefs`.

---

### Task 3: Android — bundle geoip/geosite + GeoAssets + initCoreEnv

**Files:**
- Create: `tyrax-android/app/src/main/assets/geoip.dat`, `.../assets/geosite.dat` (downloaded release data)
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/GeoAssets.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/TyraxXrayVpnService.kt:90` (`initCoreEnv`)

**Interfaces:**
- Produces: `GeoAssets.ensure(context): String` — copies `geoip.dat`/`geosite.dat` from assets to `filesDir/geo` if missing or stale, returns that dir's absolute path (or `filesDir` path if copy fails).

- [ ] **Step 1:** Download the standard v2fly/Loyalsoldier `geoip.dat` and `geosite.dat` into the assets dir (full versions; ~5–6 MB total accepted). Record source in a comment header of `GeoAssets.kt`.

- [ ] **Step 2:** Implement `GeoAssets.ensure(context)`: version-stamp using a marker file (e.g. `geo/.v` containing `BuildConfig.VERSION_CODE`); copy both `.dat` from `assets` to `filesDir/geo` when marker missing/mismatched. Wrap in `runCatching`; on failure log and return `context.filesDir.absolutePath` (routing falls back to non-geoip layers).

- [ ] **Step 3:** In `TyraxXrayVpnService.startTunnel`, replace `Libv2ray.initCoreEnv(filesDir.absolutePath, "")` with `Libv2ray.initCoreEnv(filesDir.absolutePath, GeoAssets.ensure(this))`.

- [ ] **Step 4:** Build `./gradlew.bat :app:assembleDebug` to confirm assets pack + compile. Expected: BUILD SUCCESSFUL.

- [ ] **Step 5:** Commit — `feat(android): bundle geoip/geosite and point xray asset env`.

---

### Task 4: Android — XrayConfigPatcher split routing + tests

**Files:**
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/XrayConfigPatcher.kt`
- Test: `tyrax-android/app/src/test/java/com/tyrax/data/vpn/XrayConfigPatcherTest.kt`

**Interfaces:**
- Produces: `data class SplitConfig(val enabled: Boolean, val bypassDomains: List<String>)`
- Produces: `XrayConfigPatcher.enhance(rawConfigJson: String, logDir: String? = null, split: SplitConfig = SplitConfig(false, emptyList())): String`

- [ ] **Step 1:** Add `SplitConfig` (top-level in the file). Change `enhance` signature to accept `split`. Change `buildRoutingRules()` → `buildRoutingRules(split: SplitConfig)`.

- [ ] **Step 2:** In `buildRoutingRules`, after the private-CIDR `direct` rule and BEFORE the `proxy` catch-all, when `split.enabled`:
  - domain rule: `{ "type":"field", "outboundTag":"direct", "domain": [ "domain:<d>" for d in split.bypassDomains ] }` (only if list non-empty)
  - geoip rule: `{ "type":"field", "outboundTag":"direct", "ip": ["geoip:ru"] }`
  Set `domainStrategy` to `"IPIfNonMatch"` when `split.enabled` (else keep `"AsIs"`).

- [ ] **Step 3:** Write `XrayConfigPatcherTest` using a minimal raw config (one `vless` outbound tagged `proxy`, one `freedom` tagged `direct`). Cases:
  - enabled+domains: routing rules JSON contains a rule with `outboundTag=direct` whose `domain` array includes `"domain:ozon.ru"`, and a rule with `ip` containing `"geoip:ru"`; the `proxy` rule index > both bypass rule indices.
  - disabled: no `geoip:ru` rule, no bypass domain rule.

- [ ] **Step 4:** Run `./gradlew.bat :app:testDebugUnitTest --tests "*XrayConfigPatcherTest"`. Expected: PASS.

- [ ] **Step 5:** Commit — `feat(android): xray routing bypass for RU domains + geoip:ru`.

---

### Task 5: Android — plumb SplitConfig + bypass apps into the service

**Files:**
- Modify: `tyrax-android/app/src/main/java/com/tyrax/domain/usecase/ConnectToNodeUseCase.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/XrayManager.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/TyraxXrayVpnService.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/domain/repository/VpnRepository.kt` (expose `getSplitDomains` already present in impl — add to interface if missing)

**Interfaces:**
- Consumes: `SplitTunnelPrefs`, `VpnRepository.getSplitDomains(): Result<List<String>>`, `XrayConfigPatcher.SplitConfig`
- Produces: `XrayManager.startVpn(context, xrayConfigJson, codename, split: XrayConfigPatcher.SplitConfig, bypassApps: List<String>)`; new service intent extras `EXTRA_SPLIT_ENABLED`, `EXTRA_BYPASS_DOMAINS` (`ArrayList<String>`), `EXTRA_BYPASS_APPS` (`ArrayList<String>`).

- [ ] **Step 1:** Inject `SplitTunnelPrefs` into `ConnectToNodeUseCase`. Before starting: read `enabled = splitTunnelPrefs.enabled.first()`, `dynamic = splitTunnelPrefs.dynamicBypassDomains.first()`, `server = vpnRepository.getSplitDomains().getOrDefault(SplitTunnel.RU_SPLIT_DOMAINS)`; `bypassDomains = (server + SplitTunnel.RU_SPLIT_DOMAINS + dynamic).distinct()`. Pass `SplitConfig(enabled, bypassDomains)` and `if (enabled) SplitTunnel.RU_BYPASS_APPS else emptyList()` into `XrayManager.startVpn`.

- [ ] **Step 2:** `XrayManager.startVpn` gains `split` + `bypassApps` params; retain them alongside `pendingConfig` (so `retryAfterPermission` re-passes them). `startService` puts extras on the intent: `EXTRA_SPLIT_ENABLED` (boolean), `EXTRA_BYPASS_DOMAINS` (`ArrayList(split.bypassDomains)`), `EXTRA_BYPASS_APPS` (`ArrayList(bypassApps)`).

- [ ] **Step 3:** `TyraxXrayVpnService.onStartCommand` reads the extras into fields. `startTunnel` calls `XrayConfigPatcher.enhance(configJson, logDir, SplitConfig(splitEnabled, bypassDomains))`. `establishTun()` — when `splitEnabled`, iterate `bypassApps` and `runCatching { builder.addDisallowedApplication(pkg) }` (log each; keep self-exclusion).

- [ ] **Step 4:** Add `getSplitDomains(): Result<List<String>>` to `VpnRepository` interface if absent (impl already has it).

- [ ] **Step 5:** Build `./gradlew.bat :app:assembleDebug`. Expected: BUILD SUCCESSFUL.

- [ ] **Step 6:** Commit — `feat(android): apply split-tunnel config + per-app bypass in xray service`.

---

### Task 6: Android — self-healing SplitDiagnostics + SplitStatusBus

**Files:**
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/SplitStatusBus.kt`
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/SplitDiagnostics.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/vpn/TyraxXrayVpnService.kt` (start/stop the diagnostics loop; inject prefs)
- Modify: `tyrax-android/app/src/main/java/com/tyrax/di/VpnModule.kt` (if `SplitTunnelPrefs` needs binding for service access)

**Interfaces:**
- Produces: `SplitStatusBus.status: MutableStateFlow<SplitStatus>` where `data class SplitStatus(val bypassCount: Int, val lastCheckedAt: Long, val lastAutoAdded: String?)`.
- Produces: `SplitDiagnostics.probeOnce(socksPort, markers, alreadyBypassed): List<String>` (returns domains newly detected as blocked-through-VPN) and a `run(scope, socksPort, prefs)` loop.

- [ ] **Step 1:** `SplitStatusBus` — object with `MutableStateFlow<SplitStatus>` + `reset()` (mirror `TunnelStatsBus`).
- [ ] **Step 2:** `SplitDiagnostics`: markers = `listOf("vk.com","ozon.ru","sberbank.ru","max.ru","wildberries.ru")`. For each marker not already bypassed: `directOk = httpReachable(marker, proxy=null)`; `proxyOk = httpReachable(marker, proxy=SOCKS 127.0.0.1:socksPort)` using a short-timeout `OkHttpClient` (HEAD `https://<marker>`, treat 2xx/3xx/redirect as ok). Blocked-through-VPN = `directOk && !proxyOk`. Collect those.
- [ ] **Step 3:** In the service, after tunnel UP, launch a loop (`DIAG_INTERVAL_MS = 300_000`, first run after ~10s) only when `splitEnabled`: call `probeOnce`; for each newly blocked marker → `splitTunnelPrefs.addDynamicBypass(domain)` and publish `SplitStatusBus`. Also publish bypass count on each connect. Cancel loop in `stopTunnel`/`onDestroy` and `SplitStatusBus.reset()`.
- [ ] **Step 4:** Ensure `SplitTunnelPrefs` is obtainable in the service — since `VpnService` isn't Hilt-injected here, use `EntryPointAccessors` or a simple app-level singleton accessor; document the chosen approach in the file.
- [ ] **Step 5:** Build `./gradlew.bat :app:assembleDebug`. Expected: BUILD SUCCESSFUL.
- [ ] **Step 6:** Commit — `feat(android): self-healing split diagnostics + status bus`.

---

### Task 7: Android — Settings toggle + split status UI

**Files:**
- Modify: `tyrax-android/app/src/main/java/com/tyrax/presentation/screens/settings/SettingsViewModel.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/presentation/screens/settings/SettingsScreen.kt`
- Modify: `tyrax-android/app/src/main/res/values/strings.xml`

**Interfaces:**
- Consumes: `SplitTunnelPrefs`, `SplitStatusBus.status`
- Produces: `SettingsUiState.splitEnabled: Boolean`, `SettingsUiState.splitStatus: String?`; `SettingsViewModel.setSplitEnabled(Boolean)`.

- [ ] **Step 1:** Inject `SplitTunnelPrefs` into `SettingsViewModel`; collect `enabled` and `SplitStatusBus.status` into UI state; `setSplitEnabled` writes prefs.
- [ ] **Step 2:** Add strings (RU): `settings_split_title` = "RU СПЛИТ-ТУННЕЛЬ", `settings_split_desc` = "РФ-СЕРВИСЫ ИДУТ НАПРЯМУЮ, ОСТАЛЬНОЕ — ЧЕРЕЗ TYRAX", `settings_split_status` = "RU-BYPASS: %1$d СЕРВИСОВ · АВТО", `settings_split_apply_hint` = "ИЗМЕНЕНИЯ ПРИМЕНЯТСЯ ПРИ СЛЕДУЮЩЕМ ПОДКЛЮЧЕНИИ".
- [ ] **Step 3:** In `SettingsScreen` add a brand-styled toggle row (custom, matching palette; sharp corners; red accent when ON) + status line. No Material default `Switch` visual.
- [ ] **Step 4:** Build `./gradlew.bat :app:assembleDebug`. Expected: BUILD SUCCESSFUL.
- [ ] **Step 5:** Commit — `feat(android): CONTROL split-tunnel toggle + status`.

---

### Task 8: Android — update check (BuildConfig, DTO/API, use case, prefs) + tests

**Files:**
- Modify: `tyrax-android/app/build.gradle.kts` (enable `buildConfig`, ensure `VERSION_CODE`/`VERSION_NAME` available)
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/remote/Dtos.kt` (add `AndroidUpdateDto`)
- Modify: `tyrax-android/app/src/main/java/com/tyrax/data/remote/TyraxApiService.kt` (add `getAndroidLatest` via `@GET @Url`)
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/local/UpdatePrefs.kt`
- Create: `tyrax-android/app/src/main/java/com/tyrax/domain/model/UpdateInfo.kt`
- Create: `tyrax-android/app/src/main/java/com/tyrax/domain/usecase/CheckUpdateUseCase.kt`
- Test: `tyrax-android/app/src/test/java/com/tyrax/domain/usecase/CheckUpdateUseCaseTest.kt`

**Interfaces:**
- Produces: `AndroidUpdateDto(version, versionCode, url, mandatory, notes)`; `data class UpdateInfo(version, versionCode, url, mandatory, notes)`; `CheckUpdateUseCase(): UpdateInfo?` (null when up-to-date or dismissed non-mandatory).
- Consumes: `BuildConfig.VERSION_CODE`, `UpdatePrefs.dismissedVersionCode`.

- [ ] **Step 1:** In `build.gradle.kts` `android { buildFeatures { buildConfig = true } }` (keep compose). `VERSION_CODE`/`VERSION_NAME` are emitted into `BuildConfig` by default when buildConfig is on.
- [ ] **Step 2:** Add `AndroidUpdateDto` to `Dtos.kt` (`@SerializedName("version_code") versionCode: Int`, etc.). Add to `TyraxApiService`: `@GET suspend fun getAndroidLatest(@Url url: String): AndroidUpdateDto` (full URL `https://api.tyrax.tech/download/android/latest.json` — note: NOT under `/api/v1/`). Provide the URL constant in the use case.
- [ ] **Step 3:** `UpdatePrefs` (DataStore `tyrax_update_prefs`): `dismissedVersionCode: Flow<Int>` (default 0), `setDismissed(Int)`.
- [ ] **Step 4:** `CheckUpdateUseCase(@Inject api, updatePrefs)`: fetch dto (wrap in runCatching → null on error); if `dto.versionCode <= BuildConfig.VERSION_CODE` → null; if `!dto.mandatory && dto.versionCode <= dismissed` → null; else `UpdateInfo(...)`.
- [ ] **Step 5:** Write `CheckUpdateUseCaseTest` with a fake api + in-memory prefs: (a) newer version → returns info; (b) same/older → null; (c) newer but dismissed & not mandatory → null; (d) newer, dismissed, mandatory → returns info.
- [ ] **Step 6:** Run `./gradlew.bat :app:testDebugUnitTest --tests "*CheckUpdateUseCaseTest"`. Expected: PASS.
- [ ] **Step 7:** Commit — `feat(android): update-check use case + version manifest client`.

---

### Task 9: Android — update banner UI + APK installer + manifest

**Files:**
- Create: `tyrax-android/app/src/main/java/com/tyrax/data/update/ApkInstaller.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/presentation/screens/main/MainViewModel.kt`
- Modify: `tyrax-android/app/src/main/java/com/tyrax/presentation/screens/main/MainScreen.kt`
- Modify: `tyrax-android/app/src/main/AndroidManifest.xml` (permission + FileProvider)
- Create: `tyrax-android/app/src/main/res/xml/file_paths.xml`
- Modify: `tyrax-android/app/src/main/res/values/strings.xml`

**Interfaces:**
- Consumes: `CheckUpdateUseCase`, `UpdateInfo`, `UpdatePrefs`.
- Produces: `ApkInstaller.downloadAndInstall(context, url, onProgress, onError)`; `MainUiState.updateInfo: UpdateInfo?`; `MainViewModel.onUpdateNow()`, `onUpdateLater()`.

- [ ] **Step 1:** `ApkInstaller`: OkHttp GET stream → `cacheDir/tyrax-update.apk`; on `Build.VERSION.SDK_INT >= O` check `packageManager.canRequestPackageInstalls()`, if false launch `Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES` for our package; then `FileProvider.getUriForFile(context, "${packageName}.fileprovider", file)` + `Intent(ACTION_VIEW)` type `application/vnd.android.package-archive` with `FLAG_ACTIVITY_NEW_TASK or FLAG_GRANT_READ_URI_PERMISSION`. TYRAX-cold error copy on failure.
- [ ] **Step 2:** `MainViewModel`: inject `CheckUpdateUseCase` + `UpdatePrefs`; on init call check → set `updateInfo`. `onUpdateLater()` writes `UpdatePrefs.setDismissed(updateInfo.versionCode)` and clears banner. `onUpdateNow()` triggers `ApkInstaller` (via context passed from UI or an event channel).
- [ ] **Step 3:** `MainScreen`: top banner (above status Column) when `updateInfo != null`: `ДОСТУПНА НОВАЯ ВЕРСИЯ` + `notes` + `[ОБНОВИТЬ]`; `[ПОЗЖЕ]` only when `!mandatory`. Brand-styled (black bg, red border, uppercase). Wire `onUpdateNow`/`onUpdateLater`.
- [ ] **Step 4:** Manifest: add `<uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES"/>`; inside `<application>` add FileProvider:

```xml
<provider
    android:name="androidx.core.content.FileProvider"
    android:authorities="${applicationId}.fileprovider"
    android:exported="false"
    android:grantUriPermissions="true">
    <meta-data
        android:name="android.support.FILE_PROVIDER_PATHS"
        android:resource="@xml/file_paths" />
</provider>
```
`file_paths.xml`: `<cache-path name="updates" path="." />`.

- [ ] **Step 5:** Add strings (RU): `update_banner_title` = "ДОСТУПНА НОВАЯ ВЕРСИЯ", `update_btn_now` = "ОБНОВИТЬ", `update_btn_later` = "ПОЗЖЕ", `update_downloading` = "ЗАГРУЗКА…", `update_error` = "ОБНОВЛЕНИЕ НЕ УДАЛОСЬ. ПОВТОРИТЕ.".
- [ ] **Step 6:** Build `./gradlew.bat :app:assembleDebug`. Expected: BUILD SUCCESSFUL.
- [ ] **Step 7:** Commit — `feat(android): in-app update banner + apk self-installer`.

---

### Task 10: Verification (device) + release build

- [ ] **Step 1:** `./gradlew.bat :app:assembleRelease` — BUILD SUCCESSFUL, R8 keep rules intact (libv2ray/TProxy retained).
- [ ] **Step 2:** Device: connect protocol ON → open VK / Ozon / WB / Sber / MAX → all load (RU IP). Confirm a foreign site (youtube/google) still routes via node (foreign IP via ipinfo).
- [ ] **Step 3:** Device: toggle split OFF in CONTROL → reconnect → RU services now geo-blocked again (confirms toggle). Toggle back ON.
- [ ] **Step 4:** Publish a higher `ANDROID_APP_VERSION_CODE` on backend → open app → banner appears → ОБНОВИТЬ downloads + launches installer.
- [ ] **Step 5:** Final commit if any fixes — `chore: verified RU split-tunnel + update banner on device`.

---

## Self-Review

- **Spec coverage:** §A→T2, §B→T4, §C→T3, §D→T5, §E→T6, §F→T7, §G→T1(backend)+T8+T9, testing→T1/T4/T8+T10. Covered.
- **Placeholder scan:** No TBD/TODO; each step has concrete action/paths/code or command. MAX package handled via over-listing candidates (skip-if-not-installed) — not a placeholder.
- **Type consistency:** `SplitConfig(enabled, bypassDomains)` used identically in T4/T5; `SplitStatus(bypassCount,lastCheckedAt,lastAutoAdded)` in T6/T7; `UpdateInfo(version,versionCode,url,mandatory,notes)` in T8/T9; intent extras `EXTRA_SPLIT_ENABLED/EXTRA_BYPASS_DOMAINS/EXTRA_BYPASS_APPS` in T5/T6. Consistent.
