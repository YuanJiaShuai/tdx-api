package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/injoyai/tdx/protocol"
)

func TestHikyuuDataServiceClientFetchKlineConvertsUnits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/hikyuu/kline" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("code") != "000001.SZ" ||
			r.URL.Query().Get("type") != "day" ||
			r.URL.Query().Get("recover") != "qfq" ||
			r.URL.Query().Get("limit") != "2" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"symbol":  "000001.SZ",
				"period":  "day",
				"recover": "qfq",
				"count":   1,
				"list": []map[string]interface{}{
					{
						"time":   "2026-08-28T15:00:00+08:00",
						"last":   9.5,
						"open":   10.0,
						"high":   10.5,
						"low":    9.8,
						"close":  10.2,
						"volume": 1234,
						"amount": 567.8,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := &HikyuuDataServiceClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}
	resp, err := client.FetchKline(context.Background(), "sz000001", "day", "qfq", 2)
	if err != nil {
		t.Fatalf("FetchKline returned error: %v", err)
	}
	if resp.Count != 1 || len(resp.List) != 1 {
		t.Fatalf("unexpected response size: count=%d list=%d", resp.Count, len(resp.List))
	}

	item := resp.List[0]
	if item.Open != protocol.Yuan(10) || item.Close != protocol.Yuan(10.2) {
		t.Fatalf("unexpected prices: open=%v close=%v", item.Open, item.Close)
	}
	if item.Volume != 1234 {
		t.Fatalf("unexpected day volume conversion: %d", item.Volume)
	}
	if item.Amount != protocol.Yuan(567.8*10000) {
		t.Fatalf("unexpected amount conversion: %v", item.Amount)
	}
}

func TestHikyuuVolumeToTDX(t *testing.T) {
	if got := hikyuuVolumeToTDX("day", 1234); got != 1234 {
		t.Fatalf("unexpected day volume: %d", got)
	}
	if got := hikyuuVolumeToTDX("minute1", 123400); got != 1234 {
		t.Fatalf("unexpected minute volume: %d", got)
	}
}

func TestFetchHikyuuKlineRejectsUnsupportedPeriod(t *testing.T) {
	oldClient := hikyuuClient
	hikyuuClient = &HikyuuDataServiceClient{baseURL: "http://example.invalid"}
	t.Cleanup(func() {
		hikyuuClient = oldClient
	})

	if _, err := fetchHikyuuKline(context.Background(), "000001", "minute15", 100); err == nil {
		t.Fatal("expected unsupported period error")
	}
}
