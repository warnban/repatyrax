package model

import "time"

type Invite struct {
	ID        string    `db:"id"`
	OwnerID   string    `db:"owner_id"`
	InviteeID string    `db:"invitee_id"`
	Status    string    `db:"status"` // pending | accepted
	CreatedAt time.Time `db:"created_at"`
}
