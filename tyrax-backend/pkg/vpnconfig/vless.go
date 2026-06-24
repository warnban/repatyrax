package vpnconfig

import (
	"encoding/json"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// socksInboundPort is the local SOCKS5 listener the Android Xray-core engine
// proxies through (see TYRAX VPN layer rules).
const socksInboundPort = 10808

// GenerateVlessConfig produces an Xray-core compatible JSON config for a single
// user: a local SOCKS5 inbound feeding a VLESS outbound secured with
// XTLS-Reality, pointed at the given node.
func GenerateVlessConfig(node model.Node, userUUID string) string {
	cfg := xrayConfig{
		Log: xrayLog{Loglevel: "warning"},
		Inbounds: []xrayInbound{
			{
				Tag:      "socks-in",
				Listen:   "127.0.0.1",
				Port:     socksInboundPort,
				Protocol: "socks",
				Settings: xraySocksSettings{Auth: "noauth", UDP: true},
				Sniffing: xraySniffing{Enabled: true, DestOverride: []string{"http", "tls"}},
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
								{ID: userUUID, Encryption: "none", Flow: "xtls-rprx-vision"},
							},
						},
					},
				},
				StreamSettings: &xrayStreamSettings{
					Network:  "tcp",
					Security: "reality",
					RealitySettings: &xrayRealitySettings{
						Show:        false,
						Fingerprint: "chrome",
						ServerName:  node.RealitySNI,
						PublicKey:   node.RealityPublicKey,
						ShortID:     node.RealityShortID,
						SpiderX:     "/",
					},
				},
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
				Settings: struct{}{},
			},
		},
	}

	// MarshalIndent over these fixed structs cannot fail; an empty string would
	// only ever signal a programmer error in the schema above.
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}

type xrayConfig struct {
	Log       xrayLog        `json:"log"`
	Inbounds  []xrayInbound  `json:"inbounds"`
	Outbounds []xrayOutbound `json:"outbounds"`
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
}

type xrayRealitySettings struct {
	Show        bool   `json:"show"`
	Fingerprint string `json:"fingerprint"`
	ServerName  string `json:"serverName"`
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
	SpiderX     string `json:"spiderX"`
}
