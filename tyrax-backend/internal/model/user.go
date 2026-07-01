package model

import (
	"time"
)

type SubscriptionTier string

const (
	TierFree     SubscriptionTier = "FREE"
	TierCore     SubscriptionTier = "CORE"
	TierShadow   SubscriptionTier = "SHADOW"
	TierDominion SubscriptionTier = "DOMINION"
)

type User struct {
	ID               string           `db:"id" json:"id"`
	Email            string           `db:"email" json:"email"`
	PasswordHash     string           `db:"password_hash" json:"-"`
	TelegramID       *int64           `db:"telegram_id" json:"telegram_id,omitempty"`
	SubscriptionTier SubscriptionTier `db:"subscription_tier" json:"subscription_tier"`
	SubscriptionEnd  *time.Time       `db:"subscription_end" json:"subscription_end,omitempty"`
	CreatedAt        time.Time        `db:"created_at" json:"created_at"`

	// Traffic metering (FREE-tier quota). Paid tiers ignore these.
	TrafficUsedBytes   int64      `db:"traffic_used_bytes" json:"traffic_used_bytes"`
	TrafficPeriodStart time.Time  `db:"traffic_period_start" json:"traffic_period_start"`
	// BlockedUntil is non-nil while a FREE user is locked out after exhausting
	// their quota; the tunnel stays blocked until this instant passes.
	BlockedUntil *time.Time `db:"blocked_until" json:"blocked_until,omitempty"`
}

type Node struct {
	ID       string     `db:"id" json:"id"`
	Codename string     `db:"codename" json:"codename"` // e.g. "NL-01"
	Country  string     `db:"country" json:"country"`
	Host     string     `db:"host" json:"host"`
	Port     int        `db:"port" json:"port"`
	Protocol string     `db:"protocol" json:"protocol"` // wireguard | vless | shadowsocks
	Status   NodeStatus `db:"status" json:"status"`
	PingMS   int        `db:"ping_ms" json:"ping_ms"`
	// PublicKey is the server-side WireGuard public key (not exposed in JSON).
	PublicKey string           `db:"public_key" json:"-"`
	MinTier   SubscriptionTier `db:"min_tier" json:"min_tier"`
	// Reality* hold the VLESS XTLS-Reality handshake parameters for this node.
	RealityPublicKey string `db:"reality_public_key" json:"-"`
	RealityShortID   string `db:"reality_short_id" json:"-"`
	RealitySNI       string `db:"reality_sni" json:"reality_sni,omitempty"`
	// RealityDest is the masquerade target ("host:443") the node steals its TLS
	// cert from. Server-side (3x-ui) concept; kept here for ops parity, not used
	// in the generated client config.
	RealityDest string `db:"reality_dest" json:"-"`
	// Security selects the stream security: "reality" (direct, steal-from-self)
	// or "tls" (real TLS on a CDN-proxied domain — hides the origin IP).
	Security string `db:"security" json:"-"`
	// Transport / anti-DPI parameters (RU 2026). Network selects the Xray
	// stream transport: "tcp" (legacy) or "xhttp" (default, behavioural-resistant).
	Network string `db:"network" json:"-"`
	// Flow is the VLESS user flow: "" or "xtls-rprx-vision". Vision over XHTTP
	// requires XhttpMode == "stream-one".
	Flow string `db:"flow" json:"-"`
	// XHTTP transport tuning (used only when Network == "xhttp").
	XhttpPath     string `db:"xhttp_path" json:"-"`
	XhttpMode     string `db:"xhttp_mode" json:"-"`     // auto | packet-up | stream-up | stream-one
	XPaddingBytes string `db:"x_padding_bytes" json:"-"` // e.g. "100-1000"
	// Fingerprint is the uTLS ClientHello to mimic (default "chrome").
	Fingerprint string `db:"fingerprint" json:"-"`
	// Panel* are the node's 3x-ui panel access details used by the backend to
	// register/remove per-device VLESS UUIDs via the panel API. Secrets — never
	// exposed in JSON. Empty PanelURL means "no sync" (manual / shared-UUID node).
	// Auth uses PanelToken (Bearer API token): 3x-ui >= 3.x guards POST /login
	// with CSRF, but a Bearer token bypasses CSRF on /panel/api/... routes.
	// PanelUser/PanelPass are retained for reference/ops only.
	PanelURL       string `db:"panel_url" json:"-"`
	PanelUser      string `db:"panel_user" json:"-"`
	PanelPass      string `db:"panel_pass" json:"-"`
	PanelInboundID int    `db:"panel_inbound_id" json:"-"`
	PanelToken     string `db:"panel_token" json:"-"`
}

type NodeStatus string

const (
	NodeOpen              NodeStatus = "OPEN"
	NodeMonitored         NodeStatus = "MONITORED"
	NodeHeavilyRestricted NodeStatus = "HEAVILY_RESTRICTED"
)

type ConnectionLog struct {
	ID         string    `db:"id"`
	UserID     string    `db:"user_id"`
	NodeID     string    `db:"node_id"`
	Protocol   string    `db:"protocol"`
	ConnectedAt time.Time `db:"connected_at"`
	DisconnectedAt *time.Time `db:"disconnected_at"`
}
