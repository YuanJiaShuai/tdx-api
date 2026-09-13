package workbench

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type DailyReviewBatchSummary struct {
	ID               string `json:"id"`
	Date             string `json:"date"`
	Status           string `json:"status"`
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at"`
	Error            string `json:"error,omitempty"`
	MatchedCount     int    `json:"matched_count"`
	StrategyCount    int    `json:"strategy_count"`
	FailedStrategies int    `json:"failed_strategies"`
	CandidateSymbols int    `json:"candidate_symbols"`
	KlineLoaded      int    `json:"kline_loaded"`
	KlineFailed      int    `json:"kline_failed"`
	DurationMS       int64  `json:"duration_ms"`
}

type DailyReviewStrategySummary struct {
	RunID        string `json:"run_id,omitempty"`
	StrategyID   string `json:"strategy_id"`
	StrategyName string `json:"strategy_name"`
	Status       string `json:"status"`
	Matched      int    `json:"matched"`
	Errors       int    `json:"errors"`
	Error        string `json:"error,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	FinishedAt   string `json:"finished_at,omitempty"`
	DurationMS   int64  `json:"duration_ms"`
}

type DailyReviewSignal struct {
	ID           string  `json:"id"`
	RunID        string  `json:"run_id"`
	StrategyID   string  `json:"strategy_id"`
	StrategyName string  `json:"strategy_name"`
	SignalDate   string  `json:"signal_date"`
	Symbol       string  `json:"symbol"`
	Latest       float64 `json:"latest"`
	Score        float64 `json:"score"`
	DetailJSON   string  `json:"detail_json"`
	TrackingJSON string  `json:"tracking_json"`
	CreatedAt    string  `json:"created_at"`
}

type DailyReviewStock struct {
	Symbol        string              `json:"symbol"`
	Latest        float64             `json:"latest"`
	StrategyCount int                 `json:"strategy_count"`
	StrategyIDs   []string            `json:"strategy_ids"`
	Strategies    []string            `json:"strategies"`
	MaxScore      float64             `json:"max_score"`
	AverageScore  float64             `json:"average_score"`
	TrackingJSON  string              `json:"tracking_json"`
	Status        string              `json:"status"`
	Watch         bool                `json:"watch"`
	Excluded      bool                `json:"excluded"`
	Note          DecisionNote        `json:"note"`
	Signals       []DailyReviewSignal `json:"signals"`
}

type DailyReviewSummary struct {
	StrategyCount    int   `json:"strategy_count"`
	RawSignals       int   `json:"raw_signals"`
	StockCount       int   `json:"stock_count"`
	ConsensusCount   int   `json:"consensus_count"`
	WatchCount       int   `json:"watch_count"`
	ExcludeCount     int   `json:"exclude_count"`
	CandidateSymbols int   `json:"candidate_symbols"`
	KlineLoaded      int   `json:"kline_loaded"`
	KlineFailed      int   `json:"kline_failed"`
	DurationMS       int64 `json:"duration_ms"`
}

type DailyReviewData struct {
	Date          string                       `json:"date"`
	SelectedBatch *DailyReviewBatchSummary     `json:"selected_batch,omitempty"`
	Batches       []DailyReviewBatchSummary    `json:"batches"`
	Summary       DailyReviewSummary           `json:"summary"`
	Strategies    []DailyReviewStrategySummary `json:"strategies"`
	Stocks        []DailyReviewStock           `json:"stocks"`
	Signals       []DailyReviewSignal          `json:"signals"`
	Consensus     []DailyReviewStock           `json:"consensus"`
	Watch         []string                     `json:"watch"`
	Exclude       []string                     `json:"exclude"`
	Notes         []DecisionNote               `json:"notes"`
}

type dailyBatchResult struct {
	Strategies       []dailyBatchStrategy `json:"strategies"`
	StrategyCount    int                  `json:"strategy_count"`
	FailedStrategies int                  `json:"failed_strategies"`
	CandidateSymbols int                  `json:"candidate_symbols"`
	KlineLoaded      int                  `json:"kline_loaded"`
	KlineFailed      int                  `json:"kline_failed"`
}

type dailyBatchStrategy struct {
	StrategyID string `json:"strategy_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Matched    int    `json:"matched"`
	Errors     int    `json:"errors"`
	Error      string `json:"error"`
}

func (s *AppStore) ListDailyReviewBatches(limit int) ([]DailyReviewBatchSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	runs, err := s.ListAutomationRuns(SystemStrategyDailyBatchTaskID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]DailyReviewBatchSummary, 0, len(runs))
	for _, run := range runs {
		items = append(items, dailyReviewBatchSummary(run))
	}
	return items, nil
}

func dailyReviewBatchSummary(run AutomationRun) DailyReviewBatchSummary {
	var result dailyBatchResult
	_ = json.Unmarshal([]byte(run.ResultJSON), &result)
	strategyCount := result.StrategyCount
	if strategyCount == 0 {
		strategyCount = len(result.Strategies)
	}
	return DailyReviewBatchSummary{
		ID: run.ID, Date: textDate(run.StartedAt), Status: run.Status,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, Error: run.Log,
		MatchedCount: run.MatchedCount, StrategyCount: strategyCount,
		FailedStrategies: result.FailedStrategies, CandidateSymbols: result.CandidateSymbols,
		KlineLoaded: result.KlineLoaded, KlineFailed: result.KlineFailed,
		DurationMS: elapsedMilliseconds(run.StartedAt, run.FinishedAt),
	}
}

func (s *AppStore) BuildDailyReview(date, batchID string, limit int) (DailyReviewData, error) {
	batches, err := s.ListDailyReviewBatches(40)
	if err != nil {
		return DailyReviewData{}, err
	}
	data := DailyReviewData{
		Batches: batches, Strategies: []DailyReviewStrategySummary{}, Stocks: []DailyReviewStock{},
		Signals: []DailyReviewSignal{}, Consensus: []DailyReviewStock{}, Watch: []string{}, Exclude: []string{}, Notes: []DecisionNote{},
	}
	var selected *DailyReviewBatchSummary
	for index := range batches {
		candidate := &batches[index]
		if batchID != "" && candidate.ID == batchID {
			selected = candidate
			break
		}
		if batchID == "" && (date == "" || candidate.Date == date) {
			selected = candidate
			break
		}
	}
	if selected == nil {
		if batchID != "" || date != "" {
			return data, sql.ErrNoRows
		}
		return data, nil
	}
	selectedCopy := *selected
	data.SelectedBatch = &selectedCopy
	data.Date = selected.Date

	parentRun, err := s.GetAutomationRun(selected.ID)
	if err != nil {
		return data, err
	}
	children, err := s.ListAutomationChildRuns(parentRun)
	if err != nil {
		return data, err
	}
	strategies := buildDailyStrategySummaries(parentRun, children)
	runIDs := make([]string, 0, len(children))
	for _, child := range children {
		runIDs = append(runIDs, child.ID)
	}
	results, err := s.ListSelectionResultsByRunIDs(runIDs, limit)
	if err != nil {
		return data, err
	}
	notes, err := s.ListDecisionNotes("", 5000)
	if err != nil {
		return data, err
	}
	noteMap := make(map[string]DecisionNote, len(notes))
	for _, note := range notes {
		noteMap[strings.ToUpper(note.Symbol)] = note
	}
	watchPool, _ := s.GetStockPool(DecisionWatchPoolID)
	excludePool, _ := s.GetStockPool(DecisionExcludePoolID)
	watchSet := stringSet(watchPool.Symbols)
	excludeSet := stringSet(excludePool.Symbols)

	strategyIDsByRun := make(map[string]string, len(children))
	for _, child := range children {
		strategyIDsByRun[child.ID] = strings.TrimPrefix(child.TaskID, "system-strategy:")
	}
	signals := make([]DailyReviewSignal, 0, len(results))
	stockMap := map[string]*DailyReviewStock{}
	for _, item := range results {
		symbol := strings.ToUpper(item.Symbol)
		signal := DailyReviewSignal{
			ID: item.ID, RunID: item.RunID, StrategyID: strategyIDsByRun[item.RunID], StrategyName: item.FormulaName,
			SignalDate: selected.Date, Symbol: symbol, Latest: item.Latest, Score: selectionResultScore(item.DetailJSON),
			DetailJSON: item.DetailJSON, TrackingJSON: item.TrackingJSON, CreatedAt: item.CreatedAt,
		}
		signals = append(signals, signal)
		stock := stockMap[symbol]
		if stock == nil {
			status := noteMap[symbol].Status
			if excludeSet[symbol] {
				status = "exclude"
			} else if watchSet[symbol] {
				status = "watch"
			}
			stock = &DailyReviewStock{
				Symbol: symbol, Latest: item.Latest, MaxScore: signal.Score, TrackingJSON: item.TrackingJSON,
				Status: status, Watch: watchSet[symbol], Excluded: excludeSet[symbol], Note: noteMap[symbol],
				StrategyIDs: []string{}, Strategies: []string{}, Signals: []DailyReviewSignal{},
			}
			stockMap[symbol] = stock
		}
		stock.Signals = append(stock.Signals, signal)
		stock.StrategyIDs = appendUnique(stock.StrategyIDs, signal.StrategyID)
		stock.Strategies = appendUnique(stock.Strategies, signal.StrategyName)
		if signal.Score > stock.MaxScore {
			stock.MaxScore = signal.Score
		}
		if trackingCompleteness(item.TrackingJSON) > trackingCompleteness(stock.TrackingJSON) {
			stock.TrackingJSON = item.TrackingJSON
		}
	}

	stocks := make([]DailyReviewStock, 0, len(stockMap))
	consensus := []DailyReviewStock{}
	watchCount, excludeCount := 0, 0
	for _, stock := range stockMap {
		stock.StrategyCount = len(stock.StrategyIDs)
		totalScore := 0.0
		for _, signal := range stock.Signals {
			totalScore += signal.Score
		}
		if len(stock.Signals) > 0 {
			stock.AverageScore = roundDailyFloat(totalScore / float64(len(stock.Signals)))
		}
		stock.MaxScore = roundDailyFloat(stock.MaxScore)
		if stock.Watch {
			watchCount++
		}
		if stock.Excluded {
			excludeCount++
		}
		stocks = append(stocks, *stock)
	}
	sort.Slice(stocks, func(i, j int) bool {
		if stocks[i].StrategyCount == stocks[j].StrategyCount {
			if stocks[i].MaxScore == stocks[j].MaxScore {
				return stocks[i].Symbol < stocks[j].Symbol
			}
			return stocks[i].MaxScore > stocks[j].MaxScore
		}
		return stocks[i].StrategyCount > stocks[j].StrategyCount
	})
	for _, stock := range stocks {
		if stock.StrategyCount >= 2 {
			consensus = append(consensus, stock)
		}
	}
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].Score == signals[j].Score {
			return signals[i].Symbol < signals[j].Symbol
		}
		return signals[i].Score > signals[j].Score
	})

	data.Strategies = strategies
	data.Stocks = stocks
	data.Signals = signals
	data.Consensus = consensus
	data.Watch = watchPool.Symbols
	data.Exclude = excludePool.Symbols
	data.Notes = notes
	data.Summary = DailyReviewSummary{
		StrategyCount: len(strategies), RawSignals: len(signals), StockCount: len(stocks), ConsensusCount: len(consensus),
		WatchCount: watchCount, ExcludeCount: excludeCount, CandidateSymbols: selected.CandidateSymbols,
		KlineLoaded: selected.KlineLoaded, KlineFailed: selected.KlineFailed, DurationMS: selected.DurationMS,
	}
	return data, nil
}

func (s *AppStore) ListAutomationChildRuns(parent AutomationRun) ([]AutomationRun, error) {
	query := `SELECT id,parent_run_id,task_id,task_name,task_type,status,started_at,finished_at,log,result_json,matched_count
		FROM automation_runs WHERE parent_run_id=? ORDER BY started_at`
	runs, err := scanAutomationRuns(s.db.Query(query, parent.ID))
	if err != nil || len(runs) > 0 {
		return runs, err
	}
	// Fallback for databases opened before the parent_run_id migration.
	end := parent.FinishedAt
	if end == "" {
		if started, parseErr := time.Parse(time.RFC3339, parent.StartedAt); parseErr == nil {
			end = started.Add(2 * time.Hour).Format(time.RFC3339)
		}
	}
	return scanAutomationRuns(s.db.Query(`SELECT id,parent_run_id,task_id,task_name,task_type,status,started_at,finished_at,log,result_json,matched_count
		FROM automation_runs WHERE task_id LIKE 'system-strategy:%' AND started_at>=? AND started_at<=? ORDER BY started_at`, parent.StartedAt, end))
}

func scanAutomationRuns(rows *sql.Rows, err error) ([]AutomationRun, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AutomationRun{}
	for rows.Next() {
		var run AutomationRun
		if err := rows.Scan(&run.ID, &run.ParentRunID, &run.TaskID, &run.TaskName, &run.TaskType, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Log, &run.ResultJSON, &run.MatchedCount); err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

func (s *AppStore) ListSelectionResultsByRunIDs(runIDs []string, limit int) ([]SelectionResult, error) {
	if len(runIDs) == 0 {
		return []SelectionResult{}, nil
	}
	if limit <= 0 || limit > 20000 {
		limit = 5000
	}
	placeholders := make([]string, len(runIDs))
	args := make([]interface{}, 0, len(runIDs)+1)
	for index, runID := range runIDs {
		placeholders[index] = "?"
		args = append(args, runID)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id,run_id,task_id,task_name,formula_id,formula_name,symbol,latest,detail_json,tracking_json,created_at
		FROM selection_results WHERE run_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SelectionResult{}
	for rows.Next() {
		var item SelectionResult
		if err := rows.Scan(&item.ID, &item.RunID, &item.TaskID, &item.TaskName, &item.FormulaID, &item.FormulaName, &item.Symbol, &item.Latest, &item.DetailJSON, &item.TrackingJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func buildDailyStrategySummaries(parent AutomationRun, children []AutomationRun) []DailyReviewStrategySummary {
	var parentResult dailyBatchResult
	_ = json.Unmarshal([]byte(parent.ResultJSON), &parentResult)
	parentByID := map[string]dailyBatchStrategy{}
	for _, summary := range parentResult.Strategies {
		parentByID[summary.StrategyID] = summary
	}
	items := make([]DailyReviewStrategySummary, 0, len(children)+len(parentByID))
	seen := map[string]bool{}
	for _, child := range children {
		strategyID := strings.TrimPrefix(child.TaskID, "system-strategy:")
		parentSummary := parentByID[strategyID]
		errorCount := parentSummary.Errors
		if errorCount == 0 {
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			if json.Unmarshal([]byte(child.ResultJSON), &result) == nil {
				errorCount = len(result.Errors)
			}
		}
		items = append(items, DailyReviewStrategySummary{
			RunID: child.ID, StrategyID: strategyID, StrategyName: child.TaskName, Status: child.Status,
			Matched: child.MatchedCount, Errors: errorCount, Error: child.Log,
			StartedAt: child.StartedAt, FinishedAt: child.FinishedAt, DurationMS: elapsedMilliseconds(child.StartedAt, child.FinishedAt),
		})
		seen[strategyID] = true
	}
	for strategyID, summary := range parentByID {
		if seen[strategyID] {
			continue
		}
		items = append(items, DailyReviewStrategySummary{
			StrategyID: strategyID, StrategyName: summary.Name, Status: summary.Status,
			Matched: summary.Matched, Errors: summary.Errors, Error: summary.Error,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Matched == items[j].Matched {
			return items[i].StrategyName < items[j].StrategyName
		}
		return items[i].Matched > items[j].Matched
	})
	return items
}

func selectionResultScore(detailJSON string) float64 {
	var detail struct {
		Score float64 `json:"score"`
	}
	_ = json.Unmarshal([]byte(detailJSON), &detail)
	return detail.Score
}

func trackingCompleteness(raw string) int {
	var tracking SelectionTracking
	if json.Unmarshal([]byte(raw), &tracking) != nil {
		return 0
	}
	count := 0
	for _, horizon := range tracking.Horizons {
		if horizon.Status == "complete" {
			count++
		}
	}
	return count
}

func textDate(value string) string {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.In(time.Local).Format("2006-01-02")
	}
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

func elapsedMilliseconds(startRaw, endRaw string) int64 {
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return 0
	}
	end := time.Now()
	if endRaw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, endRaw); parseErr == nil {
			end = parsed
		}
	}
	if end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

func stringSet(symbols []string) map[string]bool {
	result := make(map[string]bool, len(symbols))
	for _, symbol := range NormalizeSymbols(symbols) {
		result[symbol] = true
	}
	return result
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func roundDailyFloat(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}
