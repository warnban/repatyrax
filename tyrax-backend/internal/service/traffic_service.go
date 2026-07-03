package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
)

// Traffic quota constants for the FREE tier.
const (
	// FreeQuotaBytes is the rolling data allowance for FREE identities (1 GB).
	FreeQuotaBytes int64 = 1 * 1024 * 1024 * 1024
	// FreeBlockDuration is how long a FREE identity stays locked out after
	// exhausting the quota, counted from the moment traffic ran out.
	FreeBlockDuration = 30 * 24 * time.Hour
	// trafficPollInterval is how often the accounting sweep reads node panels.
	trafficPollInterval = 2 * time.Minute
	// trafficPollTimeout bounds a single sweep against hung panels.
	trafficPollTimeout = 60 * time.Second
)

// TrafficReader reads a device's cumulative traffic (bytes) from a node panel.
// Implemented by pkg/threexui.Syncer; a node without panel creds returns 0.
type TrafficReader interface {
	ClientTraffic(ctx context.Context, node model.Node, email string) (int64, error)
}

// TrafficService meters FREE-tier data usage and enforces the 1 GB / 30-day
// quota. It is deliberately fail-open: any panel or DB error leaves the tunnel
// working — a user is only ever blocked when blocked_until is explicitly set.
type TrafficService struct {
	userRepo   repository.UserRepository
	deviceRepo repository.DeviceRepository
	nodeRepo   repository.NodeRepository
	reader     TrafficReader
}

func NewTrafficService(
	userRepo repository.UserRepository,
	deviceRepo repository.DeviceRepository,
	nodeRepo repository.NodeRepository,
	reader TrafficReader,
) *TrafficService {
	return &TrafficService{
		userRepo:   userRepo,
		deviceRepo: deviceRepo,
		nodeRepo:   nodeRepo,
		reader:     reader,
	}
}

// EffectiveTier is the tier actually in force: a paid tier whose subscription
// has expired is treated as FREE.
func EffectiveTier(u *model.User) model.SubscriptionTier {
	if u.SubscriptionTier == model.TierFree {
		return model.TierFree
	}
	if u.SubscriptionEnd != nil && u.SubscriptionEnd.Before(time.Now()) {
		return model.TierFree
	}
	return u.SubscriptionTier
}

func effectiveTier(u *model.User) model.SubscriptionTier {
	return EffectiveTier(u)
}

// isUnlimited reports whether the effective tier has no traffic cap.
func isUnlimited(tier model.SubscriptionTier) bool {
	return tier != model.TierFree
}

// CheckBlocked applies period rollover then reports whether the user's tunnel is
// currently blocked. Fail-open: on any error it returns false (not blocked).
func (s *TrafficService) CheckBlocked(ctx context.Context, userID string) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	blocked, _ := s.rollover(ctx, user)
	return blocked, nil
}

// rollover resets an elapsed quota window / lifts an expired block, then returns
// whether the user is still blocked and until when. Only FREE identities can be
// blocked. Mutates the DB but never returns an error to callers (best-effort).
func (s *TrafficService) rollover(ctx context.Context, user *model.User) (bool, *time.Time) {
	now := time.Now()
	tier := effectiveTier(user)

	// Paid tiers are never limited; opportunistically clear a stale block.
	if isUnlimited(tier) {
		if user.BlockedUntil != nil {
			if err := s.userRepo.SetBlockedUntil(ctx, user.ID, nil); err != nil {
				slog.Warn("traffic: clear stale block", "user", user.ID, "err", err.Error())
			}
			user.BlockedUntil = nil
		}
		return false, nil
	}

	// A block that has expired starts a brand-new quota window.
	if user.BlockedUntil != nil {
		if now.After(*user.BlockedUntil) {
			if err := s.userRepo.ResetTrafficPeriod(ctx, user.ID, now); err != nil {
				slog.Warn("traffic: reset after block", "user", user.ID, "err", err.Error())
				return true, user.BlockedUntil // keep blocked if reset failed
			}
			user.TrafficUsedBytes = 0
			user.TrafficPeriodStart = now
			user.BlockedUntil = nil
			return false, nil
		}
		return true, user.BlockedUntil
	}

	// No active block: roll the 30-day window over if it has elapsed.
	if now.Sub(user.TrafficPeriodStart) >= FreeBlockDuration {
		if err := s.userRepo.ResetTrafficPeriod(ctx, user.ID, now); err != nil {
			slog.Warn("traffic: reset period", "user", user.ID, "err", err.Error())
		} else {
			user.TrafficUsedBytes = 0
			user.TrafficPeriodStart = now
		}
	}
	return false, nil
}

// Snapshot returns the traffic figures for the /subscription response, applying
// rollover first so the client sees a fresh window. limit is -1 for unlimited.
func (s *TrafficService) Snapshot(ctx context.Context, userID string) (used, limit int64, blockedUntil *time.Time, unlimited bool, err error) {
	user, ferr := s.userRepo.FindByID(ctx, userID)
	if ferr != nil {
		return 0, 0, nil, false, ferr
	}
	_, until := s.rollover(ctx, user)
	tier := effectiveTier(user)
	if isUnlimited(tier) {
		return 0, -1, nil, true, nil
	}
	return user.TrafficUsedBytes, FreeQuotaBytes, until, false, nil
}

// RunLoop drives the accounting sweep on a ticker until ctx is cancelled.
func (s *TrafficService) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(trafficPollInterval)
	defer ticker.Stop()

	// One sweep shortly after boot, then on the ticker.
	s.pollSafe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollSafe(ctx)
		}
	}
}

// pollSafe wraps Poll with a timeout; sweep errors are logged, never fatal.
func (s *TrafficService) pollSafe(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, trafficPollTimeout)
	defer cancel()
	if err := s.Poll(ctx); err != nil {
		slog.Warn("traffic: sweep failed", "err", err.Error())
	}
}

// Poll reads per-device cumulative traffic from every panel node, converts it to
// per-user deltas (handling panel-side counter resets), credits usage and locks
// out FREE identities that have exhausted their quota.
func (s *TrafficService) Poll(ctx context.Context) error {
	devices, err := s.deviceRepo.ListForAccounting(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil
	}

	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return err
	}
	panelNodes := make([]model.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Protocol == "vless" && n.PanelURL != "" {
			panelNodes = append(panelNodes, n)
		}
	}
	if len(panelNodes) == 0 {
		return nil // nothing to meter
	}

	perUser := make(map[string]int64)
	for _, d := range devices {
		var cur int64
		for _, n := range panelNodes {
			t, terr := s.reader.ClientTraffic(ctx, n, d.ID)
			if terr != nil {
				// Best-effort: skip this node, keep metering the rest.
				slog.Debug("traffic: read client", "node", n.Codename, "device", d.ID, "err", terr.Error())
				continue
			}
			cur += t
		}

		delta := cur - d.LastTrafficBytes
		if delta < 0 {
			// Panel counter was reset — count the new cumulative as the delta.
			delta = cur
		}
		if delta > 0 {
			perUser[d.UserID] += delta
		}
		if cur != d.LastTrafficBytes {
			if uerr := s.deviceRepo.UpdateLastTraffic(ctx, d.ID, cur); uerr != nil {
				slog.Warn("traffic: update last", "device", d.ID, "err", uerr.Error())
			}
		}
	}

	now := time.Now()
	for userID, delta := range perUser {
		if err := s.userRepo.IncrementTraffic(ctx, userID, delta); err != nil {
			slog.Warn("traffic: increment", "user", userID, "err", err.Error())
			continue
		}
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			continue
		}
		if effectiveTier(user) != model.TierFree {
			continue
		}
		if user.BlockedUntil == nil && user.TrafficUsedBytes >= FreeQuotaBytes {
			until := now.Add(FreeBlockDuration)
			if err := s.userRepo.SetBlockedUntil(ctx, userID, &until); err != nil {
				slog.Warn("traffic: set block", "user", userID, "err", err.Error())
			} else {
				slog.Info("traffic: FREE quota exhausted, blocked", "user", userID, "until", until)
			}
		}
	}
	return nil
}
