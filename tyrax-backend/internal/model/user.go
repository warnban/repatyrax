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
