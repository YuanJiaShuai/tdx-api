package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	rzrqRankEndpoint  = "http://111.4.248.126/rzrqEnhance/index.php"
	rzrqTrendEndpoint = "http://111.4.248.126/rzrqEnhance/index.php"
)

type RzrqRankItem struct {
	StockCode   string `json:"stockCode"`
	StockName   string `json:"stockName"`
	Date        int64  `json:"date"`
	Lrye        string `json:"lrye"`
	LryeRate    string `json:"lryeRate"`
	Rzye        string `json:"rzye"`
	RzyeRate    string `json:"rzyeRate"`
	Rqye        string `json:"rqye"`
	RqyeRate    string `json:"rqyeRate"`
	Jmr         string `json:"jmr"`
	JmrRate     string `json:"jmrRate"`
	Rzmre       string `json:"rzmre"`
	Rzche       string `json:"rzche"`
	Rzjmce      string `json:"rzjmce"`
	Yezf        string `json:"yezf"`
	ClosePrice  string `json:"close_price"`
	CloseProfit string `json:"close_profit"`
	MarketID    string `json:"marketId"`
}

type RzrqRankResponse struct {
	Type          string         `json:"type"`
	List          []RzrqRankItem `json:"list"`
	RequestedDate string         `json:"requested_date,omitempty"`
	DataDate      string         `json:"data_date,omitempty"`
	Source        string         `json:"source"`
	FetchedAt     string         `json:"fetched_at"`
}

type RzrqTrendItem struct {
	Date  string `json:"date"`
	Rzye  string `json:"rzye"`
	Rzjlr string `json:"rzjlr"`
	Spj   string `json:"spj"`
	Spzf  string `json:"spzf"`
}

type RzrqTrendResponse struct {
	Type       string          `json:"type"`
	Code       string          `json:"code"`
	Items      []RzrqTrendItem `json:"items"`
	RzyeUnit   string          `json:"rzye_unit"`
	RzjlrUnit  string          `json:"rzjlr_unit"`
	SpjUnit    string          `json:"spj_unit"`
	SpzfUnit   string          `json:"spzf_unit"`
	UpdateTime string          `json:"update_time"`
	Source     string          `json:"source"`
	FetchedAt  string          `json:"fetched_at"`
}

func handleRzrqRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}

	rzrqType := strings.TrimSpace(r.URL.Query().Get("type"))
	if rzrqType == "" {
		rzrqType = "hyList"
	}
	if !validRzrqType(rzrqType) {
		errorResponse(w, "type 必须是 hyList、gnList 或 ggList")
		return
	}

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort_key"))
	if sortKey == "" {
		sortKey = "jmr"
	}
	if !validRzrqSortKey(sortKey) {
		errorResponse(w, "sort_key 不是支持的融资融券排序字段")
		return
	}

	sortType := strings.TrimSpace(r.URL.Query().Get("sort_type"))
	if sortType == "" {
		sortType = "desc"
	}
	if sortType != "asc" && sortType != "desc" {
		errorResponse(w, "sort_type 必须是 asc 或 desc")
		return
	}

	length := parsePositiveInt(r.URL.Query().Get("length"))
	if length <= 0 {
		length = 20
	}
	if length > 100 {
		length = 100
	}
	offset := parseNonNegativeInt(r.URL.Query().Get("offset"))

	requestedDate := strings.TrimSpace(r.URL.Query().Get("date"))
	if requestedDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", requestedDate, time.Local); err != nil {
			errorResponse(w, "date 参数格式错误，应为 YYYY-MM-DD")
			return
		}
	}

	items, dataDate, err := fetchRzrqRankWithFallback(
		r.Context(), rzrqType, sortKey, sortType, requestedDate, length, offset,
	)
	if err != nil {
		errorResponse(w, "获取融资融券排名失败: "+err.Error())
		return
	}

	successResponse(w, RzrqRankResponse{
		Type:          rzrqType,
		List:          items,
		RequestedDate: requestedDate,
		DataDate:      dataDate,
		Source:        "同花顺",
		FetchedAt:     time.Now().Format(time.RFC3339),
	})
}

func handleRzrqTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	rzrqType := strings.TrimSpace(r.URL.Query().Get("type"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	data, err := fetchRzrqTrend(r.Context(), rzrqType, code)
	if err != nil {
		errorResponse(w, "获取融资融券走势失败: "+err.Error())
		return
	}
	successResponse(w, data)
}

func fetchRzrqRankWithFallback(
	ctx context.Context,
	rzrqType, sortKey, sortType, requestedDate string, length, offset int,
) ([]RzrqRankItem, string, error) {
	dates := []string{requestedDate}
	if requestedDate == "" {
		dates[0] = ""
	}
	baseDate := time.Now()
	if requestedDate != "" {
		parsed, err := time.ParseInLocation("2006-01-02", requestedDate, time.Local)
		if err != nil {
			return nil, "", err
		}
		baseDate = parsed
	}
	for i := 1; i <= 7; i++ {
		dates = append(dates, baseDate.AddDate(0, 0, -i).Format("2006-01-02"))
	}

	var lastErr error
	for _, date := range dates {
		items, err := fetchRzrqRank(ctx, rzrqType, sortKey, sortType, date, length, offset)
		if err != nil {
			lastErr = err
			continue
		}
		if len(items) > 0 {
			return items, date, nil
		}
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return []RzrqRankItem{}, "", nil
}

func fetchRzrqRank(
	ctx context.Context,
	rzrqType, sortKey, sortType, date string, length, offset int,
) ([]RzrqRankItem, error) {
	query := url.Values{
		"op":       {"getRankData"},
		"type":     {rzrqType},
		"sortKey":  {sortKey},
		"sortType": {sortType},
		"length":   {strconv.Itoa(length)},
		"offset":   {strconv.Itoa(offset)},
	}
	if date != "" {
		query.Set("date", date)
	}
	body, err := fetchRzrqUpstream(ctx, rzrqRankEndpoint+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	var response struct {
		ErrorCode int             `json:"errorCode"`
		ErrorMsg  string          `json:"errorMsg"`
		Data      []RzrqRankItem  `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("融资融券排名响应解析失败: %w", err)
	}
	if response.ErrorCode != 0 {
		return nil, fmt.Errorf("同花顺接口返回异常: code=%d, message=%s", response.ErrorCode, response.ErrorMsg)
	}
	return response.Data, nil
}

func fetchRzrqTrend(ctx context.Context, rzrqType, code string) (*RzrqTrendResponse, error) {
	query := url.Values{"op": {"newIndexData"}}
	if rzrqType != "" {
		query.Set("type", rzrqType)
	}
	if code != "" {
		query.Set("code", code)
	}
	body, err := fetchRzrqUpstream(ctx, rzrqTrendEndpoint+"?"+query.Encode())
	if err != nil {
		return nil, err
	}

	var response struct {
		ErrorCode int    `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
		Data      struct {
			Chart struct {
				RzyeUnit  string   `json:"rzyeUnit"`
				SpjUnit   string   `json:"spjUnit"`
				RzjlrUnit string   `json:"rzjlrUnit"`
				SpzfUnit  string   `json:"spzfUnit"`
				Date      []string `json:"date"`
				Rzye      []string `json:"rzye"`
				Rzjlr     []string `json:"rzjlr"`
				Spj       []string `json:"spj"`
				Spzf      []string `json:"spzf"`
			} `json:"chart"`
			UpdateTime string `json:"updateTime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("融资融券走势响应解析失败: %w", err)
	}
	if response.ErrorCode != 0 {
		return nil, fmt.Errorf("同花顺接口返回异常: code=%d, message=%s", response.ErrorCode, response.ErrorMsg)
	}

	chart := response.Data.Chart
	items := make([]RzrqTrendItem, 0, len(chart.Date))
	for i, date := range chart.Date {
		item := RzrqTrendItem{Date: date}
		if i < len(chart.Rzye) {
			item.Rzye = chart.Rzye[i]
		}
		if i < len(chart.Rzjlr) {
			item.Rzjlr = chart.Rzjlr[i]
		}
		if i < len(chart.Spj) {
			item.Spj = chart.Spj[i]
		}
		if i < len(chart.Spzf) {
			item.Spzf = chart.Spzf[i]
		}
		items = append(items, item)
	}
	return &RzrqTrendResponse{
		Type:       rzrqType,
		Code:       code,
		Items:      items,
		RzyeUnit:   chart.RzyeUnit,
		RzjlrUnit:  chart.RzjlrUnit,
		SpjUnit:    chart.SpjUnit,
		SpzfUnit:   chart.SpzfUnit,
		UpdateTime: response.Data.UpdateTime,
		Source:     "同花顺",
		FetchedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

func fetchRzrqUpstream(ctx context.Context, endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := paceMarketInfoRequest(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Close = true
		req.Host = "eq.10jqka.com.cn"
		req.Header.Set("Referer", "https://eq.10jqka.com.cn/")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Encoding", "identity")

		resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				if readErr != nil {
					return nil, readErr
				}
				return body, nil
			}
			lastErr = fmt.Errorf("上游HTTP状态异常: %s", resp.Status)
			if readErr != nil {
				lastErr = readErr
			}
		}
		if attempt < 2 {
			retryWait := time.Duration(attempt+1) * 500 * time.Millisecond
			timer := time.NewTimer(retryWait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}

func validRzrqType(value string) bool {
	switch value {
	case "hyList", "gnList", "ggList":
		return true
	default:
		return false
	}
}

func validRzrqSortKey(value string) bool {
	switch value {
	case "jmr", "rzye", "rqye", "rzmre", "rzjmce", "lrye", "yezf", "close_profit":
		return true
	default:
		return false
	}
}

func parseNonNegativeInt(value string) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed >= 0 {
		return parsed
	}
	return 0
}
