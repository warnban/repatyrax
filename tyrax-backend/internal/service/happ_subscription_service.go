package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/pkg/vpnconfig"
)

const happDeviceName = "HAPP"

// HappSubscriptionFeed is the rendered body + HTTP headers for GET /sub/:token.
type HappSubscriptionFeed struct {
	Status  int
	Body    []byte
	Headers map[string]string
}

// HappSubscriptionService provisions Happ subscription URLs for iOS/macOS clients.
// It reuses the same VLESS+Reality+XHTTP stack as TYRAX Android/Windows without
// touching /vpn/connect or native app flows.
type HappSubscriptionService interface {
	EnsureSubscriptionURL(ctx context.Context, userID string) (string, error)
	RenderFeed(ctx context.Context, token string) (*HappSubscriptionFeed, error)
}

type happSubscriptionService struct {
	userRepo   repository.UserRepository
	deviceRepo repository.DeviceRepository
	nodeRepo   repository.NodeRepository
	vpn        VPNService
	panel      PanelSyncer
	traffic    *TrafficService
	publicURL  string
	websiteURL string
	botURL     string
}

func NewHappSubscriptionService(
	userRepo repository.UserRepository,
	deviceRepo repository.DeviceRepository,
	nodeRepo repository.NodeRepository,
	vpn VPNService,
	panel PanelSyncer,
	traffic *TrafficService,
	publicAPIURL, websiteURL, botURL string,
) HappSubscriptionService {
	return &happSubscriptionService{
		userRepo:   userRepo,
		deviceRepo: deviceRepo,
		nodeRepo:   nodeRepo,
		vpn:        vpn,
		panel:      panel,
		traffic:    traffic,
		publicURL:  strings.TrimRight(publicAPIURL, "/"),
		websiteURL: strings.TrimRight(websiteURL, "/"),
		botURL:     botURL,
	}
}

func (s *happSubscriptionService) EnsureSubscriptionURL(ctx context.Context, userID string) (string, error) {
	if _, err := s.ensureHappDevice(ctx, userID); err != nil {
		return "", err
	}
	token, err := s.userRepo.EnsureSubscriptionToken(ctx, userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/sub/%s", s.publicURL, token), nil
}

func (s *happSubscriptionService) RenderFeed(ctx context.Context, token string) (*HappSubscriptionFeed, error) {
	user, err := s.userRepo.FindBySubscriptionToken(ctx, token)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return &HappSubscriptionFeed{Status: 404, Body: []byte("NOT FOUND")}, nil
		}
		return nil, err
	}

	device, err := s.ensureHappDevice(ctx, user.ID)
	if err != nil {
		if errors.Is(err, ErrDeviceLimitReached) {
			return s.blockedFeed("DEVICE LIMIT REACHED — remove a device in TYRAX or the bot"), nil
		}
		return nil, err
	}

	blocked := false
	if s.traffic != nil {
		blocked, err = s.traffic.CheckBlocked(ctx, user.ID)
		if err != nil {
			blocked = false
		}
	}

	userInfo, tierLabel := s.buildUserInfo(ctx, user)
	headers := map[string]string{
		"Content-Type":            "text/plain; charset=utf-8",
		"Content-Disposition":     `attachment; filename="tyrax.txt"`,
		"Profile-Update-Interval": "3600",
		"Subscription-Userinfo":   userInfo,
		"Support-Url":             s.botURL,
		"Announce":                fmt.Sprintf("TYRAX · %s · управление: %s", tierLabel, s.botURL),
	}

	if blocked {
		headers["Announce"] = "ЛИМИТ ИСЧЕРПАН. Продлить или купить тариф: " + s.botURL
		body := base64.StdEncoding.EncodeToString([]byte(
			"#subscription-userinfo: " + userInfo + "\n",
		))
		return &HappSubscriptionFeed{Status: 200, Body: []byte(body), Headers: headers}, nil
	}

	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	effective := effectiveTier(user)
	lines := make([]string, 0, len(nodes)+1)
	lines = append(lines, "#subscription-userinfo: "+userInfo)

	for _, node := range nodes {
		if node.Status != model.NodeOpen || node.Protocol != "vless" {
			continue
		}
		// FREE includes all nodes; paid tiers respect node.min_tier.
		if effective != model.TierFree && !tierAllows(effective, node.MinTier) {
			continue
		}
		if s.panel != nil {
			if err := s.panel.AddClient(ctx, node, device.VlessUUID, device.ID); err != nil {
				// Best-effort — same as Connect/GetConfig.
				_ = err
			}
		}
		remark := fmt.Sprintf("TYRAX-%s", node.Codename)
		lines = append(lines, vpnconfig.GenerateVlessURI(node, device.VlessUUID, remark))
	}

	if len(lines) <= 1 {
		headers["Announce"] = "НОДЫ НЕДОСТУПНЫ. Попробуй позже или напиши в поддержку."
	}

	payload := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	return &HappSubscriptionFeed{
		Status:  200,
		Body:    []byte(encoded),
		Headers: headers,
	}, nil
}

func (s *happSubscriptionService) ensureHappDevice(ctx context.Context, userID string) (*model.Device, error) {
	device, err := s.deviceRepo.FindByUserAndName(ctx, userID, happDeviceName)
	if err == nil {
		return device, nil
	}
	if !errors.Is(err, repository.ErrDeviceNotFound) {
		return nil, err
	}
	if _, err := s.vpn.AddDevice(ctx, userID, happDeviceName); err != nil {
		return nil, err
	}
	return s.deviceRepo.FindByUserAndName(ctx, userID, happDeviceName)
}

func (s *happSubscriptionService) buildUserInfo(ctx context.Context, user *model.User) (string, string) {
	tier := effectiveTier(user)
	tierLabel := string(tier)

	if isUnlimited(tier) {
		var expire int64
		if user.SubscriptionEnd != nil {
			expire = user.SubscriptionEnd.Unix()
		}
		if expire > 0 {
			return fmt.Sprintf("upload=0; download=0; expire=%d", expire), tierLabel
		}
		// Paid, no expiry field → Happ shows no expiration bar.
		return "upload=0; download=0", tierLabel
	}

	used, limit, blockedUntil, _, err := s.traffic.Snapshot(ctx, user.ID)
	if err != nil {
		used, limit = user.TrafficUsedBytes, FreeQuotaBytes
	}

	expire := user.TrafficPeriodStart.Add(FreeBlockDuration).Unix()
	if blockedUntil != nil {
		expire = blockedUntil.Unix()
	}

	return fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d", used, limit, expire), tierLabel
}

func (s *happSubscriptionService) blockedFeed(message string) *HappSubscriptionFeed {
	body := base64.StdEncoding.EncodeToString([]byte("# " + message + "\n"))
	return &HappSubscriptionFeed{
		Status: 200,
		Body:   []byte(body),
		Headers: map[string]string{
			"Content-Type":        "text/plain; charset=utf-8",
			"Content-Disposition": `attachment; filename="tyrax.txt"`,
			"Announce":            message,
		},
	}
}

func tierAllows(userTier, minTier model.SubscriptionTier) bool {
	return tierRank(userTier) >= tierRank(minTier)
}

func tierRank(t model.SubscriptionTier) int {
	switch t {
	case model.TierFree:
		return 0
	case model.TierCore:
		return 1
	case model.TierShadow:
		return 2
	case model.TierDominion:
		return 3
	default:
		return 0
	}
}
