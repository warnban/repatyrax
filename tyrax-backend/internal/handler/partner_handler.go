package handler

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/tyrax/tyrax-backend/internal/config"
	"github.com/tyrax/tyrax-backend/internal/middleware"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
)

const partnerTokenTTL = 7 * 24 * time.Hour

type PartnerHandler struct {
	cfg        *config.Config
	partnerSvc service.PartnerService
}

func NewPartnerHandler(cfg *config.Config, partnerSvc service.PartnerService) *PartnerHandler {
	return &PartnerHandler{cfg: cfg, partnerSvc: partnerSvc}
}

type partnerRegisterRequest struct {
	InviteToken string `json:"invite_token"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type partnerLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type payoutDetailsRequest struct {
	Method      string `json:"method"`
	MIRCard     string `json:"mir_card"`
	USDTAddress string `json:"usdt_address"`
	USDTNetwork string `json:"usdt_network"`
}

// ValidateInvite — GET /api/v1/partner/invites/:token
func (h *PartnerHandler) ValidateInvite(c *fiber.Ctx) error {
	if err := h.partnerSvc.ValidateInvite(c.Context(), c.Params("token")); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "INVITE INVALID"})
	}
	return c.JSON(fiber.Map{"status": "ok", "data": fiber.Map{"valid": true}})
}

// Register — POST /api/v1/partner/auth/register
func (h *PartnerHandler) Register(c *fiber.Ctx) error {
	var req partnerRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
	}
	if req.InviteToken == "" {
		req.InviteToken = strings.TrimSpace(c.Query("invite"))
	}
	p, err := h.partnerSvc.Register(c.Context(), req.InviteToken, req.Email, req.Password, req.DisplayName)
	if err != nil {
		if errors.Is(err, repository.ErrPartnerInviteInvalid) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVITE INVALID"})
		}
		if errors.Is(err, repository.ErrPartnerEmailTaken) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "EMAIL TAKEN"})
		}
		return partnerErr(c, err)
	}
	token, err := h.signPartnerToken(p.ID, p.Email)
	if err != nil {
		return partnerErr(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{"token": token, "partner_id": p.ID},
	})
}

// Login — POST /api/v1/partner/auth/login
func (h *PartnerHandler) Login(c *fiber.Ctx) error {
	var req partnerLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
	}
	p, err := h.partnerSvc.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	token, err := h.signPartnerToken(p.ID, p.Email)
	if err != nil {
		return partnerErr(c, err)
	}
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{"token": token, "partner_id": p.ID},
	})
}

// Dashboard — GET /api/v1/partner/dashboard
func (h *PartnerHandler) Dashboard(c *fiber.Ctx) error {
	partnerID, _ := c.Locals("partner_id").(string)
	data, err := h.partnerSvc.GetDashboard(c.Context(), partnerID, h.cfg.TelegramBotUsername)
	if err != nil {
		return partnerErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok", "data": data})
}

// UpdatePayoutDetails — PUT /api/v1/partner/payout-details
func (h *PartnerHandler) UpdatePayoutDetails(c *fiber.Ctx) error {
	var req payoutDetailsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	partnerID, _ := c.Locals("partner_id").(string)
	if err := h.partnerSvc.UpdatePayoutDetails(c.Context(), partnerID, req.Method, req.MIRCard, req.USDTAddress, req.USDTNetwork); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// ListPayouts — GET /api/v1/partner/payouts
func (h *PartnerHandler) ListPayouts(c *fiber.Ctx) error {
	partnerID, _ := c.Locals("partner_id").(string)
	payouts, err := h.partnerSvc.ListPayouts(c.Context(), partnerID)
	if err != nil {
		return partnerErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok", "data": fiber.Map{"payouts": payouts}})
}

func (h *PartnerHandler) signPartnerToken(partnerID, email string) (string, error) {
	claims := middleware.PartnerClaims{
		Role:      middleware.PartnerRole,
		PartnerID: partnerID,
		Email:     email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(partnerTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.PartnerJWTSecret()))
}

func partnerErr(c *fiber.Ctx, err error) error {
	slog.Error("partner handler", slog.String("error", err.Error()))
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
}

// ── Admin partner endpoints (methods on AdminHandler) ───────────────────────

type partnerSettingsRequest struct {
	DefaultCommissionRate float64 `json:"default_commission_rate"`
}

type partnerOverrideRequest struct {
	CommissionRateOverride *float64 `json:"commission_rate_override"`
}

type partnerPayoutRequest struct {
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
}

func (h *AdminHandler) ListPartners(c *fiber.Ctx) error {
	if h.partnerSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "message": "PARTNERS DISABLED"})
	}
	rows, err := h.partnerSvc.ListPartnersAdmin(c.Context())
	if err != nil {
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok", "data": fiber.Map{"partners": rows}})
}

func (h *AdminHandler) GetPartnerSettings(c *fiber.Ctx) error {
	s, err := h.partnerSvc.GetSettings(c.Context())
	if err != nil {
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok", "data": s})
}

func (h *AdminHandler) UpdatePartnerSettings(c *fiber.Ctx) error {
	var req partnerSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	if err := h.partnerSvc.UpdateSettings(c.Context(), req.DefaultCommissionRate); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AdminHandler) CreatePartnerInvite(c *fiber.Ctx) error {
	token, err := h.partnerSvc.CreateInvite(c.Context())
	if err != nil {
		return adminErr(c, err)
	}
	base := strings.TrimRight(h.cfg.PartnerPortalURL, "/")
	link := base + "/register?invite=" + token
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{"token": token, "invite_link": link},
	})
}

func (h *AdminHandler) GetPartner(c *fiber.Ctx) error {
	row, err := h.partnerSvc.GetPartnerAdmin(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, repository.ErrPartnerNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "PARTNER NOT FOUND"})
		}
		return adminErr(c, err)
	}
	payouts, _ := h.partnerSvc.ListPayouts(c.Context(), row.ID)
	return c.JSON(fiber.Map{"status": "ok", "data": fiber.Map{"partner": row, "payouts": payouts}})
}

func (h *AdminHandler) UpdatePartnerOverride(c *fiber.Ctx) error {
	var req partnerOverrideRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	if err := h.partnerSvc.UpdatePartnerOverride(c.Context(), c.Params("id"), req.CommissionRateOverride); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AdminHandler) RecordPartnerPayout(c *fiber.Ctx) error {
	var req partnerPayoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	adminUser, _ := c.Locals("admin_username").(string)
	if err := h.partnerSvc.RecordPayout(c.Context(), c.Params("id"), req.Amount, req.Note, adminUser); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
