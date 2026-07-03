# Partner Referral System — Design Spec

**Date:** 2026-07-03  
**Status:** Implemented

## Summary

Telegram-only referral attribution with partner portal, admin management, commission on first paid order within 30 days, 3-day hold, manual payouts.

## Attribution

- Ref link: `https://t.me/{bot}?start=ref_{CODE}`
- Bot `/start ref_{CODE}` attributes **new** Telegram identities only
- Existing users ignore ref code

## Commission

- First **paid** order (CORE/SHADOW/DOMINION) within 30 days of registration
- % of actual order amount (after discount)
- Global rate in `partner_settings`; optional per-partner override
- Admin grants and renewals excluded
- 3-day hold → `available`; refunds claw back

## Active user

≥3 distinct calendar days with tunnel connections (`connection_logs`)

## Payouts

- Manual admin payout (min 2 000 ₽)
- Partner sets MIR or USDT requisites (plain text, admin-visible)
- Partner portal: `partner.tyrex.tech` or `/partner` path

## Entities

`partners`, `partner_invites`, `partner_commissions`, `partner_payouts`, `users.referred_by_partner_id`
