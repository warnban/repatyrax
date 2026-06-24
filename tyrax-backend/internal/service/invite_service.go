package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
)

const maxInvitesPerDominion = 3

var (
	ErrNotDominion    = errors.New("DOMINION TIER REQUIRED")
	ErrInviteLimitHit = errors.New("INVITE LIMIT REACHED")
)

type InviteService interface {
	SendInvite(ctx context.Context, ownerID, inviteeAccountID string) error
	AcceptInvite(ctx context.Context, inviteeID, inviteID string) error
	RemoveInvite(ctx context.Context, ownerID, inviteeID string) error
	LeaveInvite(ctx context.Context, inviteeID string) error
	ListByOwner(ctx context.Context, ownerID string) ([]model.Invite, error)
}

type inviteService struct {
	userRepo   repository.UserRepository
	inviteRepo repository.InviteRepository
}

func NewInviteService(userRepo repository.UserRepository, inviteRepo repository.InviteRepository) InviteService {
	return &inviteService{userRepo: userRepo, inviteRepo: inviteRepo}
}

func (s *inviteService) SendInvite(ctx context.Context, ownerID, inviteeAccountID string) error {
	owner, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("find owner: %w", err)
	}
	if owner.SubscriptionTier != model.TierDominion {
		return ErrNotDominion
	}

	count, err := s.inviteRepo.CountActiveByOwner(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("count invites: %w", err)
	}
	if count >= maxInvitesPerDominion {
		return ErrInviteLimitHit
	}

	// Verify invitee exists.
	if _, err := s.userRepo.FindByID(ctx, inviteeAccountID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return errors.New("IDENTITY NOT FOUND")
		}
		return fmt.Errorf("find invitee: %w", err)
	}

	invite := &model.Invite{
		OwnerID:   ownerID,
		InviteeID: inviteeAccountID,
	}
	return s.inviteRepo.Create(ctx, invite)
}

func (s *inviteService) AcceptInvite(ctx context.Context, inviteeID, inviteID string) error {
	invite, err := s.inviteRepo.FindPendingByInvitee(ctx, inviteeID, inviteID)
	if err != nil {
		if errors.Is(err, repository.ErrInviteNotFound) {
			return errors.New("INVITE NOT FOUND")
		}
		return fmt.Errorf("find invite: %w", err)
	}

	if err := s.inviteRepo.Accept(ctx, invite.ID); err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}

	ownerID := invite.OwnerID
	return s.userRepo.SetParentSubscription(ctx, inviteeID, &ownerID)
}

func (s *inviteService) RemoveInvite(ctx context.Context, ownerID, inviteeID string) error {
	if err := s.inviteRepo.Delete(ctx, ownerID, inviteeID); err != nil {
		return fmt.Errorf("delete invite: %w", err)
	}
	return s.userRepo.SetParentSubscription(ctx, inviteeID, nil)
}

func (s *inviteService) LeaveInvite(ctx context.Context, inviteeID string) error {
	return s.userRepo.SetParentSubscription(ctx, inviteeID, nil)
}

func (s *inviteService) ListByOwner(ctx context.Context, ownerID string) ([]model.Invite, error) {
	return s.inviteRepo.GetByOwnerID(ctx, ownerID)
}
