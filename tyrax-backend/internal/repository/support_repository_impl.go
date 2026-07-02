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

const supportQueryTimeout = 5 * time.Second

type supportRepository struct {
	db *pgxpool.Pool
}

func NewSupportRepository(db *pgxpool.Pool) SupportRepository {
	return &supportRepository{db: db}
}

func (r *supportRepository) FindOpenByTelegramID(ctx context.Context, telegramID int64) (*model.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	query := `SELECT id, user_id, telegram_id, telegram_username, subscription_tier,
	                 status, subject, created_at, updated_at, closed_at
	            FROM support_tickets
	           WHERE telegram_id = $1 AND status = 'open'
	           ORDER BY updated_at DESC
	           LIMIT 1`
	t, err := scanTicket(r.db.QueryRow(ctx, query, telegramID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find open ticket: %w", err)
	}
	return t, nil
}

func (r *supportRepository) CreateTicket(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	query := `INSERT INTO support_tickets
		(user_id, telegram_id, telegram_username, subscription_tier, status, subject)
		VALUES ($1, $2, $3, $4, 'open', $5)
		RETURNING id, user_id, telegram_id, telegram_username, subscription_tier,
		          status, subject, created_at, updated_at, closed_at`

	t, err := scanTicket(r.db.QueryRow(ctx, query,
		ticket.UserID, ticket.TelegramID, ticket.TelegramUsername,
		ticket.SubscriptionTier, ticket.Subject))
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	return t, nil
}

func (r *supportRepository) AddMessage(ctx context.Context, ticketID, sender, body string) (*model.SupportMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	query := `INSERT INTO support_messages (ticket_id, sender, body)
		VALUES ($1, $2, $3)
		RETURNING id, ticket_id, sender, body, created_at`

	var m model.SupportMessage
	if err := r.db.QueryRow(ctx, query, ticketID, sender, body).Scan(
		&m.ID, &m.TicketID, &m.Sender, &m.Body, &m.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("add support message: %w", err)
	}

	_, err := r.db.Exec(ctx,
		"UPDATE support_tickets SET updated_at = NOW() WHERE id = $1", ticketID)
	if err != nil {
		return nil, fmt.Errorf("touch ticket: %w", err)
	}
	return &m, nil
}

func (r *supportRepository) ListTickets(ctx context.Context, status string, limit, offset int) ([]model.SupportTicket, int, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	where := ""
	args := []any{}
	if status == "open" || status == "closed" {
		where = " WHERE t.status = $1"
		args = append(args, status)
	}

	countQuery := "SELECT COUNT(*) FROM support_tickets t" + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	listArgs := append(args, limit, offset)
	limitIdx := len(listArgs) - 1
	offsetIdx := len(listArgs)

	query := fmt.Sprintf(`
		SELECT t.id, t.user_id, t.telegram_id, t.telegram_username, t.subscription_tier,
		       t.status, t.subject, t.created_at, t.updated_at, t.closed_at,
		       (SELECT body FROM support_messages m WHERE m.ticket_id = t.id ORDER BY created_at DESC LIMIT 1) AS last_message,
		       (SELECT COUNT(*) FROM support_messages m WHERE m.ticket_id = t.id) AS message_count
		  FROM support_tickets t
		  %s
		  ORDER BY
		    CASE WHEN t.status = 'open' AND t.subscription_tier = 'DOMINION' THEN 0 ELSE 1 END,
		    t.updated_at DESC
		  LIMIT $%d OFFSET $%d`, where, limitIdx, offsetIdx)

	rows, err := r.db.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	out := make([]model.SupportTicket, 0, limit)
	for rows.Next() {
		var t model.SupportTicket
		var tier string
		var statusVal string
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.TelegramID, &t.TelegramUsername, &tier,
			&statusVal, &t.Subject, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt,
			&t.LastMessagePreview, &t.MessageCount,
		); err != nil {
			return nil, 0, err
		}
		t.SubscriptionTier = model.SubscriptionTier(tier)
		t.Status = model.SupportTicketStatus(statusVal)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

func (r *supportRepository) GetTicket(ctx context.Context, ticketID string) (*model.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	query := `SELECT id, user_id, telegram_id, telegram_username, subscription_tier,
	                 status, subject, created_at, updated_at, closed_at
	            FROM support_tickets WHERE id = $1`
	t, err := scanTicket(r.db.QueryRow(ctx, query, ticketID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("TICKET NOT FOUND")
	}
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	return t, nil
}

func (r *supportRepository) ListMessages(ctx context.Context, ticketID string) ([]model.SupportMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx,
		`SELECT id, ticket_id, sender, body, created_at
		   FROM support_messages
		  WHERE ticket_id = $1
		  ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	out := make([]model.SupportMessage, 0)
	for rows.Next() {
		var m model.SupportMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Sender, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *supportRepository) CloseTicket(ctx context.Context, ticketID string) error {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx,
		`UPDATE support_tickets
		    SET status = 'closed', closed_at = NOW(), updated_at = NOW()
		  WHERE id = $1 AND status = 'open'`, ticketID)
	if err != nil {
		return fmt.Errorf("close ticket: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("TICKET NOT FOUND")
	}
	return nil
}

func (r *supportRepository) TouchTicket(ctx context.Context, ticketID string) error {
	ctx, cancel := context.WithTimeout(ctx, supportQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE support_tickets SET updated_at = NOW() WHERE id = $1", ticketID)
	if err != nil {
		return fmt.Errorf("touch ticket: %w", err)
	}
	return nil
}

func scanTicket(row rowScanner) (*model.SupportTicket, error) {
	var t model.SupportTicket
	var tier, status string
	if err := row.Scan(
		&t.ID, &t.UserID, &t.TelegramID, &t.TelegramUsername, &tier,
		&status, &t.Subject, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt,
	); err != nil {
		return nil, err
	}
	t.SubscriptionTier = model.SubscriptionTier(tier)
	t.Status = model.SupportTicketStatus(status)
	return &t, nil
}
