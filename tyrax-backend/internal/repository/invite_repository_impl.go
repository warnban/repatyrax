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

const inviteQueryTimeout = 5 * time.Second

var ErrInviteNotFound = errors.New("INVITE NOT FOUND")

type inviteRepository struct {
	db *pgxpool.Pool
}

func NewInviteRepository(db *pgxpool.Pool) InviteRepository {
	return &inviteRepository{db: db}
}

func (r *inviteRepository) Create(ctx context.Context, invite *model.Invite) error {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	err := r.db.QueryRow(ctx,
		`INSERT INTO subscription_invites (owner_id, invitee_id)
		 VALUES ($1, $2) RETURNING id, created_at`,
		invite.OwnerID, invite.InviteeID,
	).Scan(&invite.ID, &invite.CreatedAt)
	if err != nil {
		return fmt.Errorf("create invite: %w", err)
	}
	return nil
}

func (r *inviteRepository) FindPendingByInvitee(ctx context.Context, inviteeID, inviteID string) (*model.Invite, error) {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	var inv model.Invite
	err := r.db.QueryRow(ctx,
		`SELECT id, owner_id, invitee_id, status, created_at
		   FROM subscription_invites
		  WHERE id = $1 AND invitee_id = $2 AND status = 'pending'`,
		inviteID, inviteeID,
	).Scan(&inv.ID, &inv.OwnerID, &inv.InviteeID, &inv.Status, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find invite: %w", err)
	}
	return &inv, nil
}

func (r *inviteRepository) CountActiveByOwner(ctx context.Context, ownerID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM subscription_invites
		  WHERE owner_id = $1 AND status = 'pending'`,
		ownerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count invites: %w", err)
	}
	return count, nil
}

func (r *inviteRepository) GetByOwnerID(ctx context.Context, ownerID string) ([]model.Invite, error) {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx,
		`SELECT id, owner_id, invitee_id, status, created_at
		   FROM subscription_invites
		  WHERE owner_id = $1 AND status IN ('pending', 'accepted')
		  ORDER BY created_at DESC`,
		ownerID)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()

	invites := make([]model.Invite, 0)
	for rows.Next() {
		var inv model.Invite
		if err := rows.Scan(&inv.ID, &inv.OwnerID, &inv.InviteeID, &inv.Status, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invites: %w", err)
	}
	return invites, nil
}

func (r *inviteRepository) Accept(ctx context.Context, inviteID string) error {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		"UPDATE subscription_invites SET status = 'accepted' WHERE id = $1",
		inviteID)
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	return nil
}

func (r *inviteRepository) Delete(ctx context.Context, ownerID, inviteeID string) error {
	ctx, cancel := context.WithTimeout(ctx, inviteQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx,
		"DELETE FROM subscription_invites WHERE owner_id = $1 AND invitee_id = $2",
		ownerID, inviteeID)
	if err != nil {
		return fmt.Errorf("delete invite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}
