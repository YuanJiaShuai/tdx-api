package main

import (
	"math"
	"testing"
)

func assertFloat(t *testing.T, got, want float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

func TestMA(t *testing.T) {
	rows := []FormulaKline{{Close: 1}, {Close: 2}, {Close: 3}, {Close: 4}, {Close: 5}}
	assertFloat(t, ma(rows, 3), 4, "ma3")
	assertFloat(t, ma(rows, 5), 3, "ma5")
	if ma(rows, 6) != 0 {
		t.Fatalf("len<days should return 0")
	}
	if ma(rows, 0) != 0 {
		t.Fatalf("days<=0 should return 0")
	}
}

func TestStrategyChangePercent(t *testing.T) {
	assertFloat(t, strategyChangePercent(FormulaKline{YClose: 100, Close: 105}), 5, "up")
	assertFloat(t, strategyChangePercent(FormulaKline{YClose: 100, Close: 95}), -5, "down")
	if strategyChangePercent(FormulaKline{YClose: 0, Close: 105}) != 0 {
		t.Fatalf("missing yclose should return 0")
	}
}

func TestStrategyAvgVol(t *testing.T) {
	rows := []FormulaKline{{Vol: 10}, {Vol: 20}, {Vol: 30}, {Vol: 40}, {Vol: 50}}
	assertFloat(t, strategyAvgVol(rows, 2), 35, "avg2")
	assertFloat(t, strategyAvgVol(rows, 4), 25, "avg4")
	if strategyAvgVol(rows, 5) != 0 {
		t.Fatalf("len<days+1 should return 0")
	}
	if strategyAvgVol(rows, 0) != 0 {
		t.Fatalf("days<=0 should return 0")
	}
}

func TestHighestHigh(t *testing.T) {
	rows := []FormulaKline{{High: 1}, {High: 2}, {High: 3}, {High: 4}, {High: 5}}
	assertFloat(t, highestHigh(rows, 2), 4, "high2")
	if highestHigh(rows, 5) != 0 {
		t.Fatalf("len<days+1 should return 0")
	}
	if highestHigh(rows, 0) != 0 {
		t.Fatalf("days<=0 should return 0")
	}
}

func TestEMASeries(t *testing.T) {
	out := emaValues([]float64{1, 2, 3}, 2)
	assertFloat(t, out[0], 1, "ema0")
	assertFloat(t, out[1], 5.0/3.0, "ema1")
	assertFloat(t, out[2], 23.0/9.0, "ema2")
	zero := emaValues([]float64{1, 2, 3}, 0)
	if zero[1] != 0 {
		t.Fatalf("period<=0 should return zeros")
	}
}

func TestTDXSMASeries(t *testing.T) {
	out := tdxSMAValues([]float64{10, 20, 30}, 3, 1, 50)
	assertFloat(t, out[0], 110.0/3.0, "sma0")
	assertFloat(t, out[1], 280.0/9.0, "sma1")
	assertFloat(t, out[2], 830.0/27.0, "sma2")
}

func TestLatestBOLL(t *testing.T) {
	rows := []FormulaKline{{Close: 1}, {Close: 2}, {Close: 3}, {Close: 4}, {Close: 5}}
	boll, ok := latestBOLL(rows, 5, 2)
	if !ok {
		t.Fatalf("should be ok")
	}
	assertFloat(t, boll.Mid, 3, "mid")
	assertFloat(t, boll.Upper, 3+2*math.Sqrt(2), "upper")
	assertFloat(t, boll.Lower, 3-2*math.Sqrt(2), "lower")
	if _, ok := latestBOLL(rows, 6, 2); ok {
		t.Fatalf("len<period should not be ok")
	}
	if _, ok := latestBOLL(rows, 1, 2); ok {
		t.Fatalf("period<=1 should not be ok")
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(3, 5) != 5 || maxInt(5, 3) != 5 || maxInt(2, 2) != 2 {
		t.Fatalf("maxInt wrong")
	}
}

func TestEvaluateMACDSignalRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 40)
	for i := range rows {
		rows[i].Close = float64(i + 1)
	}
	if hit, _, _ := evaluateMACDSignal(rows, StrategyFactorRule{Params: map[string]interface{}{"fast": 12, "slow": 5}}, true); hit {
		t.Fatalf("fast>=slow should not hit")
	}
	if hit, _, _ := evaluateMACDSignal(rows[:10], StrategyFactorRule{Params: map[string]interface{}{}}, true); hit {
		t.Fatalf("insufficient data should not hit")
	}
}

func TestEvaluateKDJGoldenCrossRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 20)
	for i := range rows {
		rows[i].Close = float64(i + 1)
		rows[i].High = float64(i + 2)
		rows[i].Low = float64(i)
	}
	if hit, _, _ := evaluateKDJGoldenCross(rows, StrategyFactorRule{Params: map[string]interface{}{"n": 0}}); hit {
		t.Fatalf("n<=0 should not hit")
	}
	if hit, _, _ := evaluateKDJGoldenCross(rows[:5], StrategyFactorRule{Params: map[string]interface{}{}}); hit {
		t.Fatalf("insufficient data should not hit")
	}
}

func TestEvaluateRSIOversoldRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 20)
	for i := range rows {
		rows[i].Close = float64(i + 1)
	}
	if hit, _, _ := evaluateRSIOversold(rows, StrategyFactorRule{Params: map[string]interface{}{"period": 0}}); hit {
		t.Fatalf("period<=0 should not hit")
	}
	if hit, _, _ := evaluateRSIOversold(rows[:3], StrategyFactorRule{Params: map[string]interface{}{}}); hit {
		t.Fatalf("insufficient data should not hit")
	}
}

func TestEvaluateBOLLBreakoutRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 25)
	for i := range rows {
		rows[i].Close = float64(i + 1)
	}
	if hit, _, _ := evaluateBOLLBreakout(rows, StrategyFactorRule{Params: map[string]interface{}{"period": 1}}); hit {
		t.Fatalf("period<=1 should not hit")
	}
	if hit, _, _ := evaluateBOLLBreakout(rows[:10], StrategyFactorRule{Params: map[string]interface{}{}}); hit {
		t.Fatalf("insufficient data should not hit")
	}
}

func TestEvaluateVolumeBreakoutRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 25)
	for i := range rows {
		rows[i].Close = float64(i + 1)
	}
	if hit, _, _ := evaluateVolumeBreakout(rows, StrategyFactorRule{Params: map[string]interface{}{"days": 0}}); hit {
		t.Fatalf("days<=0 should not hit")
	}
	if hit, _, _ := evaluateVolumeBreakout(rows[:10], StrategyFactorRule{Params: map[string]interface{}{}}); hit {
		t.Fatalf("insufficient data should not hit")
	}
}

func TestEvaluateLocalRocketRejectsInvalidInput(t *testing.T) {
	rows := make([]FormulaKline, 25)
	for i := range rows {
		rows[i].Close = float64(i + 1)
	}
	if hit, _, _ := evaluateLocalRocket(rows, StrategyFactorRule{Params: map[string]interface{}{"lookback": 0}}); hit {
		t.Fatalf("lookback<=0 should not hit")
	}
	if hit, _, _ := evaluateLocalRocket(rows[:10], StrategyFactorRule{Params: map[string]interface{}{}}); hit {
		t.Fatalf("insufficient data should not hit")
	}
}
