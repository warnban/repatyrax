package freekassa

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"  //nolint:gosec // md5 required by FreeKassa webhook verification spec
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	apiBase = "https://api.fk.life/v1"

	// Trusted IPs from PAYMENTS.md — only these may deliver webhooks.
	TrustedIP1 = "168.119.157.136"
	TrustedIP2 = "168.119.60.227"
	TrustedIP3 = "178.154.197.79"
	TrustedIP4 = "51.250.54.238"

	// Payment method IDs per PAYMENTS.md.
	MethodSBP    = 44
	MethodCardRF = 36
)

// IsTrustedIP reports whether the given IP is an authorised FreeKassa webhook sender.
func IsTrustedIP(ip string) bool {
	switch ip {
	case TrustedIP1, TrustedIP2, TrustedIP3, TrustedIP4:
		return true
	}
	return false
}

// Client holds FreeKassa credentials loaded from env.
type Client struct {
	ShopID       int
	APIKey       string
	SecretWord2  string
	httpClient   *http.Client
}

// New creates a Client from the provided credentials.
func New(shopID int, apiKey, secretWord2 string) *Client {
	return &Client{
		ShopID:      shopID,
		APIKey:      apiKey,
		SecretWord2: secretWord2,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// CreateOrderResponse is the relevant subset of the FreeKassa order-create response.
type CreateOrderResponse struct {
	Type      string `json:"type"`
	OrderID   int64  `json:"orderId"`
	OrderHash string `json:"orderHash"`
	Location  string `json:"location"` // payment URL to redirect the user
}

// CreateOrder creates a new FreeKassa payment order.
// paymentMethodID must be MethodSBP (44) or MethodCardRF (36).
// internalOrderID is the TYRAX order UUID used to reconcile the webhook.
func (c *Client) CreateOrder(
	ctx context.Context,
	paymentMethodID int,
	email, ip string,
	amount float64,
	internalOrderID string,
) (*CreateOrderResponse, error) {
	params := map[string]any{
		"shopId":    c.ShopID,
		"nonce":     time.Now().UnixMilli(), // must be strictly increasing per spec
		"i":         paymentMethodID,
		"email":     email,
		"ip":        ip,
		"amount":    amount,
		"currency":  "RUB",
		"paymentId": internalOrderID,
	}
	params["signature"] = generateSignature(params, c.APIKey)

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("freekassa: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/orders/create", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("freekassa: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("freekassa: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("freekassa: read response: %w", err)
	}

	var result CreateOrderResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("freekassa: decode response: %w", err)
	}
	if result.Type != "success" {
		return nil, fmt.Errorf("freekassa: ORDER CREATION FAILED: %s", string(raw))
	}
	return &result, nil
}

// VerifyWebhook checks a FreeKassa webhook notification using the PAYMENTS.md algorithm:
// md5(merchantID + ":" + amount + ":" + SecretWord2 + ":" + orderID)
// Note: FreeKassa uses md5 (not SHA-256) for webhook signatures — this is per-spec.
func (c *Client) VerifyWebhook(merchantID, amount, orderID, receivedSign string) bool {
	raw := merchantID + ":" + amount + ":" + c.SecretWord2 + ":" + orderID
	//nolint:gosec
	sum := md5.Sum([]byte(raw))
	expected := hex.EncodeToString(sum[:])
	return strings.EqualFold(expected, receivedSign)
}

// generateSignature builds the HMAC-SHA256 signature required by the FreeKassa API.
// Algorithm from PAYMENTS.md: sort params by key (ksort), join values with "|", HMAC-SHA256.
func generateSignature(params map[string]any, apiKey string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([]string, 0, len(keys))
	for _, k := range keys {
		values = append(values, fmt.Sprintf("%v", params[k]))
	}
	message := strings.Join(values, "|")

	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
