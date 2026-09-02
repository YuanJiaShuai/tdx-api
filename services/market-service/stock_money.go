package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	stockMoneyRankKind = "stock-money-rank"
	stockMoneyRankTTL  = 30 * time.Minute
)

var stockMoneySorts = map[string]struct{}{
	"netamount":  {},
	"outamount":  {},
	"ratioamount": {},
	"r0_net":     {},
	"r0_out":     {},
	"r0_ratio":   {},
	"r3_net":     {},
	"r3_out":     {},
	"r3_ratio":   {},
}

type StockMoneyRankItem struct {
	Symbol      string `json:"symbol"`
	Name        any    `json:"name"`
	Trade       string `json:"trade"`
	ChangeRatio string `json:"changeratio"`
	Turnover    string `json:"turnover"`
	Amount      string `json:"amount"`
	InAmount    string `json:"inamount"`
	OutAmount   string `json:"outamount"`
	NetAmount   string `json:"netamount"`
	RatioAmount string `json:"ratioamount"`
	MainIn      string `json:"r0_in"`
	MainOut     string `json:"r0_out"`
	MainNet     string `json:"r0_net"`
	RetailIn    string `json:"r3_in"`
	RetailOut   string `json:"r3_out"`
	RetailNet   string `json:"r3_net"`
	MainRatio   string `json:"r0_ratio"`
	RetailRatio string `json:"r3_ratio"`
	ExtraRatio  string `json:"r0x_ratio"`
}

type StockMoneyRankResponse struct {
	Items     []StockMoneyRankItem `json:"items"`
	Sort      string                `json:"sort"`
	Limit     int                   `json:"limit"`
	Source    string                `json:"source"`
	FetchedAt string                `json:"fetched_at"`
}

func handleStockMoneyRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		errorResponse(w, "只支持GET请求")
		return
	}
	sortValue := strings.TrimSpace(r.URL.Query().Get("sort"))
	if _, ok := stockMoneySorts[sortValue]; !ok {
		sortValue = "netamount"
	}

	value, source, fetchedAt, err := getMarketRankCached(
		r.Context(),
		stockMoneyRankKind,
		sortValue,
		stockMoneyRankTTL,
		func(ctx context.Context) ([]StockMoneyRankItem, error) {
			return fetchStockMoneyRank(ctx, sortValue)
		},
	)
	if err != nil {
		errorResponse(w, "获取个股资金流向失败: "+err.Error())
		return
	}
	successResponse(w, StockMoneyRankResponse{
		Items:     value,
		Sort:      sortValue,
		Limit:     len(value),
		Source:    source,
		FetchedAt: fetchedAt,
	})
}

func fetchStockMoneyRank(ctx context.Context, sortValue string) ([]StockMoneyRankItem, error) {
	if err := paceMarketInfoRequest(ctx); err != nil {
		return nil, err
	}
	query := url.Values{
		"page":     {"1"},
		"num":      {"20"},
		"sort":     {sortValue},
		"asc":      {"0"},
		"bankuai":  {""},
		"shichang": {""},
	}
	endpoint := "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_ssggzj?" + query.Encode()
	body, err := fetchIndustryUpstream(ctx, endpoint, "https://finance.sina.com.cn")
	if err != nil {
		return nil, err
	}
	var items []StockMoneyRankItem
	if err := unmarshalStockMoneyRank(body, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func unmarshalStockMoneyRank(body []byte, items *[]StockMoneyRankItem) error {
	if err := json.Unmarshal(body, items); err != nil {
		return fmt.Errorf("个股资金流向响应解析失败: %w", err)
	}
	return nil
}
