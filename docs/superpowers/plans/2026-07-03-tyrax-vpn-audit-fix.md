# TYRAX Full VPN Connectivity & Speed Audit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> Supersedes the partial `2026-07-03-vpn-regression-fix.md` (its Task 1 R8 keep-rules fix is already merged as `91cdc22`). This plan is spec-backed by `docs/superpowers/specs/2026-07-03-tyrax-vpn-audit-design.md`.

**Goal:** Restore real traffic on the native Android client (ACCESS GRANTED currently loads nothing, in both split-tunnel ON and OFF), stop any client from showing a false "connected" state, and audit Android / Windows / HAPP for config parity, correctness, and RU-mobile speed.

**Architecture:** The "connected/no-traffic" symptom means the node silently rejects the proxied stream (VLESS+Reality serves the decoy site for an unknown UUID). The recent commit `1e3a7c7` downgraded a genuine `panel.AddClient` failure from hard-fail to a swallowed `slog.Warn`, so the backend now hands out configs whose UUID the node never registered. Fix = (a) confirm root cause with on-device + backend + panel evidence, (b) make connect/HAPP fail honestly when the UUID is not confirmed on the inbound, (c) correct the real defect (panel inbound id / endpoint / node row), (d) close cross-client parity + speed gaps (HAPP has no xmux; Windows native TUN has no RU split-tunnel).

**Tech Stack:** Go/Fiber backend (`pkg/threexui`, `pkg/vpnconfig`, `internal/service`), 3x-ui panel HTTP API, PostgreSQL `nodes` table, Kotlin/Android (Xray via `libv2ray.aar` v26.x), C#/.NET Windows (`xray.exe` + WinTun), HAPP external client (iOS/macOS/Windows).

## Global Constraints

- **RU split-tunnel MUST stay working** on every client: RU-geoblocked apps (Wildberries, Ozon, banks, Gosuslugi, VK, Yandex, …) exit **directly over the real Russian network**, bypassing the foreign node, while the VPN is on.
- Do **not** swap `tyrax-android/app/libs/libv2ray.aar` (working Xray core v26.x).
- **Version lock:** client Xray core major == node Xray core major.
- **Reality `sid` (shortId) MUST be non-empty**; empty sid → decoy site → looks dead.
- Emitted client params MUST equal the node inbound exactly: `port`, `pbk`↔privateKey, `sid`, `sni`, `dest`, `type=xhttp`, `xhttp path`, `xhttp mode`, `flow`, `packetEncoding`, fingerprint.
- Priorities: **correctness first, then high network speed on RU LTE.**
- TYRAX copy rules: cold/uppercase, no soft or apologetic UI strings.
- Reference node templates: `tyrax-infra/node/inbound-xhttp-reality.json`, `tyrax-infra/db/insert_node.sql`.

---

### Task 1: Confirm root cause with evidence (diagnosis gate — NO code changes)

**Goal:** Pick the real failure branch before fixing. Do steps in order; stop at the first that fails and record it. Needs the phone, backend logs, and the 3x-ui panel.

**Files:** none (investigation only).

- [ ] **Step 1: Pull the on-device Xray artefacts.** With the phone connected (ACCESS GRANTED, no internet), copy from `Android/data/com.tyrax/files/`:
  - `xray_config.json`, `xray_error.log`, `xray_access.log`
  Via `adb`: `adb pull /sdcard/Android/data/com.tyrax/files/ ./tyrax-device-logs` (or a file manager). Read `xray_error.log`.
  - Expected if H1/H2 (node rejects UUID / Reality mismatch): access log shows outbound attempts but no successful upstream; error log shows TLS/reality handshake or "connection reset" toward the node; browsing produces the decoy site.
  - Expected if H3 (client transport): errors about xhttp/xmux/DNS locally, or `initCoreEnv`/geo problems.

- [ ] **Step 2: Read the backend connect warning.** On the backend host tail logs during a phone connect:
  `docker compose logs -f backend | grep -i "panel addClient"` (or the service's log sink).
  - If you see `panel addClient (connect) ... err=...` → panel sync IS failing; capture the exact `err` (this is the smoking gun for H1). Note whether it is `non-JSON response (status 404/403)` (endpoint/token/base-path wrong) vs `addClient failed: <msg>` (panel rejected the client) vs a dial timeout (panel/node unreachable).
  - If NO warning appears → AddClient is succeeding; jump focus to H2 (node-row/param mismatch) and Step 4.

- [ ] **Step 3: Verify the UUID on the node inbound (3x-ui).** Open the panel → the VLESS inbound → Clients. From the phone's `xray_config.json` take the `vnext[0].users[0].id` UUID.
  - Search the inbound's client list for that UUID (client email = backend `device.ID`).
  - MISSING → confirms **H1** (UUID not on inbound). Check the panel's inbound **id** and compare with the `nodes.panel_inbound_id` DB value (Task 3, Fix C). Also confirm the panel API endpoint the client uses (`/panel/api/clients/add`) matches this panel version.
  - PRESENT → not H1; go to Step 4.

- [ ] **Step 4: Compare node-row params vs the live inbound (H2).** Read the `nodes` row for the codename the phone used and the inbound JSON in 3x-ui. Compare every field in this table:

  | `nodes` column | Inbound field | Must match |
  |---|---|---|
  | `reality_public_key` | pair of inbound Reality **private** key | yes |
  | `reality_short_id` (**non-empty**) | `shortIds[]` | yes |
  | `reality_sni` | `serverNames[]` | yes |
  | `host` / `port` | node IP/domain + listen port | yes |
  | `network` (`xhttp`) | transport = xhttp | yes |
  | `xhttp_path` (`/api/v1/data`) | xhttp path | yes |
  | `xhttp_mode` (`auto`) | xhttp mode | yes |
  | `flow` | client flow (empty for XHTTP profile) | yes |

  - Any mismatch (esp. empty `sid`, wrong `pbk`) → **H2** (Task 3, Fix D).

- [ ] **Step 5: Confirm the node core version (version lock).** In 3x-ui note the Xray version. Android core is `libv2ray.aar` v26.x. Different major → **Task 3, Fix F**.

- [ ] **Step 6: Record the verdict.** Write one line into this plan under Task 1 (e.g. "ROOT CAUSE = H1: AddClient returns `non-JSON 404`, UUID absent, panel_inbound_id=0"). This selects Task 3's branch. Task 2 (hardening) proceeds regardless.

---

### Task 2: Backend — never hand out a config for an unconfirmed UUID (hardening, unconditional)

**Goal:** Reverse the silent failure introduced by `1e3a7c7`. `Syncer.AddClient` already returns `nil` for a duplicate ("already exists"), so a **non-nil error means the UUID is genuinely not confirmed on the inbound** — connect must not pretend success. Surface the real error and log it; do the same for the HAPP feed.

**Files:**
- Modify: `tyrax-backend/internal/service/vpn_service.go` (Connect, vless branch ~L323-330)
- Modify: `tyrax-backend/internal/service/happ_subscription_service.go` (RenderFeed loop ~L138-144)
- Test: `tyrax-backend/internal/service/vpn_service_test.go` (create if absent)

**Interfaces:**
- Consumes: `s.panel.AddClient(ctx, node, uuid, email) error` (nil on success/duplicate; non-nil on genuine failure). `ErrNodeUnavailable` already exists in this package.
- Produces: `Connect` returns `ErrNodeUnavailable` when `AddClient` fails for a vless node; HAPP feed omits that node's line and logs the error instead of `_ = err`.

- [ ] **Step 1: Write the failing test** for connect. Create `tyrax-backend/internal/service/vpn_service_test.go` with a fake panel whose `AddClient` returns an error, and assert `Connect` returns `ErrNodeUnavailable` (not a config):

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type stubPanel struct{ addErr error }

func (s stubPanel) AddClient(ctx context.Context, n model.Node, uuid, email string) error { return s.addErr }
func (s stubPanel) DelClient(ctx context.Context, n model.Node, email string) error       { return nil }
func (s stubPanel) ClientTraffic(ctx context.Context, n model.Node, email string) (int64, error) { return 0, nil }
func (s stubPanel) Onlines(ctx context.Context, n model.Node) (int, error)                { return 0, nil }

func TestConnect_VlessAddClientFailure_ReturnsNodeUnavailable(t *testing.T) {
	node := model.Node{Codename: "NL-01", Protocol: "vless", PanelURL: "https://p", PanelInboundID: 1}
	if !connectAddClientFails(t, node, errors.New("addClient failed: bad inbound")) {
		t.Fatal("expected ErrNodeUnavailable when panel AddClient fails")
	}
}
```

> Implementer note: wire `stubPanel` into a `vpnService` with in-memory repo fakes following the existing constructor in `vpn_service.go`. `connectAddClientFails` is a helper that builds the service with a device already present (so the vless branch runs) and asserts `errors.Is(err, ErrNodeUnavailable)`. If a fake harness already exists in the package, reuse it.

- [ ] **Step 2: Run the test to verify it fails.**
Run: `cd tyrax-backend && go test ./internal/service/ -run TestConnect_VlessAddClientFailure -v`
Expected: FAIL (Connect currently returns a config, not `ErrNodeUnavailable`).

- [ ] **Step 3: Restore honest failure in `Connect`.** Replace the swallow (`vpn_service.go` vless branch):

```go
	case "vless":
		// A duplicate ("already on inbound") returns nil from Syncer.AddClient, so a
		// non-nil error here means the UUID is NOT confirmed on the node inbound.
		// Handing out a config anyway yields a live "connected" state with zero
		// traffic (Reality serves the decoy site). Fail loudly instead.
		if err := s.panel.AddClient(ctx, *node, device.VlessUUID, device.ID); err != nil {
			slog.Error("panel addClient (connect)", "node", node.Codename, "device", device.ID, "err", err.Error())
			return nil, ErrNodeUnavailable
		}
		config = vpnconfig.GenerateVlessConfig(*node, device.VlessUUID)
```

- [ ] **Step 4: Run the test to verify it passes.**
Run: `cd tyrax-backend && go test ./internal/service/ -run TestConnect_VlessAddClientFailure -v`
Expected: PASS.

- [ ] **Step 5: Fix the HAPP silent failure.** In `happ_subscription_service.go` RenderFeed loop, replace the `_ = err` block so a failed AddClient omits that node's line and logs it (never emit a dead `vless://`):

```go
		if s.panel != nil {
			if err := s.panel.AddClient(ctx, node, device.VlessUUID, device.ID); err != nil {
				slog.Warn("happ addClient", "node", node.Codename, "device", device.ID, "err", err.Error())
				continue // skip: emitting a link the node rejects looks like "no ping"
			}
		}
		lines = append(lines, vpnconfig.GenerateVlessURI(node, device.VlessUUID, nodeRemark(node)))
```

Add `"log/slog"` to the imports if not present.

- [ ] **Step 6: Run the whole service + build.**
Run: `cd tyrax-backend && go build ./... && go test ./internal/service/... ./pkg/...`
Expected: PASS / no build errors.

- [ ] **Step 7: Commit.**

```bash
git add tyrax-backend/internal/service/vpn_service.go tyrax-backend/internal/service/happ_subscription_service.go tyrax-backend/internal/service/vpn_service_test.go
git commit -m "fix(vpn): fail connect + skip happ node when panel UUID sync fails (no false ACCESS GRANTED)"
```

---

### Task 3: Correct the real defect found in Task 1 (apply ONLY the matching branch)

Each branch ends with: reconnect the phone and confirm real traffic (open youtube/google), then re-import the HAPP link and confirm the node pings.

- [ ] **Fix A — Xray not running on the node:** In 3x-ui restart Xray (or `systemctl restart x-ui` via SSH). Re-check Task 1 Step 1.

- [ ] **Fix B — inbound disabled / wrong transport:** Recreate the inbound from `tyrax-infra/node/inbound-xhttp-reality.json` (port 443, xhttp path `/api/v1/data` mode `auto`, reality dest `www.microsoft.com:443`, serverNames `["www.microsoft.com"]`, non-empty shortId, fingerprint chrome), then align the DB row (Fix D).

- [ ] **Fix C — UUID not on inbound (panel sync broken):** Most likely `nodes.panel_inbound_id` is wrong (e.g. `0`) or `panel_url`/`panel_token`/base-path is wrong, or this panel version doesn't expose `/panel/api/clients/add`.
  - Verify the inbound **id** in 3x-ui and set the DB `panel_inbound_id` to it:

```sql
UPDATE nodes SET panel_inbound_id = <REAL_INBOUND_ID> WHERE codename = '<CODENAME>';
```

  - If the warning in Task 1 Step 2 was `non-JSON response (status 404)`, the panel API path differs; confirm the client endpoint against this 3x-ui version (`/panel/api/clients/add` vs `/panel/api/inbounds/addClient`) in `tyrax-backend/pkg/threexui/client.go` and correct it, then `go build ./...` and redeploy the backend.
  - After fixing, hit `/sub/<token>` (re-runs AddClient) and re-check Task 1 Step 3. With Task 2 in place, a still-broken sync now returns `NODE UNAVAILABLE` loudly instead of a dead tunnel.

- [ ] **Fix D — node-row param mismatch:** Update the `nodes` row to match the inbound exactly, using `tyrax-infra/db/insert_node.sql` as the field template:

```sql
UPDATE nodes SET
  reality_public_key = '<PBK_MATCHING_INBOUND_PRIVATE_KEY>',
  reality_short_id   = '<NON_EMPTY_SID>',
  reality_sni        = 'www.microsoft.com',
  host = '<NODE_HOST>', port = 443,
  network = 'xhttp', xhttp_path = '/api/v1/data', xhttp_mode = 'auto',
  flow = ''
WHERE codename = '<CODENAME>';
```

  Regenerate the link from `/sub/` and re-import; reconnect the native app.

- [ ] **Fix E — node unreachable / RKN subnet block:** From an RU network (or any external host for basic reachability):

```bash
curl -v -m 8 https://<NODE_HOST>:443 --resolve <NODE_HOST>:443:<NODE_IP>
```

  Expect a TLS handshake / decoy site, not timeout/refused. If reachable externally but dead from RU → rotate the node IP (provider free IP change) or front it with the CDN/TLS profile (`tyrax-infra/node/inbound-xhttp-tls-cdn.json` + `tyrax-infra/db/insert_node_cdn.sql`, security=`tls`) to hide the origin IP behind Cloudflare.

- [ ] **Fix F — core version mismatch:** Align the node Xray core major to the client's v26.x (or vice versa). XHTTP + xmux behaviour is version-sensitive.

---

### Task 4: HAPP speed — emit xmux single-mux in the `vless://` link (RU-LTE throttle fix)

**Goal:** Android/Windows force a single multiplexed H2 connection (xmux) to avoid carrier throttling; the HAPP link carries none, so HAPP opens many connections and crawls on RU LTE. Add xmux to the xhttp `vless://` query. **Verify HAPP actually consumes these query keys before shipping** (H2 below).

**Files:**
- Modify: `tyrax-backend/pkg/vpnconfig/vless_uri.go` (`GenerateVlessURI`, xhttp block ~L51-69)
- Test: `tyrax-backend/pkg/vpnconfig/vless_uri_test.go`

**Interfaces:**
- Consumes: `model.Node` (Network, Xhttp* fields), `GenerateVlessURI(node, uuid, remark) string`.
- Produces: same signature; xhttp links additionally carry `xmux`/extra params consistent with the Android `tuneXhttpMux` values (`maxConnections=1`, `maxConcurrency=0`).

- [ ] **Step 1: Verify HAPP's supported xhttp `vless://` query keys** (documentation check, not code). Use context7 / Happ docs to confirm the exact query key(s) Happ parses for xhttp mux/extra (candidates: `extra` JSON blob, or discrete keys). Record the confirmed key spelling in a comment. If Happ does NOT support link-level xmux, STOP this task and instead document it as a known HAPP limitation in Task 7's report (do not ship params Happ ignores or chokes on).

- [ ] **Step 2: Write the failing test** in `vless_uri_test.go`:

```go
func TestGenerateVlessURI_XHTTPCarriesXmux(t *testing.T) {
	node := model.Node{
		Codename: "FI-01", Host: "203.0.113.10", Port: 443, Protocol: "vless",
		RealityPublicKey: "pubkey", RealityShortID: "abcd", RealitySNI: "www.microsoft.com",
		Network: "xhttp", XhttpPath: "/api/v1/data", XhttpMode: "auto", Fingerprint: "chrome",
	}
	uri := GenerateVlessURI(node, "550E8400-E29B-41D4-A716-446655440000", "TYRAX-FI-01")
	// exact key asserted here MUST be the one confirmed in Step 1
	if !strings.Contains(uri, "extra=") {
		t.Fatalf("xhttp link must carry xmux extra: %s", uri)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails.**
Run: `cd tyrax-backend && go test ./pkg/vpnconfig/ -run TestGenerateVlessURI_XHTTPCarriesXmux -v`
Expected: FAIL.

- [ ] **Step 4: Add the xmux params** in the `network == "xhttp"` block of `GenerateVlessURI`, using the key confirmed in Step 1. Example (JSON `extra` blob form; adjust to the confirmed spelling):

```go
	if network == "xhttp" {
		// ... existing path/mode/host code ...
		// Single multiplexed H2 connection — mirrors Android/Windows tuneXhttpMux so
		// HAPP is not throttled on RU LTE by many parallel connections.
		extra := `{"xmux":{"maxConnections":1,"maxConcurrency":0,"cMaxReuseTimes":0,"hMaxRequestTimes":"1000-5000","hMaxReusableSecs":"1800-3000","hKeepAlivePeriod":0}}`
		q.Set("extra", extra)
	}
```

- [ ] **Step 5: Run the tests to verify they pass** (and existing URI tests still pass).
Run: `cd tyrax-backend && go test ./pkg/vpnconfig/ -v`
Expected: PASS (all, including `TestGenerateVlessURI_XHTTPReality` and `_Vision`).

- [ ] **Step 6: Commit.**

```bash
git add tyrax-backend/pkg/vpnconfig/vless_uri.go tyrax-backend/pkg/vpnconfig/vless_uri_test.go
git commit -m "feat(happ): emit xmux single-mux in vless link to stop RU-LTE throttle"
```

---

### Task 5: Windows native TUN — add RU split-tunnel parity (currently missing)

**Goal:** `XrayWindowsConfigAdapter.BuildRouting()` (the native-TUN path the Windows service actually runs) has NO RU bypass and `domainStrategy=AsIs`, so on Windows RU-geoblocked apps go through the foreign node and can break (banks/Wildberries). Add the RU bypass to match Android/`XrayConfigBuilder` while keeping everything else through the proxy.

**Files:**
- Modify: `tyrax-windows/src/Tyrax.Tunnel/XrayWindowsConfigAdapter.cs` (`AdaptForNativeTun` signature + `BuildRouting`)
- Reference: `tyrax-windows/src/Tyrax.Data/SplitTunnelDefaults.cs` (RU domain list), Android `SplitTunnel.RU_SPLIT_DOMAINS`
- Test: `tyrax-windows/tests/Tyrax.Tunnel.Tests/XrayWindowsConfigAdapterTests.cs`

**Interfaces:**
- Consumes: backend SOCKS config JSON; optional `IReadOnlyList<string> splitDomains` + `bool splitEnabled`.
- Produces: `AdaptForNativeTun(string backendConfigJson, IReadOnlyList<string>? splitDomains = null, bool splitEnabled = false)`; when `splitEnabled`, routing sends `geoip:ru` + `geosite:category-ru` + `domain:<d>` to `direct` before the proxy catch-all, `domainStrategy=IPIfNonMatch`.

- [ ] **Step 1: Confirm Windows ships geoip.dat/geosite.dat** next to `xray.exe` (needed for `geoip:ru`/`geosite:category-ru`). Check `tyrax-windows/src/Tyrax.Service/Engines/EnginePaths.cs` and the packaged engine dir. If absent, add them to the engine payload (mirror Android `assets/geoip.dat`, `geosite.dat`). Record the finding.

- [ ] **Step 2: Write the failing test** in `XrayWindowsConfigAdapterTests.cs`:

```csharp
[Fact]
public void AdaptForNativeTun_SplitEnabled_RoutesRuDirect()
{
    var backend = /* minimal SOCKS+vless config JSON string */;
    var json = XrayWindowsConfigAdapter.AdaptForNativeTun(backend, new[] { "sberbank.ru" }, splitEnabled: true);
    Assert.Contains("geoip:ru", json);
    Assert.Contains("geosite:category-ru", json);
    Assert.Contains("domain:sberbank.ru", json);
}

[Fact]
public void AdaptForNativeTun_SplitDisabled_AllViaProxy()
{
    var backend = /* same config */;
    var json = XrayWindowsConfigAdapter.AdaptForNativeTun(backend, null, splitEnabled: false);
    Assert.DoesNotContain("geoip:ru", json);
}
```

> Implementer note: build the `backend` fixture from the existing tests in this file (reuse their sample config constant).

- [ ] **Step 3: Run the test to verify it fails.**
Run: `cd tyrax-windows && dotnet test --filter XrayWindowsConfigAdapter`
Expected: FAIL (method has no split params yet).

- [ ] **Step 4: Implement split-aware routing.** Change the signature and `BuildRouting`:

```csharp
public static string AdaptForNativeTun(
    string backendConfigJson,
    IReadOnlyList<string>? splitDomains = null,
    bool splitEnabled = false)
{
    var root = JsonNode.Parse(backendConfigJson)?.AsObject()
        ?? throw new ArgumentException("INVALID XRAY CONFIG", nameof(backendConfigJson));

    root.Remove("fakedns");
    root["dns"] = BuildDns();
    root["inbounds"] = new JsonArray(CreateTunInbound());
    EnhanceOutbounds(root);
    root["routing"] = BuildRouting(splitDomains, splitEnabled);

    return root.ToJsonString(new JsonSerializerOptions { WriteIndented = true });
}

private static JsonObject BuildRouting(IReadOnlyList<string>? splitDomains, bool splitEnabled)
{
    var rules = new JsonArray
    {
        new JsonObject
        {
            ["type"] = "field",
            ["outboundTag"] = "direct",
            ["ip"] = new JsonArray(
                "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
                "192.168.0.0/16", "::1/128", "fc00::/7", "fe80::/10"),
        },
    };

    if (splitEnabled)
    {
        var domains = new JsonArray { "geosite:category-ru" };
        if (splitDomains is { Count: > 0 })
            foreach (var d in splitDomains) domains.Add($"domain:{d}");
        rules.Add(new JsonObject
        {
            ["type"] = "field", ["outboundTag"] = "direct", ["domain"] = domains,
        });
        rules.Add(new JsonObject
        {
            ["type"] = "field", ["outboundTag"] = "direct", ["ip"] = new JsonArray("geoip:ru"),
        });
    }

    rules.Add(new JsonObject
    {
        ["type"] = "field", ["outboundTag"] = "proxy", ["network"] = "tcp,udp",
    });

    return new JsonObject
    {
        ["domainStrategy"] = splitEnabled ? "IPIfNonMatch" : "AsIs",
        ["rules"] = rules,
    };
}
```

- [ ] **Step 5: Wire the caller** to pass the split flag + domains. Find the `AdaptForNativeTun` call site (grep in `tyrax-windows/src/Tyrax.Service/`), pass `SplitTunnelDefaults.RuDomains` (or the server list) and the user's split toggle. Keep private `10.0.0.0/8` OUT of direct here only if the TUN subnet is `10.7.0.x` — it is (`TunGateway=10.7.0.1`), so `10.0.0.0/8` must NOT be forced direct (matches current adapter which omits it). Leave as above.

- [ ] **Step 6: Run the tests to verify they pass.**
Run: `cd tyrax-windows && dotnet test --filter XrayWindowsConfigAdapter`
Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add tyrax-windows/src/Tyrax.Tunnel/XrayWindowsConfigAdapter.cs tyrax-windows/tests/Tyrax.Tunnel.Tests/XrayWindowsConfigAdapterTests.cs
git commit -m "feat(windows): RU split-tunnel in native TUN routing (parity with Android)"
```

---

### Task 6: Cross-client parity report + full-stack verification

**Goal:** Produce the audit matrix and verify every client end-to-end (correctness + speed + split-tunnel). No hidden divergences remain.

**Files:**
- Create: `docs/superpowers/reports/2026-07-03-vpn-parity-matrix.md`

- [ ] **Step 1: Build the parity matrix** `client × parameter × value/status` covering: `pbk`, `sid`, `sni`, `port`, `type`, `xhttp path`, `xhttp mode`, `flow`, `packetEncoding`, fingerprint, `xmux` (maxConnections/maxConcurrency/hMaxReusableSecs), `domainStrategy`, DNS servers, split-tunnel mechanism, and Xray core major. Columns: Backend `GenerateVlessConfig`, Backend `GenerateVlessURI` (HAPP), Android `XrayConfigPatcher`, Windows `XrayWindowsConfigAdapter`. Mark each cell OK / divergent, and list every divergence with the resolving task (Tasks 2/4/5) or an explicit "intentional per-platform" note (e.g. Windows `hMaxReusableSecs=7200-10800` vs Android `1800-3000` — documented desktop tuning).

- [ ] **Step 2: Verify Android** (build already has R8 keep-rules from `91cdc22`). Install current release, connect with split ON:
  - Expected: ACCESS GRANTED + real traffic (youtube/google load).
  - Expected: Wildberries/Ozon/bank app works with VPN on (bypasses via RU network).
  - If Task 3 changed backend/DB, no APK rebuild is needed for those fixes.

- [ ] **Step 3: Verify HAPP** on phone + PC: re-import `/sub/<token>` feed, confirm each node pings and loads; confirm the profile is not throttled (Task 4 xmux) — a large download should sustain speed, not collapse.

- [ ] **Step 4: Verify Windows** native client: connect, confirm traffic loads and RU split-tunnel now routes RU apps direct (Task 5). Confirm the ~30-min mux recycle (`hMaxReusableSecs=7200-10800`) doesn't drop the tunnel.

- [ ] **Step 5: Confirm no false-connected regression** — with a deliberately broken panel sync (temporarily point a test node's `panel_inbound_id` at a wrong id in staging), the native app now shows `NODE UNAVAILABLE` and HAPP omits that node, instead of a dead "connected" tunnel. Revert the staging change after.

- [ ] **Step 6: Commit the report.**

```bash
git add docs/superpowers/reports/2026-07-03-vpn-parity-matrix.md
git commit -m "docs: cross-client VPN parity matrix + verification results"
```

---

## Self-Review

- **Spec coverage:** regression root-cause (Task 1) → honest-failure hardening (Task 2) → real defect fix (Task 3, branches A–F) → HAPP speed/xmux (Task 4) → Windows RU split parity (Task 5) → parity matrix + full verification incl. split-tunnel + speed (Task 6). All spec deliverables and success criteria mapped.
- **Constraint coverage:** RU split-tunnel preserved (Tasks 5/6 verify it, no task removes it); `libv2ray.aar` untouched; version-lock checked (Task 1 Step 5 / Fix F); non-empty `sid` (Task 1 Step 4 / Fix D); param parity (Task 6); no soft copy (uses `NODE UNAVAILABLE`).
- **Placeholder scan:** Task 3 branches are intentionally conditional (chosen by Task 1) but each carries exact SQL/commands. Task 4 Step 1 and Task 5 Step 1 are explicit verification steps with go/no-go outcomes, not TODOs. No "TBD".
- **Type consistency:** `AddClient` returns `error` (nil on duplicate) used consistently in Tasks 2; `AdaptForNativeTun` new signature used in Task 5 Steps 4–6; `GenerateVlessURI` signature unchanged in Task 4.
- **Ambiguity:** Task 4 ships xmux ONLY after confirming HAPP consumes the key (Step 1); otherwise it degrades to a documented limitation — no guessing shipped.
```
