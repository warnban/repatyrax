package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const userQueryTimeout = 5 * time.Second

// userColumns is the full column set scanned into model.User, in scan order.
const userColumns = "id, email, password_hash, telegram_id, username, registration_ip, last_seen_at, subscription_tier, subscription_end, created_at, traffic_used_bytes, traffic_period_start, blocked_until, subscription_token, email_verified"

var (
	ErrUserNotFound = errors.New("IDENTITY NOT FOUND")
	ErrEmailTaken   = errors.New("IDENTITY ALREADY EXISTS")
)

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "SELECT " + userColumns + " FROM users WHERE id = $1"
	u, err := scanUser(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (r *userRepository) ActivateSubscription(ctx context.Context, userID, tier string, endsAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET subscription_tier = $1, subscription_end = $2 WHERE id = $3",
		tier, endsAt, userID)
	if err != nil {
		return fmt.Errorf("activate subscription: %w", err)
	}
	return nil
}

// IncrementTraffic adds delta bytes to the user's current-period usage counter.
func (r *userRepository) IncrementTraffic(ctx context.Context, userID string, delta int64) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET traffic_used_bytes = traffic_used_bytes + $1 WHERE id = $2",
		delta, userID)
	if err != nil {
		return fmt.Errorf("increment traffic: %w", err)
	}
	return nil
}

// SetBlockedUntil locks (or, with nil, unlocks) a user's tunnel access.
func (r *userRepository) SetBlockedUntil(ctx context.Context, userID string, until *time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET blocked_until = $1 WHERE id = $2",
		until, userID)
	if err != nil {
		return fmt.Errorf("set blocked until: %w", err)
	}
	return nil
}

// ResetTrafficPeriod starts a fresh quota window: zeroes usage, stamps the period
// start to now and clears any block.
func (r *userRepository) ResetTrafficPeriod(ctx context.Context, userID string, start time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET traffic_used_bytes = 0, traffic_period_start = $1, blocked_until = NULL WHERE id = $2",
		start, userID)
	if err != nil {
		return fmt.Errorf("reset traffic period: %w", err)
	}
	return nil
}

func (r *userRepository) SetParentSubscription(ctx context.Context, inviteeID string, ownerID *string) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET parent_subscription_id = $1 WHERE id = $2",
		ownerID, inviteeID)
	if err != nil {
		return fmt.Errorf("set parent subscription: %w", err)
	}
	return nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "SELECT " + userColumns + " FROM users WHERE email = $1"
	u, err := scanUser(r.db.QueryRow(ctx, query, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *userRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "SELECT " + userColumns + " FROM users WHERE telegram_id = $1"
	u, err := scanUser(r.db.QueryRow(ctx, query, telegramID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by telegram id: %w", err)
	}
	return u, nil
}

// CreateFromTelegram provisions a FREE-tier identity bound to a Telegram account.
// On a unique-key race (two /start commands at once) it falls back to a lookup.
func (r *userRepository) CreateFromTelegram(ctx context.Context, telegramID int64, username string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "INSERT INTO users (telegram_id, username, subscription_tier, email_verified) " +
		"VALUES ($1, $2, 'FREE', TRUE) RETURNING " + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, query, telegramID, username))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return r.FindByTelegramID(ctx, telegramID)
		}
		return nil, fmt.Errorf("create user from telegram: %w", err)
	}
	return u, nil
}

// Create inserts a new identity. The database generates the UUID and stamps
// created_at; the inserted row is returned fully populated.
// A duplicate email surfaces as ErrEmailTaken (unique_violation, SQLSTATE 23505).
func (r *userRepository) Create(ctx context.Context, email, passwordHash, tier string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "INSERT INTO users (email, password_hash, subscription_tier) " +
		"VALUES ($1, $2, $3) RETURNING " + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, query, email, passwordHash, tier))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *userRepository) CreateTelegramAuthToken(ctx context.Context, token string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"INSERT INTO telegram_auth_tokens (token, expires_at) VALUES ($1, $2)",
		token, expiresAt)
	if err != nil {
		return fmt.Errorf("create telegram auth token: %w", err)
	}
	return nil
}

// ConsumeConfirmedTelegramToken returns the bound user_id for a token that the
// bot has confirmed and that has not expired, marking it used to prevent replay.
// found is false (with a nil error) when the token is still pending/expired.
func (r *userRepository) ConsumeConfirmedTelegramToken(ctx context.Context, token string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := `UPDATE telegram_auth_tokens
	             SET used_at = NOW()
	           WHERE token = $1
	             AND confirmed = TRUE
	             AND used_at IS NULL
	             AND expires_at > NOW()
	             AND user_id IS NOT NULL
	       RETURNING user_id`

	var userID string
	err := r.db.QueryRow(ctx, query, token).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("consume telegram token: %w", err)
	}
	return userID, true, nil
}

// CreateEmailVerification persists a pending confirmation (both a 6-digit code
// for in-app entry and an opaque token for the email link) for an unverified
// identity. Multiple rows may coexist for one user after resends; the newest
// unused, unexpired one wins.
func (r *userRepository) CreateEmailVerification(ctx context.Context, userID, email, code, token string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`INSERT INTO email_verifications (user_id, email, code, token, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, email, code, token, expiresAt)
	if err != nil {
		return fmt.Errorf("create email verification: %w", err)
	}
	return nil
}

// ConfirmEmailByToken consumes a link token, marks it used and flips the user's
// email_verified flag in one transaction. found is false (nil error) when the
// token is unknown, expired or already used.
func (r *userRepository) ConfirmEmailByToken(ctx context.Context, token string) (string, bool, error) {
	return r.confirmEmail(ctx,
		`UPDATE email_verifications
		    SET used_at = NOW()
		  WHERE token = $1
		    AND used_at IS NULL
		    AND expires_at > NOW()
		RETURNING user_id`,
		token)
}

// ConfirmEmailByCode is the in-app counterpart: it matches the newest unused,
// unexpired code for the given email.
func (r *userRepository) ConfirmEmailByCode(ctx context.Context, email, code string) (string, bool, error) {
	return r.confirmEmail(ctx,
		`UPDATE email_verifications
		    SET used_at = NOW()
		  WHERE id = (
		      SELECT id FROM email_verifications
		       WHERE email = $1 AND code = $2 AND used_at IS NULL AND expires_at > NOW()
		       ORDER BY created_at DESC
		       LIMIT 1
		  )
		RETURNING user_id`,
		email, code)
}

func (r *userRepository) confirmEmail(ctx context.Context, updateSQL string, args ...any) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("begin confirm email: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	if err := tx.QueryRow(ctx, updateSQL, args...).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("consume email verification: %w", err)
	}

	if _, err := tx.Exec(ctx,
		"UPDATE users SET email_verified = TRUE WHERE id = $1", userID); err != nil {
		return "", false, fmt.Errorf("mark email verified: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", false, fmt.Errorf("commit confirm email: %w", err)
	}
	return userID, true, nil
}

// MarkEmailVerified flips the flag directly — used when SMTP is not configured
// (verification disabled) so email registrations remain usable in dev.
func (r *userRepository) MarkEmailVerified(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET email_verified = TRUE WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

func (r *userRepository) FindBySubscriptionToken(ctx context.Context, token string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	query := "SELECT " + userColumns + " FROM users WHERE subscription_token = $1"
	row := r.db.QueryRow(ctx, query, token)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by subscription token: %w", err)
	}
	return user, nil
}

func (r *userRepository) EnsureSubscriptionToken(ctx context.Context, userID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	var existing *string
	err := r.db.QueryRow(ctx,
		"SELECT subscription_token FROM users WHERE id = $1", userID).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read subscription token: %w", err)
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	token, err := newSubscriptionToken()
	if err != nil {
		return "", err
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE users SET subscription_token = $1
		  WHERE id = $2 AND (subscription_token IS NULL OR subscription_token = '')`,
		token, userID)
	if err != nil {
		return "", fmt.Errorf("set subscription token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		err = r.db.QueryRow(ctx,
			"SELECT subscription_token FROM users WHERE id = $1", userID).Scan(&existing)
		if err != nil {
			return "", fmt.Errorf("read subscription token after race: %w", err)
		}
		if existing != nil && *existing != "" {
			return *existing, nil
		}
		return "", fmt.Errorf("set subscription token: no row updated")
	}
	return token, nil
}

func (r *userRepository) SetRegistrationIP(ctx context.Context, userID, ip string) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`UPDATE users SET registration_ip = $1::inet
		  WHERE id = $2 AND registration_ip IS NULL`,
		ip, userID)
	if err != nil {
		return fmt.Errorf("set registration ip: %w", err)
	}
	return nil
}

func (r *userRepository) TouchLastSeen(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET last_seen_at = NOW() WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("touch last seen: %w", err)
	}
	return nil
}

func newSubscriptionToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate subscription token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// scanUser reads a users row in userColumns order.
func scanUser(row rowScanner) (*model.User, error) {
	var u model.User
	// pgx/v5 cannot scan the subscription_tier enum directly into a named
	// string type, so read it into a plain string and cast afterwards.
	var tier string
	// email and password_hash are nullable (Telegram-provisioned identities
	// have neither), so scan them into *string and dereference when present.
	var email *string
	var passwordHash *string
	var username *string
	var regIP *string
	if err := row.Scan(
		&u.ID,
		&email,
		&passwordHash,
		&u.TelegramID,
		&username,
		&regIP,
		&u.LastSeenAt,
		&tier,
		&u.SubscriptionEnd,
		&u.CreatedAt,
		&u.TrafficUsedBytes,
		&u.TrafficPeriodStart,
		&u.BlockedUntil,
		&u.SubscriptionToken,
		&u.EmailVerified,
	); err != nil {
		return nil, err
	}
	u.SubscriptionTier = model.SubscriptionTier(tier)
	if email != nil {
		u.Email = *email
	}
	if passwordHash != nil {
		u.PasswordHash = *passwordHash
	}
	if username != nil {
		u.Username = username
	}
	if regIP != nil {
		u.RegistrationIP = regIP
	}
	return &u, nil
}
