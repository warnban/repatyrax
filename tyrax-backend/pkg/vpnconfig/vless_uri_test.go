package vpnconfig

import (
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
