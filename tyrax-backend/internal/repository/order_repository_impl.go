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

const orderQueryTimeout = 5 * time.Second

var ErrOrderNotFound = errors.New("ORDER NOT FOUND")

type orderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, order *model.Order) error {
	ctx, cancel := context.WithTimeout(ctx, orderQueryTimeout)
	defer cancel()

	query := `
		INSERT INTO orders (user_id, tier, months, amount_rub, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		order.UserID, order.Tier, order.Months, order.AmountRUB, order.PaymentMethod, model.OrderNew,
	).Scan(&order.ID, &order.CreatedAt)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (r *orderRepository) FindByID(ctx context.Context, id string) (*model.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, orderQueryTimeout)
	defer cancel()

	o, err := scanOrder(r.db.QueryRow(ctx,
		`SELECT id, user_id, tier, months, amount_rub, payment_method,
		        COALESCE(external_order_id, ''), status, created_at, paid_at
		   FROM orders WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find order by id: %w", err)
	}
	return o, nil
}

func (r *orderRepository) FindByExternalID(ctx context.Context, externalID string) (*model.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, orderQueryTimeout)
	defer cancel()

	o, err := scanOrder(r.db.QueryRow(ctx,
		`SELECT id, user_id, tier, months, amount_rub, payment_method,
		        COALESCE(external_order_id, ''), status, created_at, paid_at
		   FROM orders WHERE external_order_id = $1`, externalID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find order by external id: %w", err)
	}
	return o, nil
}

func (r *orderRepository) SetExternalID(ctx context.Context, orderID, externalID string) error {
	ctx, cancel := context.WithTimeout(ctx, orderQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE orders SET external_order_id = $1 WHERE id = $2",
		externalID, orderID)
	if err != nil {
		return fmt.Errorf("set external order id: %w", err)
	}
	return nil
}

func (r *orderRepository) MarkPaid(ctx context.Context, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, orderQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE orders SET status = $1, paid_at = NOW() WHERE id = $2",
		model.OrderPaid, orderID)
	if err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}
	return nil
}

func scanOrder(row rowScanner) (*model.Order, error) {
	var o model.Order
	if err := row.Scan(
		&o.ID,
		&o.UserID,
		&o.Tier,
		&o.Months,
		&o.AmountRUB,
		&o.PaymentMethod,
		&o.ExternalOrderID,
		&o.Status,
		&o.CreatedAt,
		&o.PaidAt,
	); err != nil {
		return nil, err
	}
	return &o, nil
}
