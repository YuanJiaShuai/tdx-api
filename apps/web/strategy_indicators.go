package main

import (
	"fmt"
	"math"
)

type macdSnapshot struct {
	DIF  float64
	DEA  float64
	Hist float64
}

type kdjSnapshot struct {
	K float64
	D float64
	J float64
}

type bollSnapshot struct {
	Mid   float64
	Upper float64
	Lower float64
}

func evaluateMACDSignal(rows []FormulaKline, rule StrategyFactorRule, golden bool) (bool, float64, string) {
	fast := intParam(rule.Params, "fast", 12)
	slow := intParam(rule.Params, "slow", 26)
	signal := intParam(rule.Params, "signal", 9)
	if fast <= 0 || slow <= 0 || signal <= 0 || fast >= slow {
		return false, 0, "MACD参数无效"
	}
	points := macdSeries(rows, fast, slow, signal)
	if len(points) < slow+signal || len(points) < 2 {
		return false, 0, fmt.Sprintf("MACD数据不足，需要至少%d根K线", slow+signal)
	}
	prev, curr := points[len(points)-2], points[len(points)-1]
	if golden {
		hit := prev.DIF <= prev.DEA && curr.DIF > curr.DEA
		return hit, curr.DIF - curr.DEA, fmt.Sprintf("MACD金叉 DIF %.4f / DEA %.4f / 柱 %.4f", curr.DIF, curr.DEA, curr.Hist)
	}
	hit := prev.DIF >= prev.DEA && curr.DIF < curr.DEA
	return hit, curr.DEA - curr.DIF, fmt.Sprintf("MACD死叉 DIF %.4f / DEA %.4f / 柱 %.4f", curr.DIF, curr.DEA, curr.Hist)
}

func evaluateKDJGoldenCross(rows []FormulaKline, rule StrategyFactorRule) (bool, float64, string) {
	n := intParam(rule.Params, "n", 9)
	kPeriod := intParam(rule.Params, "k", 3)
	dPeriod := intParam(rule.Params, "d", 3)
	if n <= 0 || kPeriod <= 0 || dPeriod <= 0 {
		return false, 0, "KDJ参数无效"
	}
	points := kdjSeries(rows, n, kPeriod, dPeriod)
	if len(points) < n+dPeriod || len(points) < 2 {
		return false, 0, fmt.Sprintf("KDJ数据不足，需要至少%d根K线", n+dPeriod)
	}
	prev, curr := points[len(points)-2], points[len(points)-1]
	hit := prev.K <= prev.D && curr.K > curr.D
	return hit, curr.J, fmt.Sprintf("KDJ金叉 K %.2f / D %.2f / J %.2f", curr.K, curr.D, curr.J)
}

func evaluateRSIOversold(rows []FormulaKline, rule StrategyFactorRule) (bool, float64, string) {
	period := intParam(rule.Params, "period", 6)
	threshold := floatParam(rule.Params, "threshold", 30)
	values := rsiSeries(rows, period)
	if period <= 0 {
		return false, 0, "RSI参数无效"
	}
	if len(values) < period+1 {
		return false, 0, fmt.Sprintf("RSI数据不足，需要至少%d根K线", period+1)
	}
	rsi := values[len(values)-1]
	hit := rsi > 0 && rsi <= threshold
	return hit, rsi, fmt.Sprintf("RSI%d %.2f <= %.2f", period, rsi, threshold)
}

func evaluateBOLLBreakout(rows []FormulaKline, rule StrategyFactorRule) (bool, float64, string) {
	period := intParam(rule.Params, "period", 20)
	width := floatParam(rule.Params, "width", 2)
	boll, ok := latestBOLL(rows, period, width)
	if period <= 1 || width <= 0 {
		return false, 0, "BOLL参数无效"
	}
	if !ok {
		return false, 0, fmt.Sprintf("BOLL数据不足，需要至少%d根K线", period)
	}
	closePrice := latest(rows).Close
	hit := closePrice > boll.Upper
	return hit, closePrice - boll.Upper, fmt.Sprintf("BOLL突破 C %.2f / 上轨 %.2f / 中轨 %.2f", closePrice, boll.Upper, boll.Mid)
}

func evaluateVolumeBreakout(rows []FormulaKline, rule StrategyFactorRule) (bool, float64, string) {
	days := intParam(rule.Params, "days", 20)
	ratio := floatParam(rule.Params, "ratio", 1.5)
	minChange := floatParam(rule.Params, "min_change", 2)
	if days <= 0 || ratio <= 0 {
		return false, 0, "放量突破参数无效"
	}
	if len(rows) < days+1 {
		return false, 0, fmt.Sprintf("放量突破数据不足，需要至少%d根K线", days+1)
	}
	row := latest(rows)
	avgVol := strategyAvgVol(rows, days)
	high := highestHigh(rows, days)
	change := strategyChangePercent(row)
	volRatio := 0.0
	if avgVol > 0 {
		volRatio = row.Vol / avgVol
	}
	hit := avgVol > 0 && high > 0 && row.Close >= high && volRatio >= ratio && change >= minChange
	return hit, volRatio, fmt.Sprintf("放量突破 C %.2f / %d日高点 %.2f / 量比 %.2f / 涨幅 %.2f%%", row.Close, days, high, volRatio, change)
}

func evaluateLocalRocket(rows []FormulaKline, rule StrategyFactorRule) (bool, float64, string) {
	lookback := intParam(rule.Params, "lookback", 20)
	volumeDays := intParam(rule.Params, "volume_days", 5)
	volumeRatioNeed := floatParam(rule.Params, "volume_ratio", 1.8)
	minChange := floatParam(rule.Params, "min_change", 3)
	shortMA := intParam(rule.Params, "short_ma", 5)
	midMA := intParam(rule.Params, "mid_ma", 10)
	if lookback <= 0 || volumeDays <= 0 || volumeRatioNeed <= 0 || shortMA <= 0 || midMA <= 0 {
		return false, 0, "本地火箭发射参数无效"
	}
	required := maxInt(lookback, maxInt(volumeDays+1, midMA))
	if len(rows) < required {
		return false, 0, fmt.Sprintf("本地火箭发射数据不足，需要至少%d根K线", required)
	}
	row := latest(rows)
	avgVol := strategyAvgVol(rows, volumeDays)
	volRatio := 0.0
	if avgVol > 0 {
		volRatio = row.Vol / avgVol
	}
	high := highestHigh(rows, lookback)
	change := strategyChangePercent(row)
	maShort := ma(rows, shortMA)
	maMid := ma(rows, midMA)
	hit := change >= minChange && volRatio >= volumeRatioNeed && high > 0 && row.Close >= high && maShort >= maMid && row.Close >= maShort
	scoreValue := change + volRatio
	return hit, scoreValue, fmt.Sprintf("本地火箭发射 涨幅 %.2f%% / 量比 %.2f / 突破%d日高点 %.2f / MA%d %.2f / MA%d %.2f", change, volRatio, lookback, high, shortMA, maShort, midMA, maMid)
}

func macdSeries(rows []FormulaKline, fast, slow, signal int) []macdSnapshot {
	closes := closeValues(rows)
	fastEMA := emaValues(closes, fast)
	slowEMA := emaValues(closes, slow)
	dif := make([]float64, len(closes))
	for i := range closes {
		dif[i] = fastEMA[i] - slowEMA[i]
	}
	dea := emaValues(dif, signal)
	points := make([]macdSnapshot, len(closes))
	for i := range closes {
		points[i] = macdSnapshot{DIF: dif[i], DEA: dea[i], Hist: (dif[i] - dea[i]) * 2}
	}
	return points
}

func kdjSeries(rows []FormulaKline, n, kPeriod, dPeriod int) []kdjSnapshot {
	if len(rows) == 0 {
		return nil
	}
	rsv := make([]float64, len(rows))
	for i := range rows {
		start := i - n + 1
		if start < 0 {
			start = 0
		}
		low, high := rows[start].Low, rows[start].High
		for _, row := range rows[start : i+1] {
			if row.Low < low {
				low = row.Low
			}
			if row.High > high {
				high = row.High
			}
		}
		if high == low {
			rsv[i] = 50
		} else {
			rsv[i] = (rows[i].Close - low) * 100 / (high - low)
		}
	}
	k := tdxSMAValues(rsv, kPeriod, 1, 50)
	d := tdxSMAValues(k, dPeriod, 1, 50)
	points := make([]kdjSnapshot, len(rows))
	for i := range rows {
		points[i] = kdjSnapshot{K: k[i], D: d[i], J: 3*k[i] - 2*d[i]}
	}
	return points
}

func rsiSeries(rows []FormulaKline, period int) []float64 {
	if len(rows) == 0 {
		return nil
	}
	up := make([]float64, len(rows))
	down := make([]float64, len(rows))
	for i := 1; i < len(rows); i++ {
		change := rows[i].Close - rows[i-1].Close
		if change > 0 {
			up[i] = change
		} else {
			down[i] = -change
		}
	}
	avgUp := tdxSMAValues(up, period, 1, 0)
	avgDown := tdxSMAValues(down, period, 1, 0)
	values := make([]float64, len(rows))
	for i := range rows {
		total := avgUp[i] + avgDown[i]
		if total > 0 {
			values[i] = avgUp[i] * 100 / total
		}
	}
	return values
}

func latestBOLL(rows []FormulaKline, period int, width float64) (bollSnapshot, bool) {
	if period <= 1 || len(rows) < period {
		return bollSnapshot{}, false
	}
	start := len(rows) - period
	sum := 0.0
	for _, row := range rows[start:] {
		sum += row.Close
	}
	mid := sum / float64(period)
	variance := 0.0
	for _, row := range rows[start:] {
		diff := row.Close - mid
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(period))
	return bollSnapshot{Mid: mid, Upper: mid + width*std, Lower: mid - width*std}, true
}

func emaValues(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 2.0 / float64(period+1)
	prev := values[0]
	for i, value := range values {
		prev = alpha*value + (1-alpha)*prev
		out[i] = prev
	}
	return out
}

func tdxSMAValues(values []float64, n int, m float64, initial float64) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || n <= 0 {
		return out
	}
	prev := initial
	for i, value := range values {
		prev = (m*value + (float64(n)-m)*prev) / float64(n)
		out[i] = prev
	}
	return out
}

func closeValues(rows []FormulaKline) []float64 {
	values := make([]float64, len(rows))
	for i, row := range rows {
		values[i] = row.Close
	}
	return values
}

func strategyChangePercent(row FormulaKline) float64 {
	if row.YClose <= 0 {
		return 0
	}
	return (row.Close - row.YClose) * 100 / row.YClose
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
