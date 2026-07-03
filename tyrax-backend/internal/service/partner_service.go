package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
)

type PartnerService interface {
	RunLoop(ctx context.Context)
	CreateInvite(ctx context.Context) (token string, err error)
	ValidateInvite(ctx context.Context, token string) error
	Register(ctx context.Context, inviteToken, email, password, displayName string) (*model.Partner, error)
	Login(ctx context.Context, email, password string) (*model.Partner, error)
	GetDashboard(ctx context.Context, partnerID, botUsername string) (map[string]interface{}, error)
	UpdatePayoutDetails(ctx context.Context, partnerID, method, mirCard, usdtAddr, usdtNet string) error
	ListPayouts(ctx context.Context, partnerID string) ([]model.PartnerPayout, error)

	GetSettings(ctx context.Context) (*model.PartnerSettings, error)
	UpdateSettings(ctx context.Context, rate float64) error
	ListPartnersAdmin(ctx context.Context) ([]model.PartnerAdminRow, error)
	GetPartnerAdmin(ctx context.Context, id string) (*model.PartnerAdminRow, error)
	UpdatePartnerOverride(ctx context.Context, id string, rate *float64) error
	RecordPayout(ctx context.Context, partnerID string, amount float64, note, adminUser string) error

	AttributeReferral(ctx context.Context, userID, refCode string) error
	ProcessFirstPaidOrder(ctx context.Context, order *model.Order) error
	ProcessOrderRefund(ctx context.Context, orderID string) error
}

type partnerService struct {
	partnerRepo repository.PartnerRepository
	userRepo    repository.UserRepository
	orderRepo   repository.OrderRepository
}

func NewPartnerService(
	partnerRepo repository.PartnerRepository,
	userRepo repository.UserRepository,
	orderRepo repository.OrderRepository,
) PartnerService {
	return &partnerService{
		partnerRepo: partnerRepo,
		userRepo:    userRepo,
		orderRepo:   orderRepo,
	}
}

func (s *partnerService) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.partnerRepo.ReleaseHeldCommissions(ctx)
			if err != nil {
				slog.Error("partner: release held commissions", slog.String("error", err.Error()))
			} else if n > 0 {
				slog.Info("partner: released held commissions", slog.Int("count", n))
			}
		}
	}
}

func (s *partnerService) CreateInvite(ctx context.Context) (string, error) {
	token, err := randomToken(24)
	if err != nil {
		return "", err
	}
	if err := s.partnerRepo.CreateInvite(ctx, token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *partnerService) ValidateInvite(ctx context.Context, token string) error {
	inv, err := s.partnerRepo.GetInvite(ctx, token)
	if err != nil {
		return err
	}
	if inv.UsedAt != nil || time.Now().After(inv.ExpiresAt) {
		return repository.ErrPartnerInviteInvalid
	}
	return nil
}

func (s *partnerService) Register(ctx context.Context, inviteToken, email, password, displayName string) (*model.Partner, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if email == "" || password == "" || displayName == "" {
		return nil, errors.New("INVALID CREDENTIALS")
	}
	if err := s.ValidateInvite(ctx, inviteToken); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	refCode, err := randomRefCode(8)
	if err != nil {
		return nil, err
	}

	p := &model.Partner{
		Email:        email,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		RefCode:      refCode,
		Status:       model.PartnerStatusActive,
	}
	created, err := s.partnerRepo.CreatePartner(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := s.partnerRepo.MarkInviteUsed(ctx, inviteToken, created.ID); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *partnerService) Login(ctx context.Context, email, password string) (*model.Partner, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	p, err := s.partnerRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("INVALID CREDENTIALS")
	}
	if p.Status != model.PartnerStatusActive {
		return nil, errors.New("ACCESS DENIED")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("INVALID CREDENTIALS")
	}
	return p, nil
}

func (s *partnerService) GetDashboard(ctx context.Context, partnerID, botUsername string) (map[string]interface{}, error) {
	p, err := s.partnerRepo.FindByID(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	stats, err := s.partnerStats(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	refLink := fmt.Sprintf("https://t.me/%s?start=ref_%s", botUsername, p.RefCode)
	return map[string]interface{}{
		"partner": map[string]interface{}{
			"id":                       p.ID,
			"email":                    p.Email,
			"display_name":             p.DisplayName,
			"ref_code":                 p.RefCode,
			"ref_link":                 refLink,
			"balance_available":        p.BalanceAvailable,
			"balance_hold":             p.BalanceHold,
			"total_paid_out":           p.TotalPaidOut,
			"payout_method":            p.PayoutMethod,
			"payout_mir_card":          p.PayoutMIRCard,
			"payout_usdt_address":      p.PayoutUSDTAddress,
			"payout_usdt_network":      p.PayoutUSDTNetwork,
			"min_payout_rub":           model.MinPayoutRUB,
			"min_payout_usdt":          model.MinPayoutUSDT,
			"hold_days":                model.CommissionHoldDays,
			"conversion_window_days":   model.ReferralConversionDays,
		},
		"stats": stats,
	}, nil
}

func (s *partnerService) UpdatePayoutDetails(ctx context.Context, partnerID, method, mirCard, usdtAddr, usdtNet string) error {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case model.PartnerPayoutMIR:
		mirCard = strings.TrimSpace(mirCard)
		if mirCard == "" {
			return errors.New("CARD REQUIRED")
		}
		return s.partnerRepo.UpdatePayoutDetails(ctx, partnerID, method, mirCard, "", "")
	case model.PartnerPayoutUSDT:
		usdtAddr = strings.TrimSpace(usdtAddr)
		usdtNet = strings.TrimSpace(usdtNet)
		if usdtAddr == "" || usdtNet == "" {
			return errors.New("USDT DETAILS REQUIRED")
		}
		return s.partnerRepo.UpdatePayoutDetails(ctx, partnerID, method, "", usdtAddr, usdtNet)
	default:
		return errors.New("INVALID PAYOUT METHOD")
	}
}

func (s *partnerService) ListPayouts(ctx context.Context, partnerID string) ([]model.PartnerPayout, error) {
	return s.partnerRepo.ListPayouts(ctx, partnerID)
}

func (s *partnerService) GetSettings(ctx context.Context) (*model.PartnerSettings, error) {
	return s.partnerRepo.GetSettings(ctx)
}

func (s *partnerService) UpdateSettings(ctx context.Context, rate float64) error {
	if rate < 0 || rate > 100 {
		return errors.New("INVALID RATE")
	}
	return s.partnerRepo.UpdateSettings(ctx, rate)
}

func (s *partnerService) ListPartnersAdmin(ctx context.Context) ([]model.PartnerAdminRow, error) {
	partners, err := s.partnerRepo.ListPartners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.PartnerAdminRow, 0, len(partners))
	for _, p := range partners {
		stats, err := s.partnerStats(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, model.PartnerAdminRow{Partner: p, Stats: stats})
	}
	return out, nil
}

func (s *partnerService) GetPartnerAdmin(ctx context.Context, id string) (*model.PartnerAdminRow, error) {
	p, err := s.partnerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	stats, err := s.partnerStats(ctx, id)
	if err != nil {
		return nil, err
	}
	payouts, err := s.partnerRepo.ListPayouts(ctx, id)
	if err != nil {
		return nil, err
	}
	row := &model.PartnerAdminRow{Partner: *p, Stats: stats}
	_ = payouts
	return row, nil
}

func (s *partnerService) UpdatePartnerOverride(ctx context.Context, id string, rate *float64) error {
	if rate != nil && (*rate < 0 || *rate > 100) {
		return errors.New("INVALID RATE")
	}
	return s.partnerRepo.UpdatePartnerOverride(ctx, id, rate)
}

func (s *partnerService) RecordPayout(ctx context.Context, partnerID string, amount float64, note, adminUser string) error {
	if amount < model.MinPayoutRUB {
		return fmt.Errorf("MINIMUM PAYOUT %d RUB", model.MinPayoutRUB)
	}
	p, err := s.partnerRepo.FindByID(ctx, partnerID)
	if err != nil {
		return err
	}
	if amount > p.BalanceAvailable {
		return errors.New("INSUFFICIENT BALANCE")
	}
	return s.partnerRepo.RecordPayout(ctx, partnerID, amount, note, adminUser)
}

func (s *partnerService) AttributeReferral(ctx context.Context, userID, refCode string) error {
	refCode = strings.TrimSpace(refCode)
	if refCode == "" {
		return nil
	}
	partner, err := s.partnerRepo.FindByRefCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, repository.ErrPartnerNotFound) {
			return nil
		}
		return err
	}
	return s.partnerRepo.SetUserReferral(ctx, userID, partner.ID)
}

func (s *partnerService) ProcessFirstPaidOrder(ctx context.Context, order *model.Order) error {
	if order.Status != model.OrderPaid {
		return nil
	}
	switch model.SubscriptionTier(order.Tier) {
	case model.TierCore, model.TierShadow, model.TierDominion:
	default:
		return nil
	}

	paidCount, err := s.orderRepo.CountPaidOrdersByUser(ctx, order.UserID)
	if err != nil {
		return err
	}
	if paidCount != 1 {
		return nil
	}

	user, err := s.userRepo.FindByID(ctx, order.UserID)
	if err != nil {
		return err
	}
	if user.ReferredByPartnerID == nil || *user.ReferredByPartnerID == "" {
		return nil
	}

	deadline := user.CreatedAt.AddDate(0, 0, model.ReferralConversionDays)
	if time.Now().UTC().After(deadline) {
		return nil
	}

	existing, err := s.partnerRepo.FindCommissionByOrder(ctx, order.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	partner, err := s.partnerRepo.FindByID(ctx, *user.ReferredByPartnerID)
	if err != nil {
		return err
	}
	if partner.Status != model.PartnerStatusActive {
		return nil
	}

	settings, err := s.partnerRepo.GetSettings(ctx)
	if err != nil {
		return err
	}
	rate := settings.DefaultCommissionRate
	if partner.CommissionRateOverride != nil {
		rate = *partner.CommissionRateOverride
	}

	amount := math.Round(order.AmountRUB*rate/100*100) / 100
	if amount <= 0 {
		return nil
	}

	holdUntil := time.Now().UTC().AddDate(0, 0, model.CommissionHoldDays)
	commission := &model.PartnerCommission{
		PartnerID:           partner.ID,
		UserID:              user.ID,
		OrderID:             order.ID,
		OrderAmountRUB:      order.AmountRUB,
		CommissionRate:      rate,
		CommissionAmountRUB: amount,
		Status:              model.CommissionHold,
		HoldUntil:           holdUntil,
	}
	if err := s.partnerRepo.CreateCommission(ctx, commission); err != nil {
		return err
	}

	slog.Info("partner commission created",
		slog.String("partner_id", partner.ID),
		slog.String("user_id", user.ID),
		slog.String("order_id", order.ID),
		slog.Float64("amount_rub", amount),
		slog.Float64("rate", rate),
	)
	return nil
}

func (s *partnerService) ProcessOrderRefund(ctx context.Context, orderID string) error {
	if err := s.partnerRepo.ClawbackCommission(ctx, orderID); err != nil {
		return err
	}
	return s.orderRepo.MarkRefunded(ctx, orderID)
}

func (s *partnerService) partnerStats(ctx context.Context, partnerID string) (model.PartnerStats, error) {
	regs, err := s.partnerRepo.CountReferrals(ctx, partnerID)
	if err != nil {
		return model.PartnerStats{}, err
	}
	active, err := s.partnerRepo.CountActiveReferrals(ctx, partnerID)
	if err != nil {
		return model.PartnerStats{}, err
	}
	conv, err := s.partnerRepo.CountConversions(ctx, partnerID)
	if err != nil {
		return model.PartnerStats{}, err
	}
	return model.PartnerStats{
		Registrations: regs,
		ActiveUsers:   active,
		Conversions:   conv,
	}, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomRefCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	rb := make([]byte, n)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(rb[i])%len(alphabet)]
	}
	return string(b), nil
}
