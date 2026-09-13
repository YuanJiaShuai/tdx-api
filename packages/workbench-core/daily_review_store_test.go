package workbench

import "testing"

func TestBuildDailyReviewAggregatesSystemStrategyBatch(t *testing.T) {
	store := newHistoricalBacktestTestStore(t)
	parent, err := store.CreateAutomationRun(AutomationTask{
		ID: SystemStrategyDailyBatchTaskID, Name: "系统策略日报", Type: "system_strategy_batch",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateAutomationRun(AutomationTask{
		ID: "system-strategy:trend", Name: "趋势策略", Type: "strategy_selection",
	}, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateAutomationRun(AutomationTask{
		ID: "system-strategy:volume", Name: "量价策略", Type: "strategy_selection",
	}, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	tracking := `{"horizons":{"d3":{"status":"complete","close_return":4.2}}}`
	if err := store.SaveSelectionResults(first, Formula{ID: "strategy:trend", Name: "趋势策略"}, []SelectionResult{
		{Symbol: "000001", Latest: 10, DetailJSON: `{"score":80}`, TrackingJSON: tracking},
		{Symbol: "000002", Latest: 20, DetailJSON: `{"score":60}`},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSelectionResults(second, Formula{ID: "strategy:volume", Name: "量价策略"}, []SelectionResult{
		{Symbol: "000001", Latest: 10, DetailJSON: `{"score":70}`},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishAutomationRun(first.ID, "success", "", `{}`, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishAutomationRun(second.ID, "success", "", `{}`, 1); err != nil {
		t.Fatal(err)
	}
	resultJSON := `{"strategy_count":2,"candidate_symbols":4606,"kline_loaded":4602,"kline_failed":4,"strategies":[{"strategy_id":"trend","name":"趋势策略","status":"success","matched":2},{"strategy_id":"volume","name":"量价策略","status":"success","matched":1}]}`
	if err := store.FinishAutomationRun(parent.ID, "success", "", resultJSON, 3); err != nil {
		t.Fatal(err)
	}

	review, err := store.BuildDailyReview("", parent.ID, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if review.SelectedBatch == nil || review.SelectedBatch.ID != parent.ID {
		t.Fatalf("selected batch = %#v", review.SelectedBatch)
	}
	if review.Summary.RawSignals != 3 || review.Summary.StockCount != 2 || review.Summary.ConsensusCount != 1 {
		t.Fatalf("unexpected summary: %#v", review.Summary)
	}
	if review.Summary.StrategyCount != 2 || review.Summary.CandidateSymbols != 4606 {
		t.Fatalf("unexpected batch facts: %#v", review.Summary)
	}
	if len(review.Consensus) != 1 || review.Consensus[0].Symbol != "000001" || review.Consensus[0].StrategyCount != 2 {
		t.Fatalf("unexpected consensus: %#v", review.Consensus)
	}
	if trackingCompleteness(review.Consensus[0].TrackingJSON) != 1 {
		t.Fatalf("completed tracking was not retained: %s", review.Consensus[0].TrackingJSON)
	}
}
