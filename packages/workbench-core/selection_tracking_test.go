package workbench

import (
	"testing"
	"time"
)

func TestEvaluateSelectionTracking(t *testing.T) {
	item := SelectionResult{ID: "hit-1", Symbol: "000001", Latest: 100, CreatedAt: "2026-01-01T18:00:00+08:00"}
	bars := []TrackingBar{
		{Date: 20260102, Open: 101, High: 103, Low: 99, Close: 102},
		{Date: 20260103, Open: 102, High: 104, Low: 98, Close: 101},
		{Date: 20260104, Open: 101, High: 105, Low: 100, Close: 103},
		{Date: 20260105, Open: 103, High: 106, Low: 101, Close: 104},
		{Date: 20260106, Open: 104, High: 107, Low: 102, Close: 104},
	}
	tracking := EvaluateSelectionTracking(item, bars, []int{1, 5, 10}, 3, 5, time.Date(2026, 1, 8, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if tracking.SignalDate != 20260101 || tracking.BasePrice != 100 {
		t.Fatalf("unexpected signal/base: %#v", tracking)
	}
	d1 := tracking.Horizons["d1"]
	if d1.Status != "complete" || d1.CloseReturn != 2 || d1.Success {
		t.Fatalf("unexpected d1: %#v", d1)
	}
	d5 := tracking.Horizons["d5"]
	if d5.Status != "complete" || d5.CloseReturn != 4 || d5.MaxDrawdown != -2 || !d5.Success {
		t.Fatalf("unexpected d5: %#v", d5)
	}
	if tracking.Horizons["d10"].Status != "pending" {
		t.Fatalf("expected pending d10: %#v", tracking.Horizons["d10"])
	}
}

func TestSummarizeSelectionTracking(t *testing.T) {
	items := []SelectionTrackingItem{
		{Tracking: SelectionTracking{Horizons: map[string]SelectionHorizon{"d5": {Status: "complete", CloseReturn: 4, MaxDrawdown: -2, Success: true}}}},
		{Tracking: SelectionTracking{Horizons: map[string]SelectionHorizon{"d5": {Status: "complete", CloseReturn: -1, MaxDrawdown: -6, Success: false}}}},
	}
	summary := SummarizeSelectionTracking(items, []int{5})
	d5 := summary.Horizons["d5"]
	if d5.Completed != 2 || d5.SuccessCount != 1 || d5.SuccessRate != 50 || d5.AverageCloseReturn != 1.5 {
		t.Fatalf("unexpected summary: %#v", d5)
	}
}
