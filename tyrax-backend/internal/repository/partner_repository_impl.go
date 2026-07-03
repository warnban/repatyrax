package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const partnerQueryTimeout = 5 * time.Second

const partnerColumns = `id, email, password_hash, display_name, ref_code, commission_rate_override,
	status, payout_method, payout_mir_card, payout_usdt_address, payout_usdt_network,
	balance_available, balance_hold, total_paid_out, created_at`

type partnerRepository struct {
	db *pgxpool.Pool
}

func NewPartnerRepository(db *pgxpool.Pool) PartnerRepository {
	return &partnerRepository{db: db}
}

func scanPartner(row pgx.Row) (*model.Partner, error) {
	var p model.Partner
	err := row.Scan(
		&p.ID, &p.Email, &p.PasswordHash, &p.DisplayName, &p.RefCode, &p.CommissionRateOverride,
		&p.Status, &p.PayoutMethod, &p.PayoutMIRCard, &p.PayoutUSDTAddress, &p.PayoutUSDTNetwork,
		&p.BalanceAvailable, &p.BalanceHold, &p.TotalPaidOut, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *partnerRepository) GetSettings(ctx context.Context) (*model.PartnerSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var s model.PartnerSettings
	err := r.db.QueryRow(ctx,
		`SELECT default_commission_rate, updated_at FROM partner_settings WHERE id = 1`,
	).Scan(&s.DefaultCommissionRate, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get partner settings: %w", err)
	}
	return &s, nil
}

func (r *partnerRepository) UpdateSettings(ctx context.Context, rate float64) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`UPDATE partner_settings SET default_commission_rate = $1, updated_at = NOW() WHERE id = 1`,
		rate,
	)
	if err != nil {
		return fmt.Errorf("update partner settings: %w", err)
	}
	return nil
}

func (r *partnerRepository) CreateInvite(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`INSERT INTO partner_invites (token) VALUES ($1)`,
		token,
	)
	if err != nil {
		return fmt.Errorf("create partner invite: %w", err)
	}
	return nil
}

func (r *partnerRepository) GetInvite(ctx context.Context, token string) (*model.PartnerInvite, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var inv model.PartnerInvite
	err := r.db.QueryRow(ctx,
		`SELECT token, created_at, expires_at, used_at, partner_id
		   FROM partner_invites WHERE token = $1`,
		token,
	).Scan(&inv.Token, &inv.CreatedAt, &inv.ExpiresAt, &inv.UsedAt, &inv.PartnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPartnerInviteInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("get partner invite: %w", err)
	}
	return &inv, nil
}

func (r *partnerRepository) MarkInviteUsed(ctx context.Context, token, partnerID string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx,
		`UPDATE partner_invites
		    SET used_at = NOW(), partner_id = $2
		  WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()`,
		token, partnerID,
	)
	if err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPartnerInviteInvalid
	}
	return nil
}

func (r *partnerRepository) CreatePartner(ctx context.Context, p *model.Partner) (*model.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	query := `INSERT INTO partners (email, password_hash, display_name, ref_code, commission_rate_override, status)
	          VALUES ($1, $2, $3, $4, $5, $6)
	          RETURNING ` + partnerColumns
	row := r.db.QueryRow(ctx, query, p.Email, p.PasswordHash, p.DisplayName, p.RefCode, p.CommissionRateOverride, p.Status)
	partner, err := scanPartner(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrPartnerEmailTaken
		}
		return nil, fmt.Errorf("create partner: %w", err)
	}
	return partner, nil
}

func (r *partnerRepository) FindByID(ctx context.Context, id string) (*model.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	row := r.db.QueryRow(ctx, `SELECT `+partnerColumns+` FROM partners WHERE id = $1`, id)
	p, err := scanPartner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPartnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find partner: %w", err)
	}
	return p, nil
}

func (r *partnerRepository) FindByEmail(ctx context.Context, email string) (*model.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	row := r.db.QueryRow(ctx, `SELECT `+partnerColumns+` FROM partners WHERE email = $1`, email)
	p, err := scanPartner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPartnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find partner by email: %w", err)
	}
	return p, nil
}

func (r *partnerRepository) FindByRefCode(ctx context.Context, refCode string) (*model.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	row := r.db.QueryRow(ctx,
		`SELECT `+partnerColumns+` FROM partners WHERE ref_code = $1 AND status = $2`,
		refCode, model.PartnerStatusActive,
	)
	p, err := scanPartner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPartnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find partner by ref code: %w", err)
	}
	return p, nil
}

func (r *partnerRepository) ListPartners(ctx context.Context) ([]model.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx,
		`SELECT `+partnerColumns+` FROM partners ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	defer rows.Close()

	var out []model.Partner
	for rows.Next() {
		var p model.Partner
		if err := rows.Scan(
			&p.ID, &p.Email, &p.PasswordHash, &p.DisplayName, &p.RefCode, &p.CommissionRateOverride,
			&p.Status, &p.PayoutMethod, &p.PayoutMIRCard, &p.PayoutUSDTAddress, &p.PayoutUSDTNetwork,
			&p.BalanceAvailable, &p.BalanceHold, &p.TotalPaidOut, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan partner: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *partnerRepository) UpdatePartnerOverride(ctx context.Context, id string, rate *float64) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`UPDATE partners SET commission_rate_override = $2 WHERE id = $1`,
		id, rate,
	)
	if err != nil {
		return fmt.Errorf("update partner override: %w", err)
	}
	return nil
}

func (r *partnerRepository) UpdatePartnerStatus(ctx context.Context, id, status string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx, `UPDATE partners SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("update partner status: %w", err)
	}
	return nil
}

func (r *partnerRepository) UpdatePayoutDetails(ctx context.Context, id, method, mirCard, usdtAddr, usdtNet string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`UPDATE partners
		    SET payout_method = $2, payout_mir_card = $3,
		        payout_usdt_address = $4, payout_usdt_network = $5
		  WHERE id = $1`,
		id, method, mirCard, usdtAddr, usdtNet,
	)
	if err != nil {
		return fmt.Errorf("update payout details: %w", err)
	}
	return nil
}

func (r *partnerRepository) SetUserReferral(ctx context.Context, userID, partnerID string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tag, err := r.db.Exec(ctx,
		`UPDATE users SET referred_by_partner_id = $2
		  WHERE id = $1 AND referred_by_partner_id IS NULL`,
		userID, partnerID,
	)
	if err != nil {
		return fmt.Errorf("set user referral: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil // already attributed or user missing
	}
	return nil
}

func (r *partnerRepository) CountReferrals(ctx context.Context, partnerID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE referred_by_partner_id = $1`,
		partnerID,
	).Scan(&n)
	return n, err
}

func (r *partnerRepository) CountActiveReferrals(ctx context.Context, partnerID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
		    SELECT cl.user_id
		      FROM connection_logs cl
		      JOIN users u ON u.id = cl.user_id
		     WHERE u.referred_by_partner_id = $1
		     GROUP BY cl.user_id
		    HAVING COUNT(DISTINCT DATE(cl.connected_at AT TIME ZONE 'UTC')) >= $2
		 ) sub`,
		partnerID, model.ActiveUserMinDays,
	).Scan(&n)
	return n, err
}

func (r *partnerRepository) CountConversions(ctx context.Context, partnerID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM partner_commissions
		  WHERE partner_id = $1 AND status != $2`,
		partnerID, model.CommissionClawedBack,
	).Scan(&n)
	return n, err
}

func (r *partnerRepository) CreateCommission(ctx context.Context, c *model.PartnerCommission) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO partner_commissions
		    (partner_id, user_id, order_id, order_amount_rub, commission_rate,
		     commission_amount_rub, status, hold_until)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		c.PartnerID, c.UserID, c.OrderID, c.OrderAmountRUB, c.CommissionRate,
		c.CommissionAmountRUB, c.Status, c.HoldUntil,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert commission: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE partners SET balance_hold = balance_hold + $2 WHERE id = $1`,
		c.PartnerID, c.CommissionAmountRUB,
	)
	if err != nil {
		return fmt.Errorf("credit hold balance: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *partnerRepository) FindCommissionByOrder(ctx context.Context, orderID string) (*model.PartnerCommission, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	var c model.PartnerCommission
	err := r.db.QueryRow(ctx,
		`SELECT id, partner_id, user_id, order_id, order_amount_rub, commission_rate,
		        commission_amount_rub, status, hold_until, created_at, clawed_back_at
		   FROM partner_commissions WHERE order_id = $1`,
		orderID,
	).Scan(
		&c.ID, &c.PartnerID, &c.UserID, &c.OrderID, &c.OrderAmountRUB, &c.CommissionRate,
		&c.CommissionAmountRUB, &c.Status, &c.HoldUntil, &c.CreatedAt, &c.ClawedBackAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find commission: %w", err)
	}
	return &c, nil
}

func (r *partnerRepository) ReleaseHeldCommissions(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
		`SELECT id, partner_id, commission_amount_rub
		   FROM partner_commissions
		  WHERE status = $1 AND hold_until <= NOW()
		  FOR UPDATE`,
		model.CommissionHold,
	)
	if err != nil {
		return 0, fmt.Errorf("select held commissions: %w", err)
	}
	defer rows.Close()

	type row struct {
		id, partnerID string
		amount        float64
	}
	var batch []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.id, &rr.partnerID, &rr.amount); err != nil {
			return 0, err
		}
		batch = append(batch, rr)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, rr := range batch {
		_, err := tx.Exec(ctx,
			`UPDATE partner_commissions SET status = $2 WHERE id = $1`,
			rr.id, model.CommissionAvailable,
		)
		if err != nil {
			return 0, fmt.Errorf("release commission: %w", err)
		}
		_, err = tx.Exec(ctx,
			`UPDATE partners
			    SET balance_hold = balance_hold - $2,
			        balance_available = balance_available + $2
			  WHERE id = $1`,
			rr.partnerID, rr.amount,
		)
		if err != nil {
			return 0, fmt.Errorf("move hold to available: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(batch), nil
}

func (r *partnerRepository) ClawbackCommission(ctx context.Context, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var partnerID string
	var amount float64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT partner_id, commission_amount_rub, status
		   FROM partner_commissions
		  WHERE order_id = $1 AND status != $2
		  FOR UPDATE`,
		orderID, model.CommissionClawedBack,
	).Scan(&partnerID, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find commission for clawback: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE partner_commissions
		    SET status = $2, clawed_back_at = NOW()
		  WHERE order_id = $1`,
		orderID, model.CommissionClawedBack,
	)
	if err != nil {
		return fmt.Errorf("mark clawback: %w", err)
	}

	switch status {
	case model.CommissionHold:
		_, err = tx.Exec(ctx,
			`UPDATE partners SET balance_hold = balance_hold - $2 WHERE id = $1`,
			partnerID, amount,
		)
	case model.CommissionAvailable:
		_, err = tx.Exec(ctx,
			`UPDATE partners SET balance_available = balance_available - $2 WHERE id = $1`,
			partnerID, amount,
		)
	}
	if err != nil {
		return fmt.Errorf("debit partner balance: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *partnerRepository) RecordPayout(ctx context.Context, partnerID string, amount float64, note, adminUser string) error {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE partners
		    SET balance_available = balance_available - $2,
		        total_paid_out = total_paid_out + $2
		  WHERE id = $1 AND balance_available >= $2`,
		partnerID, amount,
	)
	if err != nil {
		return fmt.Errorf("debit available balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("INSUFFICIENT BALANCE")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO partner_payouts (partner_id, amount_rub, note, created_by)
		 VALUES ($1, $2, $3, $4)`,
		partnerID, amount, note, adminUser,
	)
	if err != nil {
		return fmt.Errorf("insert payout: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *partnerRepository) ListPayouts(ctx context.Context, partnerID string) ([]model.PartnerPayout, error) {
	ctx, cancel := context.WithTimeout(ctx, partnerQueryTimeout)
	defer cancel()

	rows, err := r.db.Query(ctx,
		`SELECT id, partner_id, amount_rub, note, created_by, created_at
		   FROM partner_payouts
		  WHERE partner_id = $1
		  ORDER BY created_at DESC`,
		partnerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payouts: %w", err)
	}
	defer rows.Close()

	var out []model.PartnerPayout
	for rows.Next() {
		var p model.PartnerPayout
		if err := rows.Scan(&p.ID, &p.PartnerID, &p.AmountRUB, &p.Note, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
