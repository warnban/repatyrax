package model

import "time"

type SupportTicketStatus string

const (
	TicketOpen   SupportTicketStatus = "open"
	TicketClosed SupportTicketStatus = "closed"
)

type SupportTicket struct {
	ID                 string              `db:"id" json:"id"`
	UserID             *string             `db:"user_id" json:"user_id,omitempty"`
	TelegramID         int64               `db:"telegram_id" json:"telegram_id"`
	TelegramUsername   *string             `db:"telegram_username" json:"telegram_username,omitempty"`
	SubscriptionTier   SubscriptionTier    `db:"subscription_tier" json:"subscription_tier"`
	Status             SupportTicketStatus `db:"status" json:"status"`
	Subject            string              `db:"subject" json:"subject"`
	CreatedAt          time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time           `db:"updated_at" json:"updated_at"`
	ClosedAt           *time.Time          `db:"closed_at" json:"closed_at,omitempty"`
	LastMessagePreview *string             `json:"last_message,omitempty"`
	MessageCount       int                 `json:"message_count"`
}

type SupportMessage struct {
	ID        string    `db:"id" json:"id"`
	TicketID  string    `db:"ticket_id" json:"ticket_id"`
	Sender    string    `db:"sender" json:"sender"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
