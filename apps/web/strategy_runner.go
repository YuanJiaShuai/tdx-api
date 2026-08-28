package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type StrategyConfig struct {
	Universe        string               `json:"universe"`
	PoolID          string               `json:"pool_id"`
	Symbols         []string             `json:"symbols"`
	Filters         []StrategyFactorRule `json:"filters"`
	Scores          []StrategyFactorRule `json:"scores"`
	Pass            StrategyPassConfig   `json:"pass"`
	Period          string               `json:"period"`
	Right           int                  `json:"right"`
	CalcCount       int                  `json:"calc_count"`
	BatchSize       int                  `json:"batch_size"`
	ContinueOnError bool                 `json:"continue_on_error"`
}

type StrategyFactorRule struct {
	ID     string                 `json:"id"`
	Factor string                 `json:"factor"`
	Weight float64                `json:"weight"`
	Params map[string]interface{} `json:"params"`
}

type StrategyPassConfig struct {
	MinScore float64 `json:"min_score"`
	TopN     int     `json:"top_n"`
}

type StrategyRunResult struct {
	Strategy      Strategy                   `json:"strategy"`
	Total         int                        `json:"total"`
	Matched       int                        `json:"matched"`
	Items         []StrategySelectionItem    `json:"items"`
	Errors        map[string]string          `json:"errors,omitempty"`
	FormulaCache  map[string]map[string]bool `json:"-"`
	FormulaDetail map[string]map[string]any  `json:"-"`
	KlineCache    map[string][]FormulaKline  `json:"-"`
	PoolCache     map[string]map[string]bool `json:"-"`
	FormulaByName map[string]Formula         `json:"-"`
	FormulaByID   map[string]Formula         `json:"-"`
	Config        StrategyConfig             `json:"config"`
}

type StrategySelectionItem struct {
	Symbol        string                 `json:"symbol"`
	Score         float64                `json:"score"`
	Hit           bool                   `json:"hit"`
	Latest        float64                `json:"latest"`
	Reasons       []string               `json:"reasons"`
	FactorResults []StrategyFactorResult `json:"factor_results"`
}

type StrategyFactorResult struct {
	ID     string  `json:"id"`
	Factor string  `json:"factor"`
	Hit    bool    `json:"hit"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

func (r *AutomationRunner) runStrategySelection(ctx context.Context, task AutomationTask, run AutomationRun) (interface{}, []string, error) {
	var payload struct {
		StrategyID string `json:"strategy_id"`
	}
	if err := json.Unmarshal([]byte(task.PayloadJSON), &payload); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(payload.StrategyID) == "" {
		return nil, nil, errors.New("策略选股任务缺少strategy_id")
	}
	strategy, err := r.store.GetStrategy(payload.StrategyID)
	if err != nil {
		return nil, nil, err
	}
	result, err := r.executeStrategy(ctx, strategy)
	if err != nil {
		return nil, nil, err
	}
	items := strategyItemsToSelectionResults(result.Items)
	if err := r.store.SaveSelectionResults(run, Formula{ID: "strategy:" + strategy.ID, Name: strategy.Name}, items); err != nil {
		return nil, nil, err
	}
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		symbols = append(symbols, item.Symbol)
	}
	return result, symbols, nil
}

func (r *AutomationRunner) executeStrategy(ctx context.Context, strategy Strategy) (StrategyRunResult, error) {
	var cfg StrategyConfig
	if err := json.Unmarshal([]byte(strategy.ConfigJSON), &cfg); err != nil {
		return StrategyRunResult{}, err
	}
	if cfg.Period == "" {
		cfg.Period = "day"
	}
	if cfg.CalcCount <= 0 {
		cfg.CalcCount = 260
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	symbols, err := r.strategyUniverse(cfg)
	if err != nil {
		return StrategyRunResult{}, err
	}
	if len(symbols) == 0 {
		return StrategyRunResult{}, errors.New("策略股票范围为空")
	}

	result := StrategyRunResult{
		Strategy:      strategy,
		Config:        cfg,
		Total:         len(symbols),
		Errors:        map[string]string{},
		FormulaCache:  map[string]map[string]bool{},
		FormulaDetail: map[string]map[string]any{},
		KlineCache:    map[string][]FormulaKline{},
		PoolCache:     map[string]map[string]bool{},
		FormulaByName: map[string]Formula{},
		FormulaByID:   map[string]Formula{},
	}
	if err := r.prepareStrategyFormulas(ctx, &result, symbols); err != nil {
		return StrategyRunResult{}, err
	}

	items := make([]StrategySelectionItem, 0)
	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		item, ok := r.evaluateStrategySymbol(strategy, &result, symbol)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Symbol < items[j].Symbol
		}
		return items[i].Score > items[j].Score
	})
	if result.Config.Pass.TopN > 0 && len(items) > result.Config.Pass.TopN {
		items = items[:result.Config.Pass.TopN]
	}
	result.Items = items
	result.Matched = len(items)
	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func (r *AutomationRunner) strategyUniverse(cfg StrategyConfig) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Universe)) {
	case "symbols":
		return normalizeSymbols(cfg.Symbols), nil
	case "all_a", "all":
		if len(cfg.Symbols) > 0 {
			return normalizeSymbols(cfg.Symbols), nil
		}
		if symbols := limitedMarketPoolSymbols("market-all-a", strategyMaxCodes(cfg)); len(symbols) > 0 {
			return symbols, nil
		}
		return nil, errors.New("全市场代码列表不可用")
	case "market":
		poolID := cfg.PoolID
		if poolID == "" {
			poolID = "market-all-a"
		}
		if symbols := limitedMarketPoolSymbols(poolID, strategyMaxCodes(cfg)); len(symbols) > 0 {
			return symbols, nil
		}
		return nil, fmt.Errorf("市场分组代码列表不可用: %s", poolID)
	case "", "pool":
		poolID := cfg.PoolID
		if poolID == "" {
			poolID = DecisionWatchPoolID
		}
		pool, err := r.store.GetStockPool(poolID)
		if err != nil {
			return nil, err
		}
		return normalizeSymbols(pool.Symbols), nil
	default:
		return nil, fmt.Errorf("未知策略股票范围: %s", cfg.Universe)
	}
}

func strategyMaxCodes(cfg StrategyConfig) int {
	maxCodes := 300
	if cfg.BatchSize > 0 && cfg.BatchSize > maxCodes {
		maxCodes = cfg.BatchSize
	}
	return maxCodes
}

func (r *AutomationRunner) prepareStrategyFormulas(ctx context.Context, result *StrategyRunResult, symbols []string) error {
	rules := append([]StrategyFactorRule{}, result.Config.Filters...)
	rules = append(rules, result.Config.Scores...)
	for _, rule := range rules {
		if rule.Factor != "formula" {
			continue
		}
		formula, err := r.strategyFormula(rule)
		if err != nil {
			return err
		}
		result.FormulaByID[formula.ID] = formula
		result.FormulaByName[formula.Name] = formula
		if _, ok := result.FormulaCache[formula.ID]; ok {
			continue
		}
		resp, err := r.worker.Run(ctx, FormulaRunRequest{
			Symbols:   symbols,
			Script:    formula.Script,
			Args:      json.RawMessage(formula.ArgsJSON),
			Period:    chooseString(result.Config.Period, formula.Period),
			Right:     chooseInt(result.Config.Right, formula.Right),
			OutCount:  1,
			CalcCount: result.Config.CalcCount,
		})
		if err != nil {
			return err
		}
		hits, details := formulaResponseMaps(resp.Data)
		result.FormulaCache[formula.ID] = hits
		result.FormulaDetail[formula.ID] = details
	}
	return nil
}

func (r *AutomationRunner) strategyFormula(rule StrategyFactorRule) (Formula, error) {
	formulaID := stringParam(rule.Params, "formula_id", "")
	formulaName := stringParam(rule.Params, "formula_name", "")
	if formulaID != "" {
		return r.store.GetFormula(formulaID)
	}
	if formulaName == "" {
		return Formula{}, errors.New("公式因子缺少formula_id或formula_name")
	}
	formulas, err := r.store.ListFormulas()
	if err != nil {
		return Formula{}, err
	}
	for _, item := range formulas {
		if item.Name == formulaName {
			return item, nil
		}
	}
	return Formula{}, fmt.Errorf("公式不存在: %s", formulaName)
}

func (r *AutomationRunner) evaluateStrategySymbol(strategy Strategy, result *StrategyRunResult, symbol string) (StrategySelectionItem, bool) {
	item := StrategySelectionItem{Symbol: symbol, Hit: true}
	rows, err := r.strategyKline(result, symbol)
	if err != nil {
		result.Errors[symbol] = err.Error()
		return item, false
	}
	if len(rows) > 0 {
		item.Latest = rows[len(rows)-1].Close
	}
	for _, rule := range result.Config.Filters {
		fr := r.evaluateFactor(result, symbol, rows, rule, true)
		item.FactorResults = append(item.FactorResults, fr)
		item.Reasons = append(item.Reasons, fr.Reason)
		if !fr.Hit {
			item.Hit = false
			return item, false
		}
	}
	score := 0.0
	for _, rule := range result.Config.Scores {
		fr := r.evaluateFactor(result, symbol, rows, rule, false)
		item.FactorResults = append(item.FactorResults, fr)
		if fr.Hit {
			score += fr.Score
		}
		item.Reasons = append(item.Reasons, fr.Reason)
	}
	item.Score = score
	minScore := result.Config.Pass.MinScore
	if minScore <= 0 {
		minScore = 1
	}
	item.Hit = score >= minScore
	if !item.Hit {
		return item, false
	}
	return item, true
}

func (r *AutomationRunner) strategyKline(result *StrategyRunResult, symbol string) ([]FormulaKline, error) {
	if rows, ok := result.KlineCache[symbol]; ok {
		return rows, nil
	}
	rows, err := loadFormulaKline(symbol, result.Config.Period, result.Config.CalcCount)
	if err != nil {
		return nil, err
	}
	result.KlineCache[symbol] = rows
	return rows, nil
}

func (r *AutomationRunner) evaluateFactor(result *StrategyRunResult, symbol string, rows []FormulaKline, rule StrategyFactorRule, filter bool) StrategyFactorResult {
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
		fr.Reason = fmt.Sprintf("不在%s: %t", poolID, fr.Hit)
	case "min_amount":
		value := floatParam(rule.Params, "value", 0)
		amount := latest(rows).Amount
		fr.Hit = amount >= value
		fr.Reason = fmt.Sprintf("成交额 %.0f >= %.0f", amount, value)
	case "price_range":
		minValue := floatParam(rule.Params, "min", 0)
		maxValue := floatParam(rule.Params, "max", math.MaxFloat64)
		closePrice := latest(rows).Close
		fr.Hit = closePrice >= minValue && closePrice <= maxValue
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
		fr.Reason = fmt.Sprintf("涨跌幅 %.2f%% 在 %.2f-%.2f", change, minValue, maxValue)
	case "ma_trend":
		short := intParam(rule.Params, "short", 5)
		mid := intParam(rule.Params, "mid", 10)
		long := intParam(rule.Params, "long", 20)
		maShort, maMid, maLong := ma(rows, short), ma(rows, mid), ma(rows, long)
		closePrice := latest(rows).Close
		fr.Hit = closePrice >= maShort && maShort >= maMid && maMid >= maLong
		fr.Reason = fmt.Sprintf("均线多头 C %.2f / MA%d %.2f / MA%d %.2f / MA%d %.2f", closePrice, short, maShort, mid, maMid, long, maLong)
	case "volume_up":
		days := intParam(rule.Params, "days", 5)
		ratio := floatParam(rule.Params, "ratio", 1.3)
		avg := strategyAvgVol(rows, days)
		vol := latest(rows).Vol
		fr.Hit = avg > 0 && vol >= avg*ratio
		fr.Reason = fmt.Sprintf("放量 %.0f >= %.2fx %d日均量 %.0f", vol, ratio, days, avg)
	case "break_high":
		days := intParam(rule.Params, "days", 20)
		high := highestHigh(rows, days)
		closePrice := latest(rows).Close
		fr.Hit = high > 0 && closePrice >= high
		fr.Reason = fmt.Sprintf("突破%d日高点 C %.2f / H %.2f", days, closePrice, high)
	case "formula":
		formula, err := r.strategyFormula(rule)
		if err != nil {
			fr.Hit = false
			fr.Reason = err.Error()
			break
		}
		hits := result.FormulaCache[formula.ID]
		fr.Hit = hits[strategyNormalizeSymbol(symbol)]
		fr.Reason = fmt.Sprintf("公式%s命中: %t", formula.Name, fr.Hit)
	default:
		fr.Hit = false
		fr.Reason = "未知因子: " + rule.Factor
	}
	if fr.Hit {
		fr.Score = weight
	}
	return fr
}

func (r *AutomationRunner) strategyPoolContains(result *StrategyRunResult, poolID string, symbol string) bool {
	if _, ok := result.PoolCache[poolID]; !ok {
		pool, err := r.store.GetStockPool(poolID)
		set := map[string]bool{}
		if err == nil {
			for _, item := range normalizeSymbols(pool.Symbols) {
				set[item] = true
			}
		}
		result.PoolCache[poolID] = set
	}
	return result.PoolCache[poolID][strategyNormalizeSymbol(symbol)]
}

func formulaResponseMaps(data interface{}) (map[string]bool, map[string]any) {
	raw, _ := json.Marshal(data)
	decoded := map[string]interface{}{}
	_ = json.Unmarshal(raw, &decoded)
	hits := map[string]bool{}
	details := map[string]any{}
	for symbol, detail := range decoded {
		_, hit := formulaDetailHit(detail)
		normalized := strategyNormalizeSymbol(symbol)
		hits[normalized] = hit
		details[normalized] = detail
	}
	return hits, details
}

func strategyItemsToSelectionResults(items []StrategySelectionItem) []SelectionResult {
	results := make([]SelectionResult, 0, len(items))
	for _, item := range items {
		results = append(results, SelectionResult{
			Symbol:     item.Symbol,
			Latest:     item.Score,
			DetailJSON: mustJSON(item),
			CreatedAt:  nowText(),
		})
	}
	return results
}

func latest(rows []FormulaKline) FormulaKline {
	if len(rows) == 0 {
		return FormulaKline{}
	}
	return rows[len(rows)-1]
}

func ma(rows []FormulaKline, days int) float64 {
	if days <= 0 || len(rows) < days {
		return 0
	}
	sum := 0.0
	for _, row := range rows[len(rows)-days:] {
		sum += row.Close
	}
	return sum / float64(days)
}

func strategyAvgVol(rows []FormulaKline, days int) float64 {
	if days <= 0 || len(rows) < days+1 {
		return 0
	}
	end := len(rows) - 1
	start := end - days
	sum := 0.0
	for _, row := range rows[start:end] {
		sum += row.Vol
	}
	return sum / float64(days)
}

func strategyNormalizeSymbol(symbol string) string {
	items := normalizeSymbols([]string{symbol})
	if len(items) == 0 {
		return strings.TrimSpace(strings.ToUpper(symbol))
	}
	return items[0]
}

func highestHigh(rows []FormulaKline, days int) float64 {
	if days <= 0 || len(rows) < days+1 {
		return 0
	}
	end := len(rows) - 1
	start := end - days
	high := 0.0
	for _, row := range rows[start:end] {
		if row.High > high {
			high = row.High
		}
	}
	return high
}

func stringParam(params map[string]interface{}, key string, fallback string) string {
	if params == nil {
		return fallback
	}
	if value, ok := params[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func floatParam(params map[string]interface{}, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	switch value := params[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case json.Number:
		if n, err := value.Float64(); err == nil {
			return n
		}
	}
	return fallback
}

func intParam(params map[string]interface{}, key string, fallback int) int {
	return int(floatParam(params, key, float64(fallback)))
}

func chooseString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func chooseInt(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}
