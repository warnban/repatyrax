// Package threexui is a minimal client for the 3x-ui panel HTTP API. The TYRAX
// backend uses it to register/remove per-device VLESS UUIDs on a node's inbound
// (/panel/api/inbounds/addClient | delClient), so Xray on the node authenticates
// the same UUID the backend hands to the device.
//
// Auth: a Bearer API token (created in the panel UI under Settings -> Security
// -> API Token). 3x-ui >= 3.x guards POST /login with CSRF middleware (403 for
// tokenless calls), but a valid Bearer token bypasses CSRF on all /panel/api/...
// routes, so no login/session handling is needed.
package threexui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const requestTimeout = 10 * time.Second

// Client talks to a single 3x-ui panel using a Bearer API token. Safe for
// concurrent use.
type Client struct {
	base  string // e.g. https://1.2.3.4:2053/basepath (no trailing slash)
	token string
	http  *http.Client
}

// NewClient builds a panel client. The panel port often serves a self-signed
// certificate (or plain HTTP behind a firewall), so TLS verification is skipped
// on this admin-only channel.
func NewClient(base, token string) *Client {
	return &Client{
		base:  strings.TrimRight(base, "/"),
		token: token,
		http: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

type apiResp struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

// clientTrafficResp is the shape of GET /panel/api/inbounds/getClientTraffics/:email.
// obj is null when the email has no recorded stats yet.
type clientTrafficResp struct {
	Success bool `json:"success"`
	Obj     *struct {
		Up   int64 `json:"up"`
		Down int64 `json:"down"`
	} `json:"obj"`
}

// onlinesResp is the shape of POST /panel/api/inbounds/onlines — obj is the list
// of client emails currently online across the panel's inbounds.
type onlinesResp struct {
	Success bool     `json:"success"`
	Obj     []string `json:"obj"`
}

// post sends a JSON body to an API path with the Bearer token attached.
func (c *Client) post(ctx context.Context, path string, body any) (*apiResp, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panel %s: %w", path, err)
	}
	defer resp.Body.Close()

	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		// A non-JSON body (e.g. an HTML login page or a 403 CSRF page) means the
		// token is missing/invalid or the base path is wrong.
		return nil, fmt.Errorf("panel %s: non-JSON response (status %d) — check panel_token/panel_url", path, resp.StatusCode)
	}
	return &r, nil
}

// GetClientTraffic returns the cumulative up+down bytes recorded for a client
// email on this panel (GET /panel/api/inbounds/getClientTraffics/:email). A
// missing email (obj == null) is not an error — it returns 0.
func (c *Client) GetClientTraffic(ctx context.Context, email string) (int64, error) {
	path := "/panel/api/inbounds/getClientTraffics/" + url.PathEscape(email)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("panel %s: %w", path, err)
	}
	defer resp.Body.Close()

	var r clientTrafficResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("panel %s: non-JSON response (status %d) — check panel_token/panel_url", path, resp.StatusCode)
	}
	if !r.Success || r.Obj == nil {
		return 0, nil
	}
	return r.Obj.Up + r.Obj.Down, nil
}

// Onlines returns how many clients are currently online on this panel
// (POST /panel/api/inbounds/onlines). Used as a live load metric for balancing.
func (c *Client) Onlines(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/panel/api/inbounds/onlines", bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("panel onlines: %w", err)
	}
	defer resp.Body.Close()

	var r onlinesResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("panel onlines: non-JSON response (status %d)", resp.StatusCode)
	}
	if !r.Success {
		return 0, fmt.Errorf("panel onlines: success=false")
	}
	return len(r.Obj), nil
}

// AddClient registers a VLESS client (UUID + email) and attaches it to the given
// inbound via the 3x-ui >= 3.1 client API (POST /panel/api/clients/add), then
// reloads Xray so the new UUID authenticates immediately.
//
// The clients/add endpoint only persists the client to the panel's SQLite DB —
// it does NOT reload the running Xray. Without an explicit restart the new UUID
// stays inactive (Reality serves the decoy site and the client silently "can't
// connect"). A duplicate ("email already in use") is treated as success and
// skips the restart: the client is already live from its original add, so there
// is nothing to apply and we avoid tearing down every live connection on the
// node on each reconnect / hourly subscription refresh.
func (c *Client) AddClient(ctx context.Context, inboundID int, clientUUID, email, flow string) error {
	body := map[string]any{
		"client": map[string]any{
			"id":         clientUUID,
			"email":      email,
			"flow":       flow,
			"enable":     true,
			"totalGB":    0,
			"expiryTime": 0,
			"limitIp":    0,
			"tgId":       0,
		},
		"inboundIds": []int{inboundID},
	}

	r, err := c.post(ctx, "/panel/api/clients/add", body)
	if err != nil {
		return err
	}
	if !r.Success {
		if isDuplicate(r.Msg) {
			return nil
		}
		return fmt.Errorf("addClient failed: %s", r.Msg)
	}
	// A genuinely new client was persisted — reload Xray so it takes effect.
	if err := c.RestartXray(ctx); err != nil {
		return fmt.Errorf("addClient ok but xray reload failed: %w", err)
	}
	return nil
}

// RestartXray reloads the Xray core on the node (POST /panel/api/server/
// restartXrayService). 3x-ui's client add/del endpoints only mutate the panel
// DB; Xray must be reloaded for the change to become active. Reload drops live
// connections for ~1-2s, so callers must invoke this ONLY when the client set
// actually changed (a new add or a real removal), never on duplicates/no-ops.
func (c *Client) RestartXray(ctx context.Context) error {
	r, err := c.post(ctx, "/panel/api/server/restartXrayService", map[string]any{})
	if err != nil {
		return err
	}
	if !r.Success {
		return fmt.Errorf("restartXray failed: %s", r.Msg)
	}
	return nil
}

// DelClient removes a client by email from every inbound it is attached to
// (POST /panel/api/clients/del/:email, 3x-ui >= 3.1). A "not found" response is
// treated as success so the call is idempotent.
func (c *Client) DelClient(ctx context.Context, email string) error {
	path := "/panel/api/clients/del/" + url.PathEscape(email)
	r, err := c.post(ctx, path, map[string]any{})
	if err != nil {
		return err
	}
	if !r.Success {
		if isNotFound(r.Msg) {
			return nil
		}
		return fmt.Errorf("delClient failed: %s", r.Msg)
	}
	// The client was actually removed — reload Xray so it loses access now
	// (device removal / limit enforcement must not linger in the live core).
	if err := c.RestartXray(ctx); err != nil {
		return fmt.Errorf("delClient ok but xray reload failed: %w", err)
	}
	return nil
}

func isDuplicate(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "duplicate") || strings.Contains(m, "exist") || strings.Contains(m, "already")
}

func isNotFound(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "not found") || strings.Contains(m, "no ") || strings.Contains(m, "exist")
}

// Syncer builds and caches a Client per node panel and implements the panel-sync
// surface the VPN service depends on. A node with an empty PanelURL is a no-op
// (manual / shared-UUID node), so WireGuard and unconfigured nodes are unaffected.
type Syncer struct {
	mu      sync.Mutex
	clients map[string]*Client
}

func NewSyncer() *Syncer {
	return &Syncer{clients: make(map[string]*Client)}
}

func (s *Syncer) clientFor(node model.Node) (*Client, bool) {
	if node.PanelURL == "" {
		return nil, false
	}
	key := node.PanelURL + "|" + node.PanelToken
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[key]
	if !ok {
		c = NewClient(node.PanelURL, node.PanelToken)
		s.clients[key] = c
	}
	return c, true
}

func (s *Syncer) AddClient(ctx context.Context, node model.Node, clientUUID, email string) error {
	c, ok := s.clientFor(node)
	if !ok {
		return nil
	}
	return c.AddClient(ctx, node.PanelInboundID, clientUUID, email, node.Flow)
}

func (s *Syncer) DelClient(ctx context.Context, node model.Node, email string) error {
	c, ok := s.clientFor(node)
	if !ok {
		return nil
	}
	return c.DelClient(ctx, email)
}

// ClientTraffic reads a device's cumulative traffic (bytes) from a node's panel.
// Nodes without panel credentials report 0 with no error (nothing to meter).
func (s *Syncer) ClientTraffic(ctx context.Context, node model.Node, email string) (int64, error) {
	c, ok := s.clientFor(node)
	if !ok {
		return 0, nil
	}
	return c.GetClientTraffic(ctx, email)
}

// Onlines reports the number of clients currently online on a node's panel.
// A node without panel credentials cannot be metered — returns an error so the
// balancer treats its load as unknown (fail-open to ping ordering).
func (s *Syncer) Onlines(ctx context.Context, node model.Node) (int, error) {
	c, ok := s.clientFor(node)
	if !ok {
		return 0, fmt.Errorf("node %s has no panel", node.Codename)
	}
	return c.Onlines(ctx)
}
