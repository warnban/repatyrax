package handler

import (
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"

	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
)

type PaymentHandler struct {
	paymentSvc service.PaymentService
	inviteSvc  service.InviteService
	deviceRepo repository.DeviceRepository
	userRepo   repository.UserRepository
}

func NewPaymentHandler(
	paymentSvc service.PaymentService,
	inviteSvc service.InviteService,
	deviceRepo repository.DeviceRepository,
	userRepo repository.UserRepository,
) *PaymentHandler {
	return &PaymentHandler{
		paymentSvc: paymentSvc,
		inviteSvc:  inviteSvc,
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
	}
}

// POST /api/v1/payment/create
func (h *PaymentHandler) CreatePayment(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	var req struct {
		Tier          string `json:"tier"`
		PaymentMethod string `json:"payment_method"`
		Months        int    `json:"months"`
		Email         string `json:"email"`
		IP            string `json:"ip"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	if req.Tier == "" || req.PaymentMethod == "" || req.Email == "" || req.Months == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	if req.IP == "" {
		req.IP = c.IP()
	}

	result, err := h.paymentSvc.CreateOrder(c.Context(), userID, req.Tier, req.PaymentMethod, req.Months, req.Email, req.IP)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "PAYMENT CREATION FAILED"})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   result,
	})
}

// GET /api/v1/payment/status/:orderID
func (h *PaymentHandler) GetPaymentStatus(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	orderID := c.Params("orderID")
	order, err := h.paymentSvc.GetOrder(c.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "ORDER NOT FOUND"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "INTERNAL ERROR"})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"order_status": string(order.Status),
			"tier":         order.Tier,
		},
	})
}

// GET /api/v1/subscription
func (h *PaymentHandler) GetSubscription(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "INTERNAL ERROR"})
	}

	deviceCount, err := h.deviceRepo.CountByUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "INTERNAL ERROR"})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"tier":          string(user.SubscriptionTier),
			"ends_at":       user.SubscriptionEnd,
			"devices_count": deviceCount,
			"devices_limit": service.DeviceLimit(user.SubscriptionTier),
		},
	})
}

// POST /webhooks/freekassa  (no JWT)
func (h *PaymentHandler) FreekassaWebhook(c *fiber.Ctx) error {
	params := map[string]string{
		"MERCHANT_ID":       c.FormValue("MERCHANT_ID"),
		"AMOUNT":            c.FormValue("AMOUNT"),
		"intid":             c.FormValue("intid"),
		"MERCHANT_ORDER_ID": c.FormValue("MERCHANT_ORDER_ID"),
		"P_EMAIL":           c.FormValue("P_EMAIL"),
		"CUR_ID":            c.FormValue("CUR_ID"),
		"SIGN":              c.FormValue("SIGN"),
	}

	remoteIP := c.IP()
	if err := h.paymentSvc.HandleFreekassaWebhook(c.Context(), params, remoteIP); err != nil {
		// FreeKassa retries on non-"YES" response; log but return YES anyway
		// for already-paid (idempotent) cases — that is handled inside the service.
		return c.Status(fiber.StatusForbidden).SendString(err.Error())
	}

	return c.SendString("YES")
}

// POST /webhooks/crypto-pay  (no JWT)
func (h *PaymentHandler) CryptoPayWebhook(c *fiber.Ctx) error {
	body, err := io.ReadAll(c.Request().BodyStream())
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	signature := c.Get("crypto-pay-api-signature")
	if err := h.paymentSvc.HandleCryptoPayWebhook(c.Context(), body, signature); err != nil {
		return c.SendStatus(fiber.StatusForbidden)
	}

	return c.SendStatus(fiber.StatusOK)
}

// GET /api/v1/subscription/invites
// Lists pending + accepted invites sent by the current (DOMINION) owner.
func (h *PaymentHandler) GetInvites(c *fiber.Ctx) error {
	ownerID := extractUserID(c)
	if ownerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	invites, err := h.inviteSvc.ListByOwner(c.Context(), ownerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "INTERNAL ERROR"})
	}

	// model.Invite carries no JSON tags — emit the client contract explicitly.
	out := make([]fiber.Map, 0, len(invites))
	for _, inv := range invites {
		out = append(out, fiber.Map{
			"id":         inv.ID,
			"invitee_id": inv.InviteeID,
			"status":     inv.Status,
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   out,
	})
}

// POST /api/v1/subscription/invite
func (h *PaymentHandler) SendInvite(c *fiber.Ctx) error {
	ownerID := extractUserID(c)
	if ownerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.AccountID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	if err := h.inviteSvc.SendInvite(c.Context(), ownerID, req.AccountID); err != nil {
		if errors.Is(err, service.ErrNotDominion) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "DOMINION TIER REQUIRED"})
		}
		if errors.Is(err, service.ErrInviteLimitHit) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "INVITE LIMIT REACHED"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// DELETE /api/v1/subscription/invite/:accountID
func (h *PaymentHandler) RemoveInvite(c *fiber.Ctx) error {
	ownerID := extractUserID(c)
	if ownerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	inviteeID := c.Params("accountID")
	if err := h.inviteSvc.RemoveInvite(c.Context(), ownerID, inviteeID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// POST /api/v1/subscription/invite/accept
func (h *PaymentHandler) AcceptInvite(c *fiber.Ctx) error {
	inviteeID := extractUserID(c)
	if inviteeID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	var req struct {
		InviteID string `json:"invite_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.InviteID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	if err := h.inviteSvc.AcceptInvite(c.Context(), inviteeID, req.InviteID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// POST /api/v1/subscription/invite/leave
func (h *PaymentHandler) LeaveInvite(c *fiber.Ctx) error {
	inviteeID := extractUserID(c)
	if inviteeID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	if err := h.inviteSvc.LeaveInvite(c.Context(), inviteeID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}
