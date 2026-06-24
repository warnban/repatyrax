package repository

import (
	"context"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type NodeRepository interface {
	List(ctx context.Context) ([]model.Node, error)
	FindByID(ctx context.Context, id string) (*model.Node, error)
	UpdatePing(ctx context.Context, nodeID string, pingMS int) error
	GetBest(ctx context.Context) (*model.Node, error) // lowest ping, status=OPEN
}
