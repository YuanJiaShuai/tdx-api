package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type MarketServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

type marketEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type MarketQuoteStreamEvent struct {
	Type     string               `json:"type"`
	Snapshot *MarketQuoteSnapshot `json:"snapshot,omitempty"`
}

type MarketQuoteSnapshot struct {
	Symbol string          `json:"symbol"`
	Code   string          `json:"code"`
	Quote  *protocol.Quote `json:"quote"`
}

func NewMarketServiceClient() *MarketServiceClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MARKET_SERVICE_URL")), "/")
	return &MarketServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (c *MarketServiceClient) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *MarketServiceClient) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	if !c.Enabled() {
		return errors.New("MARKET_SERVICE_URL 未配置")
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("market-service %s: %s", resp.Status, string(body))
	}
	var envelope marketEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		if envelope.Message == "" {
			envelope.Message = "market-service 返回失败"
		}
		return errors.New(envelope.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *MarketServiceClient) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	if !c.Enabled() {
		return errors.New("MARKET_SERVICE_URL 未配置")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("market-service %s: %s", resp.Status, string(respBody))
	}
	var envelope marketEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		if envelope.Message == "" {
			envelope.Message = "market-service 返回失败"
		}
		return errors.New(envelope.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *MarketServiceClient) KlineHistory(ctx context.Context, code, klineType string, limit int) (*protocol.KlineResp, error) {
	query := url.Values{}
	query.Set("code", code)
	query.Set("type", klineType)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var resp protocol.KlineResp
	if err := c.get(ctx, "/api/kline-history", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *MarketServiceClient) IndexKline(ctx context.Context, code, klineType string, limit int) (*protocol.KlineResp, error) {
	query := url.Values{}
	query.Set("code", code)
	query.Set("type", klineType)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var resp protocol.KlineResp
	if err := c.get(ctx, "/api/index", query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *MarketServiceClient) Codes(ctx context.Context) ([]*tdx.CodeModel, error) {
	var resp struct {
		Codes []struct {
			Code     string `json:"code"`
			Name     string `json:"name"`
			Exchange string `json:"exchange"`
		} `json:"codes"`
	}
	if err := c.get(ctx, "/api/codes", nil, &resp); err != nil {
		return nil, err
	}
	models := make([]*tdx.CodeModel, 0, len(resp.Codes))
	for _, item := range resp.Codes {
		models = append(models, &tdx.CodeModel{
			Name:     item.Name,
			Code:     item.Code,
			Exchange: item.Exchange,
		})
	}
	return models, nil
}

func (c *MarketServiceClient) StockCodes(ctx context.Context, maxCodes int) ([]string, error) {
	query := url.Values{}
	query.Set("prefix", "false")
	if maxCodes > 0 {
		query.Set("limit", strconv.Itoa(maxCodes))
	}
	var resp struct {
		List []string `json:"list"`
	}
	if err := c.get(ctx, "/api/stock-codes", query, &resp); err != nil {
		return nil, err
	}
	return resp.List, nil
}

func (c *MarketServiceClient) RawData(ctx context.Context, path string, query url.Values) (interface{}, error) {
	var raw interface{}
	if err := c.get(ctx, path, query, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *MarketServiceClient) SubscribeQuotes(ctx context.Context, codes []string, onEvent func(MarketQuoteStreamEvent)) error {
	if !c.Enabled() {
		return errors.New("MARKET_SERVICE_URL 未配置")
	}
	query := url.Values{}
	query.Set("codes", strings.Join(codes, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/stream/quotes?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("market-service SSE %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	eventType := ""
	data := ""
	emit := func() {
		if data == "" {
			return
		}
		var event MarketQuoteStreamEvent
		if json.Unmarshal([]byte(data), &event) == nil {
			if event.Type == "" {
				event.Type = eventType
			}
			onEvent(event)
		}
		eventType = ""
		data = ""
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data != "" {
				data += "\n"
			}
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			emit()
		}
	}
	emit()
	return scanner.Err()
}

func (c *MarketServiceClient) PullKlineTask(ctx context.Context, payload SystemSyncPayload, tables []string, codes []string) (map[string]interface{}, error) {
	req := map[string]interface{}{
		"codes":      codes,
		"tables":     tables,
		"limit":      payload.Limit,
		"start_date": payload.StartDate,
	}
	var resp map[string]interface{}
	if err := c.post(ctx, "/api/tasks/pull-kline", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *MarketServiceClient) SyncGbbqTask(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.post(ctx, "/api/tasks/sync-gbbq", map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
