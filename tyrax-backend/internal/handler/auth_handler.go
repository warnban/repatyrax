package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/tyrax/tyrax-backend/internal/middleware"
	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
)

const (
	// bcryptCost is deliberately high (12) — credential theft must be expensive.
	bcryptCost = 12
	// tokenTTL bounds the JWT lifetime to 30 days.
	tokenTTL = 30 * 24 * time.Hour
	// telegramTokenTTL bounds the Telegram deep-link auth token to 10 minutes.
	telegramTokenTTL = 10 * time.Minute
)

// AuthHandler owns identity issuance: registration, login, and the Telegram
// deep-link flow. It signs every session token with the process JWT secret.
type AuthHandler struct {
	userRepo    repository.UserRepository
	jwtSecret   string
	botUsername string
}

func NewAuthHandler(userRepo repository.UserRepository, jwtSecret, botUsername string) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		jwtSecret:   jwtSecret,
		botUsername: botUsername,
	}
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register — POST /api/v1/auth/register
// Creates a FREE-tier identity and returns a signed session token.
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req credentials
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		slog.Error("auth: hash password", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	user, err := h.userRepo.Create(c.Context(), req.Email, string(hash), "FREE")
	if err != nil {
		if errors.Is(err, repository.ErrEmailTaken) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "IDENTITY ALREADY EXISTS"})
		}
		slog.Error("auth: create user", slog.String("email", req.Email), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	token, err := h.signToken(user.ID, string(user.SubscriptionTier), user.Email)
	if err != nil {
		slog.Error("auth: sign token", slog.String("user_id", user.ID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	slog.Info("identity registered",
		slog.String("user_id", user.ID),
		slog.String("tier", string(user.SubscriptionTier)),
	)
	if ip := clientIP(c); ip != "" {
		_ = h.userRepo.SetRegistrationIP(c.Context(), user.ID, ip)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":   token,
			"user_id": user.ID,
			"tier":    string(user.SubscriptionTier),
			"email":   user.Email,
		},
	})
}

// Login — POST /api/v1/auth/login
// Verifies credentials and returns a signed session token.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req credentials
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
	}

	user, err := h.userRepo.FindByEmail(c.Context(), req.Email)
	if err != nil {
		// Do not leak whether the identity exists — same response for both.
		if errors.Is(err, repository.ErrUserNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
		}
		slog.Error("auth: find user", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "INVALID CREDENTIALS"})
	}

	token, err := h.signToken(user.ID, string(user.SubscriptionTier), user.Email)
	if err != nil {
		slog.Error("auth: sign token", slog.String("user_id", user.ID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	slog.Info("identity authenticated", slog.String("user_id", user.ID))
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":   token,
			"user_id": user.ID,
			"tier":    string(user.SubscriptionTier),
			"email":   user.Email,
		},
	})
}

// TelegramInit — GET /api/v1/auth/telegram-init
// Mints a short-lived one-time token the client passes into the Telegram bot
// deep link. The bot later confirms it server-side; the client polls callback.
func (h *AuthHandler) TelegramInit(c *fiber.Ctx) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		slog.Error("auth: generate telegram token", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	token := hex.EncodeToString(raw)

	if err := h.userRepo.CreateTelegramAuthToken(c.Context(), token, time.Now().Add(telegramTokenTTL)); err != nil {
		slog.Error("auth: persist telegram token", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	slog.Info("telegram auth initiated")
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"bot_username": h.botUsername,
			// Deep link the client opens to launch the bot with the one-time token.
			"bot_url": fmt.Sprintf("https://t.me/%s?start=%s", h.botUsername, token),
			"token":   token,
		},
	})
}

// TelegramCallback — POST /api/v1/auth/telegram-callback
// Polled by the client after launching the bot. Returns a session token once
// the bot has confirmed the token, or 202 {status:"pending"} while waiting.
func (h *AuthHandler) TelegramCallback(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	user, token, found, err := h.resolveTelegramSession(c.Context(), req.Token)
	if err != nil {
		slog.Error("auth: resolve telegram session", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	if !found {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "pending"})
	}

	slog.Info("telegram auth confirmed", slog.String("user_id", user.ID))
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":   token,
			"user_id": user.ID,
			"tier":    string(user.SubscriptionTier),
			"email":   user.Email,
		},
	})
}

// TelegramStatus — GET /api/v1/auth/telegram-status?token=xxx
// GET counterpart of TelegramCallback used by the Android client to poll the
// Telegram login. Returns {status:"pending"} while waiting, or
// {status:"ok", jwt:"...", data:{...}} once the bot has confirmed the token.
// The `data` object mirrors the login/register payload so the existing
// ApiResponse<AuthDataDto> client parser can read the token directly.
func (h *AuthHandler) TelegramStatus(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	user, jwtStr, found, err := h.resolveTelegramSession(c.Context(), token)
	if err != nil {
		slog.Error("auth: resolve telegram session", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	if !found {
		return c.JSON(fiber.Map{"status": "pending"})
	}

	slog.Info("telegram auth confirmed (poll)", slog.String("user_id", user.ID))
	return c.JSON(fiber.Map{
		"status": "ok",
		"jwt":    jwtStr,
		"data": fiber.Map{
			"token":   jwtStr,
			"user_id": user.ID,
			"tier":    string(user.SubscriptionTier),
			"email":   user.Email,
		},
	})
}

// GetProfile — GET /api/v1/auth/profile (JWT required)
// Returns the current identity's profile, including whether a Telegram account
// is linked (drives the "Привязать Telegram" affordance on the client).
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	userID := extractUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "ACCESS DENIED"})
	}

	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "IDENTITY NOT FOUND"})
		}
		slog.Error("auth: get profile", slog.String("user_id", userID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"user_id":         user.ID,
			"email":           user.Email,
			"tier":            string(user.SubscriptionTier),
			"telegram_linked": user.TelegramID != nil,
		},
	})
}

// resolveTelegramSession consumes a bot-confirmed token and mints a session JWT.
// found is false (nil error) while the token is still pending/expired/unknown.
func (h *AuthHandler) resolveTelegramSession(ctx context.Context, token string) (*model.User, string, bool, error) {
	userID, found, err := h.userRepo.ConsumeConfirmedTelegramToken(ctx, token)
	if err != nil || !found {
		return nil, "", found, err
	}

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, "", true, err
	}

	signed, err := h.signToken(user.ID, string(user.SubscriptionTier), user.Email)
	if err != nil {
		return nil, "", true, err
	}
	return user, signed, true, nil
}

// signToken issues an HS256 JWT carrying user_id, tier and email, valid for tokenTTL.
func (h *AuthHandler) signToken(userID, tier, email string) (string, error) {
	now := time.Now()
	claims := middleware.TyraxClaims{
		UserID:           userID,
		SubscriptionTier: tier,
		Email:            email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}

func clientIP(c *fiber.Ctx) string {
	if fwd := c.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	return c.IP()
}
