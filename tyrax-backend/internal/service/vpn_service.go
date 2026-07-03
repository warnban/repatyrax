package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"

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

// TrafficGuard reports whether a user's tunnel is currently blocked (e.g. a FREE
// identity that exhausted its quota). Implemented by *TrafficService. Optional:
// a nil guard disables enforcement entirely (fail-open).
type TrafficGuard interface {
	CheckBlocked(ctx context.Context, userID string) (bool, error)
}

// NodeLoadProvider returns a node's live online-client count and whether that
// reading is fresh enough to trust. Implemented by *NodeBalancer. Optional: a
// nil provider disables load balancing (fail-open to ping ordering).
type NodeLoadProvider interface {
	NodeLoad(nodeID string) (count int, fresh bool)
}

var (
	ErrDeviceLimitReached = errors.New("DEVICE LIMIT REACHED")
	ErrNodeUnavailable    = errors.New("NODE UNAVAILABLE")
	ErrTrafficLimit       = errors.New("TRAFFIC LIMIT REACHED")
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
	// Load is the live online-client count used for balancing, or -1 when unknown
	// (never sampled / stale / no panel). Exposed for observability.
	Load int `json:"load"`
}

type VPNService interface {
	AddDevice(ctx context.Context, userID, deviceName string) (*DeviceConfig, error)
	Connect(ctx context.Context, userID, deviceName, codename string) (*VPNConfig, error)
	GetConfig(ctx context.Context, userID, devicePublicKey string) (*VPNConfig, error)
	GetNodes(ctx context.Context) ([]NodeResponse, error)
	ListDevices(ctx context.Context, userID string) ([]model.Device, error)
	DeleteDevice(ctx context.Context, deviceID, userID string) error
	GetSplitDomains(ctx context.Context) ([]string, error)
	RecordDisconnect(ctx context.Context, userID string) error
}

type vpnService struct {
	nodeRepo   repository.NodeRepository
	deviceRepo repository.DeviceRepository
	userRepo   repository.UserRepository
	connRepo   repository.ConnectionRepository
	panel      PanelSyncer
	traffic    TrafficGuard
	load       NodeLoadProvider
}

func NewVPNService(nodeRepo repository.NodeRepository, deviceRepo repository.DeviceRepository, userRepo repository.UserRepository, connRepo repository.ConnectionRepository, panel PanelSyncer, traffic TrafficGuard, load NodeLoadProvider) VPNService {
	return &vpnService{
		nodeRepo:   nodeRepo,
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		connRepo:   connRepo,
		panel:      panel,
		traffic:    traffic,
		load:       load,
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
	// FREE-tier quota gate. Fail-open: only refuse when the guard is certain the
	// user is blocked; any error leaves the tunnel working exactly as before.
	if s.traffic != nil {
		if blocked, gerr := s.traffic.CheckBlocked(ctx, userID); gerr == nil && blocked {
			return nil, ErrTrafficLimit
		}
	}

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
		// Register on this node's inbound before handing config. Best-effort:
		// AddDevice already tries once; a transient panel failure must not block
		// connect when the UUID is already on the inbound.
		if err := s.panel.AddClient(ctx, *node, device.VlessUUID, device.ID); err != nil {
			slog.Warn("panel addClient (connect)", "node", node.Codename, "device", device.ID, "err", err.Error())
		}
		config = vpnconfig.GenerateVlessConfig(*node, device.VlessUUID)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}

	cfg := &VPNConfig{
		Protocol: node.Protocol,
		Config:   config,
	}
	s.afterConnect(ctx, userID, node)
	return cfg, nil
}

func (s *vpnService) afterConnect(ctx context.Context, userID string, node *model.Node) {
	_ = s.userRepo.TouchLastSeen(ctx, userID)
	if s.connRepo != nil {
		if _, err := s.connRepo.LogConnect(ctx, userID, node.ID, node.Protocol); err != nil {
			slog.Warn("connection log", slog.String("user_id", userID), slog.String("error", err.Error()))
		}
	}
}

func (s *vpnService) RecordDisconnect(ctx context.Context, userID string) error {
	if s.connRepo == nil {
		return nil
	}
	return s.connRepo.LogDisconnect(ctx, userID)
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

	// nodeRepo.List returns ping-ascending order; balanceOrder promotes the
	// least-loaded OPEN nodes to the front (fail-open to the ping order).
	nodes = s.balanceOrder(nodes)

	resp := make([]NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		load := -1
		if s.load != nil {
			if c, fresh := s.load.NodeLoad(n.ID); fresh {
				load = c
			}
		}
		resp = append(resp, NodeResponse{
			ID:       n.ID,
			Codename: n.Codename,
			Country:  n.Country,
			Status:   string(n.Status),
			PingMS:   n.PingMS,
			Load:     load,
		})
	}
	return resp, nil
}

// balanceOrder reorders nodes so clients (which honour server order) try the
// least-loaded node first. Only OPEN vless nodes with a fresh load reading take
// part; everything else keeps the incoming ping order and trails behind. If no
// node has a fresh reading, the original ping order is returned unchanged so the
// tunnel behaves exactly as before (fail-open).
func (s *vpnService) balanceOrder(nodes []model.Node) []model.Node {
	if s.load == nil {
		return nodes
	}

	balanced := make([]model.Node, 0, len(nodes))
	rest := make([]model.Node, 0, len(nodes))
	loadOf := make(map[string]int, len(nodes))

	for _, n := range nodes {
		if n.Status == model.NodeOpen && n.Protocol == "vless" && n.PanelURL != "" {
			if c, fresh := s.load.NodeLoad(n.ID); fresh {
				loadOf[n.ID] = c
				balanced = append(balanced, n)
				continue
			}
		}
		rest = append(rest, n)
	}

	if len(balanced) == 0 {
		return nodes // nothing fresh to balance on — keep ping order
	}

	// Shuffle first, then stable-sort by load: equal-load nodes end up in random
	// relative order, spreading simultaneous connects instead of herding them all
	// onto one node within a sampling window.
	rand.Shuffle(len(balanced), func(i, j int) { balanced[i], balanced[j] = balanced[j], balanced[i] })
	sort.SliceStable(balanced, func(i, j int) bool {
		return loadOf[balanced[i].ID] < loadOf[balanced[j].ID]
	})

	return append(balanced, rest...)
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
		"yandex.ru", "ya.ru", "yandex.net", "dzen.ru", "vk.com", "vkontakte.ru", "ok.ru", "mail.ru",
		"max.ru", "oneme.ru",
		"gosuslugi.ru", "nalog.gov.ru", "gostech.ru", "mos.ru",
		"sberbank.ru", "sberbank.com", "sber.ru", "tinkoff.ru", "tbank.ru", "vtb.ru", "alfabank.ru", "raiffeisen.ru",
		"ozon.ru", "ozon.com", "wildberries.ru", "wildberries.com", "megamarket.ru", "aliexpress.ru",
		"mvideo.ru", "dns-shop.ru", "citilink.ru",
		"avito.ru", "hh.ru", "kinopoisk.ru", "ivi.ru", "rutube.ru",
		"2gis.ru", "gismeteo.ru", "drom.ru", "auto.ru", "rbc.ru", "kommersant.ru", "ria.ru", "lenta.ru", "meduza.io",
	}, nil
}
