package vpnconfig

import (
	"encoding/json"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const socksInboundPort = 10808

// streamSettings transport defaults applied when a node row leaves them blank
// (e.g. pre-009 rows or partially configured nodes).
const (
	defaultNetwork       = "xhttp"
	defaultXhttpPath     = "/api/v1/data"
	defaultXhttpMode     = "auto"
	defaultXPaddingBytes = "100-1000"
	defaultFingerprint   = "chrome"
)

func GenerateVlessConfig(node model.Node, userUUID string) string {
	cfg := xrayConfig{
		Log: xrayLog{Loglevel: "warning"},
		FakeDNS: []xrayFakeDNS{
			{IPPool: "198.18.0.0/15", PoolSize: 65535},
		},
		DNS: &xrayDNS{
			QueryStrategy: "UseIPv4",
			Servers: []any{
				"fakedns",
				map[string]any{
					"address":  "1.1.1.1",
					"port":     53,
					"proxyTag": "proxy",
				},
			},
		},
		Inbounds: []xrayInbound{
			{
				Tag:      "socks-in",
				Listen:   "127.0.0.1",
				Port:     socksInboundPort,
				Protocol: "socks",
				Settings: xraySocksSettings{Auth: "noauth", UDP: true},
				Sniffing: xraySniffing{
					Enabled:      true,
					DestOverride: []string{"fakedns", "http", "tls", "quic"},
				},
			},
		},
		Outbounds: []xrayOutbound{
			{
				Tag:      "proxy",
				Protocol: "vless",
				Settings: xrayVlessSettings{
					Vnext: []xrayVnext{
						{
							Address: node.Host,
							Port:    node.Port,
							Users: []xrayVlessUser{
								{ID: userUUID, Encryption: "none", Flow: node.Flow},
							},
						},
					},
				},
				StreamSettings: buildStreamSettings(node),
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
				Settings: struct{}{},
			},
			{
				Tag:      "dns-out",
				Protocol: "dns",
				Settings: struct{}{},
			},
		},
		Routing: &xrayRouting{
			DomainStrategy: "IPIfNonMatch",
			Rules: []xrayRule{
				{
					Type: "field", InboundTag: []string{"socks-in"},
					Port: "53", Network: "tcp,udp", OutboundTag: "dns-out",
				},
				{
					Type: "field", OutboundTag: "proxy",
					IP: []string{"198.18.0.0/15", "240.0.0.0/4"},
				},
				{
					Type: "field", OutboundTag: "direct",
					IP: []string{
						"127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
						"192.168.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
					},
				},
				{Type: "field", OutboundTag: "proxy", Network: "tcp,udp"},
			},
		},
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}

// buildStreamSettings assembles the proxy outbound's transport + Reality security
// from the node row, defaulting blank fields to the Profile A (XHTTP) defaults.
// Network "xhttp" defeats behavioural DPI by splitting the tunnel into normal
// HTTP request/response transactions with size-normalising padding; "tcp" keeps
// the legacy raw-TCP path. Reality (steal-from-self TLS) is always applied.
func buildStreamSettings(node model.Node) *xrayStreamSettings {
	network := node.Network
	if network == "" {
		network = defaultNetwork
	}

	fingerprint := node.Fingerprint
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}

	security := node.Security
	if security == "" {
		security = "reality"
	}

	ss := &xrayStreamSettings{
		Network:  network,
		Security: security,
	}

	switch security {
	case "tls":
		// CDN profile: real TLS on a Cloudflare-proxied domain. serverName is the
		// proxied domain (host), and the edge presents a valid public cert, so
		// allowInsecure stays false. The origin IP is hidden behind the CDN.
		sni := node.RealitySNI
		if sni == "" {
			sni = node.Host
		}
		ss.TLSSettings = &xrayTLSSettings{
			ServerName:    sni,
			Fingerprint:   fingerprint,
			AllowInsecure: false,
			Alpn:          []string{"h2", "http/1.1"},
		}
	default:
		ss.Security = "reality"
		ss.RealitySettings = &xrayRealitySettings{
			Show:        false,
			Fingerprint: fingerprint,
			ServerName:  node.RealitySNI,
			PublicKey:   node.RealityPublicKey,
			ShortID:     node.RealityShortID,
			SpiderX:     "",
		}
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
		// XTLS-Vision over XHTTP is only compatible with the single-connection
		// "stream-one" mode; force it so a Vision node can't emit a broken config.
		if node.Flow == "xtls-rprx-vision" {
			mode = "stream-one"
		}
		padding := node.XPaddingBytes
		if padding == "" {
			padding = defaultXPaddingBytes
		}
		xhttp := &xrayXhttpSettings{
			Path: path,
			Mode: mode,
			Extra: &xrayXhttpExtra{
				XPaddingBytes: padding,
			},
		}
		// For the CDN/TLS profile, the HTTP Host header must be the proxied
		// domain so Cloudflare routes the request to the right origin.
		if security == "tls" {
			xhttp.Host = node.Host
		}
		ss.XhttpSettings = xhttp
	}

	return ss
}

type xrayConfig struct {
	Log       xrayLog        `json:"log"`
	FakeDNS   []xrayFakeDNS  `json:"fakedns,omitempty"`
	DNS       *xrayDNS       `json:"dns,omitempty"`
	Inbounds  []xrayInbound  `json:"inbounds"`
	Outbounds []xrayOutbound `json:"outbounds"`
	Routing   *xrayRouting   `json:"routing,omitempty"`
}

type xrayFakeDNS struct {
	IPPool   string `json:"ipPool"`
	PoolSize int    `json:"poolSize"`
}

type xrayDNS struct {
	Servers       []any  `json:"servers"`
	QueryStrategy string `json:"queryStrategy"`
}

type xrayRouting struct {
	DomainStrategy string     `json:"domainStrategy"`
	Rules          []xrayRule `json:"rules"`
}

type xrayRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Network     string   `json:"network,omitempty"`
	Port        string   `json:"port,omitempty"`
	IP          []string `json:"ip,omitempty"`
}

type xrayLog struct {
	Loglevel string `json:"loglevel"`
}

type xrayInbound struct {
	Tag      string            `json:"tag"`
	Listen   string            `json:"listen"`
	Port     int               `json:"port"`
	Protocol string            `json:"protocol"`
	Settings xraySocksSettings `json:"settings"`
	Sniffing xraySniffing      `json:"sniffing"`
}

type xraySocksSettings struct {
	Auth string `json:"auth"`
	UDP  bool   `json:"udp"`
}

type xraySniffing struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

type xrayOutbound struct {
	Tag            string              `json:"tag"`
	Protocol       string              `json:"protocol"`
	Settings       any                 `json:"settings"`
	StreamSettings *xrayStreamSettings `json:"streamSettings,omitempty"`
}

type xrayVlessSettings struct {
	Vnext []xrayVnext `json:"vnext"`
}

type xrayVnext struct {
	Address string          `json:"address"`
	Port    int             `json:"port"`
	Users   []xrayVlessUser `json:"users"`
}

type xrayVlessUser struct {
	ID         string `json:"id"`
	Encryption string `json:"encryption"`
	Flow       string `json:"flow"`
}

type xrayStreamSettings struct {
	Network         string               `json:"network"`
	Security        string               `json:"security"`
	RealitySettings *xrayRealitySettings `json:"realitySettings,omitempty"`
	TLSSettings     *xrayTLSSettings     `json:"tlsSettings,omitempty"`
	XhttpSettings   *xrayXhttpSettings   `json:"xhttpSettings,omitempty"`
}

type xrayTLSSettings struct {
	ServerName    string   `json:"serverName"`
	Fingerprint   string   `json:"fingerprint"`
	AllowInsecure bool     `json:"allowInsecure"`
	Alpn          []string `json:"alpn,omitempty"`
}

type xrayXhttpSettings struct {
	Path  string          `json:"path"`
	Mode  string          `json:"mode"`
	Host  string          `json:"host,omitempty"`
	Extra *xrayXhttpExtra `json:"extra,omitempty"`
}

type xrayXhttpExtra struct {
	XPaddingBytes string `json:"xPaddingBytes,omitempty"`
}

type xrayRealitySettings struct {
	Show        bool   `json:"show"`
	Fingerprint string `json:"fingerprint"`
	ServerName  string `json:"serverName"`
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
	SpiderX     string `json:"spiderX"`
}
