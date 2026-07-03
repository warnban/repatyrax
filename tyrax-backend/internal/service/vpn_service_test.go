package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// stubPanel matches the PanelSyncer interface (AddClient + DelClient). AddClient
// returns whatever addErr is set to so we can simulate a genuine sync failure.
type stubPanel struct{ addErr error }

func (s stubPanel) AddClient(ctx context.Context, n model.Node, uuid, email string) error { return s.addErr }
func (s stubPanel) DelClient(ctx context.Context, n model.Node, email string) error       { return nil }

// stubNodeRepo implements repository.NodeRepository, always returning node.
type stubNodeRepo struct{ node *model.Node }

func (r stubNodeRepo) List(ctx context.Context) ([]model.Node, error)                        { return []model.Node{*r.node}, nil }
func (r stubNodeRepo) FindByID(ctx context.Context, id string) (*model.Node, error)          { return r.node, nil }
func (r stubNodeRepo) GetByCodename(ctx context.Context, codename string) (*model.Node, error) { return r.node, nil }
func (r stubNodeRepo) UpdatePing(ctx context.Context, nodeID string, pingMS int) error       { return nil }
func (r stubNodeRepo) GetBest(ctx context.Context) (*model.Node, error)                      { return r.node, nil }

// stubDeviceRepo implements repository.DeviceRepository, always returning device.
type stubDeviceRepo struct{ device *model.Device }

func (r stubDeviceRepo) Create(ctx context.Context, device *model.Device) error { return nil }
func (r stubDeviceRepo) CountByUser(ctx context.Context, userID string) (int, error) { return 1, nil }
func (r stubDeviceRepo) GetAllClientIPs(ctx context.Context) ([]string, error) { return nil, nil }
func (r stubDeviceRepo) GetByUserID(ctx context.Context, userID string) ([]model.Device, error) {
	return nil, nil
}
func (r stubDeviceRepo) FindByPublicKey(ctx context.Context, publicKey string) (*model.Device, error) {
	return r.device, nil
}
func (r stubDeviceRepo) FindByUserAndName(ctx context.Context, userID, name string) (*model.Device, error) {
	return r.device, nil
}
func (r stubDeviceRepo) Delete(ctx context.Context, id, userID string) error                 { return nil }
func (r stubDeviceRepo) ListForAccounting(ctx context.Context) ([]model.Device, error)       { return nil, nil }
func (r stubDeviceRepo) UpdateLastTraffic(ctx context.Context, deviceID string, total int64) error {
	return nil
}

// connectAddClientFails builds a vpnService with an existing device on an OPEN
// vless node whose panel AddClient fails, then reports whether Connect surfaces
// ErrNodeUnavailable (rather than handing out a config for an unconfirmed UUID).
func connectAddClientFails(t *testing.T, node model.Node, addErr error) bool {
	t.Helper()
	node.Status = model.NodeOpen // ensure the vless branch runs, not the status gate
	device := &model.Device{ID: "dev-1", UserID: "user-1", Name: "TYRAX", VlessUUID: "uuid-123"}
	svc := NewVPNService(
		stubNodeRepo{node: &node},
		stubDeviceRepo{device: device},
		nil, // userRepo — unused when the device already exists
		nil, // connRepo — unused on the failure path
		stubPanel{addErr: addErr},
		nil, // traffic guard — fail-open
		nil, // load provider
	)
	_, err := svc.Connect(context.Background(), device.UserID, device.Name, node.Codename)
	return errors.Is(err, ErrNodeUnavailable)
}

func TestConnect_VlessAddClientFailure_ReturnsNodeUnavailable(t *testing.T) {
	node := model.Node{Codename: "NL-01", Protocol: "vless", PanelURL: "https://p", PanelInboundID: 1}
	if !connectAddClientFails(t, node, errors.New("addClient failed: bad inbound")) {
		t.Fatal("expected ErrNodeUnavailable when panel AddClient fails")
	}
}
