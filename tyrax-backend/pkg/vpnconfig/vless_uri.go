package vpnconfig

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// GenerateVlessURI builds a share link for Happ / v2rayNG / Streisand.
// Transport defaults match GenerateVlessConfig (Profile A: XHTTP + Reality).
func GenerateVlessURI(node model.Node, userUUID, remark string) string {
	network := node.Network
	if network == "" {
		network = defaultNetwork
	}
	security := node.Security
	if security == "" {
		security = "reality"
	}
	fp := node.Fingerprint
	if fp == "" {
		fp = defaultFingerprint
	}

	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("security", security)
	q.Set("type", network)
	q.Set("fp", fp)

	switch security {
	case "tls":
		sni := node.RealitySNI
		if sni == "" {
			sni = node.Host
		}
		q.Set("sni", sni)
	default:
		q.Set("security", "reality")
		q.Set("sni", node.RealitySNI)
		q.Set("pbk", node.RealityPublicKey)
		q.Set("sid", node.RealityShortID)
	}

	if node.Flow != "" {
		q.Set("flow", node.Flow)
	}

	if network == "xhttp" {
		path := node.XhttpPath
		if path == "" {
			path = defaultXhttpPath
		}
		mode := node.XhttpMode
		if mode == "" {
			mode = defaultXhttpMode
		}
		q.Set("path", path)
		q.Set("mode", mode)
		if security == "tls" {
			q.Set("host", node.Host)
		}
	}

	tag := remark
	if tag == "" {
		tag = fmt.Sprintf("TYRAX-%s", node.Codename)
	}

	uuid := strings.ToLower(userUUID)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, node.Host, node.Port, q.Encode(), url.PathEscape(tag))
}
