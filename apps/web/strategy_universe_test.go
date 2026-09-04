package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStrategyUniverseExpressionEvaluatesSetRecipe(t *testing.T) {
	runner := &AutomationRunner{}
	expr := StrategyUniverseExpression{
		Mode: "expression",
		Include: []StrategyUniverseTerm{
			{Symbols: []string{"000002", "000001"}},
			{Symbols: []string{"600000"}},
		},
		Intersect: []StrategyUniverseTerm{
			{Symbols: []string{"000001", "600000", "688001"}},
		},
		Exclude: []StrategyUniverseTerm{
			{Symbols: []string{"600000"}},
		},
	}

	got, err := runner.evaluateUniverseExpression(expr, 0)
	if err != nil {
		t.Fatalf("evaluateUniverseExpression() error = %v", err)
	}
	want := []string{"000001"}
	if !reflect.DeepEqual(got.Symbols, want) {
		t.Fatalf("symbols = %#v, want %#v", got.Symbols, want)
	}
	if got.Total != 1 || got.Scanned != 1 || got.Truncated {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if len(got.Sources) != 4 {
		t.Fatalf("sources length = %d, want 4", len(got.Sources))
	}
}

func TestStrategyUniverseExpressionAppliesScanLimit(t *testing.T) {
	result := buildUniverseResult(StrategyUniverseExpression{}, []string{"000003", "000001", "000002"}, nil, 2)
	if !result.Truncated || result.Total != 3 || result.Scanned != 2 {
		t.Fatalf("unexpected limit summary: %+v", result)
	}
	want := []string{"000001", "000002"}
	if !reflect.DeepEqual(result.Symbols, want) {
		t.Fatalf("symbols = %#v, want %#v", result.Symbols, want)
	}
}

func TestStrategyUniverseExpressionKeepsLegacyStringConfig(t *testing.T) {
	var cfg StrategyConfig
	if err := json.Unmarshal([]byte(`{"universe":"market","pool_id":"market-all-a"}`), &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if cfg.Universe.Legacy != "market" || cfg.Universe.isExpression() {
		t.Fatalf("unexpected legacy universe: %+v", cfg.Universe)
	}
	raw, err := json.Marshal(cfg.Universe)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(raw) != `"market"` {
		t.Fatalf("legacy universe JSON = %s, want %q", raw, `"market"`)
	}
}
