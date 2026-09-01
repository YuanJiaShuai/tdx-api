package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
	"github.com/injoyai/tdx/protocol"
)

type MarketStore struct {
	db *sql.DB
}

type FinancialSnapshot struct {
	Symbol            string                 `json:"symbol"`
	Code              string                 `json:"code"`
	Exchange          string                 `json:"exchange"`
	Market            string                 `json:"market"`
	Source            string                 `json:"source,omitempty"`
	SourceError       string                 `json:"source_error,omitempty"`
	CachedAt          string                 `json:"cached_at,omitempty"`
	UpdatedDate       string                 `json:"updated_date,omitempty"`
	IPODate           string                 `json:"ipo_date,omitempty"`
	IndustryCode      uint16                 `json:"industry_code"`
	ProvinceCode      uint16                 `json:"province_code"`
	FloatShares       float64                `json:"float_shares"`
	TotalShares       float64                `json:"total_shares"`
	Shareholders      float64                `json:"shareholders"`
	TotalAssets       float64                `json:"total_assets"`
	NetAssets         float64                `json:"net_assets"`
	MainRevenue       float64                `json:"main_revenue"`
	OperatingProfit   float64                `json:"operating_profit"`
	NetProfit         float64                `json:"net_profit"`
	OperatingCashflow float64                `json:"operating_cashflow"`
	Raw               map[string]interface{} `json:"raw,omitempty"`
	FetchedAt         string                 `json:"fetched_at"`
}

type NewsItem struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	ExternalID  string   `json:"external_id,omitempty"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary,omitempty"`
	Content     string   `json:"content,omitempty"`
	URL         string   `json:"url,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	Symbols     []string `json:"symbols,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Hash        string   `json:"hash"`
	CreatedAt   string   `json:"created_at"`
}

type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
}

func marketDatabasePath() string {
	if value := strings.TrimSpace(os.Getenv("MARKET_DATABASE_PATH")); value != "" {
		return value
	}
	dir := strings.TrimSpace(os.Getenv("MARKET_DATA_DIR"))
	if dir == "" {
		dir = "./data/database"
	}
	return filepath.Join(dir, "market.db")
}

func openMarketStore() (*MarketStore, error) {
	path := marketDatabasePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &MarketStore{db: db}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS financial_snapshots (
			symbol TEXT PRIMARY KEY, code TEXT NOT NULL, exchange TEXT NOT NULL, market TEXT NOT NULL,
			data_json TEXT NOT NULL, fetched_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS news_items (
			id TEXT PRIMARY KEY, source TEXT NOT NULL, external_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '', published_at TEXT NOT NULL DEFAULT '',
			symbols_json TEXT NOT NULL DEFAULT '[]', tags_json TEXT NOT NULL DEFAULT '[]',
			hash TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_news_published_at ON news_items(published_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_news_source_external ON news_items(source, external_id)`,
		`CREATE TABLE IF NOT EXISTS long_tiger_cache (
			trade_date TEXT PRIMARY KEY, data_json TEXT NOT NULL, fetched_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS market_snapshots (
			kind TEXT NOT NULL, data_key TEXT NOT NULL, trade_date TEXT NOT NULL,
			data_json TEXT NOT NULL, fetched_at TEXT NOT NULL,
			PRIMARY KEY(kind, data_key, trade_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_market_snapshots_latest
			ON market_snapshots(kind, data_key, fetched_at DESC)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return store, nil
}

type marketSnapshotRow struct {
	Data      []byte
	TradeDate string
	FetchedAt string
}

func (s *MarketStore) getMarketSnapshot(kind, dataKey, tradeDate string) (marketSnapshotRow, error) {
	var row marketSnapshotRow
	err := s.db.QueryRow(
		`SELECT data_json, trade_date, fetched_at
		 FROM market_snapshots
		 WHERE kind=? AND data_key=? AND trade_date=?
		 LIMIT 1`,
		kind, dataKey, tradeDate,
	).Scan(&row.Data, &row.TradeDate, &row.FetchedAt)
	return row, err
}

func (s *MarketStore) getLatestMarketSnapshot(kind, dataKey string) (marketSnapshotRow, error) {
	var row marketSnapshotRow
	err := s.db.QueryRow(
		`SELECT data_json, trade_date, fetched_at
		 FROM market_snapshots
		 WHERE kind=? AND data_key=?
		 ORDER BY trade_date DESC, fetched_at DESC
		 LIMIT 1`,
		kind, dataKey,
	).Scan(&row.Data, &row.TradeDate, &row.FetchedAt)
	return row, err
}

func (s *MarketStore) saveMarketSnapshot(kind, dataKey, tradeDate string, data []byte, fetchedAt string) error {
	_, err := s.db.Exec(
		`INSERT INTO market_snapshots(kind, data_key, trade_date, data_json, fetched_at)
		 VALUES(?,?,?,?,?)
		 ON CONFLICT(kind, data_key, trade_date) DO UPDATE SET
		 data_json=excluded.data_json, fetched_at=excluded.fetched_at`,
		kind, dataKey, tradeDate, data, fetchedAt,
	)
	return err
}

func closeMarketStore() {
	if marketStore != nil && marketStore.db != nil {
		_ = marketStore.db.Close()
	}
	marketStore = nil
}

func dateText(value uint32) string {
	if value == 0 {
		return ""
	}
	text := strconv.FormatUint(uint64(value), 10)
	if len(text) != 8 {
		return text
	}
	return text[:4] + "-" + text[4:6] + "-" + text[6:]
}

func (s *MarketStore) SaveFinancial(item FinancialSnapshot) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO financial_snapshots(symbol,code,exchange,market,data_json,fetched_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(symbol) DO UPDATE SET
		code=excluded.code,exchange=excluded.exchange,market=excluded.market,
		data_json=excluded.data_json,fetched_at=excluded.fetched_at`,
		item.Symbol, item.Code, item.Exchange, item.Market, raw, item.FetchedAt)
	return err
}

func (s *MarketStore) GetFinancial(symbol string) (FinancialSnapshot, error) {
	var item FinancialSnapshot
	var raw []byte
	err := s.db.QueryRow(`SELECT data_json FROM financial_snapshots WHERE symbol=?`, symbol).Scan(&raw)
	if err != nil {
		return item, err
	}
	err = json.Unmarshal(raw, &item)
	return item, err
}

func normalizeNewsItem(item NewsItem) (NewsItem, error) {
	item.Source = strings.TrimSpace(item.Source)
	item.Title = strings.TrimSpace(item.Title)
	if item.Source == "" {
		item.Source = "manual"
	}
	if item.Title == "" {
		return item, errors.New("资讯标题不能为空")
	}
	var err error
	item.Symbols, err = normalizeNewsSymbols(item.Symbols)
	if err != nil {
		return item, err
	}
	item.Tags = uniqueStrings(item.Tags)
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if item.Hash == "" {
		sum := sha256.Sum256([]byte(strings.Join([]string{item.Source, item.ExternalID, item.Title, item.URL, item.PublishedAt}, "\x00")))
		item.Hash = fmt.Sprintf("%x", sum[:])
	}
	return item, nil
}

func normalizeNewsSymbols(values []string) ([]string, error) {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		symbol, err := NormalizeSymbol(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[symbol.Symbol]; ok {
			continue
		}
		seen[symbol.Symbol] = struct{}{}
		result = append(result, symbol.Symbol)
	}
	return result, nil
}

func uniqueStrings(values []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	return result
}

func (s *MarketStore) SaveNews(item NewsItem) (NewsItem, error) {
	item, err := normalizeNewsItem(item)
	if err != nil {
		return item, err
	}
	symbols, _ := json.Marshal(item.Symbols)
	tags, _ := json.Marshal(item.Tags)
	_, err = s.db.Exec(`INSERT INTO news_items
		(id,source,external_id,title,summary,content,url,published_at,symbols_json,tags_json,hash,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(hash) DO UPDATE SET
		summary=excluded.summary,content=excluded.content,symbols_json=excluded.symbols_json,
		tags_json=excluded.tags_json,published_at=excluded.published_at`,
		item.ID, item.Source, item.ExternalID, item.Title, item.Summary, item.Content, item.URL,
		item.PublishedAt, symbols, tags, item.Hash, item.CreatedAt)
	if err != nil {
		return item, err
	}
	return item, nil
}

func (s *MarketStore) ListNews(symbol, keyword string, limit int) ([]NewsItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []interface{}{}
	where := []string{"1=1"}
	if symbol != "" {
		normalized, err := NormalizeSymbol(symbol)
		if err != nil {
			return nil, err
		}
		where = append(where, "symbols_json LIKE ?")
		args = append(args, "%"+normalized.Symbol+"%")
	}
	if keyword != "" {
		where = append(where, "(title LIKE ? OR summary LIKE ? OR content LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id,source,external_id,title,summary,content,url,published_at,symbols_json,tags_json,hash,created_at
		FROM news_items WHERE `+strings.Join(where, " AND ")+` ORDER BY published_at DESC,created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NewsItem{}
	for rows.Next() {
		var item NewsItem
		var symbols, tags string
		if err := rows.Scan(&item.ID, &item.Source, &item.ExternalID, &item.Title, &item.Summary, &item.Content,
			&item.URL, &item.PublishedAt, &symbols, &tags, &item.Hash, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(symbols), &item.Symbols)
		_ = json.Unmarshal([]byte(tags), &item.Tags)
		items = append(items, item)
	}
	return items, rows.Err()
}

func parseNewsTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, time.RFC1123Z, time.RFC1123, "Mon, 02 Jan 2006 15:04:05 -0700"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return value
}

func fetchRSSNews(feedURL, source string) ([]NewsItem, error) {
	parsed, err := url.Parse(feedURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("资讯源URL无效")
	}
	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tdx-workbench/1.0")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("资讯源返回: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	if source == "" {
		source = feed.Channel.Title
	}
	items := make([]NewsItem, 0, len(feed.Channel.Items))
	for _, entry := range feed.Channel.Items {
		items = append(items, NewsItem{
			Source: source, ExternalID: entry.GUID, Title: entry.Title, Summary: entry.Description,
			URL: entry.Link, PublishedAt: parseNewsTime(entry.PubDate),
		})
	}
	return items, nil
}

func exchangeForSymbol(symbol Symbol) protocol.Exchange {
	switch symbol.Exchange {
	case "SH":
		return protocol.ExchangeSH
	case "BJ":
		return protocol.ExchangeBJ
	default:
		return protocol.ExchangeSZ
	}
}

func handleStandardFinance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	symbol, err := NormalizeSymbol(r.URL.Query().Get("code"))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if client == nil || marketStore == nil {
		errorResponse(w, "行情服务未初始化")
		return
	}
	item, err := loadFinancialContext(r.Context(), symbol)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取标准财务信息失败: %v", err))
		return
	}
	successResponse(w, item)
}

func handleNews(w http.ResponseWriter, r *http.Request) {
	if marketStore == nil {
		errorResponse(w, "行情服务未初始化")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := marketStore.ListNews(r.URL.Query().Get("symbol"), strings.TrimSpace(r.URL.Query().Get("q")), parsePositiveLimit(r.URL.Query().Get("limit")))
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var payload struct {
			Items []NewsItem `json:"items"`
			NewsItem
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		items := payload.Items
		if len(items) == 0 && payload.Title != "" {
			items = []NewsItem{payload.NewsItem}
		}
		if len(items) == 0 {
			errorResponse(w, "资讯不能为空")
			return
		}
		saved := []NewsItem{}
		for _, item := range items {
			item, err := marketStore.SaveNews(item)
			if err != nil {
				errorResponse(w, err.Error())
				return
			}
			saved = append(saved, item)
		}
		successResponse(w, map[string]interface{}{"count": len(saved), "items": saved})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleNewsSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || marketStore == nil {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	items, err := syncNewsFeed(r.Context(), strings.TrimSpace(req.URL), strings.TrimSpace(req.Source))
	if err != nil {
		errorResponse(w, "采集资讯失败: "+err.Error())
		return
	}
	count := 0
	for _, item := range items {
		if _, err := marketStore.SaveNews(item); err != nil {
			errorResponse(w, err.Error())
			return
		}
		count++
	}
	successResponse(w, map[string]interface{}{"source": req.Source, "count": count})
}

func parsePositiveLimit(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func handleAnalysisContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	rawCodes := r.URL.Query().Get("codes")
	if rawCodes == "" {
		rawCodes = r.URL.Query().Get("code")
	}
	symbols, err := normalizeCodeParam(rawCodes)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if len(symbols) == 0 {
		errorResponse(w, "codes不能为空")
		return
	}
	if len(symbols) > 20 {
		errorResponse(w, "一次最多获取20个分析上下文")
		return
	}
	klineLimit := parsePositiveLimit(r.URL.Query().Get("kline_limit"))
	if klineLimit <= 0 || klineLimit > 240 {
		klineLimit = 60
	}
	newsLimit := parsePositiveLimit(r.URL.Query().Get("news_limit"))
	if newsLimit <= 0 || newsLimit > 50 {
		newsLimit = 10
	}
	items := make([]map[string]interface{}, 0, len(symbols))
	for _, symbol := range symbols {
		item := map[string]interface{}{"symbol": symbol.Symbol, "market": symbol.Market}
		if quotes, quoteErr := fetchQuoteSnapshots(r.Context(), []Symbol{symbol}); quoteErr == nil && len(quotes) > 0 {
			item["source"] = quotes[0].Source
			item["quote"] = quotes[0].Quote
		}
		if finance, financeErr := loadFinancialContext(r.Context(), symbol); financeErr == nil {
			item["finance"] = finance
		} else {
			item["finance_error"] = financeErr.Error()
		}
		if klines, klineErr := getQfqKlineDay(symbol.TDXCode); klineErr == nil {
			if len(klines.List) > klineLimit {
				klines.List = klines.List[len(klines.List)-klineLimit:]
				klines.Count = uint16(len(klines.List))
			}
			item["kline_day"] = klines
		}
		if news, newsErr := marketStore.ListNews(symbol.Symbol, "", newsLimit); newsErr == nil {
			item["news"] = news
		}
		items = append(items, item)
	}
	successResponse(w, map[string]interface{}{"generated_at": time.Now().Format(time.RFC3339), "items": items})
}
