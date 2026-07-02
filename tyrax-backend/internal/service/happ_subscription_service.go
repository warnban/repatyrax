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
		"Profile-Title":           encodeProfileTitle("TYRAX · " + strings.ToUpper(tierLabel)),
		"Profile-Update-Interval": "3600",
		"Subscription-Userinfo":   userInfo,
		"Support-Url":             s.botURL,
		"Announce":                fmt.Sprintf("TYRAX · %s · управление: %s", tierLabel, s.botURL),
	}
	if s.websiteURL != "" {
		headers["Profile-Web-Page-Url"] = s.websiteURL
	}

	if blocked {
		headers["Announce"] = "ЛИМИТ ИСЧЕРПАН. Продлить или купить тариф: " + s.botURL
		body := "#subscription-userinfo: " + userInfo + "\n"
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
		lines = append(lines, vpnconfig.GenerateVlessURI(node, device.VlessUUID, nodeRemark(node)))
	}

	if len(lines) <= 1 {
		headers["Announce"] = "НОДЫ НЕДОСТУПНЫ. Попробуй позже или напиши в поддержку."
	}

	payload := strings.Join(lines, "\n")
	return &HappSubscriptionFeed{
		Status:  200,
		Body:    []byte(payload),
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
	return &HappSubscriptionFeed{
		Status: 200,
		Body:   []byte("# " + message + "\n"),
		Headers: map[string]string{
			"Content-Type":  "text/plain; charset=utf-8",
			"Profile-Title": encodeProfileTitle("TYRAX"),
			"Announce":      message,
		},
	}
}

// encodeProfileTitle renders a subscription display name for the `profile-title`
// header. Clients (Happ, v2rayN, Streisand, Nekoray) accept a base64:-prefixed
// value; base64 is mandatory here because the brand separator "·" is non-ASCII
// and raw non-ASCII bytes are illegal in HTTP header values. Without this header
// Happ falls back to showing the raw host (api.tyrax.tech) as the profile name.
func encodeProfileTitle(title string) string {
	return "base64:" + base64.StdEncoding.EncodeToString([]byte(title))
}

// nodeRemark builds the per-node display name shown in Happ / v2ray clients:
// "<flag> <Country> · <NN>", e.g. "🇫🇮 Finland · 01". The flag is derived from
// the codename's ISO-3166-1 alpha-2 prefix ("FI-01" -> "FI"); the numeric
// suffix keeps names unique when one country hosts several nodes.
func nodeRemark(node model.Node) string {
	code, suffix := node.Codename, ""
	if i := strings.IndexByte(node.Codename, '-'); i > 0 {
		code = node.Codename[:i]
		suffix = node.Codename[i+1:]
	}

	name := strings.TrimSpace(node.Country)
	if name == "" {
		name = node.Codename
	}

	label := name
	if flag := flagEmoji(code); flag != "" {
		label = flag + " " + name
	}
	if suffix != "" {
		label += " · " + suffix
	}
	return label
}

// flagEmoji converts an ISO-3166-1 alpha-2 country code into its Unicode
// regional-indicator flag emoji (e.g. "FI" -> "🇫🇮"). Returns "" for anything
// that is not exactly two A-Z letters so callers can fall back to plain text.
func flagEmoji(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < 2; i++ {
		c := code[i]
		if c < 'A' || c > 'Z' {
			return ""
		}
		b.WriteRune(rune(0x1F1E6 + rune(c-'A')))
	}
	return b.String()
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
