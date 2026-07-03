-- Partner referral program

CREATE TABLE partner_settings (
    id                      INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    default_commission_rate NUMERIC(5, 2) NOT NULL DEFAULT 20.00,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO partner_settings (id, default_commission_rate) VALUES (1, 20.00);

CREATE TABLE partners (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email                   TEXT NOT NULL UNIQUE,
    password_hash           TEXT NOT NULL,
    display_name            TEXT NOT NULL,
    ref_code                TEXT NOT NULL UNIQUE,
    commission_rate_override NUMERIC(5, 2),
    status                  TEXT NOT NULL DEFAULT 'active',
    payout_method           TEXT,
    payout_mir_card         TEXT,
    payout_usdt_address     TEXT,
    payout_usdt_network     TEXT,
    balance_available       NUMERIC(12, 2) NOT NULL DEFAULT 0,
    balance_hold            NUMERIC(12, 2) NOT NULL DEFAULT 0,
    total_paid_out          NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE partner_invites (
    token       TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    used_at     TIMESTAMPTZ,
    partner_id  UUID REFERENCES partners(id)
);

ALTER TABLE users
    ADD COLUMN referred_by_partner_id UUID REFERENCES partners(id);

CREATE INDEX idx_users_referred_by_partner ON users(referred_by_partner_id);

CREATE TABLE partner_commissions (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partner_id           UUID NOT NULL REFERENCES partners(id),
    user_id              UUID NOT NULL REFERENCES users(id),
    order_id             UUID NOT NULL REFERENCES orders(id) UNIQUE,
    order_amount_rub     NUMERIC(12, 2) NOT NULL,
    commission_rate      NUMERIC(5, 2) NOT NULL,
    commission_amount_rub NUMERIC(12, 2) NOT NULL,
    status               TEXT NOT NULL DEFAULT 'hold',
    hold_until           TIMESTAMPTZ NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    clawed_back_at       TIMESTAMPTZ,
    UNIQUE (partner_id, user_id)
);

CREATE INDEX idx_partner_commissions_partner ON partner_commissions(partner_id);
CREATE INDEX idx_partner_commissions_status ON partner_commissions(status, hold_until);

CREATE TABLE partner_payouts (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partner_id UUID NOT NULL REFERENCES partners(id),
    amount_rub NUMERIC(12, 2) NOT NULL,
    note       TEXT,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_partner_payouts_partner ON partner_payouts(partner_id);
