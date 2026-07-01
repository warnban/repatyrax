package model

import "time"

type Device struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"user_id"`
	Name      string    `db:"name" json:"name"`
	PublicKey string    `db:"public_key" json:"public_key"`
	ClientIP  string    `db:"client_ip" json:"client_ip"`
	VlessUUID string    `db:"vless_uuid" json:"vless_uuid"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	// LastTrafficBytes is the cumulative up+down last read from the node panel,
	// used only by the traffic-accounting sweep. Not exposed to clients.
	LastTrafficBytes int64 `db:"last_traffic_bytes" json:"-"`
}
