package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
)

type AdminService interface {
	GrantSubscription(ctx context.Context, userID, tier string, period model.GrantPeriod) (*model.User, error)
	RevokeSubscription(ctx context.Context, userID string) error
}

type adminService struct {
	userRepo repository.UserRepository
	adminRepo repository.AdminRepository
}

func NewAdminService(userRepo repository.UserRepository, adminRepo repository.AdminRepository) AdminService {
	return &adminService{userRepo: userRepo, adminRepo: adminRepo}
}

func (s *adminService) GrantSubscription(ctx context.Context, userID, tier string, period model.GrantPeriod) (*model.User, error) {
	if !model.ValidGrantPeriod(string(period)) {
		return nil, errors.New("INVALID PERIOD")
	}
	switch model.SubscriptionTier(tier) {
	case model.TierCore, model.TierShadow, model.TierDominion:
	default:
		return nil, errors.New("INVALID TIER")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	base := time.Now()
	if user.SubscriptionEnd != nil && user.SubscriptionEnd.After(base) &&
		user.SubscriptionTier != model.TierFree {
		base = *user.SubscriptionEnd
	}

	endsAt := period.DurationFrom(base)
	if err := s.userRepo.ActivateSubscription(ctx, userID, tier, endsAt); err != nil {
		return nil, fmt.Errorf("grant subscription: %w", err)
	}
	return s.userRepo.FindByID(ctx, userID)
}

func (s *adminService) RevokeSubscription(ctx context.Context, userID string) error {
	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		return err
	}
	return s.adminRepo.RevokeSubscription(ctx, userID)
}
