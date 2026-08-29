package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type QuoteAlert struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Rule      string    `json:"rule"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type QuoteMonitor struct {
	store       *AppStore
	market      *MarketServiceClient
	mu          sync.RWMutex
	alerts      []QuoteAlert
	lastTrigger map[string]time.Time
	cancel      context.CancelFunc
}

func NewQuoteMonitor(store *AppStore, market *MarketServiceClient) *QuoteMonitor {
	return &QuoteMonitor{
		store: store, market: market, lastTrigger: map[string]time.Time{},
	}
}

func (m *QuoteMonitor) Start() {
	if m == nil || m.market == nil || !m.market.Enabled() || !envBool("SELECTION_QUOTE_MONITOR_ENABLED", true) {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.run(ctx)
}

func (m *QuoteMonitor) Stop() {
	if m != nil && m.cancel != nil {
		m.cancel()
	}
}

func (m *QuoteMonitor) run(ctx context.Context) {
	for {
		codes := m.monitorCodes()
		if len(codes) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		streamCtx, cancel := context.WithCancel(ctx)
		err := m.market.SubscribeQuotes(streamCtx, codes, m.handleEvent)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("行情订阅监测断开: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *QuoteMonitor) monitorCodes() []string {
	if m == nil || m.store == nil {
		return nil
	}
	set := map[string]struct{}{}
	if pool, err := m.store.GetStockPool(DecisionWatchPoolID); err == nil {
		for _, symbol := range pool.Symbols {
			set[normalizeMonitorSymbol(symbol)] = struct{}{}
		}
	}
	if notes, err := m.store.ListDecisionNotes("", 500); err == nil {
		for _, note := range notes {
			if note.PlanBuy > 0 || note.StopLoss > 0 {
				set[normalizeMonitorSymbol(note.Symbol)] = struct{}{}
			}
		}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		if code != "" {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	if len(codes) > 200 {
		codes = codes[:200]
	}
	return codes
}

func normalizeMonitorSymbol(symbol string) string {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if len(symbol) == 9 && strings.Contains(symbol, ".") {
		return symbol
	}
	if len(symbol) == 8 {
		return symbol[2:] + "." + symbol[:2]
	}
	return symbol
}

func (m *QuoteMonitor) handleEvent(event MarketQuoteStreamEvent) {
	if event.Type != "quote" || event.Snapshot == nil || event.Snapshot.Quote == nil {
		return
	}
	price := event.Snapshot.Quote.K.Close.Float64()
	if price <= 0 {
		return
	}
	note, err := m.store.GetDecisionNote(event.Snapshot.Code)
	if err != nil {
		return
	}
	if note.StopLoss > 0 && price <= note.StopLoss {
		m.trigger(event.Snapshot.Symbol, price, "stop_loss", note.StopLoss,
			fmt.Sprintf("%s 跌破止损价 %.3f，当前 %.3f", event.Snapshot.Symbol, note.StopLoss, price))
		return
	}
	if note.PlanBuy > 0 && price <= note.PlanBuy {
		m.trigger(event.Snapshot.Symbol, price, "plan_buy", note.PlanBuy,
			fmt.Sprintf("%s 达到计划关注价 %.3f，当前 %.3f", event.Snapshot.Symbol, note.PlanBuy, price))
	}
}

func (m *QuoteMonitor) trigger(symbol string, price float64, rule string, threshold float64, message string) {
	key := symbol + ":" + rule
	now := time.Now()
	m.mu.Lock()
	if previous, ok := m.lastTrigger[key]; ok && now.Sub(previous) < 15*time.Minute {
		m.mu.Unlock()
		return
	}
	m.lastTrigger[key] = now
	alert := QuoteAlert{
		ID: fmt.Sprintf("%d", now.UnixNano()), Symbol: symbol, Price: price,
		Rule: rule, Threshold: threshold, Message: message, CreatedAt: now,
	}
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > 200 {
		m.alerts = m.alerts[len(m.alerts)-200:]
	}
	m.mu.Unlock()

	hooks, _ := m.store.ListEnabledWebhooks()
	logs := sendWebhooks(context.Background(), hooks, WebhookEvent{
		Event: "quote.alert", Status: "triggered", Message: message,
		MatchedCount: 1, MatchedSymbols: []string{symbol}, Result: alert,
	})
	if len(logs) > 0 {
		log.Printf("行情告警通知: %s", strings.Join(logs, "; "))
	}
}

func (m *QuoteMonitor) ListAlerts(limit int) []QuoteAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	if limit == 0 {
		return []QuoteAlert{}
	}
	result := make([]QuoteAlert, 0, limit)
	for i := len(m.alerts) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, m.alerts[i])
	}
	return result
}

func handleQuoteAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || quoteMonitor == nil {
		errorResponse(w, "只支持GET请求")
		return
	}
	successResponse(w, quoteMonitor.ListAlerts(parsePositiveInt(r.URL.Query().Get("limit"))))
}

var quoteMonitor *QuoteMonitor
