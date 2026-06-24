package repository

import (
	"context"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type DeviceRepository interface {
	Create(ctx context.Context, device *model.Device) error
	CountByUser(ctx context.Context, userID string) (int, error)
	GetAllClientIPs(ctx context.Context) ([]string, error)
	GetByUserID(ctx context.Context, userID string) ([]model.Device, error)
	FindByPublicKey(ctx context.Context, publicKey string) (*model.Device, error)
	FindByUserAndName(ctx context.Context, userID, name string) (*model.Device, error)
	Delete(ctx context.Context, id, userID string) error
}
