package repository

import (
	"context"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// EmailConfirmResult carries the identity fields needed to mint a session JWT
// after a verification code is consumed — avoids a second DB round-trip.
type EmailConfirmResult struct {
	UserID string
	Tier   string
	Email  string
}

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByTelegramID(ctx context.Context, telegramID int64) (*model.User, error)
	Create(ctx context.Context, email, passwordHash, tier string) (*model.User, error)
	CreateFromTelegram(ctx context.Context, telegramID int64, username string) (*model.User, error)
	ActivateSubscription(ctx context.Context, userID, tier string, endsAt time.Time) error
	SetParentSubscription(ctx context.Context, inviteeID string, ownerID *string) error

	// Traffic metering (FREE-tier quota enforcement).
	IncrementTraffic(ctx context.Context, userID string, delta int64) error
	SetBlockedUntil(ctx context.Context, userID string, until *time.Time) error
	ResetTrafficPeriod(ctx context.Context, userID string, start time.Time) error

	// Telegram deep-link auth flow.
	CreateTelegramAuthToken(ctx context.Context, token string, expiresAt time.Time) error
	ConsumeConfirmedTelegramToken(ctx context.Context, token string) (userID string, found bool, err error)

	// Email confirmation flow (email/password registrations).
	CreateEmailVerification(ctx context.Context, userID, email, code, token string, expiresAt time.Time) error
	// InvalidatePendingEmailVerifications marks all unused codes for this user as
	// consumed so only the latest resend remains valid.
	InvalidatePendingEmailVerifications(ctx context.Context, userID string) error
	// DiscardEmailVerificationByCode marks a freshly created row used when SMTP fails.
	DiscardEmailVerificationByCode(ctx context.Context, email, code string) error
	ConfirmEmailByCode(ctx context.Context, email, code string) (EmailConfirmResult, bool, error)
	MarkEmailVerified(ctx context.Context, userID string) error

	// Happ / external subscription feed.
	FindBySubscriptionToken(ctx context.Context, token string) (*model.User, error)
	EnsureSubscriptionToken(ctx context.Context, userID string) (string, error)

	SetRegistrationIP(ctx context.Context, userID, ip string) error
	TouchLastSeen(ctx context.Context, userID string) error
}
