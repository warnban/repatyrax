package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const connQueryTimeout = 5 * time.Second

type connectionRepository struct {
	db *pgxpool.Pool
}

func NewConnectionRepository(db *pgxpool.Pool) ConnectionRepository {
	return &connectionRepository{db: db}
}

func (r *connectionRepository) LogConnect(ctx context.Context, userID, nodeID, protocol string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, connQueryTimeout)
	defer cancel()

	var id string
	err := r.db.QueryRow(ctx,
		`INSERT INTO connection_logs (user_id, node_id, protocol)
		 VALUES ($1, $2, $3)
		 RETURNING id`, userID, nodeID, protocol).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("log connect: %w", err)
	}
	return id, nil
}

func (r *connectionRepository) LogDisconnect(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, connQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`UPDATE connection_logs
		    SET disconnected_at = NOW()
		  WHERE user_id = $1 AND disconnected_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("log disconnect: %w", err)
	}
	return nil
}

func (r *connectionRepository) ListByUser(ctx context.Context, userID string, limit int) ([]model.ConnectionLog, error) {
	ctx, cancel := context.WithTimeout(ctx, connQueryTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, node_id, protocol, connected_at, disconnected_at
		   FROM connection_logs
		  WHERE user_id = $1
		  ORDER BY connected_at DESC
		  LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	out := make([]model.ConnectionLog, 0)
	for rows.Next() {
		var cl model.ConnectionLog
		if err := rows.Scan(&cl.ID, &cl.UserID, &cl.NodeID, &cl.Protocol, &cl.ConnectedAt, &cl.DisconnectedAt); err != nil {
			return nil, err
		}
		out = append(out, cl)
	}
	return out, rows.Err()
}

func (r *connectionRepository) CountActiveByUser(ctx context.Context, userID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, connQueryTimeout)
	defer cancel()

	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM connection_logs
		  WHERE user_id = $1 AND disconnected_at IS NULL`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count active connections: %w", err)
	}
	return n, nil
}
