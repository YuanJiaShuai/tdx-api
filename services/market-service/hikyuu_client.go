package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/injoyai/tdx/protocol"
)

type HikyuuDataServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

type hikyuuEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type hikyuuKlineResponse struct {
	Symbol  string                 `json:"symbol"`
	Period  string                 `json:"period"`
	Recover string                 `json:"recover"`
	Count   int                    `json:"count"`
	List    []hikyuuKlineItem      `json:"list"`
	Meta    map[string]interface{} `json:"meta"`
}

type hikyuuKlineItem struct {
	Time   string  `json:"time"`
	Last   float64 `json:"last"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

func NewHikyuuDataServiceClient() *HikyuuDataServiceClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("HIKYUU_DATA_SERVICE_URL")), "/")
	return &HikyuuDataServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (c *HikyuuDataServiceClient) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *HikyuuDataServiceClient) queryKline(ctx context.Context, code, klineType, recover string, limit int) (*hikyuuKlineResponse, error) {
	if !c.Enabled() {
		return nil, errors.New("HIKYUU_DATA_SERVICE_URL 未配置")
	}
	query := url.Values{}
	query.Set("code", code)
	query.Set("type", klineType)
	query.Set("recover", recover)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	endpoint := c.baseURL + "/api/hikyuu/kline?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var envelope hikyuuEnvelope
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hikyuu-data-service %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 {
		if envelope.Message == "" {
			envelope.Message = "hikyuu-data-service 返回失败"
		}
		return nil, errors.New(envelope.Message)
	}
	if len(envelope.Data) == 0 {
		return nil, errors.New("hikyuu-data-service 返回空数据")
	}
	var result hikyuuKlineResponse
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HikyuuDataServiceClient) FetchKline(ctx context.Context, code, klineType, recover string, limit int) (*protocol.KlineResp, error) {
	symbol, err := NormalizeSymbol(code)
	if err != nil {
		return nil, err
	}
	code = symbol.Symbol

	resp, err := c.queryKline(ctx, code, klineType, recover, limit)
	if err != nil {
		return nil, err
	}
	list := make([]*protocol.Kline, 0, len(resp.List))
	for _, item := range resp.List {
		t, err := time.Parse(time.RFC3339, item.Time)
		if err != nil {
			return nil, err
		}
		list = append(list, &protocol.Kline{
			Time:   t,
			Last:   protocol.Yuan(item.Last),
			Open:   protocol.Yuan(item.Open),
			High:   protocol.Yuan(item.High),
			Low:    protocol.Yuan(item.Low),
			Close:  protocol.Yuan(item.Close),
			Volume: hikyuuVolumeToTDX(klineType, item.Volume),
			Amount: protocol.Yuan(item.Amount * 10000),
		})
	}
	count := len(list)
	if count > int(^uint16(0)) {
		count = int(^uint16(0))
	}
	return &protocol.KlineResp{
		Count: uint16(count),
		List:  list,
	}, nil
}

func hikyuuVolumeToTDX(klineType string, volume float64) int64 {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(klineType)), "minute") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(klineType)), "min") {
		return int64(math.Round(volume / 100))
	}
	return int64(math.Round(volume))
}

func fetchHikyuuKline(ctx context.Context, code, klineType string, limit int) (*protocol.KlineResp, error) {
	if hikyuuClient == nil {
		return nil, errors.New("hikyuu 数据服务未初始化")
	}

	period := strings.ToLower(strings.TrimSpace(klineType))
	if period == "" {
		period = "day"
	}
	switch period {
	case "day", "week", "month", "minute1", "minute5":
	default:
		return nil, fmt.Errorf("hikyuu 不支持K线类型: %s", klineType)
	}

	recover := "none"
	switch period {
	case "day", "week", "month":
		recover = "qfq"
	}
	return hikyuuClient.FetchKline(ctx, code, period, recover, limit)
}
