package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const adminQueryTimeout = 8 * time.Second
const onlineWindow = 5 * time.Minute

type adminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) ListUsers(ctx context.Context, search string, limit, offset int) ([]model.AdminUserRow, int, error) {
	ctx, cancel := context.WithTimeout(ctx, adminQueryTimeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	where := ""
	filterArgs := []any{}
	if search != "" {
		where = ` WHERE (
			u.email ILIKE $1 OR
			u.username ILIKE $1 OR
			CAST(u.telegram_id AS TEXT) ILIKE $1 OR
			CAST(u.id AS TEXT) ILIKE $1
		)`
		filterArgs = append(filterArgs, "%"+search+"%")
	}

	countQuery := "SELECT COUNT(*) FROM users u" + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	listArgs := append(filterArgs, onlineWindow, limit, offset)
	intervalIdx := len(filterArgs) + 1
	limitIdx := len(filterArgs) + 2
	offsetIdx := len(filterArgs) + 3

	query := fmt.Sprintf(`
		SELECT
			u.id, u.email, u.password_hash, u.telegram_id, u.username,
			CAST(u.registration_ip AS TEXT), u.last_seen_at,
			u.subscription_tier, u.subscription_end, u.created_at,
			u.traffic_used_bytes, u.traffic_period_start, u.blocked_until, u.subscription_token,
			COALESCE(d.cnt, 0) AS device_count,
			COALESCE(c.active_cnt, 0) AS active_connections,
			CASE
				WHEN u.last_seen_at IS NOT NULL AND u.last_seen_at > NOW() - $%d::interval THEN TRUE
				WHEN COALESCE(c.active_cnt, 0) > 0 THEN TRUE
				ELSE FALSE
			END AS is_online
		FROM users u
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS cnt FROM devices GROUP BY user_id
		) d ON d.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS active_cnt
			  FROM connection_logs
			 WHERE disconnected_at IS NULL
			 GROUP BY user_id
		) c ON c.user_id = u.id
		%s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d`, intervalIdx, where, limitIdx, offsetIdx)

	rows, err := r.db.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	out := make([]model.AdminUserRow, 0, limit)
	for rows.Next() {
		row, err := scanAdminUserRow(rows)
		if err != nil {
			return nil, 0, err
		}
		row.EffectiveTier = effectiveTier(&row.User)
		out = append(out, *row)
	}
	return out, total, rows.Err()
}

func (r *adminRepository) GetUserDetail(ctx context.Context, userID string) (*model.AdminUserDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, adminQueryTimeout)
	defer cancel()

	query := `
		SELECT
			u.id, u.email, u.password_hash, u.telegram_id, u.username,
			CAST(u.registration_ip AS TEXT), u.last_seen_at,
			u.subscription_tier, u.subscription_end, u.created_at,
			u.traffic_used_bytes, u.traffic_period_start, u.blocked_until, u.subscription_token,
			COALESCE(d.cnt, 0) AS device_count,
			COALESCE(c.active_cnt, 0) AS active_connections,
			CASE
				WHEN u.last_seen_at IS NOT NULL AND u.last_seen_at > NOW() - $2::interval THEN TRUE
				WHEN COALESCE(c.active_cnt, 0) > 0 THEN TRUE
				ELSE FALSE
			END AS is_online
		FROM users u
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS cnt FROM devices GROUP BY user_id
		) d ON d.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS active_cnt
			  FROM connection_logs
			 WHERE disconnected_at IS NULL
			 GROUP BY user_id
		) c ON c.user_id = u.id
		WHERE u.id = $1`

	row, err := scanAdminUserRow(r.db.QueryRow(ctx, query, userID, onlineWindow))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user detail: %w", err)
	}
	row.EffectiveTier = effectiveTier(&row.User)

	connRows, err := r.db.Query(ctx,
		`SELECT id, user_id, node_id, protocol, connected_at, disconnected_at
		   FROM connection_logs
		  WHERE user_id = $1
		  ORDER BY connected_at DESC
		  LIMIT 50`, userID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer connRows.Close()

	connections := make([]model.ConnectionLog, 0)
	for connRows.Next() {
		var cl model.ConnectionLog
		if err := connRows.Scan(&cl.ID, &cl.UserID, &cl.NodeID, &cl.Protocol, &cl.ConnectedAt, &cl.DisconnectedAt); err != nil {
			return nil, err
		}
		connections = append(connections, cl)
	}

	orderRows, err := r.db.Query(ctx,
		`SELECT id, user_id, tier, months, amount_rub, payment_method,
		        COALESCE(external_order_id, ''), status, created_at, paid_at
		   FROM orders
		  WHERE user_id = $1
		  ORDER BY created_at DESC
		  LIMIT 20`, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer orderRows.Close()

	orders := make([]model.Order, 0)
	for orderRows.Next() {
		o, err := scanOrder(orderRows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *o)
	}

	return &model.AdminUserDetail{
		AdminUserRow: *row,
		Connections:  connections,
		Orders:       orders,
	}, nil
}

func (r *adminRepository) RevokeSubscription(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, adminQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE users SET subscription_tier = 'FREE', subscription_end = NULL WHERE id = $1",
		userID)
	if err != nil {
		return fmt.Errorf("revoke subscription: %w", err)
	}
	return nil
}

func scanAdminUserRow(row rowScanner) (*model.AdminUserRow, error) {
	var u model.User
	var tier string
	var email, passwordHash, username, regIP *string
	var deviceCount, activeConnections int
	var isOnline bool

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
		&deviceCount,
		&activeConnections,
		&isOnline,
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

	return &model.AdminUserRow{
		User:              u,
		DeviceCount:       deviceCount,
		IsOnline:          isOnline,
		ActiveConnections: activeConnections,
	}, nil
}

func effectiveTier(u *model.User) model.SubscriptionTier {
	if u.SubscriptionTier == model.TierFree {
		return model.TierFree
	}
	if u.SubscriptionEnd != nil && u.SubscriptionEnd.Before(time.Now()) {
		return model.TierFree
	}
	return u.SubscriptionTier
}
