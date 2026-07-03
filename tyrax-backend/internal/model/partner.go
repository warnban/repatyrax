package model

import "time"

const (
	PartnerStatusActive    = "active"
	PartnerStatusSuspended = "suspended"

	PartnerPayoutMIR   = "mir"
	PartnerPayoutUSDT  = "usdt"

	CommissionHold      = "hold"
	CommissionAvailable = "available"
	CommissionClawedBack = "clawed_back"

	MinPayoutRUB  = 2000
	MinPayoutUSDT = 20

	ReferralConversionDays = 30
	CommissionHoldDays     = 3
	ActiveUserMinDays      = 3
)

type Partner struct {
	ID                     string     `db:"id" json:"id"`
	Email                  string     `db:"email" json:"email"`
	PasswordHash           string     `db:"password_hash" json:"-"`
	DisplayName            string     `db:"display_name" json:"display_name"`
	RefCode                string     `db:"ref_code" json:"ref_code"`
	CommissionRateOverride *float64   `db:"commission_rate_override" json:"commission_rate_override,omitempty"`
	Status                 string     `db:"status" json:"status"`
	PayoutMethod           *string    `db:"payout_method" json:"payout_method,omitempty"`
	PayoutMIRCard          *string    `db:"payout_mir_card" json:"payout_mir_card,omitempty"`
	PayoutUSDTAddress      *string    `db:"payout_usdt_address" json:"payout_usdt_address,omitempty"`
	PayoutUSDTNetwork      *string    `db:"payout_usdt_network" json:"payout_usdt_network,omitempty"`
	BalanceAvailable       float64    `db:"balance_available" json:"balance_available"`
	BalanceHold            float64    `db:"balance_hold" json:"balance_hold"`
	TotalPaidOut           float64    `db:"total_paid_out" json:"total_paid_out"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
}

type PartnerInvite struct {
	Token     string     `db:"token" json:"token"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	UsedAt    *time.Time `db:"used_at" json:"used_at,omitempty"`
	PartnerID *string    `db:"partner_id" json:"partner_id,omitempty"`
}

type PartnerSettings struct {
	DefaultCommissionRate float64   `db:"default_commission_rate" json:"default_commission_rate"`
	UpdatedAt             time.Time `db:"updated_at" json:"updated_at"`
}

type PartnerCommission struct {
	ID                  string     `db:"id" json:"id"`
	PartnerID           string     `db:"partner_id" json:"partner_id"`
	UserID              string     `db:"user_id" json:"user_id"`
	OrderID             string     `db:"order_id" json:"order_id"`
	OrderAmountRUB      float64    `db:"order_amount_rub" json:"order_amount_rub"`
	CommissionRate      float64    `db:"commission_rate" json:"commission_rate"`
	CommissionAmountRUB float64    `db:"commission_amount_rub" json:"commission_amount_rub"`
	Status              string     `db:"status" json:"status"`
	HoldUntil           time.Time  `db:"hold_until" json:"hold_until"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	ClawedBackAt        *time.Time `db:"clawed_back_at" json:"clawed_back_at,omitempty"`
}

type PartnerPayout struct {
	ID        string    `db:"id" json:"id"`
	PartnerID string    `db:"partner_id" json:"partner_id"`
	AmountRUB float64   `db:"amount_rub" json:"amount_rub"`
	Note      *string   `db:"note" json:"note,omitempty"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type PartnerStats struct {
	Registrations int `json:"registrations"`
	ActiveUsers   int `json:"active_users"`
	Conversions   int `json:"conversions"`
}

type PartnerAdminRow struct {
	Partner
	Stats PartnerStats `json:"stats"`
}
