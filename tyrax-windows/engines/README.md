# engines/

Native binaries the TYRAX Windows client bundles and ships next to `TyraxService`.
These are **not committed to git** (large, redistributable, versioned upstream) —
run the fetch script once after cloning:

```powershell
pwsh -File .\fetch-engines.ps1          # latest release
pwsh -File .\fetch-engines.ps1 -XrayVersion v26.3.27   # pinned
```

| File | Source | Role |
|---|---|---|
| `xray.exe` | XTLS/Xray-core | TUN inbound (WinTun) + VLESS/Reality/XHTTP outbound |
| `wintun.dll` | wintun.net (WireGuard) | userspace TUN driver, loaded by xray |
| `geoip.dat` | Xray-core zip | GEOIP routing db |
| `geosite.dat` | Xray-core zip | GEOSITE routing db |

The installer copies these into the service install dir; during development the
service resolves them from this folder (see `EnginePaths`).
