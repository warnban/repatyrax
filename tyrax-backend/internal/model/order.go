package model

import "time"

type PaymentMethod string

const (
	PaymentSBP    PaymentMethod = "SBP"     // FreeKassa i=44
	PaymentCardRF PaymentMethod = "CARD_RF" // FreeKassa i=36
	PaymentCrypto PaymentMethod = "CRYPTO"  // CryptoPay
)

type OrderStatus string

const (
	OrderNew       OrderStatus = "NEW"
	OrderPaid      OrderStatus = "PAID"
	OrderCancelled OrderStatus = "CANCELLED"
	OrderRefunded  OrderStatus = "REFUNDED"
)

type Order struct {
	ID              string        `db:"id"`
	UserID          string        `db:"user_id"`
	Tier            string        `db:"tier"`    // CORE / SHADOW / DOMINION
	Months          int           `db:"months"`  // 1 / 3 / 6 / 12
	AmountRUB       float64       `db:"amount_rub"`
	PaymentMethod   PaymentMethod `db:"payment_method"`
	ExternalOrderID string        `db:"external_order_id"`
	Status          OrderStatus   `db:"status"`
	CreatedAt       time.Time     `db:"created_at"`
	PaidAt          *time.Time    `db:"paid_at"`
}
