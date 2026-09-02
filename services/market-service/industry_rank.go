package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	industryRankKind      = "industry-rank"
	industryMoneyRankKind = "industry-money-rank"
	industryRankTTL       = 10 * time.Second
	industryMoneyRankTTL  = time.Minute
)

type IndustryRankItem struct {
	BoardName     string `json:"bd_name"`
	BoardCode     string `json:"bd_code"`
	Latest        string `json:"bd_zxj"`
	Change        string `json:"bd_zd"`
	ChangePercent string `json:"bd_zdf"`
	Change5D      string `json:"bd_zdf5"`
	Change20D     string `json:"bd_zdf20"`
	LeaderCode    string `json:"nzg_code"`
	LeaderName    string `json:"nzg_name"`
	LeaderLatest  string `json:"nzg_zxj"`
	LeaderChange  string `json:"nzg_zd"`
	LeaderPercent string `json:"nzg_zdf"`
}

type IndustryRankResponse struct {
	Items     []IndustryRankItem `json:"items"`
	Sort      string             `json:"sort"`
	Limit     int                `json:"limit"`
	Source    string             `json:"source"`
	FetchedAt string             `json:"fetched_at"`
}

type IndustryMoneyRankItem struct {
	Category       string `json:"cate_type"`
	CategoryCode   string `json:"category"`
	Name           string `json:"name"`
	AveragePrice   string `json:"avg_price"`
	AverageChange  string `json:"avg_changeratio"`
	Turnover       string `json:"turnover"`
	InAmount       string `json:"inamount"`
	OutAmount      string `json:"outamount"`
	NetAmount      string `json:"netamount"`
	NetRatio       string `json:"ratioamount"`
	LeaderSymbol   string `json:"ts_symbol"`
	LeaderName     string `json:"ts_name"`
	LeaderPrice    string `json:"ts_trade"`
	LeaderChange   string `json:"ts_changeratio"`
	LeaderNetRatio string `json:"ts_ratioamount"`
}

type IndustryMoneyRankResponse struct {
	Items     []IndustryMoneyRankItem `json:"items"`
	Category  string                  `json:"category"`
	Sort      string                  `json:"sort"`
	Source    string                  `json:"source"`
	FetchedAt string                  `json:"fetched_at"`
}

type industryRankCacheEntry struct {
	Value     any
	ExpiresAt time.Time
	FetchedAt string
}

var industryRankCache = struct {
	sync.Mutex
	Items map[string]industryRankCacheEntry
}{Items: make(map[string]industryRankCacheEntry)}

func handleIndustryRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	sortValue := r.URL.Query().Get("sort")
	if sortValue != "1" {
		sortValue = "0"
	}
	limit := 150
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 150 {
		limit = 150
	}

	cacheKey := fmt.Sprintf("sort=%s&limit=%d", sortValue, limit)
	value, source, fetchedAt, err := getIndustryRankCached(
		r.Context(),
		industryRankKind,
		cacheKey,
		industryRankTTL,
		func(ctx context.Context) ([]IndustryRankItem, error) {
			return fetchIndustryRank(ctx, sortValue, limit)
		},
	)
	if err != nil {
		errorResponse(w, "获取行业涨幅排名失败: "+err.Error())
		return
	}
	successResponse(w, IndustryRankResponse{
		Items:     value,
		Sort:      sortValue,
		Limit:     limit,
		Source:    source,
		FetchedAt: fetchedAt,
	})
}

func handleIndustryMoneyRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "" {
		category = "0"
	}
	sortValue := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortValue == "" {
		sortValue = "netamount"
	}

	cacheKey := fmt.Sprintf("category=%s&sort=%s", category, sortValue)
	value, source, fetchedAt, err := getIndustryRankCached(
		r.Context(),
		industryMoneyRankKind,
		cacheKey,
		industryMoneyRankTTL,
		func(ctx context.Context) ([]IndustryMoneyRankItem, error) {
			return fetchIndustryMoneyRank(ctx, category, sortValue)
		},
	)
	if err != nil {
		errorResponse(w, "获取行业资金排名失败: "+err.Error())
		return
	}
	successResponse(w, IndustryMoneyRankResponse{
		Items:     value,
		Category:  category,
		Sort:      sortValue,
		Source:    source,
		FetchedAt: fetchedAt,
	})
}

func getIndustryRankCached[T any](
	ctx context.Context,
	kind string,
	dataKey string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
) (T, string, string, error) {
	now := time.Now()
	cacheKey := kind + ":" + dataKey
	industryRankCache.Lock()
	if entry, ok := industryRankCache.Items[cacheKey]; ok && now.Before(entry.ExpiresAt) {
		value, ok := entry.Value.(T)
		fetchedAt := entry.FetchedAt
		industryRankCache.Unlock()
		if ok {
			return value, "cache", fetchedAt, nil
		}
	} else {
		industryRankCache.Unlock()
	}

	value, err := fetch(ctx)
	if err == nil {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			var zero T
			return zero, "", "", marshalErr
		}
		fetchedAt := time.Now().Format(time.RFC3339)
		if marketStore != nil {
			_ = marketStore.saveMarketSnapshot(kind, dataKey, time.Now().Format("2006-01-02"), raw, fetchedAt)
		}
		industryRankCache.Lock()
		industryRankCache.Items[cacheKey] = industryRankCacheEntry{
			Value: value, ExpiresAt: time.Now().Add(ttl), FetchedAt: fetchedAt,
		}
		industryRankCache.Unlock()
		return value, "upstream", fetchedAt, nil
	}

	if marketStore != nil {
		row, dbErr := marketStore.getMarketSnapshot(kind, dataKey, now.Format("2006-01-02"))
		if errors.Is(dbErr, sql.ErrNoRows) {
			row, dbErr = marketStore.getLatestMarketSnapshot(kind, dataKey)
		}
		if dbErr == nil {
			var fallback T
			if unmarshalErr := json.Unmarshal(row.Data, &fallback); unmarshalErr == nil {
				industryRankCache.Lock()
				industryRankCache.Items[cacheKey] = industryRankCacheEntry{
					Value: fallback, ExpiresAt: now.Add(ttl), FetchedAt: row.FetchedAt,
				}
				industryRankCache.Unlock()
				return fallback, "database", row.FetchedAt, nil
			}
		}
	}

	var zero T
	return zero, "", "", err
}

func fetchIndustryRank(ctx context.Context, sortValue string, limit int) ([]IndustryRankItem, error) {
	if err := paceMarketInfoRequest(ctx); err != nil {
		return nil, err
	}
	query := url.Values{
		"l":         {strconv.Itoa(limit)},
		"p":         {"1"},
		"t":         {"01/averatio"},
		"ordertype": {""},
		"o":         {sortValue},
	}
	endpoint := "https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktHs/rank?" + query.Encode()
	body, err := fetchIndustryUpstream(ctx, endpoint, "https://stockapp.finance.qq.com/")
	if err != nil {
		return nil, err
	}
	var response struct {
		Code int                `json:"code"`
		Msg  string             `json:"msg"`
		Data []IndustryRankItem `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("行业涨幅排名响应解析失败: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("行业涨幅排名接口返回异常: code=%d, message=%s", response.Code, response.Msg)
	}
	return response.Data, nil
}

func fetchIndustryMoneyRank(ctx context.Context, category, sortValue string) ([]IndustryMoneyRankItem, error) {
	if err := paceMarketInfoRequest(ctx); err != nil {
		return nil, err
	}
	query := url.Values{
		"page":   {"1"},
		"num":    {"20"},
		"sort":   {sortValue},
		"asc":    {"0"},
		"fenlei": {category},
	}
	endpoint := "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_bk?" + query.Encode()
	body, err := fetchIndustryUpstream(ctx, endpoint, "https://finance.sina.com.cn")
	if err != nil {
		return nil, err
	}
	var items []IndustryMoneyRankItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("行业资金排名响应解析失败: %w", err)
	}
	return items, nil
}

func fetchIndustryUpstream(ctx context.Context, endpoint, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60")
	if strings.Contains(endpoint, "sina.com.cn") {
		req.Header.Set("Host", "vip.stock.finance.sina.com.cn")
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("上游HTTP状态异常: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func handleMarketTradingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	now := time.Now()
	isWorkday := now.Weekday() != time.Saturday && now.Weekday() != time.Sunday
	if manager != nil && manager.Workday != nil {
		isWorkday = manager.Workday.Is(now)
	}
	minute := now.Hour()*60 + now.Minute()
	inSession := (minute >= 9*60+30 && minute < 11*60+30) || (minute >= 13*60 && minute < 15*60)
	successResponse(w, map[string]any{
		"date":       now.Format("2006-01-02"),
		"is_trading": isWorkday && inSession,
		"checked_at": now.Format(time.RFC3339),
		"timezone":   now.Location().String(),
	})
}
