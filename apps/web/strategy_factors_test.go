package main

import (
	"math"
	"testing"
)

func TestClamp01(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{-2, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{3, 1},
	}
	for _, item := range cases {
		if got := clamp01(item.in); math.Abs(got-item.want) > 1e-9 {
			t.Errorf("clamp01(%v) = %v, want %v", item.in, got, item.want)
		}
	}
}

// factorTestRows builds rows where each day's close is given; highs default to close+1,
// yclose chains to the previous close and volume/amount are fixed.
func factorTestRows(closes []float64, highs []float64) []FormulaKline {
	rows := make([]FormulaKline, len(closes))
	prev := 0.0
	for i, closePrice := range closes {
		high := closePrice + 1
		if i < len(highs) && highs[i] > 0 {
			high = highs[i]
		}
		rows[i] = FormulaKline{
			YClose: prev,
			Close:  closePrice,
			High:   high,
			Low:    closePrice - 1,
			Vol:    1000,
			Amount: 1e8,
		}
		prev = closePrice
	}
	return rows
}

func factorRunner() (*AutomationRunner, *StrategyRunResult) {
	return &AutomationRunner{}, &StrategyRunResult{
		PoolCache:     map[string]map[string]bool{},
		FormulaCache:  map[string]map[string]bool{},
		FormulaDetail: map[string]map[string]any{},
		KlineCache:    map[string][]FormulaKline{},
	}
}

func evalFactor(t *testing.T, factor string, weight float64, params map[string]interface{}, rows []FormulaKline, filter bool) StrategyFactorResult {
	t.Helper()
	r, result := factorRunner()
	return r.evaluateFactor(result, "000001", rows, StrategyFactorRule{ID: "t", Factor: factor, Weight: weight, Params: params}, filter)
}

func TestFactorDrawdownFromHigh(t *testing.T) {
	closes := make([]float64, 61)
	for i := range closes {
		closes[i] = 10
	}
	highs := make([]float64, 61)
	for i := 0; i < 60; i++ {
		highs[i] = 20
	}
	params := map[string]interface{}{"days": float64(60), "min": float64(5), "max": float64(15)}

	// 回撤10%落在5-15区间:强度1,score=weight20
	closes[60] = 18
	fr := evalFactor(t, "drawdown_from_high", 20, params, factorTestRows(closes, highs), false)
	if !fr.Hit || math.Abs(fr.Score-20) > 1e-9 {
		t.Fatalf("回撤10%%区间内: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}

	// 回撤2.5%低于下限:强度0.5,score=10
	closes[60] = 19.5
	fr = evalFactor(t, "drawdown_from_high", 20, params, factorTestRows(closes, highs), false)
	if !fr.Hit || math.Abs(fr.Score-10) > 1e-9 {
		t.Fatalf("回撤2.5%%低于下限: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}

	// 回撤20%超出上限:强度0.5,score=10
	closes[60] = 16
	fr = evalFactor(t, "drawdown_from_high", 20, params, factorTestRows(closes, highs), false)
	if !fr.Hit || math.Abs(fr.Score-10) > 1e-9 {
		t.Fatalf("回撤20%%超出上限: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}
}

func TestFactorGainDays(t *testing.T) {
	closes := make([]float64, 21)
	for i := range closes {
		closes[i] = 10
	}
	params := map[string]interface{}{"days": float64(20), "min": float64(5), "max": float64(50)}

	// 20日涨幅20%落在5-50区间:强度1,score=weight25
	closes[20] = 12
	fr := evalFactor(t, "gain_days", 25, params, factorTestRows(closes, nil), false)
	if !fr.Hit || math.Abs(fr.Score-25) > 1e-9 {
		t.Fatalf("涨幅20%%区间内: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}

	// 20日涨幅3%低于下限:强度0.6,score=15
	closes[20] = 10.3
	fr = evalFactor(t, "gain_days", 25, params, factorTestRows(closes, nil), false)
	if !fr.Hit || math.Abs(fr.Score-15) > 1e-9 {
		t.Fatalf("涨幅3%%低于下限: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}
}

func TestFactorMaxAmplitude(t *testing.T) {
	closes := []float64{10, 10, 10, 10, 10}
	params := map[string]interface{}{"days": float64(5), "max": float64(8)}

	// 振幅4% <= 8%:命中;filter模式下score为0
	rows := factorTestRows(closes, []float64{10.2, 10.2, 10.2, 10.2, 10.2})
	for i := range rows {
		rows[i].Low = rows[i].Close - 0.2
	}
	fr := evalFactor(t, "max_amplitude", 0, params, rows, true)
	if !fr.Hit || fr.Score != 0 {
		t.Fatalf("振幅4%%应命中: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}

	// 振幅20% > 8%:不命中
	rows = factorTestRows(closes, []float64{11, 11, 11, 11, 11})
	for i := range rows {
		rows[i].Low = rows[i].Close - 1
	}
	fr = evalFactor(t, "max_amplitude", 0, params, rows, true)
	if fr.Hit {
		t.Fatalf("振幅20%%不应命中: reason=%s", fr.Reason)
	}
}

func TestFactorIntensityTimesWeight(t *testing.T) {
	closes := make([]float64, 21)
	for i := range closes {
		closes[i] = 10
	}
	highs := make([]float64, 21)
	for i := 0; i < 20; i++ {
		highs[i] = 10
	}
	params := map[string]interface{}{"days": float64(20)}

	// 突破3%:强度1,score=weight15
	closes[20] = 10.3
	fr := evalFactor(t, "break_high", 15, params, factorTestRows(closes, highs), false)
	if !fr.Hit || math.Abs(fr.Score-15) > 1e-9 {
		t.Fatalf("突破3%%: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}

	// 突破1.5%:强度0.5,score=7.5
	closes[20] = 10.15
	fr = evalFactor(t, "break_high", 15, params, factorTestRows(closes, highs), false)
	if !fr.Hit || math.Abs(fr.Score-7.5) > 1e-9 {
		t.Fatalf("突破1.5%%: hit=%v score=%v reason=%s", fr.Hit, fr.Score, fr.Reason)
	}
}
