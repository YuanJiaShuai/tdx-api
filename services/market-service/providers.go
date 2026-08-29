package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type QuoteProvider interface {
	FetchQuotes(ctx context.Context, symbols []Symbol) ([]QuoteSnapshot, error)
}

type FinanceProvider interface {
	FetchFinancial(ctx context.Context, symbol Symbol) (FinancialSnapshot, error)
}

type NewsProvider interface {
	SyncFeed(ctx context.Context, feedURL, source string) ([]NewsItem, error)
}

type TDXQuoteProvider struct {
	client *tdx.Client
}

func (p *TDXQuoteProvider) FetchQuotes(ctx context.Context, symbols []Symbol) ([]QuoteSnapshot, error) {
	if p == nil || p.client == nil {
		return nil, errors.New("行情客户端未初始化")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	codes := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		codes = append(codes, symbol.TDXCode)
	}
	quotes, err := p.client.GetQuote(codes...)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	snapshots := make([]QuoteSnapshot, 0, len(quotes))
	for i, quote := range quotes {
		if i >= len(symbols) || quote == nil {
			continue
		}
		snapshots = append(snapshots, QuoteSnapshot{
			Symbol:    symbols[i].Symbol,
			Code:      symbols[i].Code,
			Exchange:  symbols[i].Exchange,
			Market:    symbols[i].Market,
			Source:    "tdx",
			FetchedAt: now,
			Quote:     quote,
		})
	}
	return snapshots, nil
}

type CachedQuoteProvider struct {
	primary  QuoteProvider
	mu       sync.RWMutex
	cache    map[string]QuoteSnapshot
	freshTTL time.Duration
	staleTTL time.Duration
}

func NewCachedQuoteProvider(primary QuoteProvider) *CachedQuoteProvider {
	freshTTL := providerDuration("MARKET_QUOTE_CACHE_TTL_SECONDS", 15)
	staleTTL := providerDuration("MARKET_QUOTE_CACHE_STALE_SECONDS", 300)
	if staleTTL < freshTTL {
		staleTTL = freshTTL
	}
	return &CachedQuoteProvider{
		primary:  primary,
		cache:    make(map[string]QuoteSnapshot),
		freshTTL: freshTTL,
		staleTTL: staleTTL,
	}
}

func (p *CachedQuoteProvider) FetchQuotes(ctx context.Context, symbols []Symbol) ([]QuoteSnapshot, error) {
	if p == nil || p.primary == nil {
		return nil, errors.New("行情 provider 未初始化")
	}
	if len(symbols) == 0 {
		return []QuoteSnapshot{}, nil
	}

	upstream, upstreamErr := p.primary.FetchQuotes(ctx, symbols)
	bySymbol := make(map[string]QuoteSnapshot, len(upstream))
	for _, snapshot := range upstream {
		if snapshot.Quote == nil {
			continue
		}
		snapshot.Source = "tdx"
		snapshot.Stale = false
		snapshot.SourceError = ""
		bySymbol[snapshot.Symbol] = snapshot
		p.mu.Lock()
		p.cache[snapshot.Symbol] = snapshot
		p.mu.Unlock()
	}

	result := make([]QuoteSnapshot, 0, len(symbols))
	missing := make([]string, 0)
	for _, symbol := range symbols {
		if snapshot, ok := bySymbol[symbol.Symbol]; ok {
			result = append(result, snapshot)
			continue
		}
		if snapshot, ok := p.cached(symbol.Symbol, upstreamErr); ok {
			result = append(result, snapshot)
			continue
		}
		missing = append(missing, symbol.Symbol)
	}
	if len(missing) > 0 {
		if upstreamErr != nil {
			return nil, upstreamErr
		}
		return nil, fmt.Errorf("行情 provider 返回不完整，缺少: %s", strings.Join(missing, ","))
	}
	return result, nil
}

func (p *CachedQuoteProvider) cached(symbol string, upstreamErr error) (QuoteSnapshot, bool) {
	p.mu.RLock()
	snapshot, ok := p.cache[symbol]
	p.mu.RUnlock()
	if !ok || snapshot.Quote == nil {
		return QuoteSnapshot{}, false
	}
	age := time.Since(snapshot.FetchedAt)
	if age < 0 || age > p.staleTTL {
		return QuoteSnapshot{}, false
	}
	snapshot.Source = "cache"
	snapshot.CachedAt = snapshot.FetchedAt.Format(time.RFC3339)
	snapshot.Stale = age > p.freshTTL
	if upstreamErr != nil {
		snapshot.SourceError = upstreamErr.Error()
	}
	return snapshot, true
}

type TDXFinanceProvider struct {
	client *tdx.Client
}

func (p *TDXFinanceProvider) FetchFinancial(ctx context.Context, symbol Symbol) (FinancialSnapshot, error) {
	if p == nil || p.client == nil {
		return FinancialSnapshot{}, errors.New("行情客户端未初始化")
	}
	select {
	case <-ctx.Done():
		return FinancialSnapshot{}, ctx.Err()
	default:
	}
	info, err := p.client.GetFinanceInfo(exchangeForSymbol(symbol), symbol.Code)
	if err != nil {
		return FinancialSnapshot{}, err
	}
	item := financialSnapshot(symbol, info)
	item.Source = "tdx"
	return item, nil
}

type CachedFinanceProvider struct {
	primary FinanceProvider
	store   *MarketStore
}

func (p *CachedFinanceProvider) FetchFinancial(ctx context.Context, symbol Symbol) (FinancialSnapshot, error) {
	if p == nil || p.primary == nil {
		return FinancialSnapshot{}, errors.New("财务 provider 未初始化")
	}
	item, err := p.primary.FetchFinancial(ctx, symbol)
	if err == nil {
		if p.store != nil {
			_ = p.store.SaveFinancial(item)
		}
		return item, nil
	}
	if p.store != nil {
		if cached, cacheErr := p.store.GetFinancial(symbol.Symbol); cacheErr == nil {
			cached.Source = "cache"
			cached.SourceError = err.Error()
			cached.CachedAt = time.Now().Format(time.RFC3339)
			return cached, nil
		}
	}
	return FinancialSnapshot{}, err
}

type RSSNewsProvider struct{}

func (p *RSSNewsProvider) SyncFeed(ctx context.Context, feedURL, source string) ([]NewsItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	_ = p
	return fetchRSSNews(feedURL, source)
}

var (
	quoteProvider   QuoteProvider
	financeProvider FinanceProvider
	newsProvider    NewsProvider
)

func initMarketProviders() {
	if client == nil {
		quoteProvider = nil
		financeProvider = nil
		newsProvider = nil
		return
	}
	quoteProvider = NewCachedQuoteProvider(&TDXQuoteProvider{client: client})
	financeProvider = &CachedFinanceProvider{
		primary: &TDXFinanceProvider{client: client},
		store:   marketStore,
	}
	newsProvider = &RSSNewsProvider{}
}

func providerDuration(name string, fallbackSeconds int) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || seconds <= 0 {
		seconds = fallbackSeconds
	}
	return time.Duration(seconds) * time.Second
}

func fetchQuoteSnapshots(ctx context.Context, symbols []Symbol) ([]QuoteSnapshot, error) {
	if quoteProvider == nil {
		return nil, errors.New("行情 provider 未初始化")
	}
	return quoteProvider.FetchQuotes(ctx, symbols)
}

func loadFinancialContext(ctx context.Context, symbol Symbol) (FinancialSnapshot, error) {
	if financeProvider == nil {
		return FinancialSnapshot{}, errors.New("财务 provider 未初始化")
	}
	return financeProvider.FetchFinancial(ctx, symbol)
}

func syncNewsFeed(ctx context.Context, feedURL, source string) ([]NewsItem, error) {
	if newsProvider == nil {
		return nil, errors.New("资讯 provider 未初始化")
	}
	return newsProvider.SyncFeed(ctx, feedURL, source)
}

func financialSnapshot(symbol Symbol, item *protocol.FinanceInfo) FinancialSnapshot {
	rawBytes, _ := json.Marshal(item)
	raw := map[string]interface{}{}
	_ = json.Unmarshal(rawBytes, &raw)
	return FinancialSnapshot{
		Symbol: symbol.Symbol, Code: symbol.Code, Exchange: symbol.Exchange, Market: symbol.Market,
		UpdatedDate: dateText(item.UpdatedDate), IPODate: dateText(item.IPODate),
		IndustryCode: item.Industry, ProvinceCode: item.Province,
		FloatShares: item.LiuTongGuBen, TotalShares: item.ZongGuBen, Shareholders: item.GuDongRenShu,
		TotalAssets: item.ZongZiChan, NetAssets: item.JingZiChan, MainRevenue: item.ZhuYingShouRu,
		OperatingProfit: item.YingYeLiRun, NetProfit: item.JingLiRun, OperatingCashflow: item.JingYingXianJinLiu,
		Raw: raw, FetchedAt: time.Now().Format(time.RFC3339), Source: "tdx",
	}
}
