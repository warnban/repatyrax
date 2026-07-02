package handler

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/tyrax/tyrax-backend/internal/config"
	"github.com/tyrax/tyrax-backend/internal/middleware"
	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/internal/service"
)

const adminTokenTTL = 12 * time.Hour

type SupportMessenger interface {
	SendUserMessage(telegramID int64, text string) error
}

type AdminHandler struct {
	cfg          *config.Config
	adminRepo    repository.AdminRepository
	supportRepo  repository.SupportRepository
	userRepo     repository.UserRepository
	adminSvc     service.AdminService
	messenger    SupportMessenger
}

func NewAdminHandler(
	cfg *config.Config,
	adminRepo repository.AdminRepository,
	supportRepo repository.SupportRepository,
	userRepo repository.UserRepository,
	adminSvc service.AdminService,
	messenger SupportMessenger,
) *AdminHandler {
	return &AdminHandler{
		cfg:         cfg,
		adminRepo:   adminRepo,
		supportRepo: supportRepo,
		userRepo:    userRepo,
		adminSvc:    adminSvc,
		messenger:   messenger,
	}
}

type adminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type grantSubscriptionRequest struct {
	Tier   string `json:"tier"`
	Period string `json:"period"`
}

type ticketReplyRequest struct {
	Body string `json:"body"`
}

// Login — POST /api/v1/admin/auth/login
func (h *AdminHandler) Login(c *fiber.Ctx) error {
	if h.cfg.AdminUsername == "" || h.cfg.AdminPasswordHash == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error", "message": "ADMIN ACCESS DISABLED",
		})
	}

	var req adminLoginRequest
	if err := c.BodyParser(&req); err != nil || req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error", "message": "INVALID CREDENTIALS",
		})
	}

	if req.Username != h.cfg.AdminUsername {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status": "error", "message": "ACCESS DENIED",
		})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(h.cfg.AdminPasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status": "error", "message": "ACCESS DENIED",
		})
	}

	token, err := h.signAdminToken(req.Username)
	if err != nil {
		slog.Error("admin: sign token", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error", "message": "SYSTEM FAILURE",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":    token,
			"username": req.Username,
		},
	})
}

func (h *AdminHandler) signAdminToken(username string) (string, error) {
	claims := middleware.AdminClaims{
		Role:     middleware.AdminRole,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(adminTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.AdminJWTSecret))
}

// Stats — GET /api/v1/admin/stats
func (h *AdminHandler) Stats(c *fiber.Ctx) error {
	users, total, err := h.adminRepo.ListUsers(c.Context(), "", 1, 0)
	if err != nil {
		return adminErr(c, err)
	}
	_ = users
	openTickets, openTotal, err := h.supportRepo.ListTickets(c.Context(), "open", 1, 0)
	if err != nil {
		return adminErr(c, err)
	}
	_ = openTickets
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"users_total":       total,
			"open_tickets":      openTotal,
		},
	})
}

// ListUsers — GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("q"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	users, total, err := h.adminRepo.ListUsers(c.Context(), search, limit, offset)
	if err != nil {
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"users":  users,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetUser — GET /api/v1/admin/users/:id
func (h *AdminHandler) GetUser(c *fiber.Ctx) error {
	user, err := h.adminRepo.GetUserDetail(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "IDENTITY NOT FOUND"})
		}
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok", "data": user})
}

// GrantSubscription — POST /api/v1/admin/users/:id/subscription
func (h *AdminHandler) GrantSubscription(c *fiber.Ctx) error {
	var req grantSubscriptionRequest
	if err := c.BodyParser(&req); err != nil || req.Tier == "" || req.Period == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	user, err := h.adminSvc.GrantSubscription(c.Context(), c.Params("id"), strings.ToUpper(req.Tier), model.GrantPeriod(req.Period))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "IDENTITY NOT FOUND"})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok", "data": user})
}

// RevokeSubscription — DELETE /api/v1/admin/users/:id/subscription
func (h *AdminHandler) RevokeSubscription(c *fiber.Ctx) error {
	if err := h.adminSvc.RevokeSubscription(c.Context(), c.Params("id")); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "IDENTITY NOT FOUND"})
		}
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// ListTickets — GET /api/v1/admin/support/tickets
func (h *AdminHandler) ListTickets(c *fiber.Ctx) error {
	status := strings.TrimSpace(c.Query("status"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	tickets, total, err := h.supportRepo.ListTickets(c.Context(), status, limit, offset)
	if err != nil {
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"tickets": tickets,
			"total":   total,
		},
	})
}

// GetTicket — GET /api/v1/admin/support/tickets/:id
func (h *AdminHandler) GetTicket(c *fiber.Ctx) error {
	ticket, err := h.supportRepo.GetTicket(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "TICKET NOT FOUND"})
	}
	messages, err := h.supportRepo.ListMessages(c.Context(), ticket.ID)
	if err != nil {
		return adminErr(c, err)
	}
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"ticket":   ticket,
			"messages": messages,
		},
	})
}

// ReplyTicket — POST /api/v1/admin/support/tickets/:id/reply
func (h *AdminHandler) ReplyTicket(c *fiber.Ctx) error {
	var req ticketReplyRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	ticket, err := h.supportRepo.GetTicket(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "TICKET NOT FOUND"})
	}
	if ticket.Status != model.TicketOpen {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "TICKET CLOSED"})
	}

	body := strings.TrimSpace(req.Body)
	if _, err := h.supportRepo.AddMessage(c.Context(), ticket.ID, "admin", body); err != nil {
		return adminErr(c, err)
	}

	if h.messenger != nil {
		msg := "▓ TYRAX SUPPORT ▓\n\n" + body
		if err := h.messenger.SendUserMessage(ticket.TelegramID, msg); err != nil {
			slog.Error("admin: send support reply", slog.String("error", err.Error()))
		}
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// CloseTicket — POST /api/v1/admin/support/tickets/:id/close
func (h *AdminHandler) CloseTicket(c *fiber.Ctx) error {
	ticket, err := h.supportRepo.GetTicket(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "TICKET NOT FOUND"})
	}

	if err := h.supportRepo.CloseTicket(c.Context(), ticket.ID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "TICKET NOT FOUND"})
	}

	if h.messenger != nil {
		_ = h.messenger.SendUserMessage(ticket.TelegramID,
			"▓ TYRAX SUPPORT ▓\n\nТикет закрыт. Если нужна помощь — напиши снова.")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func adminErr(c *fiber.Ctx, err error) error {
	slog.Error("admin handler", slog.String("error", err.Error()))
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"status": "error", "message": "SYSTEM FAILURE",
	})
}
