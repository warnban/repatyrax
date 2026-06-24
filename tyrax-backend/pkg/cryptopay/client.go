package cryptopay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiBase     = "https://pay.crypt.bot/api"
	tyraxBotURL = "https://t.me/tyraxvpnbot"
)

// Client holds the CryptoPay API token.
type Client struct {
	token      string
	httpClient *http.Client
}

// New creates a Client from the provided API token.
func New(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Invoice is the relevant subset of a CryptoPay invoice response.
type Invoice struct {
	InvoiceID      int64  `json:"invoice_id"`
	Status         string `json:"status"`
	BotInvoiceURL  string `json:"bot_invoice_url"`
	MiniAppURL     string `json:"mini_app_invoice_url"`
	WebAppURL      string `json:"web_app_invoice_url"`
	ExpirationDate string `json:"expiration_date"`
}

type createInvoiceBody struct {
	CurrencyType   string  `json:"currency_type"`
	Fiat           string  `json:"fiat"`
	AcceptedAssets string  `json:"accepted_assets"`
	Amount         string  `json:"amount"`
	Description    string  `json:"description"`
	Payload        string  `json:"payload"`
	PaidBtnName    string  `json:"paid_btn_name"`
	PaidBtnURL     string  `json:"paid_btn_url"`
	ExpiresIn      int     `json:"expires_in"`
}

type apiResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

// CreateInvoice creates a fiat-denominated (RUB) CryptoPay invoice.
// payload is encoded as "userID|orderID" per PAYMENTS.md.
// Returns the full Invoice including bot_invoice_url.
func (c *Client) CreateInvoice(ctx context.Context, amountRUB float64, tier, userID, orderID string) (*Invoice, error) {
	reqBody := createInvoiceBody{
		CurrencyType:   "fiat",
		Fiat:           "RUB",
		AcceptedAssets: "USDT,TON,BTC",
		Amount:         fmt.Sprintf("%.2f", amountRUB),
		Description:    "TYRAX " + tier,
		Payload:        userID + "|" + orderID,
		PaidBtnName:    "openBot",
		PaidBtnURL:     tyraxBotURL,
		ExpiresIn:      3600,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cryptopay: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/createInvoice", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cryptopay: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Crypto-Pay-API-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cryptopay: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cryptopay: read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("cryptopay: decode response: %w", err)
	}
	if !apiResp.OK {
		return nil, fmt.Errorf("cryptopay: INVOICE CREATION FAILED: %s", string(raw))
	}

	var invoice Invoice
	if err := json.Unmarshal(apiResp.Result, &invoice); err != nil {
		return nil, fmt.Errorf("cryptopay: decode invoice: %w", err)
	}
	return &invoice, nil
}

// VerifyWebhook validates the crypto-pay-api-signature header per PAYMENTS.md:
// secret = sha256(token), signature = HMAC-SHA256(secret, body).
func (c *Client) VerifyWebhook(body []byte, receivedSignature string) bool {
	secretArr := sha256.Sum256([]byte(c.token))
	mac := hmac.New(sha256.New, secretArr[:])
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return expected == receivedSignature
}
