package repository

import (
	"context"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type SupportRepository interface {
	FindOpenByTelegramID(ctx context.Context, telegramID int64) (*model.SupportTicket, error)
	CreateTicket(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error)
	AddMessage(ctx context.Context, ticketID, sender, body string) (*model.SupportMessage, error)
	ListTickets(ctx context.Context, status string, limit, offset int) ([]model.SupportTicket, int, error)
	GetTicket(ctx context.Context, ticketID string) (*model.SupportTicket, error)
	ListMessages(ctx context.Context, ticketID string) ([]model.SupportMessage, error)
	CloseTicket(ctx context.Context, ticketID string) error
	TouchTicket(ctx context.Context, ticketID string) error
}

type ConnectionRepository interface {
	LogConnect(ctx context.Context, userID, nodeID, protocol string) (string, error)
	LogDisconnect(ctx context.Context, userID string) error
	ListByUser(ctx context.Context, userID string, limit int) ([]model.ConnectionLog, error)
	CountActiveByUser(ctx context.Context, userID string) (int, error)
}

type AdminRepository interface {
	ListUsers(ctx context.Context, search string, limit, offset int) ([]model.AdminUserRow, int, error)
	GetUserDetail(ctx context.Context, userID string) (*model.AdminUserDetail, error)
	RevokeSubscription(ctx context.Context, userID string) error
}
