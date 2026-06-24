package model

import "time"

type Device struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"user_id"`
	Name      string    `db:"name" json:"name"`
	PublicKey string    `db:"public_key" json:"public_key"`
	ClientIP  string    `db:"client_ip" json:"client_ip"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
