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
			Date:   20260101 + i,
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

func TestFactorMarketMomentumUsesSameDayBenchmarkHistory(t *testing.T) {
	stockRows := factorTestRows(make([]float64, 21), nil)
	benchmarkCloses := make([]float64, 21)
	for index := range benchmarkCloses {
		benchmarkCloses[index] = 100
	}
	benchmarkCloses[20] = 107
	benchmarkRows := factorTestRows(benchmarkCloses, nil)
	runner, result := factorRunner()
	result.KlineCache[strategyBenchmarkSymbol] = benchmarkRows
	rule := StrategyFactorRule{ID: "market", Factor: "market_momentum", Params: map[string]interface{}{"days": 20.0, "min": 6.0, "max": 100.0}}
	if got := runner.evaluateFactor(result, "000001", stockRows, rule, true); !got.Hit {
		t.Fatalf("7%% benchmark momentum should pass: %+v", got)
	}
	benchmarkRows[20].Close = 105
	if got := runner.evaluateFactor(result, "000001", stockRows, rule, true); got.Hit {
		t.Fatalf("5%% benchmark momentum should fail: %+v", got)
	}
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

func TestRangeFactorsActAsHardFilters(t *testing.T) {
	closes := make([]float64, 61)
	highs := make([]float64, 61)
	for index := range closes {
		closes[index] = 10
		highs[index] = 10
	}
	closes[60] = 12
	if got := evalFactor(t, "gain_days", 20, map[string]interface{}{"days": 20.0, "min": 5.0, "max": 15.0}, factorTestRows(closes, highs), true); got.Hit {
		t.Fatalf("20%% gain must fail a 5-15%% hard filter: %+v", got)
	}

	for index := 0; index < 60; index++ {
		highs[index] = 20
	}
	closes[60] = 16
	if got := evalFactor(t, "drawdown_from_high", 20, map[string]interface{}{"days": 60.0, "min": 5.0, "max": 15.0}, factorTestRows(closes, highs), true); got.Hit {
		t.Fatalf("20%% drawdown must fail a 5-15%% hard filter: %+v", got)
	}
}

func TestAdvancedStrategyFactorsAreRegistered(t *testing.T) {
	factors := []string{
		"average_turnover", "boll_width_expanding", "breakout_strength", "consecutive_gain_days",
		"consolidation_days", "decline_deceleration", "distance_from_high", "distance_from_ma",
		"distance_from_support", "kdj_approaching", "kdj_k_slope", "ma_convergence",
		"ma_convergence_days", "ma_distance", "macd_approaching", "macd_convergence_speed",
		"macd_histogram", "macd_positive", "narrow_range", "price_above_ma_cluster",
		"price_break_high", "range_volatility", "rsi_value", "support_not_broken", "turnover_surge",
		"volume_below_average", "volume_burst_after_shrink", "volume_change_rate",
		"volume_consecutive_rise", "volume_ratio", "volume_steady_rise",
	}
	rows := factorTestRows(make([]float64, 100), nil)
	for index := range rows {
		rows[index].Open = 10
		rows[index].High = 10.2
		rows[index].Low = 9.8
		rows[index].Close = 10
		rows[index].YClose = 10
		rows[index].Vol = 1000
	}
	for _, factor := range factors {
		recognized, _, _, _, _ := evaluateAdvancedStrategyFactor(rows, StrategyFactorRule{Factor: factor, Params: map[string]interface{}{}}, false)
		if !recognized {
			t.Errorf("advanced factor %q is not registered", factor)
		}
	}
}

func TestAdvancedBreakoutAndVolumeFactors(t *testing.T) {
	closes := make([]float64, 21)
	highs := make([]float64, 21)
	for index := range closes {
		closes[index], highs[index] = 10, 10
	}
	closes[20], highs[20] = 10.2, 10.3
	rows := factorTestRows(closes, highs)
	if got := evalFactor(t, "price_break_high", 0, map[string]interface{}{"days": 20.0}, rows, true); !got.Hit {
		t.Fatalf("price breakout should pass: %+v", got)
	}
	for index := 0; index < 15; index++ {
		rows[index].Vol = 100
	}
	for index := 15; index < 20; index++ {
		rows[index].Vol = 80
	}
	rows[20].Vol = 300
	params := map[string]interface{}{"lookback": 5.0, "shrink_threshold": 2.0, "burst_ratio": 2.0, "min_turnover": 3.0}
	if got := evalFactor(t, "volume_burst_after_shrink", 0, params, rows, true); !got.Hit {
		t.Fatalf("volume burst after contraction should pass: %+v", got)
	}
}
