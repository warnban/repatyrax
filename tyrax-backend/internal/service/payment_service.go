package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/pkg/cryptopay"
	"github.com/tyrax/tyrax-backend/pkg/freekassa"
)

type CreateOrderResult struct {
	OrderID    string  `json:"order_id"`
	PaymentURL string  `json:"payment_url"`
	AmountRUB  float64 `json:"amount_rub"`
}

type PaymentService interface {
	CreateOrder(ctx context.Context, userID, tier, paymentMethod string, months int, email, ip string) (*CreateOrderResult, error)
	GetOrder(ctx context.Context, userID, orderID string) (*model.Order, error)
	HandleFreekassaWebhook(ctx context.Context, params map[string]string, remoteIP string) error
	HandleCryptoPayWebhook(ctx context.Context, body []byte, signature string) error
	ActivateSubscription(ctx context.Context, userID, tier string, months int) error
}

type paymentService struct {
	orderRepo   repository.OrderRepository
	userRepo    repository.UserRepository
	freekassa   *freekassa.Client
	cryptopay   *cryptopay.Client
	partnerSvc  PartnerService
}

func NewPaymentService(
	orderRepo repository.OrderRepository,
	userRepo repository.UserRepository,
	fk *freekassa.Client,
	cp *cryptopay.Client,
	partnerSvc PartnerService,
) PaymentService {
	return &paymentService{
		orderRepo:  orderRepo,
		userRepo:   userRepo,
		freekassa:  fk,
		cryptopay:  cp,
		partnerSvc: partnerSvc,
	}
}

func (s *paymentService) CreateOrder(ctx context.Context, userID, tier, paymentMethod string, months int, email, ip string) (*CreateOrderResult, error) {
	amount := CalculatePrice(tier, months)
	if amount == 0 {
		return nil, errors.New("INVALID TIER OR PERIOD")
	}

	order := &model.Order{
		UserID:        userID,
		Tier:          tier,
		Months:        months,
		AmountRUB:     amount,
		PaymentMethod: model.PaymentMethod(paymentMethod),
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order record: %w", err)
	}

	var (
		paymentURL      string
		externalOrderID string
	)

	switch model.PaymentMethod(paymentMethod) {
	case model.PaymentSBP:
		resp, err := s.freekassa.CreateOrder(ctx, freekassa.MethodSBP, email, ip, amount, order.ID)
		if err != nil {
			return nil, err
		}
		paymentURL = resp.Location
		externalOrderID = fmt.Sprintf("%d", resp.OrderID)

	case model.PaymentCardRF:
		resp, err := s.freekassa.CreateOrder(ctx, freekassa.MethodCardRF, email, ip, amount, order.ID)
		if err != nil {
			return nil, err
		}
		paymentURL = resp.Location
		externalOrderID = fmt.Sprintf("%d", resp.OrderID)

	case model.PaymentCrypto:
		inv, err := s.cryptopay.CreateInvoice(ctx, amount, tier, userID, order.ID)
		if err != nil {
			return nil, err
		}
		paymentURL = inv.BotInvoiceURL
		externalOrderID = fmt.Sprintf("%d", inv.InvoiceID)

	default:
		return nil, errors.New("INVALID PAYMENT METHOD")
	}

	if err := s.orderRepo.SetExternalID(ctx, order.ID, externalOrderID); err != nil {
		return nil, fmt.Errorf("set external id: %w", err)
	}

	return &CreateOrderResult{
		OrderID:    order.ID,
		PaymentURL: paymentURL,
		AmountRUB:  amount,
	}, nil
}

func (s *paymentService) GetOrder(ctx context.Context, userID, orderID string) (*model.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, errors.New("ACCESS DENIED")
	}
	return order, nil
}

func (s *paymentService) HandleFreekassaWebhook(ctx context.Context, params map[string]string, remoteIP string) error {
	if !freekassa.IsTrustedIP(remoteIP) {
		return errors.New("ACCESS DENIED")
	}

	merchantID := params["MERCHANT_ID"]
	amount := params["AMOUNT"]
	orderID := params["MERCHANT_ORDER_ID"]
	sign := params["SIGN"]

	if !s.freekassa.VerifyWebhook(merchantID, amount, orderID, sign) {
		return errors.New("INVALID SIGNATURE")
	}

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return errors.New("ORDER NOT FOUND")
		}
		return fmt.Errorf("find order: %w", err)
	}

	// Idempotency: skip if already processed.
	if order.Status == model.OrderPaid {
		return nil
	}

	if err := s.orderRepo.MarkPaid(ctx, order.ID); err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}

	slog.Info("webhook payment confirmed",
		slog.String("provider", "freekassa"),
		slog.String("order_id", order.ID),
		slog.String("user_id", order.UserID),
		slog.String("tier", order.Tier),
		slog.Int("months", order.Months),
		slog.Float64("amount_rub", order.AmountRUB),
		slog.String("reported_amount", amount),
	)

	order.Status = model.OrderPaid
	if s.partnerSvc != nil {
		if err := s.partnerSvc.ProcessFirstPaidOrder(ctx, order); err != nil {
			slog.Error("partner commission", slog.String("order_id", order.ID), slog.String("error", err.Error()))
		}
	}

	return s.ActivateSubscription(ctx, order.UserID, order.Tier, order.Months)
}

func (s *paymentService) HandleCryptoPayWebhook(ctx context.Context, body []byte, signature string) error {
	if !s.cryptopay.VerifyWebhook(body, signature) {
		return errors.New("INVALID SIGNATURE")
	}

	// Minimal parse to get update_type and payload.
	var webhook struct {
		UpdateType string `json:"update_type"`
		Payload    struct {
			Payload string `json:"payload"` // "userID|orderID"
		} `json:"payload"`
	}
	if err := parseJSON(body, &webhook); err != nil {
		return fmt.Errorf("parse webhook body: %w", err)
	}

	if webhook.UpdateType != "invoice_paid" {
		return nil // not an event we act on
	}

	parts := strings.SplitN(webhook.Payload.Payload, "|", 2)
	if len(parts) != 2 {
		return errors.New("INVALID PAYLOAD")
	}
	userID, orderID := parts[0], parts[1]

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("find order: %w", err)
	}
	if order.UserID != userID {
		return errors.New("PAYLOAD MISMATCH")
	}

	if order.Status == model.OrderPaid {
		return nil
	}

	if err := s.orderRepo.MarkPaid(ctx, order.ID); err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}

	slog.Info("webhook payment confirmed",
		slog.String("provider", "crypto-pay"),
		slog.String("order_id", order.ID),
		slog.String("user_id", order.UserID),
		slog.String("tier", order.Tier),
		slog.Int("months", order.Months),
		slog.Float64("amount_rub", order.AmountRUB),
	)

	order.Status = model.OrderPaid
	if s.partnerSvc != nil {
		if err := s.partnerSvc.ProcessFirstPaidOrder(ctx, order); err != nil {
			slog.Error("partner commission", slog.String("order_id", order.ID), slog.String("error", err.Error()))
		}
	}

	return s.ActivateSubscription(ctx, order.UserID, order.Tier, order.Months)
}

func (s *paymentService) ActivateSubscription(ctx context.Context, userID, tier string, months int) error {
	endsAt := time.Now().UTC().AddDate(0, months, 0)
	if err := s.userRepo.ActivateSubscription(ctx, userID, tier, endsAt); err != nil {
		return fmt.Errorf("activate subscription: %w", err)
	}
	return nil
}
