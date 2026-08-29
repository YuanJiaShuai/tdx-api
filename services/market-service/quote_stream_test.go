package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestQuoteHubUpdateCacheOnlyMarksChangedPayload(t *testing.T) {
	hub := NewQuoteHub()
	symbol, err := NormalizeSymbol("600519.SH")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := QuoteSnapshot{
		Symbol:    symbol.Symbol,
		Code:      symbol.Code,
		Exchange:  symbol.Exchange,
		Market:    symbol.Market,
		Source:    "tdx",
		FetchedAt: time.Now(),
	}

	if !hub.updateCache(snapshot, "first") {
		t.Fatal("first payload should be treated as changed")
	}
	if hub.updateCache(snapshot, "first") {
		t.Fatal("same payload should not be treated as changed")
	}
	if !hub.updateCache(snapshot, "second") {
		t.Fatal("new payload should be treated as changed")
	}
}

func TestCachedQuoteProviderFallsBackToCache(t *testing.T) {
	primary := &stubQuoteProvider{
		err: errStubQuote,
	}
	provider := NewCachedQuoteProvider(primary)
	symbol, err := NormalizeSymbol("600519.SH")
	if err != nil {
		t.Fatal(err)
	}

	provider.mu.Lock()
	provider.cache[symbol.Symbol] = QuoteSnapshot{
		Symbol:    symbol.Symbol,
		Code:      symbol.Code,
		Exchange:  symbol.Exchange,
		Market:    symbol.Market,
		Source:    "tdx",
		FetchedAt: time.Now().Add(-10 * time.Second),
		Quote:     &protocol.Quote{},
	}
	provider.mu.Unlock()

	snapshots, err := provider.FetchQuotes(t.Context(), []Symbol{symbol})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one cached snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Source != "cache" {
		t.Fatalf("expected cache source, got %s", snapshots[0].Source)
	}
	if snapshots[0].SourceError == "" {
		t.Fatal("expected upstream error to be preserved")
	}
}

var errStubQuote = errors.New("upstream unavailable")

type stubQuoteProvider struct {
	err error
}

func (s *stubQuoteProvider) FetchQuotes(ctx context.Context, symbols []Symbol) ([]QuoteSnapshot, error) {
	return nil, s.err
}
