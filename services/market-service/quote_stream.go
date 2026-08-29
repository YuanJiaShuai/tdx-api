package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/injoyai/tdx/protocol"
)

const (
	defaultQuotePollInterval = 3 * time.Second
	quoteBatchSize           = 50
)

type QuoteSnapshot struct {
	Symbol      string          `json:"symbol"`
	Code        string          `json:"code"`
	Exchange    string          `json:"exchange"`
	Market      string          `json:"market"`
	Source      string          `json:"source"`
	FetchedAt   time.Time       `json:"fetched_at"`
	CachedAt    string          `json:"cached_at,omitempty"`
	Stale       bool            `json:"stale,omitempty"`
	SourceError string          `json:"source_error,omitempty"`
	Quote       *protocol.Quote `json:"quote"`
}

type QuoteStreamEvent struct {
	Type      string         `json:"type"`
	Snapshot  *QuoteSnapshot `json:"snapshot,omitempty"`
	Message   string         `json:"message,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type quoteSubscriber struct {
	id      uint64
	symbols map[string]Symbol
	events  chan QuoteStreamEvent
}

type QuoteHub struct {
	mu          sync.RWMutex
	subscribers map[uint64]*quoteSubscriber
	cache       map[string]QuoteSnapshot
	lastPayload map[string]string
	nextID      uint64
	cancel      context.CancelFunc
}

func NewQuoteHub() *QuoteHub {
	return &QuoteHub{
		subscribers: make(map[uint64]*quoteSubscriber),
		cache:       make(map[string]QuoteSnapshot),
		lastPayload: make(map[string]string),
	}
}

var quoteHub = NewQuoteHub()

func quotePollInterval() time.Duration {
	seconds, err := strconv.Atoi(os.Getenv("MARKET_QUOTE_POLL_INTERVAL_SECONDS"))
	if err != nil || seconds < 1 {
		return defaultQuotePollInterval
	}
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func (h *QuoteHub) Subscribe(ctx context.Context, symbols []Symbol) (<-chan QuoteStreamEvent, func()) {
	h.mu.Lock()
	h.nextID++
	sub := &quoteSubscriber{
		id:      h.nextID,
		symbols: make(map[string]Symbol, len(symbols)),
		events:  make(chan QuoteStreamEvent, 32),
	}
	for _, symbol := range symbols {
		sub.symbols[symbol.Symbol] = symbol
		if cached, ok := h.cache[symbol.Symbol]; ok {
			sub.events <- QuoteStreamEvent{
				Type:      "quote",
				Snapshot:  &cached,
				Timestamp: time.Now(),
			}
		}
	}
	h.subscribers[sub.id] = sub
	if h.cancel == nil {
		runCtx, cancel := context.WithCancel(context.Background())
		h.cancel = cancel
		go h.run(runCtx)
	}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.unsubscribe(sub.id)
		})
	}
	return sub.events, cancel
}

func (h *QuoteHub) unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscribers[id]; !ok {
		return
	}
	delete(h.subscribers, id)
	if len(h.subscribers) == 0 && h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

func (h *QuoteHub) run(ctx context.Context) {
	h.poll()
	ticker := time.NewTicker(quotePollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.poll()
		}
	}
}

func (h *QuoteHub) poll() {
	h.mu.RLock()
	requested := make(map[string]Symbol)
	for _, sub := range h.subscribers {
		for key, symbol := range sub.symbols {
			requested[key] = symbol
		}
	}
	h.mu.RUnlock()
	if len(requested) == 0 || quoteProvider == nil {
		return
	}

	symbols := make([]Symbol, 0, len(requested))
	for _, symbol := range requested {
		symbols = append(symbols, symbol)
	}
	for start := 0; start < len(symbols); start += quoteBatchSize {
		end := start + quoteBatchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		h.pollBatch(symbols[start:end])
	}
}

func (h *QuoteHub) pollBatch(symbols []Symbol) {
	snapshots, err := fetchQuoteSnapshots(context.Background(), symbols)
	if err != nil {
		log.Printf("行情订阅轮询失败: %v", err)
		return
	}

	for _, snapshot := range snapshots {
		payload, err := json.Marshal(snapshot.Quote)
		if err != nil {
			continue
		}
		if h.updateCache(snapshot, string(payload)) {
			h.broadcast(snapshot)
		}
	}
}

func (h *QuoteHub) updateCache(snapshot QuoteSnapshot, payload string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	previous, exists := h.lastPayload[snapshot.Symbol]
	h.cache[snapshot.Symbol] = snapshot
	h.lastPayload[snapshot.Symbol] = payload
	return !exists || previous != payload
}

func (h *QuoteHub) broadcast(snapshot QuoteSnapshot) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	event := QuoteStreamEvent{
		Type:      "quote",
		Snapshot:  &snapshot,
		Timestamp: time.Now(),
	}
	for _, sub := range h.subscribers {
		if _, ok := sub.symbols[snapshot.Symbol]; !ok {
			continue
		}
		select {
		case sub.events <- event:
		default:
			select {
			case <-sub.events:
			default:
			}
			select {
			case sub.events <- event:
			default:
			}
		}
	}
}

func handleQuoteStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	symbols, err := normalizeCodeParam(r.URL.Query().Get("codes"))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if len(symbols) == 0 {
		errorResponse(w, "codes不能为空")
		return
	}
	if len(symbols) > 200 {
		errorResponse(w, "一次最多订阅200只股票")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		errorResponse(w, "当前服务器不支持流式响应")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, cancel := quoteHub.Subscribe(r.Context(), symbols)
	defer cancel()

	writeEvent := func(event QuoteStreamEvent) error {
		body, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, body); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := writeEvent(QuoteStreamEvent{Type: "ready", Timestamp: time.Now()}); err != nil {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if err := writeEvent(event); err != nil {
				return
			}
		case now := <-heartbeat.C:
			if err := writeEvent(QuoteStreamEvent{Type: "heartbeat", Timestamp: now}); err != nil {
				return
			}
		}
	}
}

func handleStandardQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	symbols, err := normalizeCodeParam(r.URL.Query().Get("codes"))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if len(symbols) == 0 {
		errorResponse(w, "codes不能为空")
		return
	}
	if len(symbols) > quoteBatchSize {
		errorResponse(w, "一次最多查询50只股票")
		return
	}

	snapshots, err := fetchQuoteSnapshots(r.Context(), symbols)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取标准行情失败: %v", err))
		return
	}
	successResponse(w, snapshots)
}
