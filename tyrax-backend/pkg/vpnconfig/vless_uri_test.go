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
	for _, part := range []string{"security=reality", "type=xhttp", "pbk=pubkey", "sid=abcd", "sni=www.microsoft.com", "mode=auto"} {
		if !strings.Contains(uri, part) {
			t.Fatalf("missing %q in %s", part, uri)
		}
	}
}
