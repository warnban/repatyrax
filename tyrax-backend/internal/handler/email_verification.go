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

// issueVerification mints a fresh 6-digit code + link token, persists it, and
// dispatches the confirmation email asynchronously. Persistence errors are
// returned; SMTP latency/errors never block the caller.
func (h *AuthHandler) issueVerification(userID, email string) error {
	code, err := randomDigits(6)
	if err != nil {
		return err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate verify token: %w", err)
	}
	token := hex.EncodeToString(raw)

	ctx, cancel := context.WithTimeout(context.Background(), verifyIssueTimeout)
	defer cancel()
	if err := h.userRepo.CreateEmailVerification(ctx, userID, email, code, token, time.Now().Add(emailVerifyTTL)); err != nil {
		return err
	}

	go h.sendVerificationEmail(email, code, token)
	return nil
}

func (h *AuthHandler) sendVerificationEmail(email, code, token string) {
	if h.mailer == nil || !h.mailer.Enabled() {
		return
	}
	link := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", strings.TrimRight(h.publicAPIURL, "/"), token)
	subject := "TYRAX — ПОДТВЕРДИ ДОСТУП"
	if err := h.mailer.Send(email, subject, verificationText(code, link), verificationHTML(code, link, h.supportEmail)); err != nil {
		slog.Error("auth: send verification email", slog.String("email", email), slog.String("error", err.Error()))
	}
}

// VerifyEmailPage — GET /api/v1/auth/verify-email?token=xxx
// The target of the confirmation link in the email. Renders a branded HTML page
// and flips the identity to verified on success.
func (h *AuthHandler) VerifyEmailPage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")

	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString(verifyResultHTML(false, "ССЫЛКА НЕВЕРНА"))
	}

	userID, found, err := h.userRepo.ConfirmEmailByToken(c.Context(), token)
	if err != nil {
		slog.Error("auth: confirm email by token", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).SendString(verifyResultHTML(false, "СИСТЕМНЫЙ СБОЙ. ПОПРОБУЙ ПОЗЖЕ."))
	}
	if !found {
		return c.Status(fiber.StatusGone).SendString(verifyResultHTML(false, "ССЫЛКА УСТАРЕЛА ИЛИ УЖЕ ИСПОЛЬЗОВАНА"))
	}

	slog.Info("email confirmed via link", slog.String("user_id", userID))
	return c.SendString(verifyResultHTML(true, ""))
}

// VerifyEmailCode — POST /api/v1/auth/verify {email, code}
// In-app confirmation: validates the 6-digit code and returns a fresh session
// token so the client can proceed immediately after entering it.
func (h *AuthHandler) VerifyEmailCode(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}
	email := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" || code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID REQUEST"})
	}

	userID, found, err := h.userRepo.ConfirmEmailByCode(c.Context(), email, code)
	if err != nil {
		slog.Error("auth: confirm email by code", slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}
	if !found {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "INVALID OR EXPIRED CODE"})
	}

	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		slog.Error("auth: load user after verify", slog.String("user_id", userID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	token, err := h.signToken(user.ID, string(user.SubscriptionTier), user.Email)
	if err != nil {
		slog.Error("auth: sign token after verify", slog.String("user_id", user.ID), slog.String("error", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "SYSTEM FAILURE"})
	}

	slog.Info("email confirmed via code", slog.String("user_id", user.ID))
	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"token":          token,
			"user_id":        user.ID,
			"tier":           string(user.SubscriptionTier),
			"email":          user.Email,
			"email_verified": true,
		},
	})
}

// ResendVerification — POST /api/v1/auth/resend-verification {email}
// Re-sends a confirmation code. Always responds ok to avoid leaking which emails
// are registered.
func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	_ = c.BodyParser(&req)
	email := normalizeEmail(req.Email)

	if email != "" && h.verifyEmail {
		user, err := h.userRepo.FindByEmail(c.Context(), email)
		switch {
		case err == nil && !user.EmailVerified:
			if e := h.issueVerification(user.ID, user.Email); e != nil {
				slog.Error("auth: resend verification", slog.String("user_id", user.ID), slog.String("error", e.Error()))
			}
		case err != nil && !isNotFound(err):
			slog.Error("auth: resend lookup", slog.String("error", err.Error()))
		}
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data":   fiber.Map{"message": "IF THE IDENTITY EXISTS, A NEW CODE WAS SENT"},
	})
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == repository.ErrUserNotFound.Error()
}

// normalizeEmail lower-cases and trims so lookups and uniqueness are consistent.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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

// ── Email + result page templates (TYRAX brand: black, red accent, uppercase) ─

func verificationText(code, link string) string {
	return "TYRAX — ПОДТВЕРЖДЕНИЕ ДОСТУПА\n\n" +
		"Код подтверждения: " + code + "\n\n" +
		"Или открой ссылку:\n" + link + "\n\n" +
		"Ссылка и код действуют 24 часа.\n" +
		"Если это был не ты — просто игнорируй письмо."
}

func verificationHTML(code, link, support string) string {
	safeCode := html.EscapeString(code)
	safeLink := html.EscapeString(link)
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
		`Введи код в приложении или открой ссылку ниже. Действует 24 часа.` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 28px;">` +
		`<div style="font-family:'Courier New',monospace;font-weight:700;font-size:38px;letter-spacing:10px;color:#ffffff;background:#000000;border:1px solid #FF1E1E;padding:20px;text-align:center;">` + safeCode + `</div>` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 40px;" align="center">` +
		`<a href="` + safeLink + `" style="display:inline-block;font-family:Arial,Helvetica,sans-serif;font-weight:700;font-size:14px;letter-spacing:1px;color:#000000;background:#FF1E1E;text-decoration:none;padding:16px 32px;">ПОДТВЕРДИТЬ ДОСТУП</a>` +
		`</td></tr>` +
		`<tr><td style="padding:0 40px 40px;font-family:Arial,Helvetica,sans-serif;font-size:11px;line-height:18px;color:#666666;">` +
		`Если это был не ты — просто игнорируй письмо.<br>Поддержка: ` + safeSupport +
		`</td></tr>` +
		`</table></td></tr></table></body></html>`
}

func verifyResultHTML(ok bool, errMsg string) string {
	accent := "#FF1E1E"
	title := "ДОСТУП ПОДТВЕРЖДЁН"
	sub := "Возвращайся в приложение TYRAX и войди в систему."
	if !ok {
		title = "ОШИБКА"
		sub = html.EscapeString(errMsg)
	}
	return `<!DOCTYPE html><html lang="ru"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>TYRAX</title></head>` +
		`<body style="margin:0;background:#000;display:flex;min-height:100vh;align-items:center;justify-content:center;font-family:Arial,Helvetica,sans-serif;">` +
		`<div style="text-align:center;padding:32px;max-width:420px;">` +
		`<div style="font-weight:900;font-size:34px;letter-spacing:6px;color:#fff;">TYRAX</div>` +
		`<div style="font-weight:700;font-size:16px;letter-spacing:2px;color:` + accent + `;margin:24px 0 12px;">` + title + `</div>` +
		`<div style="font-size:14px;line-height:22px;color:#cccccc;">` + sub + `</div>` +
		`</div></body></html>`
}
