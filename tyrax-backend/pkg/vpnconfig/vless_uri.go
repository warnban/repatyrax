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
		// XTLS-Vision over XHTTP requires stream-one — same rule as buildStreamSettings.
		if node.Flow == "xtls-rprx-vision" {
			mode = "stream-one"
		}
		q.Set("path", path)
		q.Set("mode", mode)
		if security == "tls" {
			q.Set("host", node.Host)
		}
		// Single multiplexed H2 connection — mirrors Android tuneXhttpMux and the Windows
		// adapter so HAPP is not throttled on RU LTE by many parallel connections. Per the
		// Xray VLESS share-link standard (XTLS/Xray-core Discussion #716, section 4.3.19
		// "(XHTTP) extra", #4000), the whole XHTTP `extra` JSON is shared as the URL-encoded
		// `extra=` query key; xmux nests inside it. url.Values.Encode() handles the escaping.
		extra := `{"xmux":{"maxConcurrency":0,"maxConnections":1,"cMaxReuseTimes":0,"hMaxRequestTimes":"1000-5000","hMaxReusableSecs":"1800-3000","hKeepAlivePeriod":0}}`
		q.Set("extra", extra)
	}

	tag := remark
	if tag == "" {
		tag = fmt.Sprintf("TYRAX-%s", node.Codename)
	}

	uuid := strings.ToLower(userUUID)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, node.Host, node.Port, q.Encode(), url.PathEscape(tag))
}
