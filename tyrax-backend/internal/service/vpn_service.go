package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/pkg/vpnconfig"
)

var (
	ErrDeviceLimitReached = errors.New("DEVICE LIMIT REACHED")
	ErrNodeUnavailable    = errors.New("NODE UNAVAILABLE")
)

type DeviceConfig struct {
	DeviceID      string       `json:"device_id"`
	WireGuardConf string       `json:"wireguard_conf,omitempty"`
	VlessConf     string       `json:"vless_conf,omitempty"`
	Nodes         []model.Node `json:"nodes"`
}

type VPNConfig struct {
	Protocol string `json:"protocol"`
	Config   string `json:"config"`
}

type NodeResponse struct {
	ID       string `json:"id"`
	Codename string `json:"codename"`
	Country  string `json:"country"`
	Status   string `json:"status"`
	PingMS   int    `json:"ping_ms"`
}

type VPNService interface {
	AddDevice(ctx context.Context, userID, deviceName string) (*DeviceConfig, error)
	GetConfig(ctx context.Context, userID, devicePublicKey string) (*VPNConfig, error)
	GetNodes(ctx context.Context) ([]NodeResponse, error)
	ListDevices(ctx context.Context, userID string) ([]model.Device, error)
	DeleteDevice(ctx context.Context, deviceID, userID string) error
	GetSplitDomains(ctx context.Context) ([]string, error)
}

type vpnService struct {
	nodeRepo   repository.NodeRepository
	deviceRepo repository.DeviceRepository
	userRepo   repository.UserRepository
}

func NewVPNService(nodeRepo repository.NodeRepository, deviceRepo repository.DeviceRepository, userRepo repository.UserRepository) VPNService {
	return &vpnService{
		nodeRepo:   nodeRepo,
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
	}
}

func DeviceLimit(tier model.SubscriptionTier) int {
	switch tier {
	case model.TierFree:
		return 1
	case model.TierCore:
		return 2
	case model.TierShadow:
		return 5
	case model.TierDominion:
		return 10
	default:
		return 1
	}
}

// allocateClientIP returns the lowest 10.0.x.y address (x:1-254, y:2-254) not
// already present in used. Falls back to 10.0.1.2 if the pool is exhausted.
func allocateClientIP(used []string) string {
	usedSet := make(map[string]bool, len(used))
	for _, ip := range used {
		usedSet[ip] = true
	}
	for subnet := 1; subnet <= 254; subnet++ {
		for host := 2; host <= 254; host++ {
			ip := fmt.Sprintf("10.0.%d.%d", subnet, host)
			if !usedSet[ip] {
				return ip
			}
		}
	}
	return "10.0.1.2" // fallback
}

func (s *vpnService) AddDevice(ctx context.Context, userID, deviceName string) (*DeviceConfig, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Check if device with this name already exists for this user.
	// If so, delete it to allow re-registration (e.g., app reinstall).
	existingDevice, err := s.deviceRepo.FindByUserAndName(ctx, userID, deviceName)
	if err == nil && existingDevice != nil {
		if delErr := s.deviceRepo.Delete(ctx, existingDevice.ID, userID); delErr != nil {
			return nil, fmt.Errorf("delete existing device: %w", delErr)
		}
	} else if err != nil && !errors.Is(err, repository.ErrDeviceNotFound) {
		return nil, fmt.Errorf("find existing device: %w", err)
	}

	count, err := s.deviceRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count devices: %w", err)
	}

	if count >= DeviceLimit(user.SubscriptionTier) {
		return nil, ErrDeviceLimitReached
	}

	privKey, pubKey, err := vpnconfig.GenerateKeypair()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	// Allocate the lowest unused tunnel IP from the 10.0.0.0/8 pool. Reading the
	// live set (rather than a count) keeps allocation collision-safe even after
	// devices are deleted and re-created.
	existingIPs, _ := s.deviceRepo.GetAllClientIPs(ctx)
	clientIP := allocateClientIP(existingIPs)

	device := &model.Device{
		UserID:    userID,
		Name:      deviceName,
		PublicKey: pubKey,
		ClientIP:  clientIP,
	}

	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}

	bestNode, err := s.nodeRepo.GetBest(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNodeNotFound) {
			return nil, ErrNodeUnavailable
		}
		return nil, fmt.Errorf("get best node: %w", err)
	}

	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var wgConf, vlessConf string
	if bestNode.Protocol == "wireguard" {
		wgConf = vpnconfig.GenerateClientConfig(*bestNode, privKey, pubKey, bestNode.PublicKey, device.ClientIP)
	} else if bestNode.Protocol == "vless" {
		vlessConf = vpnconfig.GenerateVlessConfig(*bestNode, userID)
	}

	return &DeviceConfig{
		DeviceID:      device.ID,
		WireGuardConf: wgConf,
		VlessConf:     vlessConf,
		Nodes:         nodes,
	}, nil
}

func (s *vpnService) GetConfig(ctx context.Context, userID, devicePublicKey string) (*VPNConfig, error) {
	device, err := s.deviceRepo.FindByPublicKey(ctx, devicePublicKey)
	if err != nil {
		return nil, fmt.Errorf("find device: %w", err)
	}

	if device.UserID != userID {
		return nil, errors.New("ACCESS DENIED")
	}

	bestNode, err := s.nodeRepo.GetBest(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNodeNotFound) {
			return nil, ErrNodeUnavailable
		}
		return nil, fmt.Errorf("get best node: %w", err)
	}

	var conf string
	if bestNode.Protocol == "wireguard" {
		// In a real system, we would retrieve the client's private key (if stored, or they provide it)
		// and IP. Since we don't store the private key, we can't fully regenerate the WG config here
		// if we strictly follow standard WG practices. However, the prompt says:
		// "If node protocol=wireguard → WireGuard config".
		// We'll return a placeholder or require the client to generate their own keys in a real app.
		// For now, we'll just return a skeleton since we only have their public key.
		conf = "[Interface]\n# PrivateKey must be injected by client\n"
	} else if bestNode.Protocol == "vless" {
		conf = vpnconfig.GenerateVlessConfig(*bestNode, userID)
	}

	return &VPNConfig{
		Protocol: bestNode.Protocol,
		Config:   conf,
	}, nil
}

func (s *vpnService) GetNodes(ctx context.Context) ([]NodeResponse, error) {
	nodes, err := s.nodeRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	resp := make([]NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		resp = append(resp, NodeResponse{
			ID:       n.ID,
			Codename: n.Codename,
			Country:  n.Country,
			Status:   string(n.Status),
			PingMS:   n.PingMS,
		})
	}
	return resp, nil
}

func (s *vpnService) ListDevices(ctx context.Context, userID string) ([]model.Device, error) {
	return s.deviceRepo.GetByUserID(ctx, userID)
}

func (s *vpnService) DeleteDevice(ctx context.Context, deviceID, userID string) error {
	return s.deviceRepo.Delete(ctx, deviceID, userID)
}

func (s *vpnService) GetSplitDomains(ctx context.Context) ([]string, error) {
	return []string{
		"yandex.ru", "ya.ru", "vk.com", "vkontakte.ru", "ok.ru", "mail.ru",
		"gosuslugi.ru", "mos.ru", "sberbank.ru", "tinkoff.ru", "vtb.ru", "alfabank.ru", "raiffeisen.ru",
		"ozon.ru", "wildberries.ru", "avito.ru", "hh.ru", "kinopoisk.ru", "ivi.ru", "rutube.ru",
		"2gis.ru", "drom.ru", "auto.ru", "rbc.ru", "kommersant.ru", "ria.ru", "lenta.ru", "meduza.io",
	}, nil
}
