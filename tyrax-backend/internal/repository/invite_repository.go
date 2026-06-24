package repository

import (
	"context"

	"github.com/tyrax/tyrax-backend/internal/model"
)

type InviteRepository interface {
	Create(ctx context.Context, invite *model.Invite) error
	FindPendingByInvitee(ctx context.Context, inviteeID, inviteID string) (*model.Invite, error)
	CountActiveByOwner(ctx context.Context, ownerID string) (int, error)
	GetByOwnerID(ctx context.Context, ownerID string) ([]model.Invite, error)
	Accept(ctx context.Context, inviteID string) error
	Delete(ctx context.Context, ownerID, inviteeID string) error
}
