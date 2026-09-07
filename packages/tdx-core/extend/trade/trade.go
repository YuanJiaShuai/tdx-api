// Package trade provides the HTTP client used with TdxTradeServer.
// It is intentionally separate from the TDX market-data Client: this API
// talks to a local broker adapter and can place real orders.
package trade

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// QueryCategory identifies TdxTradeServer query_data categories.
type QueryCategory int

const (
	QueryCash QueryCategory = iota
	QueryStocks
	QueryOrdersToday
	QueryDealsToday
	QueryCancelableOrders
	QueryShareholders
	QueryMarginBalance
	QueryStockLoanBalance
	QueryOperableMarginStocks
	QueryNewStocks QueryCategory = 12
	QueryNewStockQuota
	QueryNewStockNumbers
	QueryNewStockHits
)

// Order is an order payload accepted by send_orders.
type Order struct {
	Category  int     `json:"category"`
	PriceType int     `json:"price_type"`
	GDDM      string  `json:"gddm"`
	ZQDM      string  `json:"zqdm"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

// CancelOrder is a cancellation payload accepted by cancel_orders.
type CancelOrder struct {
	ExchangeID string `json:"exchange_id"`
	HTH        string `json:"hth"`
}

// Response is the unmodified TdxTradeServer JSON response.
type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// API is a TdxTradeServer HTTP client.
type API struct {
	Endpoint string
	Client   *http.Client
	Encoding string
	key      []byte
	iv       []byte
}

// New creates a trade API client. key and iv may both be nil to disable transport encryption.
func New(endpoint string, key, iv []byte) (*API, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("trade endpoint is empty")
	}
	if (key == nil) != (iv == nil) {
		return nil, errors.New("trade AES key and IV must be supplied together")
	}
	if key != nil {
		if _, err := aes.NewCipher(key); err != nil {
			return nil, fmt.Errorf("invalid trade AES key: %w", err)
		}
		if len(iv) != aes.BlockSize {
			return nil, fmt.Errorf("invalid trade AES IV length: %d", len(iv))
		}
	}
	return &API{Endpoint: endpoint, Client: http.DefaultClient, Encoding: "utf-8", key: append([]byte(nil), key...), iv: append([]byte(nil), iv...)}, nil
}

// Call invokes a TdxTradeServer function.
func (a *API) Call(ctx context.Context, function string, params any) (*Response, error) {
	payload := map[string]any{"func": function}
	if params != nil {
		payload["params"] = params
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if a.encrypted() {
		encoded, err := a.encrypt(plain)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(encoded)
	} else {
		body = bytes.NewReader(plain)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint, body)
	if err != nil {
		return nil, err
	}
	if !a.encrypted() {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := a.Client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trade server HTTP status %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if a.encrypted() {
		data, err = a.decrypt(string(data))
		if err != nil {
			return nil, err
		}
	}
	var result Response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode trade response: %w", err)
	}
	return &result, nil
}

func (a *API) encrypted() bool { return len(a.key) > 0 }

func (a *API) encrypt(plain []byte) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}
	padded := make([]byte, ((len(plain)/aes.BlockSize)+1)*aes.BlockSize)
	copy(padded, plain)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, a.iv).CryptBlocks(out, padded)
	return url.QueryEscape(base64.StdEncoding.EncodeToString(out)), nil
}

func (a *API) decrypt(encoded string) ([]byte, error) {
	encoded, err := url.QueryUnescape(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("invalid encrypted trade response length")
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, a.iv).CryptBlocks(out, ciphertext)
	return bytes.TrimRight(out, "\x00"), nil
}

// Ping checks the local trade server.
func (a *API) Ping(ctx context.Context) (*Response, error) {
	return a.Call(ctx, "ping", map[string]any{})
}

// Logon logs into a broker connection managed by TdxTradeServer.
func (a *API) Logon(ctx context.Context, ip string, port int, version string, yybID int, accountID, tradeAccount, tradePassword, txPassword string) (*Response, error) {
	return a.Call(ctx, "logon", map[string]any{"ip": ip, "port": port, "version": version, "yyb_id": yybID, "account_no": accountID, "trade_account": tradeAccount, "jy_password": tradePassword, "tx_password": txPassword})
}

func (a *API) Logoff(ctx context.Context, clientID any) (*Response, error) {
	return a.Call(ctx, "logoff", map[string]any{"client_id": clientID})
}
func (a *API) QueryData(ctx context.Context, clientID any, category QueryCategory) (*Response, error) {
	return a.Call(ctx, "query_data", map[string]any{"client_id": clientID, "category": category})
}
func (a *API) QueryHistoryData(ctx context.Context, clientID any, category QueryCategory, beginDate, endDate string) (*Response, error) {
	return a.Call(ctx, "query_history_data", map[string]any{"client_id": clientID, "category": category, "begin_date": beginDate, "end_date": endDate})
}
func (a *API) QueryDatas(ctx context.Context, clientID any, categories []QueryCategory) (*Response, error) {
	return a.Call(ctx, "query_datas", map[string]any{"client_id": clientID, "categories": categories})
}
func (a *API) SendOrder(ctx context.Context, clientID any, order Order) (*Response, error) {
	return a.Call(ctx, "send_order", map[string]any{"client_id": clientID, "category": order.Category, "price_type": order.PriceType, "gddm": order.GDDM, "zqdm": order.ZQDM, "price": order.Price, "quantity": order.Quantity})
}
func (a *API) SendOrders(ctx context.Context, clientID any, orders []Order) (*Response, error) {
	return a.Call(ctx, "send_orders", map[string]any{"client_id": clientID, "orders": orders})
}
func (a *API) CancelOrder(ctx context.Context, clientID any, order CancelOrder) (*Response, error) {
	return a.Call(ctx, "cancel_order", map[string]any{"client_id": clientID, "exchange_id": order.ExchangeID, "hth": order.HTH})
}
func (a *API) CancelOrders(ctx context.Context, clientID any, orders []CancelOrder) (*Response, error) {
	return a.Call(ctx, "cancel_orders", map[string]any{"client_id": clientID, "orders": orders})
}
func (a *API) GetQuote(ctx context.Context, clientID any, code string) (*Response, error) {
	return a.Call(ctx, "get_quote", map[string]any{"client_id": clientID, "code": code})
}
func (a *API) GetQuotes(ctx context.Context, clientID any, codes []string) (*Response, error) {
	return a.Call(ctx, "get_quotes", map[string]any{"client_id": clientID, "zqdms": codes})
}
func (a *API) Repay(ctx context.Context, clientID any, amount float64) (*Response, error) {
	return a.Call(ctx, "repay", map[string]any{"client_id": clientID, "amount": amount})
}
func (a *API) GetActiveClients(ctx context.Context) (*Response, error) {
	return a.Call(ctx, "get_active_clients", nil)
}
