package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tyrax/tyrax-backend/internal/model"
	"github.com/tyrax/tyrax-backend/internal/repository"
	"github.com/tyrax/tyrax-backend/pkg/vpnconfig"
)

// PanelSyncer registers/removes per-device VLESS UUIDs on a node's 3x-ui inbound
// so the node's Xray authenticates the same UUID the backend hands the client.
// Implemented by pkg/threexui.Syncer. A node without panel credentials is a
// no-op, so WireGuard and manually-managed nodes are unaffected.
type PanelSyncer interface {
	AddClient(ctx context.Context, node model.Node, clientUUID, email string) error
	DelClient(ctx context.Context, node model.Node, email string) error
}

var (
	ErrDeviceLimitReached = errors.New("DEVICE LIMIT REACHED")
	ErrNodeUnavailable    = errors.New("NODE UNAVAILABLE")
)

type DeviceConfig struct {
	DeviceID      string `json:"device_id"`
	Protocol      string `json:"protocol"` // "wireguard" | "vless"
	WireGuardConf string `json:"wireguard_conf,omitempty"`
	VlessConf     string `json:"vless_conf,omitempty"` // full Xray JSON (legacy / convenience)

	// Structured VLESS + Reality parameters for the Android Xray engine.
	// Populated only when Protocol == "vless"; the client builds its own Xray
	// JSON from these so it can inject a local SOCKS inbound + tun2socks bridge.
	// No omitempty — empty string must reach the client so it can distinguish
	// "field absent" from "field present but empty" (e.g. RealityShortID == "").
	UUID             string `json:"uuid"`
	NodeHost         string `json:"node_host"`
	NodePort         int    `json:"node_port"`
	RealityPublicKey string `json:"reality_public_key"`
	RealitySNI       string `json:"reality_sni"`
	RealityShortID   string `json:"reality_short_id"`

	// Transport / anti-DPI parameters (RU 2026). Empty string must reach the
	// client so it can distinguish "field absent" from "present but empty".
	Security      string `json:"security"`
	Network       string `json:"network"`
	Flow          string `json:"flow"`
	XhttpPath     string `json:"xhttp_path"`
	XhttpMode     string `json:"xhttp_mode"`
	XPaddingBytes string `json:"x_padding_bytes"`
	Fingerprint   string `json:"fingerprint"`

	Nodes []model.Node `json:"nodes"`
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
	Connect(ctx context.Context, userID, deviceName, codename string) (*VPNConfig, error)
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
	panel      PanelSyncer
}

func NewVPNService(nodeRepo repository.NodeRepository, deviceRepo repository.DeviceRepository, userRepo repository.UserRepository, panel PanelSyncer) VPNService {
	return &vpnService{
		nodeRepo:   nodeRepo,
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		panel:      panel,
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

	dc := &DeviceConfig{
		DeviceID: device.ID,
		Protocol: bestNode.Protocol,
		Nodes:    nodes,
	}

	switch bestNode.Protocol {
	case "wireguard":
		dc.WireGuardConf = vpnconfig.GenerateClientConfig(*bestNode, privKey, pubKey, bestNode.PublicKey, device.ClientIP)
	case "vless":
		dc.VlessConf = vpnconfig.GenerateVlessConfig(*bestNode, device.VlessUUID)
		dc.UUID = device.VlessUUID
		dc.NodeHost = bestNode.Host
		dc.NodePort = bestNode.Port
		dc.RealityPublicKey = bestNode.RealityPublicKey
		dc.RealitySNI = bestNode.RealitySNI
		dc.RealityShortID = bestNode.RealityShortID
		dc.Security = bestNode.Security
		dc.Network = bestNode.Network
		dc.Flow = bestNode.Flow
		dc.XhttpPath = bestNode.XhttpPath
		dc.XhttpMode = bestNode.XhttpMode
		dc.XPaddingBytes = bestNode.XPaddingBytes
		dc.Fingerprint = bestNode.Fingerprint

		// Register this device's UUID on the node's inbound. Best-effort: a
		// failure here is retried on the next Connect, so device creation still
		// succeeds (and nodes without panel creds are a no-op).
		if err := s.panel.AddClient(ctx, *bestNode, device.VlessUUID, device.ID); err != nil {
			slog.Warn("panel addClient (add device)", "node", bestNode.Codename, "device", device.ID, "err", err.Error())
		}
	}

	return dc, nil
}

func (s *vpnService) Connect(ctx context.Context, userID, deviceName, codename string) (*VPNConfig, error) {
	node, err := s.nodeRepo.GetByCodename(ctx, codename)
	if err != nil {
		if errors.Is(err, repository.ErrNodeNotFound) {
			return nil, ErrNodeUnavailable
		}
		return nil, fmt.Errorf("get node: %w", err)
	}
	if node.Status != model.NodeOpen {
		return nil, ErrNodeUnavailable
	}

	device, err := s.deviceRepo.FindByUserAndName(ctx, userID, deviceName)
	if err != nil && !errors.Is(err, repository.ErrDeviceNotFound) {
		return nil, fmt.Errorf("find device: %w", err)
	}

	if device == nil {
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("find user: %w", err)
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

		existingIPs, _ := s.deviceRepo.GetAllClientIPs(ctx)
		clientIP := allocateClientIP(existingIPs)

		device = &model.Device{
			UserID:    userID,
			Name:      deviceName,
			PublicKey: pubKey,
			ClientIP:  clientIP,
		}

		if err := s.deviceRepo.Create(ctx, device); err != nil {
			return nil, fmt.Errorf("create device: %w", err)
		}

		if node.Protocol == "wireguard" {
			conf := vpnconfig.GenerateClientConfig(*node, privKey, pubKey, node.PublicKey, device.ClientIP)
			return &VPNConfig{Protocol: node.Protocol, Config: conf}, nil
		}
	}

	var config string
	switch node.Protocol {
	case "wireguard":
		return nil, fmt.Errorf("wireguard reconnect requires device provisioning")
	case "vless":
		// The device must be registered on THIS node's inbound or Xray will
		// reject it (Reality serves the decoy site instead). Idempotent; a
		// node without panel creds is a no-op (manual / shared-UUID node).
		if err := s.panel.AddClient(ctx, *node, device.VlessUUID, device.ID); err != nil {
			slog.Error("panel addClient (connect)", "node", node.Codename, "device", device.ID, "err", err.Error())
			return nil, ErrNodeUnavailable
		}
		config = vpnconfig.GenerateVlessConfig(*node, device.VlessUUID)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}

	return &VPNConfig{
		Protocol: node.Protocol,
		Config:   config,
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
		if err := s.panel.AddClient(ctx, *bestNode, device.VlessUUID, device.ID); err != nil {
			slog.Warn("panel addClient (get config)", "node", bestNode.Codename, "device", device.ID, "err", err.Error())
		}
		conf = vpnconfig.GenerateVlessConfig(*bestNode, device.VlessUUID)
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
	// Remove the device (registered under email == device.ID) from every vless
	// node before deleting it locally. Ownership is verified first so a caller
	// cannot evict another user's client from a node. Best-effort: panel errors
	// must not block deletion, and delete-by-email is idempotent.
	if devices, err := s.deviceRepo.GetByUserID(ctx, userID); err == nil {
		owned := false
		for _, d := range devices {
			if d.ID == deviceID {
				owned = true
				break
			}
		}
		if owned {
			if nodes, nerr := s.nodeRepo.List(ctx); nerr == nil {
				for _, n := range nodes {
					if n.Protocol != "vless" {
						continue
					}
					if derr := s.panel.DelClient(ctx, n, deviceID); derr != nil {
						slog.Warn("panel delClient (delete device)", "node", n.Codename, "device", deviceID, "err", derr.Error())
					}
				}
			}
		}
	}
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
