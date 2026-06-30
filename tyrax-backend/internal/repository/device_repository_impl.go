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

const deviceQueryTimeout = 5 * time.Second

var ErrDeviceNotFound = errors.New("DEVICE NOT FOUND")

type deviceRepository struct {
	db *pgxpool.Pool
}

func NewDeviceRepository(db *pgxpool.Pool) DeviceRepository {
	return &deviceRepository{db: db}
}

func (r *deviceRepository) Create(ctx context.Context, device *model.Device) error {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	query := `
		INSERT INTO devices (user_id, name, public_key, client_ip)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, vless_uuid
	`
	err := r.db.QueryRow(ctx, query, device.UserID, device.Name, device.PublicKey, device.ClientIP).Scan(&device.ID, &device.CreatedAt, &device.VlessUUID)
	if err != nil {
		return fmt.Errorf("create device: %w", err)
	}
	return nil
}

func (r *deviceRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM devices WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices: %w", err)
	}
	return count, nil
}

// GetAllClientIPs returns every allocated tunnel IP, so the service can pick the
// lowest unused address (collision-safe even after devices are deleted).
func (r *deviceRepository) GetAllClientIPs(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx, "SELECT client_ip FROM devices WHERE client_ip IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("list client ips: %w", err)
	}
	defer rows.Close()

	ips := make([]string, 0)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("scan client ip: %w", err)
		}
		ips = append(ips, ip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client ips: %w", err)
	}
	return ips, nil
}

func (r *deviceRepository) GetByUserID(ctx context.Context, userID string) ([]model.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx,
		"SELECT id, user_id, name, public_key, client_ip, vless_uuid, created_at FROM devices WHERE user_id = $1 ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := make([]model.Device, 0)
	for rows.Next() {
		var d model.Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.PublicKey, &d.ClientIP, &d.VlessUUID, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return devices, nil
}

func (r *deviceRepository) FindByPublicKey(ctx context.Context, publicKey string) (*model.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	query := "SELECT id, user_id, name, public_key, client_ip, vless_uuid, created_at FROM devices WHERE public_key = $1"
	var d model.Device
	err := r.db.QueryRow(ctx, query, publicKey).Scan(&d.ID, &d.UserID, &d.Name, &d.PublicKey, &d.ClientIP, &d.VlessUUID, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find device by public key: %w", err)
	}
	return &d, nil
}

func (r *deviceRepository) FindByUserAndName(ctx context.Context, userID, name string) (*model.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	query := "SELECT id, user_id, name, public_key, client_ip, vless_uuid, created_at FROM devices WHERE user_id = $1 AND name = $2"
	var d model.Device
	err := r.db.QueryRow(ctx, query, userID, name).Scan(&d.ID, &d.UserID, &d.Name, &d.PublicKey, &d.ClientIP, &d.VlessUUID, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find device by user and name: %w", err)
	}
	return &d, nil
}

func (r *deviceRepository) Delete(ctx context.Context, id, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, deviceQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx, "DELETE FROM devices WHERE id = $1 AND user_id = $2", id, userID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceNotFound
	}
	return nil
}
