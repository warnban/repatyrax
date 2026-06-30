// Package threexui is a minimal client for the 3x-ui panel HTTP API. The TYRAX
// backend uses it to register/remove per-device VLESS UUIDs on a node's inbound
// (login -> /panel/api/inbounds/addClient | delClient), so Xray on the node
// authenticates the same UUID the backend hands to the device.
package threexui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
)

const requestTimeout = 10 * time.Second

// Client talks to a single 3x-ui panel. Safe for concurrent use; it lazily logs
// in and re-authenticates once if the session has expired.
type Client struct {
	base string // e.g. https://1.2.3.4:2053/basepath (no trailing slash)
	user string
	pass string
	http *http.Client

	mu       sync.Mutex
	loggedIn bool
}

// NewClient builds a panel client. The panel port usually serves a self-signed
// certificate, so TLS verification is skipped on this admin-only channel.
func NewClient(base, user, pass string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		base: strings.TrimRight(base, "/"),
		user: user,
		pass: pass,
		http: &http.Client{
			Timeout: requestTimeout,
			Jar:     jar,
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

func (c *Client) login(ctx context.Context) error {
	form := url.Values{"username": {c.user}, "password": {c.pass}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("panel login: %w", err)
	}
	defer resp.Body.Close()

	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("panel login: bad response (status %d)", resp.StatusCode)
	}
	if !r.Success {
		return fmt.Errorf("panel login rejected: %s", r.Msg)
	}
	c.loggedIn = true
	return nil
}

// post sends a JSON body to an API path, logging in first and retrying once if
// the session looks expired.
func (c *Client) post(ctx context.Context, path string, body any) (*apiResp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}

	r, err := c.rawPost(ctx, path, body)
	if err == nil && r != nil {
		return r, nil
	}

	// Session may have expired — re-login once and retry.
	c.loggedIn = false
	if lerr := c.login(ctx); lerr != nil {
		if err != nil {
			return nil, err
		}
		return nil, lerr
	}
	return c.rawPost(ctx, path, body)
}

func (c *Client) rawPost(ctx context.Context, path string, body any) (*apiResp, error) {
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

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panel %s: %w", path, err)
	}
	defer resp.Body.Close()

	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		// A non-JSON body (e.g. the login HTML page) means the session expired.
		return nil, fmt.Errorf("panel %s: non-JSON response (status %d)", path, resp.StatusCode)
	}
	return &r, nil
}

// AddClient registers a VLESS client (UUID + email) on the given inbound. A
// "duplicate" response is treated as success so the call is idempotent.
func (c *Client) AddClient(ctx context.Context, inboundID int, clientUUID, email, flow string) error {
	clientObj := map[string]any{
		"id":         clientUUID,
		"flow":       flow,
		"email":      email,
		"enable":     true,
		"limitIp":    0,
		"totalGB":    0,
		"expiryTime": 0,
		"subId":      "",
		"tgId":       "",
		"reset":      0,
	}
	settings, err := json.Marshal(map[string]any{"clients": []any{clientObj}})
	if err != nil {
		return fmt.Errorf("marshal client settings: %w", err)
	}
	body := map[string]any{"id": inboundID, "settings": string(settings)}

	r, err := c.post(ctx, "/panel/api/inbounds/addClient", body)
	if err != nil {
		return err
	}
	if !r.Success {
		if isDuplicate(r.Msg) {
			return nil
		}
		return fmt.Errorf("addClient failed: %s", r.Msg)
	}
	return nil
}

// DelClient removes a client (by UUID) from the given inbound. A "not found"
// response is treated as success so the call is idempotent.
func (c *Client) DelClient(ctx context.Context, inboundID int, clientUUID string) error {
	path := fmt.Sprintf("/panel/api/inbounds/%d/delClient/%s", inboundID, clientUUID)
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
	key := node.PanelURL + "|" + node.PanelUser
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[key]
	if !ok {
		c = NewClient(node.PanelURL, node.PanelUser, node.PanelPass)
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

func (s *Syncer) DelClient(ctx context.Context, node model.Node, clientUUID string) error {
	c, ok := s.clientFor(node)
	if !ok {
		return nil
	}
	return c.DelClient(ctx, node.PanelInboundID, clientUUID)
}
