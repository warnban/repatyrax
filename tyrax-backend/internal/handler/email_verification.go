package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/tyrax/tyrax-backend/internal/repository"
)

const verifyIssueTimeout = 5 * time.Second

// issueVerification mints a fresh 6-digit code, persists it, and sends email.
// Only the in-app code path is supported — no magic links (Mail.ru prefetch broke them).
func (h *AuthHandler) issueVerification(userID, email string) (emailSent bool, err error) {
	code, err := randomDigits(6)
	if err != nil {
		return false, err
	}
	// token column remains for schema compatibility; never emailed or exposed.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return false, fmt.Errorf("generate verify token: %w", err)
	}
	token := hex.EncodeToString(raw)

	ctx, cancel := context.WithTimeout(context.Background(), verifyIssueTimeout)
	defer cancel()
	if err := h.userRepo.InvalidatePendingEmailVerifications(ctx, userID); err != nil {
		return false, err
	}
	if err := h.userRepo.CreateEmailVerification(ctx, userID, email, code, token, time.Now().Add(emailVerifyTTL)); err != nil {
		return false, err
	}

	if err := h.sendVerificationEmail(email, code); err != nil {
		_ = h.userRepo.DiscardEmailVerificationByCode(ctx, email, code)
		return false, err
	}
	return true, nil
}

func (h *AuthHandler) sendVerificationEmail(email, code string) error {
	if h.mailer == nil || !h.mailer.Enabled() {
		slog.Warn("auth: verification email skipped — mailer disabled", slog.String("email", email))
		return fmt.Errorf("mailer disabled")
	}
	subject := "TYRAX — ПОДТВЕРДИ ДОСТУП"
	if err := h.mailer.Send(email, subject, verificationText(code), verificationHTML(code, h.supportEmail)); err != nil {
		slog.Error("auth: send verification email", slog.String("email", email), slog.String("error", err.Error()))
		return err
	}
	slog.Info("auth: verification email sent", slog.String("email", email))
	return nil
}

// VerifyEmailCode — POST /api/v1/auth/verify {email, code}
func (h *AuthHandler) VerifyEmailCode(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	email := normalizeEmail(req.Email)
	code := normalizeVerificationCode(req.Code)
	if email == "" || len(code) != 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	confirmed, found, err := h.userRepo.ConfirmEmailByCode(c.Context(), email, code)
	if err != nil {
		slog.Error("auth: confirm email by code", slog.String("email", email), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	if !found {
		if user, lookupErr := h.userRepo.FindByEmail(c.Context(), email); lookupErr == nil && user.EmailVerified {
			return h.issueVerifiedSession(c, user.ID, string(user.SubscriptionTier), user.Email)
		}
		slog.Warn("auth: invalid verification code", slog.String("email", email))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID OR EXPIRED CODE"})
	}

	slog.Info("email confirmed via code", slog.String("user_id", confirmed.UserID))
	return h.issueVerifiedSession(c, confirmed.UserID, confirmed.Tier, confirmed.Email)
}

func (h *AuthHandler) issueVerifiedSession(c *fiber.Ctx, userID, tier, email string) error {
	token, err := h.signToken(userID, tier, email)
	if err != nil {
		slog.Error("auth: sign token after verify", slog.String("user_id", userID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":          token,
			"user_id":        userID,
			"tier":           tier,
			"email":          email,
			"email_verified": true,
		},
	})
}

// ResendVerification — POST /api/v1/auth/resend-verification {email}
func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	_ = c.BodyParser(&req)
	email := normalizeEmail(req.Email)

	if email != "" && h.verifyEmail {
		emailSent := false
		user, err := h.userRepo.FindByEmail(c.Context(), email)
		switch {
		case err == nil && !user.EmailVerified:
			var sendErr error
			emailSent, sendErr = h.issueVerification(user.ID, user.Email)
			if sendErr != nil {
				slog.Error("auth: resend verification", slog.String("user_id", user.ID), slog.String("error", sendErr.Error()))
			}
		case err == nil && user.EmailVerified:
			slog.Info("auth: resend skipped — already verified", slog.String("user_id", user.ID))
		case err != nil && !isNotFound(err):
			slog.Error("auth: resend lookup", slog.String("error", err.Error()))
		}

		return c.JSON(fiber.Map{
			"status": "ok",
			"data": fiber.Map{
				"message":    "IF THE IDENTITY EXISTS, A NEW CODE WAS SENT",
				"email_sent": emailSent,
			},
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   fiber.Map{"message": "IF THE IDENTITY EXISTS, A NEW CODE WAS SENT", "email_sent": false},
	})
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == repository.ErrUserNotFound.Error()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeVerificationCode(code string) string {
	code = strings.TrimSpace(code)
	var b strings.Builder
	for _, r := range code {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randomDigits(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	const digits = "0123456789"
	out := make([]byte, n)
	for i := range b {
		out[i] = digits[int(b[i])%10]
	}
	return string(out), nil
}

func verificationText(code string) string {
	return "TYRAX — ПОДТВЕРЖДЕНИЕ ДОСТУПА\n\n" +
		"Код: " + code + "\n\n" +
		"Введи его в приложении TYRAX.\n" +
		"Код действует 24 часа.\n" +
		"Если это был не ты — просто игнорируй письмо."
}

func verificationHTML(code, support string) string {
	safeCode := html.EscapeString(code)
	safeSupport := html.EscapeString(support)
	return `<!DOCTYPE html><html lang="ru"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"></head>` +
		`<body style="margin:0;padding:0;background:#000000;">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#000000;">` +
		`<tr><td align="center" style="padding:48px 16px;">` +
		`<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="max-width:480px;width:100%;background:#0a0a0a;border:1px solid #1a1a1a;">` +
		`<tr><td style="padding:40px 40px 24px;">` +
		`<div style="font-family:Arial,Helvetica,sans-serif;font-weight:900;font-size:28px;letter-spacing:4px;color:#ffffff;">TYRAX</div>` +
		`<div style="font-family:Arial,Helvetica,sans-serif;font-weight:700;font-size:13px;letter-spacing:2px;color:#FF1E1E;margin-top:8px;">ПОДТВЕРДИ ДОСТУП</div>` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 24px;font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:22px;color:#cccccc;">` +
		`Введи код в приложении TYRAX. «Отправить снова» в приложении отменяет предыдущий код.` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 28px;">` +
		`<div style="font-family:'Courier New',monospace;font-weight:700;font-size:38px;letter-spacing:10px;color:#ffffff;background:#000000;border:1px solid #FF1E1E;padding:20px;text-align:center;">` + safeCode + `</div>` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 40px;font-family:Arial,Helvetica,sans-serif;font-size:11px;line-height:18px;color:#666666;">` +
		`Если это был не ты — просто игнорируй письмо.<br>Поддержка: ` + safeSupport +
		`</td></tr>` +
		`</table></td></tr></table></body></html>`
}
