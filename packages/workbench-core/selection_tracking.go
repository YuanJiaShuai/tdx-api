package workbench

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	DefaultTrackingTargetReturn  = 3.0
	DefaultTrackingDrawdownLimit = 5.0
)

// TrackingBar is the small, provider-neutral OHLC shape required to evaluate
// a selection signal. Keeping it in workbench-core lets each market adapter
// reuse the same deterministic calculation.
type TrackingBar struct {
	Date   int
	YClose float64
	Open   float64
	High   float64
	Low    float64
	Close  float64
}

type SelectionHorizon struct {
	HorizonDays   int     `json:"horizon_days"`
	Status        string  `json:"status"` // pending / complete / unavailable
	AsOfDate      int     `json:"as_of_date,omitempty"`
	OpenReturn    float64 `json:"open_return,omitempty"`
	CloseReturn   float64 `json:"close_return,omitempty"`
	MaxGain       float64 `json:"max_gain,omitempty"`
	MaxDrawdown   float64 `json:"max_drawdown,omitempty"`
	TargetReturn  float64 `json:"target_return"`
	DrawdownLimit float64 `json:"drawdown_limit"`
	Success       bool    `json:"success"`
	Reason        string  `json:"reason,omitempty"`
}

type SelectionTracking struct {
	Version       string                      `json:"version"`
	SignalDate    int                         `json:"signal_date"`
	BasePrice     float64                     `json:"base_price"`
	TargetReturn  float64                     `json:"target_return"`
	DrawdownLimit float64                     `json:"drawdown_limit"`
	UpdatedAt     string                      `json:"updated_at"`
	Horizons      map[string]SelectionHorizon `json:"horizons"`
}

type SelectionTrackingItem struct {
	Result   SelectionResult   `json:"result"`
	Tracking SelectionTracking `json:"tracking"`
	Error    string            `json:"error,omitempty"`
}

type SelectionHorizonSummary struct {
	Completed          int     `json:"completed"`
	Pending            int     `json:"pending"`
	Unavailable        int     `json:"unavailable"`
	SuccessCount       int     `json:"success_count"`
	SuccessRate        float64 `json:"success_rate"`
	AverageOpenReturn  float64 `json:"average_open_return"`
	AverageCloseReturn float64 `json:"average_close_return"`
	AverageMaxGain     float64 `json:"average_max_gain"`
	AverageMaxDrawdown float64 `json:"average_max_drawdown"`
}

type SelectionTrackingSummary struct {
	Total    int                                `json:"total"`
	Horizons map[string]SelectionHorizonSummary `json:"horizons"`
}

func DefaultTrackingPolicy(targetReturn, drawdownLimit float64) (float64, float64) {
	if targetReturn <= 0 {
		targetReturn = DefaultTrackingTargetReturn
	}
	if drawdownLimit <= 0 {
		drawdownLimit = DefaultTrackingDrawdownLimit
	}
	return targetReturn, drawdownLimit
}

func SummarizeSelectionTracking(items []SelectionTrackingItem, horizons []int) SelectionTrackingSummary {
	if len(horizons) == 0 {
		horizons = []int{1, 5, 10}
	}
	summary := SelectionTrackingSummary{Total: len(items), Horizons: map[string]SelectionHorizonSummary{}}
	for _, horizon := range horizons {
		key := horizonKey(horizon)
		var openTotal, closeTotal, gainTotal, drawdownTotal float64
		row := SelectionHorizonSummary{}
		for _, item := range items {
			metric, ok := item.Tracking.Horizons[key]
			if !ok {
				row.Pending++
				continue
			}
			switch metric.Status {
			case "complete":
				row.Completed++
				openTotal += metric.OpenReturn
				closeTotal += metric.CloseReturn
				gainTotal += metric.MaxGain
				drawdownTotal += metric.MaxDrawdown
				if metric.Success {
					row.SuccessCount++
				}
			case "unavailable":
				row.Unavailable++
			default:
				row.Pending++
			}
		}
		if row.Completed > 0 {
			count := float64(row.Completed)
			row.SuccessRate = roundTrackingPercent(float64(row.SuccessCount) / count * 100)
			row.AverageOpenReturn = roundTrackingPercent(openTotal / count)
			row.AverageCloseReturn = roundTrackingPercent(closeTotal / count)
			row.AverageMaxGain = roundTrackingPercent(gainTotal / count)
			row.AverageMaxDrawdown = roundTrackingPercent(drawdownTotal / count)
		}
		summary.Horizons[key] = row
	}
	return summary
}

// EvaluateSelectionTracking computes forward returns from bars strictly after
// the signal date. Values are percentages (3 means +3%), matching the review
// API's existing convention.
func EvaluateSelectionTracking(item SelectionResult, bars []TrackingBar, horizons []int, targetReturn, drawdownLimit float64, now time.Time) SelectionTracking {
	targetReturn, drawdownLimit = DefaultTrackingPolicy(targetReturn, drawdownLimit)
	if len(horizons) == 0 {
		horizons = []int{1, 5, 10}
	}
	tracking := SelectionTracking{
		Version:       "1",
		SignalDate:    selectionResultSignalDate(item),
		BasePrice:     0,
		TargetReturn:  targetReturn,
		DrawdownLimit: drawdownLimit,
		UpdatedAt:     now.Format(time.RFC3339),
		Horizons:      map[string]SelectionHorizon{},
	}
	if tracking.SignalDate == 0 {
		for _, horizon := range horizons {
			tracking.Horizons[horizonKey(horizon)] = unavailableHorizon(horizon, targetReturn, drawdownLimit, "无法解析信号日期")
		}
		return tracking
	}

	ordered := append([]TrackingBar(nil), bars...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date < ordered[j].Date })
	forward := make([]TrackingBar, 0, len(ordered))
	for _, bar := range ordered {
		if bar.Date <= tracking.SignalDate && bar.Close > 0 {
			tracking.BasePrice = bar.Close
		}
		if bar.Date > tracking.SignalDate && bar.Date > 0 {
			forward = append(forward, bar)
		}
	}
	if tracking.BasePrice <= 0 {
		tracking.BasePrice = item.Latest
	}
	if tracking.BasePrice <= 0 && len(forward) > 0 {
		tracking.BasePrice = forward[0].YClose
		if tracking.BasePrice <= 0 {
			tracking.BasePrice = forward[0].Open
		}
	}
	if tracking.BasePrice <= 0 {
		for _, horizon := range horizons {
			tracking.Horizons[horizonKey(horizon)] = unavailableHorizon(horizon, targetReturn, drawdownLimit, "缺少有效基准价格")
		}
		return tracking
	}

	for _, horizon := range horizons {
		result := SelectionHorizon{HorizonDays: horizon, TargetReturn: targetReturn, DrawdownLimit: drawdownLimit}
		if horizon <= 0 {
			result.Status = "unavailable"
			result.Reason = "观察窗口必须大于0"
			tracking.Horizons[horizonKey(horizon)] = result
			continue
		}
		if len(forward) == 0 {
			result.Status = "pending"
			result.Reason = "暂无信号后的交易日K线"
			tracking.Horizons[horizonKey(horizon)] = result
			continue
		}
		if len(forward) < horizon {
			result.Status = "pending"
			result.AsOfDate = forward[len(forward)-1].Date
			result.Reason = fmt.Sprintf("已获得%d/%d个交易日", len(forward), horizon)
			tracking.Horizons[horizonKey(horizon)] = result
			continue
		}
		window := forward[:horizon]
		result.Status = "complete"
		result.AsOfDate = window[len(window)-1].Date
		result.OpenReturn = roundTrackingPercent(pctTracking(window[0].Open, tracking.BasePrice))
		result.CloseReturn = roundTrackingPercent(pctTracking(window[len(window)-1].Close, tracking.BasePrice))
		maxGain := math.Inf(-1)
		maxDrawdown := math.Inf(1)
		for _, bar := range window {
			if gain := pctTracking(bar.High, tracking.BasePrice); gain > maxGain {
				maxGain = gain
			}
			if drawdown := pctTracking(bar.Low, tracking.BasePrice); drawdown < maxDrawdown {
				maxDrawdown = drawdown
			}
		}
		result.MaxGain = roundTrackingPercent(maxGain)
		result.MaxDrawdown = roundTrackingPercent(maxDrawdown)
		result.Success = result.CloseReturn >= targetReturn && result.MaxDrawdown >= -drawdownLimit
		if result.Success {
			result.Reason = "达到收益与回撤标准"
		} else {
			result.Reason = "未达到收益与回撤标准"
		}
		tracking.Horizons[horizonKey(horizon)] = result
	}
	return tracking
}

func selectionResultSignalDate(item SelectionResult) int {
	if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
		return trackingDateInt(t)
	}
	if len(item.CreatedAt) >= 10 {
		if t, err := time.Parse("2006-01-02", item.CreatedAt[:10]); err == nil {
			return trackingDateInt(t)
		}
	}
	return 0
}

func trackingDateInt(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }

func horizonKey(horizon int) string { return fmt.Sprintf("d%d", horizon) }

func unavailableHorizon(horizon int, targetReturn, drawdownLimit float64, reason string) SelectionHorizon {
	return SelectionHorizon{HorizonDays: horizon, Status: "unavailable", TargetReturn: targetReturn, DrawdownLimit: drawdownLimit, Reason: reason}
}

func pctTracking(value, base float64) float64 {
	if base <= 0 || value == 0 {
		return 0
	}
	return (value/base - 1) * 100
}

func roundTrackingPercent(value float64) float64 {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0
	}
	return math.Round(value*100) / 100
}
