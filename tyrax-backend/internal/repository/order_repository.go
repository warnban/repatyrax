package repository

import (
	"context"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) error
	FindByID(ctx context.Context, id string) (*model.Order, error)
	FindByExternalID(ctx context.Context, externalID string) (*model.Order, error)
	SetExternalID(ctx context.Context, orderID, externalID string) error
	MarkPaid(ctx context.Context, orderID string) error
	MarkRefunded(ctx context.Context, orderID string) error
	CountPaidOrdersByUser(ctx context.Context, userID string) (int, error)
}
