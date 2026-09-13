package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFormulaWorkerRunRetriesEmptyResponse(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request FormulaRunRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if !request.ForceFallback {
			t.Error("batch request must preserve force_fallback across retries")
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(FormulaRunResponse{Code: 0, Message: "success", Engine: "fallback", Data: map[string]interface{}{}})
	}))
	defer server.Close()

	client := &FormulaWorkerClient{baseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := client.Run(ctx, FormulaRunRequest{
		Symbols: []string{"000001"}, Script: "RESULT:C;", Period: "day", ForceFallback: true,
		Data: map[string][]FormulaKline{"000001": {{Date: 20260609, Close: 10}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Engine != "fallback" || calls.Load() != 2 {
		t.Fatalf("response=%+v calls=%d, want fallback after one retry", response, calls.Load())
	}
}

func TestFormulaWorkerRunDoesNotRetryBusinessError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(FormulaRunResponse{Code: -1, Message: "公式语法错误"})
	}))
	defer server.Close()

	client := &FormulaWorkerClient{baseURL: server.URL, httpClient: server.Client()}
	_, err := client.Run(context.Background(), FormulaRunRequest{
		Symbols: []string{"000001"}, Script: "INVALID", Period: "day",
		Data: map[string][]FormulaKline{"000001": {{Date: 20260609, Close: 10}}},
	})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d, want one non-retried business failure", err, calls.Load())
	}
}
