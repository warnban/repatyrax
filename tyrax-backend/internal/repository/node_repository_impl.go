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

// nodeQueryTimeout bounds every node query against a hung database.
const nodeQueryTimeout = 5 * time.Second

// ErrNodeNotFound is returned when no node matches the requested criteria.
var ErrNodeNotFound = errors.New("NODE UNAVAILABLE")

// nodeColumns lists the columns selected into model.Node, in scan order.
const nodeColumns = "id, codename, country, host, port, protocol, status, ping_ms, public_key, min_tier, reality_public_key, reality_short_id, reality_sni"

type nodeRepository struct {
	db *pgxpool.Pool
}

func NewNodeRepository(db *pgxpool.Pool) NodeRepository {
	return &nodeRepository{db: db}
}

func (r *nodeRepository) List(ctx context.Context) ([]model.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, nodeQueryTimeout)
	defer cancel()

	query := "SELECT " + nodeColumns + " FROM nodes ORDER BY ping_ms ASC, codename ASC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]model.Node, 0)
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return nodes, nil
}

func (r *nodeRepository) FindByID(ctx context.Context, id string) (*model.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, nodeQueryTimeout)
	defer cancel()

	query := "SELECT " + nodeColumns + " FROM nodes WHERE id = $1"

	n, err := scanNode(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find node by id: %w", err)
	}
	return n, nil
}

func (r *nodeRepository) UpdatePing(ctx context.Context, nodeID string, pingMS int) error {
	ctx, cancel := context.WithTimeout(ctx, nodeQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx, "UPDATE nodes SET ping_ms = $1 WHERE id = $2", pingMS, nodeID)
	if err != nil {
		return fmt.Errorf("update node ping: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (r *nodeRepository) GetBest(ctx context.Context) (*model.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, nodeQueryTimeout)
	defer cancel()

	query := "SELECT " + nodeColumns + " FROM nodes WHERE status = $1 ORDER BY ping_ms ASC, codename ASC LIMIT 1"

	n, err := scanNode(r.db.QueryRow(ctx, query, model.NodeOpen))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get best node: %w", err)
	}
	return n, nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (*model.Node, error) {
	var n model.Node
	// pgx/v5 cannot scan a PostgreSQL enum directly into a named string type,
	// so read the enum columns into plain strings and cast afterwards.
	var status string
	var minTier string
	// Nullable string columns must scan into *string; a NULL would otherwise
	// fail to scan into a plain string. Dereference into the model when present.
	var publicKey *string
	var realityPublicKey *string
	var realityShortID *string
	var realitySNI *string
	if err := row.Scan(
		&n.ID,
		&n.Codename,
		&n.Country,
		&n.Host,
		&n.Port,
		&n.Protocol,
		&status,
		&n.PingMS,
		&publicKey,
		&minTier,
		&realityPublicKey,
		&realityShortID,
		&realitySNI,
	); err != nil {
		return nil, err
	}
	n.Status = model.NodeStatus(status)
	n.MinTier = model.SubscriptionTier(minTier)
	if publicKey != nil {
		n.PublicKey = *publicKey
	}
	if realityPublicKey != nil {
		n.RealityPublicKey = *realityPublicKey
	}
	if realityShortID != nil {
		n.RealityShortID = *realityShortID
	}
	if realitySNI != nil {
		n.RealitySNI = *realitySNI
	}
	return &n, nil
}
