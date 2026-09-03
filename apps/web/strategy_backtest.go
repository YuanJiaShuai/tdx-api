package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type StrategyBacktestRequest struct {
	StrategyID    string  `json:"strategy_id"`
	Engine        string  `json:"engine,omitempty"`
	StartDate     string  `json:"start_date,omitempty"`
	EndDate       string  `json:"end_date,omitempty"`
	HistoryCount  int     `json:"history_count,omitempty"`
	SymbolLimit   int     `json:"symbol_limit,omitempty"`
	InitialCash   float64 `json:"initial_cash,omitempty"`
	BuyCost       float64 `json:"buy_cost,omitempty"`
	SellCost      float64 `json:"sell_cost,omitempty"`
	StopLoss      float64 `json:"stop_loss,omitempty"`
	ProfitTrigger float64 `json:"profit_trigger,omitempty"`
	TrailingStop  float64 `json:"trailing_stop,omitempty"`
	MaxHold       int     `json:"max_hold,omitempty"`
	ExitMA        int     `json:"exit_ma,omitempty"`
	FastMA        int     `json:"fast_ma,omitempty"`
	SlowMA        int     `json:"slow_ma,omitempty"`
}

type StrategyBacktestResult struct {
	Strategy          Strategy                      `json:"strategy"`
	Config            StrategyConfig                `json:"config"`
	Request           StrategyBacktestRequest       `json:"request"`
	Symbols           int                           `json:"symbols"`
	Signals           int                           `json:"signals"`
	Trades            []StrategyBacktestTrade       `json:"trades"`
	EquityCurve       []StrategyBacktestEquityPoint `json:"equity_curve,omitempty"`
	Metrics           StrategyBacktestMetrics       `json:"metrics"`
	Warnings          []string                      `json:"warnings,omitempty"`
	Errors            map[string]string             `json:"errors,omitempty"`
	Engine            string                        `json:"engine"`
	CalculationEngine string                        `json:"calculation_engine,omitempty"`
	DataRevision      string                        `json:"data_revision,omitempty"`
}

type StrategyBacktestTrade struct {
	Symbol        string                 `json:"symbol"`
	EntryDate     string                 `json:"entry_date"`
	ExitDate      string                 `json:"exit_date"`
	EntryPrice    float64                `json:"entry_price"`
	ExitPrice     float64                `json:"exit_price"`
	Return        float64                `json:"return"`
	HoldDays      int                    `json:"hold_days"`
	Reason        string                 `json:"reason"`
	EntryScore    float64                `json:"entry_score"`
	EntryReasons  []string               `json:"entry_reasons,omitempty"`
	FactorResults []StrategyFactorResult `json:"factor_results,omitempty"`
}

type StrategyBacktestEquityPoint struct {
	Date   int     `json:"date"`
	Equity float64 `json:"equity"`
}

type StrategyBacktestMetrics struct {
	Symbols      int     `json:"symbols"`
	Trades       int     `json:"trades"`
	WinRate      float64 `json:"win_rate"`
	TotalReturn  float64 `json:"total_return"`
	CAGR         float64 `json:"cagr"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	ProfitFactor float64 `json:"profit_factor"`
	AvgTrade     float64 `json:"avg_trade"`
	Exposure     float64 `json:"exposure"`
	AvgHoldDays  float64 `json:"avg_hold_days"`
}

type strategyBacktestSymbolResult struct {
	Symbol      string
	Points      []StrategyBacktestEquityPoint
	Trades      []StrategyBacktestTrade
	Signals     int
	StartDate   int
	EndDate     int
	InitialCash float64
	FinalEquity float64
}

func (r *AutomationRunner) runStrategyBacktest(ctx context.Context, strategy Strategy, req StrategyBacktestRequest) (StrategyBacktestResult, error) {
	if strings.EqualFold(strings.TrimSpace(req.Engine), "hikyuu") {
		return r.runHikyuuReferenceBacktest(ctx, strategy, req)
	}
	var cfg StrategyConfig
	if err := json.Unmarshal([]byte(strategy.ConfigJSON), &cfg); err != nil {
		return StrategyBacktestResult{}, err
	}
	if cfg.Period == "" {
		cfg.Period = "day"
	}
	if cfg.CalcCount <= 0 {
		cfg.CalcCount = 260
	}
	if req.HistoryCount <= 0 {
		req.HistoryCount = cfg.CalcCount
	}
	if req.HistoryCount <= 0 {
		req.HistoryCount = 520
	}
	if req.HistoryCount > 800 {
		req.HistoryCount = 800
	}
	if req.SymbolLimit <= 0 {
		req.SymbolLimit = cfg.BatchSize
	}
	if req.SymbolLimit <= 0 {
		req.SymbolLimit = 80
	}
	if req.InitialCash <= 0 {
		req.InitialCash = 100000
	}
	if req.BuyCost < 0 {
		req.BuyCost = 0.0005
	}
	if req.SellCost < 0 {
		req.SellCost = 0.001
	}
	if req.StopLoss <= 0 {
		req.StopLoss = 0.08
	}
	if req.ProfitTrigger <= 0 {
		req.ProfitTrigger = 0.12
	}
	if req.TrailingStop <= 0 {
		req.TrailingStop = 0.10
	}
	if req.MaxHold <= 0 {
		req.MaxHold = 40
	}
	if req.ExitMA <= 0 {
		req.ExitMA = 20
	}

	if hasFormulaFactor(cfg) {
		return StrategyBacktestResult{}, errors.New("回测暂不支持公式因子，请先改为本地因子再运行")
	}

	symbols, err := r.strategyUniverse(cfg)
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	if len(symbols) > req.SymbolLimit {
		symbols = symbols[:req.SymbolLimit]
	}
	if len(symbols) == 0 {
		return StrategyBacktestResult{}, errors.New("策略股票范围为空")
	}

	startDate, err := parseBacktestDate(req.StartDate)
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	endDate, err := parseBacktestDate(req.EndDate)
	if err != nil {
		return StrategyBacktestResult{}, err
	}

	backtestResult := StrategyBacktestResult{
		Strategy:          strategy,
		Config:            cfg,
		Request:           req,
		Symbols:           0,
		Warnings:          []string{},
		Errors:            map[string]string{},
		Engine:            "go",
		CalculationEngine: "tdx-workbench-go",
	}

	symbolResults := make([]strategyBacktestSymbolResult, 0, len(symbols))
	totalSignals := 0
	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return backtestResult, ctx.Err()
		default:
		}

		rows, loadErr := loadFormulaKline(symbol, cfg.Period, req.HistoryCount)
		if loadErr != nil {
			backtestResult.Errors[symbol] = loadErr.Error()
			continue
		}
		result, runErr := r.backtestSymbol(strategy, cfg, req, symbol, rows, startDate, endDate)
		if runErr != nil {
			backtestResult.Errors[symbol] = runErr.Error()
			continue
		}
		if len(result.Points) == 0 {
			continue
		}
		symbolResults = append(symbolResults, result)
		totalSignals += result.Signals
		backtestResult.Trades = append(backtestResult.Trades, result.Trades...)
	}
	if len(backtestResult.Errors) == 0 {
		backtestResult.Errors = nil
	}
	backtestResult.Signals = totalSignals
	backtestResult.Symbols = len(symbolResults)

	if len(symbolResults) == 0 {
		return backtestResult, errors.New("回测没有可用标的")
	}

	curve, metrics := aggregateBacktestResults(symbolResults, req.InitialCash)
	backtestResult.EquityCurve = curve
	metrics.Symbols = len(symbolResults)
	metrics.Trades = len(backtestResult.Trades)
	metrics.WinRate, metrics.ProfitFactor, metrics.AvgTrade, metrics.AvgHoldDays = backtestTradeStats(backtestResult.Trades)
	metrics.TotalReturn, metrics.CAGR, metrics.MaxDrawdown, metrics.Exposure = backtestCurveStats(curve, symbolResults, req.InitialCash)
	backtestResult.Metrics = metrics

	sort.Slice(backtestResult.Trades, func(i, j int) bool {
		if backtestResult.Trades[i].ExitDate == backtestResult.Trades[j].ExitDate {
			if backtestResult.Trades[i].Symbol == backtestResult.Trades[j].Symbol {
				return backtestResult.Trades[i].EntryDate > backtestResult.Trades[j].EntryDate
			}
			return backtestResult.Trades[i].Symbol < backtestResult.Trades[j].Symbol
		}
		return backtestResult.Trades[i].ExitDate > backtestResult.Trades[j].ExitDate
	})

	return backtestResult, nil
}

type hikyuuBacktestEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Engine            string                        `json:"engine"`
		CalculationEngine string                        `json:"calculation_engine"`
		Symbols           int                           `json:"symbols"`
		Signals           int                           `json:"signals"`
		Trades            []StrategyBacktestTrade       `json:"trades"`
		EquityCurve       []StrategyBacktestEquityPoint `json:"equity_curve"`
		Metrics           StrategyBacktestMetrics       `json:"metrics"`
		Warnings          []string                      `json:"warnings"`
		Meta              struct {
			DataRevision string `json:"data_revision"`
		} `json:"meta"`
	} `json:"data"`
}

func (r *AutomationRunner) runHikyuuReferenceBacktest(ctx context.Context, strategy Strategy, req StrategyBacktestRequest) (StrategyBacktestResult, error) {
	var cfg StrategyConfig
	if err := json.Unmarshal([]byte(strategy.ConfigJSON), &cfg); err != nil {
		return StrategyBacktestResult{}, err
	}
	if cfg.Period == "" {
		cfg.Period = "day"
	}
	symbols, err := r.strategyUniverse(cfg)
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	if req.SymbolLimit > 0 && len(symbols) > req.SymbolLimit {
		symbols = symbols[:req.SymbolLimit]
	}
	if len(symbols) == 0 {
		return StrategyBacktestResult{}, errors.New("策略股票范围为空")
	}
	if req.InitialCash <= 0 {
		req.InitialCash = 100000
	}
	if req.HistoryCount <= 0 {
		req.HistoryCount = 520
	}
	if req.FastMA <= 0 {
		req.FastMA = 5
	}
	if req.SlowMA <= req.FastMA {
		req.SlowMA = 20
	}
	if req.BuyCost <= 0 {
		req.BuyCost = 0.0005
	}
	if req.SellCost <= 0 {
		req.SellCost = 0.001
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("HIKYUU_DATA_SERVICE_URL")), "/")
	if baseURL == "" {
		return StrategyBacktestResult{}, errors.New("未配置 HIKYUU_DATA_SERVICE_URL")
	}
	payload := map[string]interface{}{
		"symbols":       symbols,
		"type":          cfg.Period,
		"start":         req.StartDate,
		"end":           req.EndDate,
		"history_count": req.HistoryCount,
		"initial_cash":  req.InitialCash,
		"buy_cost":      req.BuyCost,
		"sell_cost":     req.SellCost,
		"fast":          req.FastMA,
		"slow":          req.SlowMA,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/hikyuu/backtest", bytes.NewReader(body))
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Minute}).Do(httpReq)
	if err != nil {
		return StrategyBacktestResult{}, fmt.Errorf("Hikyuu 回测请求失败: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return StrategyBacktestResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StrategyBacktestResult{}, fmt.Errorf("Hikyuu 回测返回 %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	var envelope hikyuuBacktestEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return StrategyBacktestResult{}, fmt.Errorf("Hikyuu 回测响应格式错误: %w", err)
	}
	if envelope.Code != 0 {
		return StrategyBacktestResult{}, errors.New(envelope.Message)
	}
	return StrategyBacktestResult{
		Strategy:          strategy,
		Config:            cfg,
		Request:           req,
		Symbols:           envelope.Data.Symbols,
		Signals:           envelope.Data.Signals,
		Trades:            envelope.Data.Trades,
		EquityCurve:       envelope.Data.EquityCurve,
		Metrics:           envelope.Data.Metrics,
		Warnings:          envelope.Data.Warnings,
		Engine:            "hikyuu",
		CalculationEngine: envelope.Data.CalculationEngine,
		DataRevision:      envelope.Data.Meta.DataRevision,
	}, nil
}

func (r *AutomationRunner) backtestSymbol(strategy Strategy, cfg StrategyConfig, req StrategyBacktestRequest, symbol string, rows []FormulaKline, startDate, endDate int) (strategyBacktestSymbolResult, error) {
	result := strategyBacktestSymbolResult{
		Symbol:      strategyNormalizeSymbol(symbol),
		InitialCash: req.InitialCash,
	}
	if len(rows) < 2 {
		return result, errors.New("K线不足")
	}

	startIdx := 0
	if startDate > 0 {
		for startIdx < len(rows) && rows[startIdx].Date < startDate {
			startIdx++
		}
	}
	if startIdx >= len(rows) {
		return result, nil
	}
	endIdx := len(rows) - 1
	if endDate > 0 {
		for endIdx >= 0 && rows[endIdx].Date > endDate {
			endIdx--
		}
	}
	if endIdx <= startIdx {
		return result, nil
	}

	warmupStart := startIdx - backtestWarmupBars(cfg)
	if warmupStart < 0 {
		warmupStart = 0
	}

	cash := req.InitialCash
	shares := 0.0
	entryPrice := 0.0
	entryIdx := -1
	entryDate := 0
	var entrySignal StrategySelectionItem
	highSinceEntry := 0.0
	signalRunner := &StrategyRunResult{
		Config:    cfg,
		PoolCache: map[string]map[string]bool{},
	}

	for i := warmupStart; i <= endIdx; i++ {
		row := rows[i]
		if row.Date < startDate && startDate > 0 {
			continue
		}

		result.Points = append(result.Points, StrategyBacktestEquityPoint{
			Date:   row.Date,
			Equity: cash + shares*row.Close,
		})

		if shares > 0 {
			if row.High > highSinceEntry {
				highSinceEntry = row.High
			}
			holdDays := i - entryIdx
			gain := 0.0
			if entryPrice > 0 {
				gain = row.Close/entryPrice - 1
			}
			drawFromHigh := 0.0
			if highSinceEntry > 0 {
				drawFromHigh = row.Close/highSinceEntry - 1
			}
			exitReason := ""
			switch {
			case gain <= -req.StopLoss:
				exitReason = "stop"
			case gain >= req.ProfitTrigger && drawFromHigh <= -req.TrailingStop:
				exitReason = "trail"
			case row.Close < ma(rows[:i+1], req.ExitMA):
				exitReason = "ma"
			case holdDays >= req.MaxHold:
				exitReason = "time"
			}

			if exitReason != "" && i < len(rows)-1 && i < endIdx {
				exitFill := rows[i+1].Open
				if exitFill <= 0 {
					exitFill = rows[i+1].Close
				}
				proceeds := shares * exitFill * (1 - req.SellCost)
				result.Trades = append(result.Trades, StrategyBacktestTrade{
					Symbol:        result.Symbol,
					EntryDate:     backtestDateText(entryDate),
					ExitDate:      backtestDateText(rows[i+1].Date),
					EntryPrice:    entryPrice,
					ExitPrice:     exitFill,
					Return:        tradeReturn(entryPrice, exitFill, req.BuyCost, req.SellCost),
					HoldDays:      i + 1 - entryIdx,
					Reason:        exitReason,
					EntryScore:    entrySignal.Score,
					EntryReasons:  append([]string{}, entrySignal.Reasons...),
					FactorResults: append([]StrategyFactorResult{}, entrySignal.FactorResults...),
				})
				cash = proceeds
				shares = 0
				entryPrice = 0
				entryIdx = -1
				entryDate = 0
				entrySignal = StrategySelectionItem{}
				highSinceEntry = 0
			}
			continue
		}

		if i < startIdx || i >= len(rows)-1 {
			continue
		}

		signal, ok := r.backtestStrategySignal(signalRunner, cfg, symbol, rows[:i+1])
		if !ok || !signal.Hit {
			continue
		}

		buyFill := rows[i+1].Open
		if buyFill <= 0 {
			buyFill = rows[i+1].Close
		}
		if buyFill <= 0 {
			continue
		}
		shares = cash * (1 - req.BuyCost) / buyFill
		cash = 0
		entryPrice = buyFill
		entryIdx = i + 1
		entryDate = rows[i+1].Date
		highSinceEntry = entryPrice
		result.Signals++
		entrySignal = signal
	}

	if shares > 0 {
		last := rows[endIdx]
		exitFill := last.Close
		if exitFill <= 0 {
			exitFill = rows[endIdx].Open
		}
		if exitFill > 0 {
			proceeds := shares * exitFill * (1 - req.SellCost)
			cash = proceeds
			shares = 0
			result.Trades = append(result.Trades, StrategyBacktestTrade{
				Symbol:     result.Symbol,
				EntryDate:  backtestDateText(entryDate),
				ExitDate:   backtestDateText(last.Date),
				EntryPrice: entryPrice,
				ExitPrice:  exitFill,
				Return:     tradeReturn(entryPrice, exitFill, req.BuyCost, req.SellCost),
				HoldDays:   endIdx - entryIdx,
				Reason:     "final",
			})
		}
	}

	result.FinalEquity = cash
	if len(result.Points) == 0 && len(rows) > 0 {
		result.Points = append(result.Points, StrategyBacktestEquityPoint{
			Date:   rows[endIdx].Date,
			Equity: cash,
		})
	}
	return result, nil
}

func (r *AutomationRunner) backtestStrategySignal(result *StrategyRunResult, cfg StrategyConfig, symbol string, rows []FormulaKline) (StrategySelectionItem, bool) {
	item := StrategySelectionItem{Symbol: strategyNormalizeSymbol(symbol), Hit: true}
	if len(rows) == 0 {
		return item, false
	}
	item.Latest = rows[len(rows)-1].Close
	for _, rule := range cfg.Filters {
		fr := r.evaluateBacktestFactor(result, symbol, rows, rule, true)
		item.FactorResults = append(item.FactorResults, fr)
		item.Reasons = append(item.Reasons, fr.Reason)
		if !fr.Hit {
			item.Hit = false
			return item, false
		}
	}
	score := 0.0
	for _, rule := range cfg.Scores {
		fr := r.evaluateBacktestFactor(result, symbol, rows, rule, false)
		item.FactorResults = append(item.FactorResults, fr)
		item.Reasons = append(item.Reasons, fr.Reason)
		if fr.Hit {
			score += fr.Score
		}
	}
	item.Score = score
	minScore := cfg.Pass.MinScore
	if minScore <= 0 {
		minScore = 1
	}
	item.Hit = score >= minScore
	return item, item.Hit
}

func (r *AutomationRunner) evaluateBacktestFactor(result *StrategyRunResult, symbol string, rows []FormulaKline, rule StrategyFactorRule, filter bool) StrategyFactorResult {
	fr := StrategyFactorResult{ID: rule.ID, Factor: rule.Factor}
	weight := rule.Weight
	if filter {
		weight = 0
	}
	switch rule.Factor {
	case "pool_exclude":
		poolID := stringParam(rule.Params, "pool_id", DecisionExcludePoolID)
		inPool := r.strategyPoolContains(result, poolID, symbol)
		fr.Hit = !inPool
		fr.Value = inPool
		fr.Reason = fmt.Sprintf("不在%s: %t", poolID, fr.Hit)
	case "min_amount":
		value := floatParam(rule.Params, "value", 0)
		amount := latest(rows).Amount
		fr.Hit = amount >= value
		fr.Value = amount
		fr.Reason = fmt.Sprintf("成交额 %.0f >= %.0f", amount, value)
	case "price_range":
		minValue := floatParam(rule.Params, "min", 0)
		maxValue := floatParam(rule.Params, "max", math.MaxFloat64)
		closePrice := latest(rows).Close
		fr.Hit = closePrice >= minValue && closePrice <= maxValue
		fr.Value = closePrice
		fr.Reason = fmt.Sprintf("收盘价 %.2f 在 %.2f-%.2f", closePrice, minValue, maxValue)
	case "change_range":
		minValue := floatParam(rule.Params, "min", -math.MaxFloat64)
		maxValue := floatParam(rule.Params, "max", math.MaxFloat64)
		row := latest(rows)
		change := 0.0
		if row.YClose > 0 {
			change = (row.Close - row.YClose) * 100 / row.YClose
		}
		fr.Hit = change >= minValue && change <= maxValue
		fr.Value = change
		fr.Reason = fmt.Sprintf("涨跌幅 %.2f%% 在 %.2f-%.2f", change, minValue, maxValue)
	case "ma_trend":
		short := intParam(rule.Params, "short", 5)
		mid := intParam(rule.Params, "mid", 10)
		long := intParam(rule.Params, "long", 20)
		maShort, maMid, maLong := ma(rows, short), ma(rows, mid), ma(rows, long)
		closePrice := latest(rows).Close
		fr.Hit = closePrice >= maShort && maShort >= maMid && maMid >= maLong
		fr.Value = map[string]float64{"close": closePrice, "short": maShort, "mid": maMid, "long": maLong}
		fr.Reason = fmt.Sprintf("均线多头 C %.2f / MA%d %.2f / MA%d %.2f / MA%d %.2f", closePrice, short, maShort, mid, maMid, long, maLong)
	case "volume_up":
		days := intParam(rule.Params, "days", 5)
		ratio := floatParam(rule.Params, "ratio", 1.3)
		avg := strategyAvgVol(rows, days)
		vol := latest(rows).Vol
		fr.Hit = avg > 0 && vol >= avg*ratio
		fr.Value = map[string]float64{"volume": vol, "avg_volume": avg}
		fr.Reason = fmt.Sprintf("放量 %.0f >= %.2fx %d日均量 %.0f", vol, ratio, days, avg)
	case "break_high":
		days := intParam(rule.Params, "days", 20)
		high := highestHigh(rows, days)
		closePrice := latest(rows).Close
		fr.Hit = high > 0 && closePrice >= high
		fr.Value = map[string]float64{"close": closePrice, "high": high}
		fr.Reason = fmt.Sprintf("突破%d日高点 C %.2f / H %.2f", days, closePrice, high)
	case "macd_golden_cross":
		fr.Hit, fr.Score, fr.Reason = evaluateMACDSignal(rows, rule, true)
		fr.Value = fr.Score
	case "macd_dead_cross":
		fr.Hit, fr.Score, fr.Reason = evaluateMACDSignal(rows, rule, false)
		fr.Value = fr.Score
	case "kdj_golden_cross":
		fr.Hit, fr.Score, fr.Reason = evaluateKDJGoldenCross(rows, rule)
		fr.Value = fr.Score
	case "rsi_oversold":
		fr.Hit, fr.Score, fr.Reason = evaluateRSIOversold(rows, rule)
		fr.Value = fr.Score
	case "boll_breakout":
		fr.Hit, fr.Score, fr.Reason = evaluateBOLLBreakout(rows, rule)
		fr.Value = fr.Score
	case "volume_breakout":
		fr.Hit, fr.Score, fr.Reason = evaluateVolumeBreakout(rows, rule)
		fr.Value = fr.Score
	case "local_rocket":
		fr.Hit, fr.Score, fr.Reason = evaluateLocalRocket(rows, rule)
		fr.Value = fr.Score
	case "formula":
		fr.Hit = false
		fr.Reason = "回测暂不支持公式因子"
	default:
		fr.Hit = false
		fr.Reason = "未知因子: " + rule.Factor
	}
	if fr.Hit {
		fr.Score = weight
	}
	return fr
}

func aggregateBacktestResults(items []strategyBacktestSymbolResult, initialCash float64) ([]StrategyBacktestEquityPoint, StrategyBacktestMetrics) {
	if len(items) == 0 {
		return nil, StrategyBacktestMetrics{}
	}

	dates := map[int]struct{}{}
	for _, item := range items {
		for _, point := range item.Points {
			dates[point.Date] = struct{}{}
		}
	}
	orderedDates := make([]int, 0, len(dates))
	for date := range dates {
		orderedDates = append(orderedDates, date)
	}
	sort.Ints(orderedDates)

	indices := make([]int, len(items))
	current := make([]float64, len(items))
	for i := range current {
		current[i] = initialCash
	}
	curve := make([]StrategyBacktestEquityPoint, 0, len(orderedDates))
	for _, date := range orderedDates {
		equity := 0.0
		for i := range items {
			for indices[i] < len(items[i].Points) && items[i].Points[indices[i]].Date <= date {
				current[i] = items[i].Points[indices[i]].Equity
				indices[i]++
			}
			equity += current[i]
		}
		curve = append(curve, StrategyBacktestEquityPoint{Date: date, Equity: equity})
	}

	return curve, StrategyBacktestMetrics{}
}

func backtestCurveStats(curve []StrategyBacktestEquityPoint, items []strategyBacktestSymbolResult, initialCash float64) (float64, float64, float64, float64) {
	if len(curve) == 0 {
		return 0, 0, 0, 0
	}
	finalEquity := curve[len(curve)-1].Equity
	totalInitial := initialCash * float64(len(items))
	totalReturn := 0.0
	if totalInitial > 0 {
		totalReturn = finalEquity/totalInitial - 1
	}

	startDate := curve[0].Date
	endDate := curve[len(curve)-1].Date
	years := backtestYearsBetween(startDate, endDate)
	cagr := 0.0
	if totalInitial > 0 && finalEquity > 0 && years > 0 {
		cagr = math.Pow(finalEquity/totalInitial, 1/years) - 1
	}

	peak := 0.0
	maxDrawdown := 0.0
	for _, point := range curve {
		if point.Equity > peak {
			peak = point.Equity
		}
		if peak > 0 {
			drawdown := point.Equity/peak - 1
			if drawdown < maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	exposureDays := 0
	for _, item := range items {
		for _, trade := range item.Trades {
			exposureDays += trade.HoldDays
		}
	}
	exposure := 0.0
	totalBars := len(curve)
	if totalBars > 0 && len(items) > 0 {
		exposure = float64(exposureDays) / float64(totalBars*len(items))
	}
	return totalReturn, cagr, maxDrawdown, exposure
}

func backtestTradeStats(trades []StrategyBacktestTrade) (winRate, profitFactor, avgTrade, avgHoldDays float64) {
	if len(trades) == 0 {
		return 0, 0, 0, 0
	}
	wins := 0
	sumWins := 0.0
	sumLoss := 0.0
	hold := 0.0
	sumTrade := 0.0
	for _, trade := range trades {
		sumTrade += trade.Return
		hold += float64(trade.HoldDays)
		if trade.Return > 0 {
			wins++
			sumWins += trade.Return
		} else {
			sumLoss += trade.Return
		}
	}
	winRate = float64(wins) / float64(len(trades))
	if sumLoss < 0 {
		profitFactor = sumWins / math.Abs(sumLoss)
	} else if sumWins > 0 {
		profitFactor = math.Inf(1)
	}
	avgTrade = sumTrade / float64(len(trades))
	avgHoldDays = hold / float64(len(trades))
	return
}

func backtestWarmupBars(cfg StrategyConfig) int {
	warmup := 120
	for _, rule := range append(append([]StrategyFactorRule{}, cfg.Filters...), cfg.Scores...) {
		switch rule.Factor {
		case "ma_trend":
			long := intParam(rule.Params, "long", 20)
			warmup = maxInt(warmup, long+5)
		case "volume_up":
			days := intParam(rule.Params, "days", 5)
			warmup = maxInt(warmup, days+5)
		case "break_high":
			days := intParam(rule.Params, "days", 20)
			warmup = maxInt(warmup, days+5)
		case "macd_golden_cross", "macd_dead_cross":
			slow := intParam(rule.Params, "slow", 26)
			signal := intParam(rule.Params, "signal", 9)
			warmup = maxInt(warmup, slow+signal+5)
		case "kdj_golden_cross":
			n := intParam(rule.Params, "n", 9)
			d := intParam(rule.Params, "d", 3)
			warmup = maxInt(warmup, n+d+5)
		case "rsi_oversold":
			period := intParam(rule.Params, "period", 6)
			warmup = maxInt(warmup, period+5)
		case "boll_breakout":
			period := intParam(rule.Params, "period", 20)
			warmup = maxInt(warmup, period+5)
		case "volume_breakout":
			days := intParam(rule.Params, "days", 20)
			warmup = maxInt(warmup, days+5)
		case "local_rocket":
			lookback := intParam(rule.Params, "lookback", 20)
			volumeDays := intParam(rule.Params, "volume_days", 5)
			midMA := intParam(rule.Params, "mid_ma", 10)
			warmup = maxInt(warmup, maxInt(lookback, maxInt(volumeDays+5, midMA+5)))
		}
	}
	return warmup
}

func hasFormulaFactor(cfg StrategyConfig) bool {
	for _, rule := range append(append([]StrategyFactorRule{}, cfg.Filters...), cfg.Scores...) {
		if strings.TrimSpace(rule.Factor) == "formula" {
			return true
		}
	}
	return false
}

func parseBacktestDate(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	for _, layout := range []string{"20060102", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return dateInt(t), nil
		}
	}
	return 0, fmt.Errorf("日期格式错误: %s", value)
}

func backtestDateText(date int) string {
	if date <= 0 {
		return ""
	}
	y := date / 10000
	m := (date / 100) % 100
	d := date % 100
	if y <= 0 || m <= 0 || d <= 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func backtestYearsBetween(startDate, endDate int) float64 {
	if startDate <= 0 || endDate <= 0 || endDate <= startDate {
		return 0
	}
	start := backtestDateTime(startDate)
	end := backtestDateTime(endDate)
	if start.IsZero() || end.IsZero() {
		return 0
	}
	return math.Max(end.Sub(start).Hours()/24/365.25, 1e-9)
}

func backtestDateTime(date int) time.Time {
	y := date / 10000
	m := (date / 100) % 100
	d := date % 100
	if y <= 0 || m <= 0 || d <= 0 {
		return time.Time{}
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
}

func tradeReturn(entryPrice, exitPrice, buyCost, sellCost float64) float64 {
	if entryPrice <= 0 || exitPrice <= 0 {
		return 0
	}
	buyFill := entryPrice * (1 + buyCost)
	sellFill := exitPrice * (1 - sellCost)
	if buyFill <= 0 {
		return 0
	}
	return sellFill/buyFill - 1
}
