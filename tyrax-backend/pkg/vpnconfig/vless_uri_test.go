package vpnconfig

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/tyrax/tyrax-backend/internal/model"
)

func TestGenerateVlessURI_XHTTPReality(t *testing.T) {
	node := model.Node{
		Codename:         "NL-01",
		Host:             "203.0.113.10",
		Port:             443,
		Protocol:         "vless",
		RealityPublicKey: "pubkey",
		RealityShortID:   "abcd",
		RealitySNI:       "www.microsoft.com",
		Network:          "xhttp",
		XhttpPath:        "/api/v1/data",
		XhttpMode:        "auto",
		Fingerprint:      "chrome",
	}
	uri := GenerateVlessURI(node, "550E8400-E29B-41D4-A716-446655440000", "TYRAX-NL-01")
	if !strings.HasPrefix(uri, "vless://550e8400-e29b-41d4-a716-446655440000@203.0.113.10:443?") {
		t.Fatalf("bad prefix: %s", uri)
	}
	for _, part := range []string{"security=reality", "type=xhttp", "path=%2Fapi%2Fv1%2Fdata", "pbk=pubkey", "sid=abcd", "sni=www.microsoft.com", "mode=auto"} {
		if !strings.Contains(uri, part) {
			t.Fatalf("missing %q in %s", part, uri)
		}
	}
}

func TestGenerateVlessURI_XHTTPCarriesXmux(t *testing.T) {
	node := model.Node{
		Codename:         "FI-01",
		Host:             "203.0.113.10",
		Port:             443,
		Protocol:         "vless",
		RealityPublicKey: "pubkey",
		RealityShortID:   "abcd",
		RealitySNI:       "www.microsoft.com",
		Network:          "xhttp",
		XhttpPath:        "/api/v1/data",
		XhttpMode:        "auto",
		Fingerprint:      "chrome",
	}
	uri := GenerateVlessURI(node, "550E8400-E29B-41D4-A716-446655440000", "TYRAX-FI-01")
	// Xray share-link standard (XTLS/Xray-core Discussion #716, section 4.3.19 XHTTP
	// `extra`, added by RPRX 2024-11-11 / #4000): the whole XHTTP `extra` JSON — which
	// contains `xmux` — is shared as a single URL-encoded `extra=` query key.
	if !strings.Contains(uri, "extra=") {
		t.Fatalf("xhttp link must carry xmux extra: %s", uri)
	}
	// The xmux single-mux payload must survive url.Values.Encode() (URL-encoded JSON).
	q := uri[strings.Index(uri, "?")+1:]
	if hash := strings.Index(q, "#"); hash >= 0 {
		q = q[:hash]
	}
	values, err := url.ParseQuery(q)
	if err != nil {
		t.Fatalf("query not parseable: %v", err)
	}
	extra := values.Get("extra")
	if extra == "" {
		t.Fatalf("extra query key missing/empty in %s", uri)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extra), &decoded); err != nil {
		t.Fatalf("extra is not valid JSON after decode: %v (%q)", err, extra)
	}
	xmux, ok := decoded["xmux"].(map[string]any)
	if !ok {
		t.Fatalf("extra JSON missing xmux object: %q", extra)
	}
	if xmux["maxConnections"] != float64(1) {
		t.Fatalf("xmux.maxConnections must be 1: %q", extra)
	}
	if xmux["maxConcurrency"] != float64(0) {
		t.Fatalf("xmux.maxConcurrency must be 0: %q", extra)
	}
}

func TestGenerateVlessURI_XHTTPRealityVision(t *testing.T) {
	node := model.Node{
		Codename:         "NL-01",
		Host:             "203.0.113.10",
		Port:             443,
		Protocol:         "vless",
		RealityPublicKey: "pubkey",
		RealityShortID:   "abcd",
		RealitySNI:       "www.microsoft.com",
		Network:          "xhttp",
		XhttpPath:        "/api/v1/data",
		XhttpMode:        "auto",
		Flow:             "xtls-rprx-vision",
		Fingerprint:      "chrome",
	}
	uri := GenerateVlessURI(node, "550E8400-E29B-41D4-A716-446655440000", "TYRAX-NL-01")
	if !strings.Contains(uri, "flow=xtls-rprx-vision") {
		t.Fatalf("missing flow in %s", uri)
	}
	if !strings.Contains(uri, "mode=stream-one") {
		t.Fatalf("vision profile must force stream-one, got %s", uri)
	}
	if strings.Contains(uri, "mode=auto") {
		t.Fatalf("must not keep auto mode with vision flow: %s", uri)
	}
}
