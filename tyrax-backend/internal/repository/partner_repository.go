package repository

import (
	"context"
	"errors"

	"github.com/tyrax/tyrax-backend/internal/model"
)

var (
	ErrPartnerNotFound      = errors.New("PARTNER NOT FOUND")
	ErrPartnerInviteInvalid = errors.New("INVITE INVALID")
	ErrPartnerEmailTaken    = errors.New("EMAIL TAKEN")
)

type PartnerRepository interface {
	GetSettings(ctx context.Context) (*model.PartnerSettings, error)
	UpdateSettings(ctx context.Context, rate float64) error

	CreateInvite(ctx context.Context, token string) error
	GetInvite(ctx context.Context, token string) (*model.PartnerInvite, error)
	MarkInviteUsed(ctx context.Context, token, partnerID string) error

	CreatePartner(ctx context.Context, p *model.Partner) (*model.Partner, error)
	FindByID(ctx context.Context, id string) (*model.Partner, error)
	FindByEmail(ctx context.Context, email string) (*model.Partner, error)
	FindByRefCode(ctx context.Context, refCode string) (*model.Partner, error)
	ListPartners(ctx context.Context) ([]model.Partner, error)
	UpdatePartnerOverride(ctx context.Context, id string, rate *float64) error
	UpdatePartnerStatus(ctx context.Context, id, status string) error
	UpdatePayoutDetails(ctx context.Context, id string, method, mirCard, usdtAddr, usdtNet string) error

	SetUserReferral(ctx context.Context, userID, partnerID string) error
	CountReferrals(ctx context.Context, partnerID string) (int, error)
	CountActiveReferrals(ctx context.Context, partnerID string) (int, error)
	CountConversions(ctx context.Context, partnerID string) (int, error)

	CreateCommission(ctx context.Context, c *model.PartnerCommission) error
	FindCommissionByOrder(ctx context.Context, orderID string) (*model.PartnerCommission, error)
	ReleaseHeldCommissions(ctx context.Context) (int, error)
	ClawbackCommission(ctx context.Context, orderID string) error

	RecordPayout(ctx context.Context, partnerID string, amount float64, note, adminUser string) error
	ListPayouts(ctx context.Context, partnerID string) ([]model.PartnerPayout, error)
}
