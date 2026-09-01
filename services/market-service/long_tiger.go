package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const longTigerCacheTTL = 10 * time.Second

var longTigerFetchMu sync.Mutex

type LongTigerRank struct {
	AccumAmount      float64 `json:"ACCUM_AMOUNT"`
	BillboardBuyAmt  float64 `json:"BILLBOARD_BUY_AMT"`
	BillboardDealAmt float64 `json:"BILLBOARD_DEAL_AMT"`
	BillboardNetAmt  float64 `json:"BILLBOARD_NET_AMT"`
	BillboardSellAmt float64 `json:"BILLBOARD_SELL_AMT"`
	ChangeRate       float64 `json:"CHANGE_RATE"`
	ClosePrice       float64 `json:"CLOSE_PRICE"`
	DealAmountRatio  float64 `json:"DEAL_AMOUNT_RATIO"`
	DealNetRatio     float64 `json:"DEAL_NET_RATIO"`
	Explain          string  `json:"EXPLAIN"`
	Explanation      string  `json:"EXPLANATION"`
	FreeMarketCap    float64 `json:"FREE_MARKET_CAP"`
	SecuCode         string  `json:"SECUCODE"`
	SecurityCode     string  `json:"SECURITY_CODE"`
	SecurityName     string  `json:"SECURITY_NAME_ABBR"`
	SecurityTypeCode string  `json:"SECURITY_TYPE_CODE"`
	TradeDate        string  `json:"TRADE_DATE"`
	TurnoverRate     float64 `json:"TURNOVERRATE"`
}

type LongTigerResponse struct {
	RequestedDate string          `json:"requested_date"`
	TradeDate     string          `json:"trade_date"`
	Items         []LongTigerRank `json:"items"`
	Source        string          `json:"source"`
	CachedAt      string          `json:"cached_at,omitempty"`
}

type longTigerCacheRow struct {
	Data      string
	FetchedAt string
}

func handleLongTiger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		errorResponse(w, "date 必须是 YYYY-MM-DD 格式")
		return
	}
	if marketStore == nil {
		errorResponse(w, "行情服务未初始化")
		return
	}

	result, err := loadLongTiger(r.Context(), marketStore, date)
	if err != nil {
		errorResponse(w, "获取龙虎榜失败: "+err.Error())
		return
	}
	successResponse(w, result)
}

func loadLongTiger(ctx context.Context, store *MarketStore, date string) (LongTigerResponse, error) {
	longTigerFetchMu.Lock()
	defer longTigerFetchMu.Unlock()

	cached, cacheErr := store.getLongTigerCache(date)
	if cacheErr == nil {
		items, err := decodeLongTigerItems(cached.Data)
		if err == nil {
			return LongTigerResponse{
				RequestedDate: date,
				TradeDate:     date,
				Items:         items,
				Source:        "database",
				CachedAt:      cached.FetchedAt,
			}, nil
		}
	}

	items, err := fetchLongTigerUpstream(ctx, date)
	if err != nil {
		if cacheErr == nil {
			if staleItems, decodeErr := decodeLongTigerItems(cached.Data); decodeErr == nil {
				return LongTigerResponse{
					RequestedDate: date,
					TradeDate:     date,
					Items:         staleItems,
					Source:        "stale-database",
					CachedAt:      cached.FetchedAt,
				}, nil
			}
		}
		return LongTigerResponse{}, err
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return LongTigerResponse{}, err
	}
	fetchedAt := time.Now().Format(time.RFC3339)
	if err := store.saveLongTigerCache(date, string(raw), fetchedAt); err != nil {
		return LongTigerResponse{}, err
	}
	_ = store.saveMarketSnapshot("long-tiger", "all", date, raw, fetchedAt)
	return LongTigerResponse{
		RequestedDate: date,
		TradeDate:     date,
		Items:         items,
		Source:        "eastmoney",
		CachedAt:      fetchedAt,
	}, nil
}

func cacheFresh(value string) bool {
	fetchedAt, err := time.Parse(time.RFC3339, value)
	return err == nil && time.Since(fetchedAt) >= 0 && time.Since(fetchedAt) < longTigerCacheTTL
}

func (s *MarketStore) getLongTigerCache(date string) (longTigerCacheRow, error) {
	var row longTigerCacheRow
	err := s.db.QueryRow(
		`SELECT data_json, fetched_at FROM long_tiger_cache WHERE trade_date=?`,
		date,
	).Scan(&row.Data, &row.FetchedAt)
	return row, err
}

func (s *MarketStore) saveLongTigerCache(date, data, fetchedAt string) error {
	_, err := s.db.Exec(
		`INSERT INTO long_tiger_cache(trade_date,data_json,fetched_at)
		 VALUES(?,?,?) ON CONFLICT(trade_date) DO UPDATE SET
		 data_json=excluded.data_json,fetched_at=excluded.fetched_at`,
		date, data, fetchedAt,
	)
	return err
}

func decodeLongTigerItems(raw string) ([]LongTigerRank, error) {
	items := []LongTigerRank{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func fetchLongTigerUpstream(ctx context.Context, date string) ([]LongTigerRank, error) {
	query := url.Values{
		"callback":    {"callback"},
		"sortColumns": {"TURNOVERRATE,TRADE_DATE,SECURITY_CODE"},
		"sortTypes":   {"-1,-1,1"},
		"pageSize":    {"500"},
		"pageNumber":  {"1"},
		"reportName":  {"RPT_DAILYBILLBOARD_DETAILSNEW"},
		"columns":     {"SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,FREE_MARKET_CAP,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE,SECURITY_TYPE_CODE"},
		"source":      {"WEB"},
		"client":      {"WEB"},
		"filter":      {fmt.Sprintf("(TRADE_DATE<='%s')(TRADE_DATE>='%s')", date, date)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://datacenter.eastmoney.com/api/data/v1/get?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Host", "datacenter.eastmoney.com")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://data.eastmoney.com")
	req.Header.Set("Referer", "https://data.eastmoney.com/stock/tradedetail.html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/140.0 Safari/537.36")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("东方财富返回 HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Result struct {
			Data []LongTigerRank `json:"data"`
		} `json:"result"`
	}
	payload := strings.TrimSpace(string(body))
	if strings.HasPrefix(payload, "callback(") {
		payload = strings.TrimSpace(strings.TrimPrefix(payload, "callback("))
		if end := strings.LastIndex(payload, ")"); end >= 0 {
			payload = strings.TrimSpace(payload[:end])
		}
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return nil, errors.New("东方财富龙虎榜响应解析失败")
	}
	if envelope.Result.Data == nil {
		return []LongTigerRank{}, nil
	}
	return envelope.Result.Data, nil
}
