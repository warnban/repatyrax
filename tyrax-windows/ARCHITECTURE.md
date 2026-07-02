# TYRAX — WINDOWS CLIENT ARCHITECTURE

> Read `TYRAX_CONTEXT.md` and `.cursorrules` first. This document is the source of truth
> for the Windows desktop client. It mirrors the Android client and reuses the existing
> Go backend (`api.tyrax.tech/api/v1`) with no server changes for the core flows.

---

## 1. GOAL

Ship a Windows desktop PROTOCOL client in the same brand and ecosystem as the Android app:
same accounts, same devices, same nodes, same backend, same VLESS/Reality/XHTTP engine.
Aggressive, minimal, terminal/glitch aesthetic. Silent, automatic, "just works".

---

## 2. LOCKED DECISIONS

| Decision | Choice |
|---|---|
| UI stack | WPF (.NET 8, C#) — full control over the black/red terminal aesthetic |
| Privilege model | Unprivileged WPF UI + privileged Windows Service, named-pipe IPC (WireGuard model) |
| Tunnel engine | Xray-core (`xray.exe`) SOCKS5 + `xjasonlyu/tun2socks` on WinTun (1:1 with Android) |
| Scope | Full parity with Android, delivered in phases |
| Installer | Inno Setup (service + WinTun + engines) |
| Auto-update | Velopack (delta updates, rollback, no Store) |
| Targets | Windows 10 (1809+) & 11, x64 |
| Code signing | dev: self-signed; release: OV/EV Authenticode (sign our exe/msi) |
| Split tunnel | Xray routing rules (RU domains → `direct`), NOT route-table based |
| Auto-connect on boot | ON by default (reconnect last node) |

---

## 3. HIGH-LEVEL ARCHITECTURE

```
User session (no admin)                 Windows Service (LocalSystem, auto-start)
┌───────────────────────┐   named pipe  ┌───────────────────────────────────────┐
│ Tyrax.App (WPF, tray)  │ <===========> │ TyraxService (tunnel orchestrator)     │
│  MVVM · screens        │  status/cmds  │   ├─ xray.exe  (SOCKS5 127.0.0.1:10808) │
│  status · ENTER        │               │   │            + VLESS/Reality/XHTTP    │
└─────────┬─────────────┘               │   ├─ tun2socks + WinTun (TUN → SOCKS)   │
          │ HTTPS + JWT                  │   └─ routes · DNS · kill-switch (WFP)   │
          v                              └───────────────────┬───────────────────┘
   TYRAX Backend  <──────────────────────── xray VLESS ──────┘
   api.tyrax.tech/api/v1                                      → NODE (VPS + 3x-ui)
```

Tunnel data flow (identical in spirit to Android):
1. Service writes `xray` config.json: local SOCKS5 inbound + VLESS/Reality/XHTTP outbound,
   built from the structured params the backend returns from `/vpn/connect` and `/vpn/device`.
2. `tun2socks` binds a WinTun adapter and forwards all captured traffic to `127.0.0.1:10808`.
3. Service sets the default route via the TUN adapter and a `/32` route to the node host via
   the real gateway (avoids the tunnel routing loop); configures DNS.
4. Disconnect tears routes down, stops processes, removes the adapter.

---

## 4. SOLUTION STRUCTURE (Clean Architecture — mirrors Android)

```
tyrax-windows/
├── Tyrax.Windows.sln
├── src/
│   ├── Tyrax.Core/      # domain: models, use cases, interfaces. No external deps
│   ├── Tyrax.Data/      # Refit API client, JWT interceptor, DPAPI token store, node cache
│   ├── Tyrax.Tunnel/    # C# port of XrayConfigBuilder + VlessConfig, config assembly
│   ├── Tyrax.Ipc/       # named-pipe contracts (Connect/Disconnect/Status commands, events)
│   ├── Tyrax.Service/   # Windows Service: xray/tun2socks lifecycle, routes, WFP, supervisor
│   └── Tyrax.App/       # WPF UI: TyraxTheme, screens, ViewModels, tray
├── engines/             # xray.exe, tun2socks.exe, wintun.dll, geoip.dat, geosite.dat
├── installer/           # Inno Setup script + Velopack config
└── assets/              # icons, fonts
```

Layer mapping Android → Windows:
`domain/` → `Tyrax.Core` · `data/remote`+`data/local` → `Tyrax.Data` ·
`data/vpn` → `Tyrax.Tunnel` + `Tyrax.Service` · `presentation/` → `Tyrax.App`.
Ports: `XrayConfigBuilder.kt`, `VlessConfig.kt`, `ConnectionSupervisor.kt`.

---

## 5. BACKEND REUSE (no changes for core)

| Feature | Endpoint | Windows note |
|---|---|---|
| Register / login | `POST /auth/register`, `/auth/login` | JWT stored via DPAPI |
| Telegram login | `GET /auth/telegram-init` → open `bot_url` → poll `/auth/telegram-status?token=` | same as Android |
| Profile | `GET /auth/profile` | |
| Devices | `POST /vpn/device` (name `WIN-<hostname>`), `GET /vpn/devices`, `DELETE /vpn/device/{id}` | Windows = 1 device vs tier limit |
| Nodes | `GET /nodes` (ping+load ordered) | client refines with own TCP-ping |
| Connect | `POST /vpn/connect {name, codename}` → structured VLESS params | |
| Split domains | `GET /vpn/split-domains` | RU domains → xray `direct` |
| Subscription / pay | `GET /subscription`, `POST /payment/create` → `payment_url`, poll `/payment/status/{id}` | browser + polling |
| Invites (DOMINION) | `GET/POST/DELETE /subscription/invite*` | |

---

## 6. WINDOWS-SPECIFIC SUBSYSTEMS

- **Split tunnel (Phase 6 ✅ — route-pinned, NOT xray-layer):** the RU list from
  `/vpn/split-domains` (fallback `SplitTunnelDefaults`) is resolved at connect and each IP is
  pinned as a `/32` via the physical gateway — the same trick that keeps the node reachable.
  Those IPs bypass the WinTun default routes, so RU services see the user's real IP and go
  direct. **Why not xray domain routing (as the original plan assumed):** with tun2socks the
  WinTun captures *all* traffic; if xray sent a RU domain to its `freedom`/`direct` outbound,
  that packet would hit the `0.0.0.0/1` TUN route again → back into tun2socks → back into xray:
  an infinite loop. xray's own outbound to arbitrary RU IPs isn't route-pinned (only the node is),
  so domain-direct is only loop-safe with per-process exclusion of `xray.exe` (WFP — deferred
  with the kill-switch). Route-pinning is the Windows equivalent of Android's per-app RU
  exclusion and works today. Trade-off: CDN IP churn can stale a route until the next connect;
  the curated bank/gov list (stable IPs) is well covered. Handoff re-pins the RU `/32`s via the
  new gateway; teardown removes them all.
- **Auto-reconnect / node selection:** port of `ConnectionSupervisor` — 30s loop, health check
  (latency/loss), silent switch to the best node.
- **Network handoff (Wi-Fi ↔ Ethernet ↔ LTE):** subscribe to `NetworkChange.NetworkAddressChanged`,
  re-add routes / rebind after the adapter changes.
- **Kill-switch (Phase 4):** WFP rules blocking non-tunnel traffic while the PROTOCOL is up.
- **Autostart / tray:** service auto-starts at boot; UI minimizes to tray; `ENTER`/`DISCONNECT`
  from the tray menu.
- **Secret storage:** JWT + device UUID encrypted with DPAPI (`ProtectedData`, CurrentUser).

---

## 7. UI (WPF) — BRAND RULES

- Palette strictly `#000000 / #FFFFFF / #FF1E1E`, `FontWeight` Black/Bold, UPPERCASE status,
  `CornerRadius=0`, borderless window, no Fluent chrome.
- Screens: Splash → Onboarding (3 slides) → Auth (login/register/Telegram) → Main
  (`STATUS: OUTSIDE SYSTEM` / `ACCESS GRANTED`, giant `ENTER`/`DISCONNECT`, node tag) →
  Nodes → Subscription → Control (settings) → Identity/Devices.
- Strings live in `.resx` (analog of `strings.xml`) using TYRAX vocabulary.
- Connection glitch animation: `BREACHING NETWORK… → NODE ACQUIRED → ACCESS GRANTED`.
- Custom glitch loader instead of the default progress control.

---

## 8. INSTALLER & AUTO-UPDATE (Phase 7 ✅)

- **Publish:** `build\publish.ps1` publishes UI + service **self-contained win-x64 into one
  folder** (they target the identical runtime, so shared framework files overlap byte-for-byte
  → one runtime copy, no .NET needed on the target) and stages `engines\` beside the service.
  `-FrameworkDependent` switches to a small runtime-dependent build.
- **Installer:** `installer\tyrax.iss` (Inno Setup 6) — machine-wide `%ProgramFiles%\TYRAX`,
  `sc create TyraxProtocol start= auto` + crash-recovery + start, Start Menu / desktop /
  autostart shortcuts. Uninstall kills `TYRAX.exe`/`xray.exe`/`tun2socks.exe`, then
  `sc stop`+`sc delete`, then removes files. See `installer\README.md`.
- **Auto-update — Inno, NOT Velopack (decision).** Velopack assumes a per-user, self-managing
  app dir and can't own a machine-wide privileged service, so it's the wrong model here.
  Instead the app is the installer's unit; `UpdateChecker` polls
  `https://api.tyrax.tech/download/windows/latest.json` (`{version,url}`), and when a newer
  build exists the Main screen shows a red **UPDATE … AVAILABLE** banner that opens the signed
  installer's download page. It never downloads/runs anything silently — the new installer does
  the safe stop-tunnel → replace → restart via its own `sc` teardown/create.
- **Signing:** dev builds are unsigned; release sets `TYRAX_PFX`(`_PASS`) so `publish.ps1`
  Authenticode-signs `TYRAX.exe`, `TyraxService.exe`, `xray.exe`, `tun2socks.exe`, then the
  installer is signed too. OV/EV strongly recommended. Only the signing step changes.
- **Versioning:** keep `Tyrax.App.csproj <Version>`, `tyrax.iss #define AppVersion` and the
  manifest `version` in sync (documented in `installer\README.md`).

---

## 9. DEVELOPMENT PHASES

| Phase | Content | Done when |
|---|---|---|
| 0. Scaffold ✅ | Solution, layers, DI, TyraxTheme, IPC contracts, engine bundling | Empty window + service stub talk over the pipe |
| 1. Tunnel core ✅ | Port `XrayConfigBuilder`/`VlessConfig`, run xray+tun2socks+WinTun, routes/DNS | Manual node connect with full TUN works |
| 2. Auth + API ✅ | Refit client, login/register, Telegram flow, DPAPI, device registration | Login → device created on backend |
| 3. Main + Nodes ✅ | Status screen, ENTER/DISCONNECT via IPC, node list, best-node auto-select | Full connect/disconnect from UI |
| 4. Reliability ✅ | ConnectionSupervisor (silent node failover), engine watchdog + health probe, network handoff, IPC auto-reconnect | Silent "Android-grade" operation (kill-switch deferred, see §11) |
| 5. Subscription/pay/invites ✅ | CONTROL screen (tiers + metering), UNLOCK (months × method quote → `/payment/create`, open URL, poll status→PAID), DOMINION invites | Monetization parity |
| 6. Split tunnel ✅ | RU `/vpn/split-domains` resolved + pinned as `/32` via physical gateway (loop-safe; xray-layer split needs WFP, see §6) | RU services go direct |
| 7. Installer/update ✅ | `publish.ps1` (self-contained) + Inno Setup (`tyrax.iss`) service register/start + uninstall cleanup + signing hook + in-app update banner (Velopack dropped, see §8) | Installer + update path |
| 8. Onboarding/polish ✅ | First-run 3-slide onboarding (secure-store flag), BREACHING glitch flicker, system tray (close-to-tray + OPEN/TOGGLE/EXIT), autostart via installer | Release candidate |

---

## 10. TESTING PHASE 1 (manual, requires elevation)

The tunnel mutates the routing table, DNS and creates a WinTun adapter, so the
service must run **elevated**. During development run it from an **Administrator**
terminal (in production the installer registers it as a LocalSystem service).

```powershell
# 1. Populate native engines once
pwsh -File tyrax-windows\engines\fetch-engines.ps1

# 2. Build
dotnet build tyrax-windows\Tyrax.Windows.sln -c Debug

# 3. Run the service in an ELEVATED console (it hosts the named pipe + tunnel)
dotnet run --project tyrax-windows\src\Tyrax.Service\Tyrax.Service.csproj -c Debug

# 4. Run the UI (normal console). ENTER sends a Connect command over the pipe.
dotnet run --project tyrax-windows\src\Tyrax.App\Tyrax.App.csproj -c Debug
```

Until Phase 2 wires the backend, `MainViewModel` sends placeholder VLESS params;
replace them (or POST `/vpn/connect` yourself) with a **valid** node UUID +
Reality key to see a real breach. Engine data flow:
`WinTun (TYRAX) → tun2socks → SOCKS5 127.0.0.1:10808 → xray VLESS → NODE`.

Notes:
- `NetworkConfigurator` pins the node `/32` via the physical gateway and installs
  `0.0.0.0/1` + `128.0.0.0/1` via the TUN — the physical default is never deleted,
  so a crash can't strand the machine offline (reboot / adapter removal restores it).
- Live tx/rx stats are read straight off the WinTun adapter's byte counters
  (`NetworkStats`) and pushed once a second in `IpcStatus` — more robust than
  scraping the tun2socks REST API and counts exactly what crosses the tunnel.

## 11. OPEN ITEMS

1. tun2socks engine: default `xjasonlyu/tun2socks` v2.6.0 (WinTun, standalone exe). Alternative: `sing-box tun`.
2. **Kill-switch — deferred, needs WFP.** A correct leak-proof kill-switch must permit
   traffic by the TUN adapter's interface (WFP `FWPM_LAYER_ALE_AUTH_CONNECT` + node-IP
   permit + block-rest). A `netsh advfirewall` version can't express per-adapter permits
   and would block the very traffic tun2socks routes into the TUN, so it was intentionally
   left out rather than shipped as a dangerous/incorrect stub. To be implemented as a
   dedicated WFP subsystem with startup-cleanup of stale filters. Until then, leak exposure
   is limited to the brief window during connect/failover (routes, not a firewall, carry all
   traffic once up).
3. Auto-connect on Windows startup: ON by default.

## 12. RELIABILITY MODEL (Phase 4)

Mirrors the Android `ConnectionSupervisor` + `TunnelHealth`, split across the privilege boundary:

- **UI `ConnectionSupervisor`** (`Tyrax.App/Services`): loop-based node iteration. ENTER
  loads OPEN candidates (backend load-balanced order), breaches the first, then watches the
  IPC status stream. On `Error`/drop while the user still wants a connection it silently
  switches to the next candidate; after cycling all candidates twice it backs off 8 s and
  refreshes the list. A chosen node is moved to the front but the rest stay as fallback.
- **Service watchdog + health** (`TunnelSupervisor.MonitorLoopAsync`): while Connected it
  (a) watches `xray`/`tun2socks` liveness — a dead engine degrades immediately, and
  (b) probes `https://www.gstatic.com/generate_204` **through** the SOCKS inbound every 30 s;
  4 sustained failures/throttles (>9 s) end the tunnel in `Error`. The UI reacts by switching
  node. This keeps node selection (which needs the JWT/API) UI-side while liveness detection
  stays where the engines live.
- **Network handoff** (`NetworkChange.NetworkAddressChanged` → `NetworkConfigurator.RepinNodeAsync`):
  re-pins the node `/32` through the new physical gateway on Wi-Fi ↔ Ethernet ↔ LTE changes;
  the TUN default routes are untouched. If re-pin fails it degrades so the UI reconnects.
- **IPC auto-reconnect** (`ShellViewModel`): if the service pipe drops, the UI shows
  `SERVICE OFFLINE` and a single background loop redials every 2 s until the service answers.
