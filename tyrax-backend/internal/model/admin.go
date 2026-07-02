package model

import "time"

// AdminUserRow is the list-view projection for the admin panel.
type AdminUserRow struct {
	User
	DeviceCount        int  `json:"device_count"`
	IsOnline           bool `json:"is_online"`
	EffectiveTier      SubscriptionTier `json:"effective_tier"`
	ActiveConnections  int  `json:"active_connections"`
}

// AdminUserDetail extends a user with related admin data.
type AdminUserDetail struct {
	AdminUserRow
	Connections []ConnectionLog `json:"connections"`
	Orders      []Order         `json:"orders"`
}

// GrantPeriod identifies admin manual subscription grants.
type GrantPeriod string

const (
	Grant7Days  GrantPeriod = "7d"
	Grant14Days GrantPeriod = "14d"
	Grant1Month GrantPeriod = "1m"
	Grant3Month GrantPeriod = "3m"
	Grant6Month GrantPeriod = "6m"
	Grant12Month GrantPeriod = "12m"
)

func (p GrantPeriod) DurationFrom(base time.Time) time.Time {
	switch p {
	case Grant7Days:
		return base.AddDate(0, 0, 7)
	case Grant14Days:
		return base.AddDate(0, 0, 14)
	case Grant1Month:
		return base.AddDate(0, 1, 0)
	case Grant3Month:
		return base.AddDate(0, 3, 0)
	case Grant6Month:
		return base.AddDate(0, 6, 0)
	case Grant12Month:
		return base.AddDate(0, 12, 0)
	default:
		return base
	}
}

func ValidGrantPeriod(p string) bool {
	switch GrantPeriod(p) {
	case Grant7Days, Grant14Days, Grant1Month, Grant3Month, Grant6Month, Grant12Month:
		return true
	default:
		return false
	}
}
