package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type StrategyUniverseExpression struct {
	Mode      string                 `json:"-"`
	Legacy    string                 `json:"-"`
	Include   []StrategyUniverseTerm `json:"include,omitempty"`
	Intersect []StrategyUniverseTerm `json:"intersect,omitempty"`
	Exclude   []StrategyUniverseTerm `json:"exclude,omitempty"`
}

type StrategyUniverseTerm struct {
	Pool    string   `json:"pool,omitempty"`
	PoolID  string   `json:"pool_id,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

type StrategyUniverseResult struct {
	Expression StrategyUniverseExpression `json:"expression"`
	Total      int                        `json:"total"`
	Scanned    int                        `json:"scanned"`
	Limit      int                        `json:"limit,omitempty"`
	Truncated  bool                       `json:"truncated"`
	Symbols    []string                   `json:"symbols"`
	Sample     []string                   `json:"sample"`
	Sources    []StrategyUniverseSource   `json:"sources,omitempty"`
}

type StrategyUniverseSource struct {
	Role   string `json:"role"`
	PoolID string `json:"pool_id,omitempty"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
}

func (u *StrategyUniverseExpression) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*u = StrategyUniverseExpression{}
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var legacy string
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		*u = StrategyUniverseExpression{Mode: "legacy", Legacy: legacy}
		return nil
	}
	var expr struct {
		Include   []StrategyUniverseTerm `json:"include"`
		Intersect []StrategyUniverseTerm `json:"intersect"`
		Exclude   []StrategyUniverseTerm `json:"exclude"`
	}
	if err := json.Unmarshal(data, &expr); err != nil {
		return err
	}
	*u = StrategyUniverseExpression{
		Mode:      "expression",
		Include:   expr.Include,
		Intersect: expr.Intersect,
		Exclude:   expr.Exclude,
	}
	return nil
}

func (u StrategyUniverseExpression) MarshalJSON() ([]byte, error) {
	if u.isExpression() {
		return json.Marshal(struct {
			Include   []StrategyUniverseTerm `json:"include,omitempty"`
			Intersect []StrategyUniverseTerm `json:"intersect,omitempty"`
			Exclude   []StrategyUniverseTerm `json:"exclude,omitempty"`
		}{
			Include:   u.Include,
			Intersect: u.Intersect,
			Exclude:   u.Exclude,
		})
	}
	return json.Marshal(u.Legacy)
}

func (u StrategyUniverseExpression) isExpression() bool {
	return u.Mode == "expression" || len(u.Include) > 0 || len(u.Intersect) > 0 || len(u.Exclude) > 0
}

func (t StrategyUniverseTerm) poolID() string {
	if strings.TrimSpace(t.Pool) != "" {
		return strings.TrimSpace(t.Pool)
	}
	return strings.TrimSpace(t.PoolID)
}

func (r *AutomationRunner) strategyUniverseResult(cfg StrategyConfig, maxCodes int) (StrategyUniverseResult, error) {
	if cfg.Universe.isExpression() {
		return r.evaluateUniverseExpression(cfg.Universe, maxCodes)
	}
	return r.evaluateLegacyUniverse(cfg, maxCodes)
}

func (r *AutomationRunner) evaluateLegacyUniverse(cfg StrategyConfig, maxCodes int) (StrategyUniverseResult, error) {
	legacy := strings.ToLower(strings.TrimSpace(cfg.Universe.Legacy))
	expr := StrategyUniverseExpression{Mode: "legacy", Legacy: cfg.Universe.Legacy}
	var symbols []string
	var sources []StrategyUniverseSource
	switch legacy {
	case "symbols":
		symbols = normalizeSymbols(cfg.Symbols)
		sources = append(sources, StrategyUniverseSource{Role: "include", Name: "手动代码", Count: len(symbols)})
	case "all_a", "all":
		if len(cfg.Symbols) > 0 {
			symbols = normalizeSymbols(cfg.Symbols)
			sources = append(sources, StrategyUniverseSource{Role: "include", Name: "手动代码", Count: len(symbols)})
			break
		}
		pool, err := r.resolveUniversePool("market-all-a")
		if err != nil || len(pool.Symbols) == 0 {
			if fallback := limitedMarketPoolSymbols("market-all-a", maxCodes); len(fallback) > 0 {
				symbols = fallback
				sources = append(sources, StrategyUniverseSource{Role: "include", PoolID: "market-all-a", Name: "全部A股", Count: len(symbols)})
				break
			}
			if err != nil {
				return StrategyUniverseResult{}, err
			}
			return StrategyUniverseResult{}, errors.New("全市场代码列表不可用")
		}
		symbols = normalizeSymbols(pool.Symbols)
		sources = append(sources, StrategyUniverseSource{Role: "include", PoolID: pool.ID, Name: pool.Name, Count: len(symbols)})
	case "market":
		poolID := cfg.PoolID
		if poolID == "" {
			poolID = "market-all-a"
		}
		pool, err := r.resolveUniversePool(poolID)
		if err != nil {
			return StrategyUniverseResult{}, fmt.Errorf("市场分组代码列表不可用: %s", poolID)
		}
		symbols = normalizeSymbols(pool.Symbols)
		if len(symbols) == 0 {
			symbols = limitedMarketPoolSymbols(poolID, maxCodes)
		}
		if len(symbols) == 0 {
			return StrategyUniverseResult{}, fmt.Errorf("市场分组代码列表不可用: %s", poolID)
		}
		sources = append(sources, StrategyUniverseSource{Role: "include", PoolID: pool.ID, Name: pool.Name, Count: len(symbols)})
	case "", "pool":
		poolID := cfg.PoolID
		if poolID == "" {
			poolID = DecisionWatchPoolID
		}
		pool, err := r.resolveUniversePool(poolID)
		if err != nil {
			return StrategyUniverseResult{}, err
		}
		symbols = normalizeSymbols(pool.Symbols)
		sources = append(sources, StrategyUniverseSource{Role: "include", PoolID: pool.ID, Name: pool.Name, Count: len(symbols)})
	default:
		return StrategyUniverseResult{}, fmt.Errorf("未知策略股票范围: %s", cfg.Universe.Legacy)
	}
	return buildUniverseResult(expr, symbols, sources, maxCodes), nil
}

func (r *AutomationRunner) evaluateUniverseExpression(expr StrategyUniverseExpression, maxCodes int) (StrategyUniverseResult, error) {
	if len(expr.Include) == 0 {
		return StrategyUniverseResult{}, errors.New("选股范围表达式缺少include起点池")
	}
	include, includeSources, err := r.unionUniverseTerms("include", expr.Include)
	if err != nil {
		return StrategyUniverseResult{}, err
	}
	current := include
	sources := includeSources
	for _, term := range expr.Intersect {
		set, source, err := r.universeTermSet("intersect", term)
		if err != nil {
			return StrategyUniverseResult{}, err
		}
		sources = append(sources, source)
		for symbol := range current {
			if !set[symbol] {
				delete(current, symbol)
			}
		}
	}
	exclude, excludeSources, err := r.unionUniverseTerms("exclude", expr.Exclude)
	if err != nil {
		return StrategyUniverseResult{}, err
	}
	sources = append(sources, excludeSources...)
	for symbol := range exclude {
		delete(current, symbol)
	}
	return buildUniverseResult(expr, setKeys(current), sources, maxCodes), nil
}

func (r *AutomationRunner) unionUniverseTerms(role string, terms []StrategyUniverseTerm) (map[string]bool, []StrategyUniverseSource, error) {
	result := map[string]bool{}
	sources := make([]StrategyUniverseSource, 0, len(terms))
	for _, term := range terms {
		set, source, err := r.universeTermSet(role, term)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, source)
		for symbol := range set {
			result[symbol] = true
		}
	}
	return result, sources, nil
}

func (r *AutomationRunner) universeTermSet(role string, term StrategyUniverseTerm) (map[string]bool, StrategyUniverseSource, error) {
	if len(term.Symbols) > 0 {
		symbols := normalizeSymbols(term.Symbols)
		return symbolSet(symbols), StrategyUniverseSource{Role: role, Name: "手动代码", Count: len(symbols)}, nil
	}
	poolID := term.poolID()
	if poolID == "" {
		return nil, StrategyUniverseSource{}, errors.New("选股范围池ID不能为空")
	}
	pool, err := r.resolveUniversePool(poolID)
	if err != nil {
		return nil, StrategyUniverseSource{}, err
	}
	symbols := normalizeSymbols(pool.Symbols)
	return symbolSet(symbols), StrategyUniverseSource{Role: role, PoolID: pool.ID, Name: pool.Name, Count: len(symbols)}, nil
}

func (r *AutomationRunner) resolveUniversePool(poolID string) (StockPool, error) {
	poolID = strings.TrimSpace(poolID)
	if poolID == "" {
		return StockPool{}, errors.New("股票池ID不能为空")
	}
	return r.store.GetStockPool(poolID)
}

func buildUniverseResult(expr StrategyUniverseExpression, symbols []string, sources []StrategyUniverseSource, maxCodes int) StrategyUniverseResult {
	symbols = normalizeSymbols(symbols)
	sort.Strings(symbols)
	total := len(symbols)
	truncated := false
	if maxCodes > 0 && len(symbols) > maxCodes {
		symbols = append([]string{}, symbols[:maxCodes]...)
		truncated = true
	} else {
		symbols = append([]string{}, symbols...)
	}
	sample := symbols
	if len(sample) > 20 {
		sample = sample[:20]
	}
	return StrategyUniverseResult{
		Expression: expr,
		Total:      total,
		Scanned:    len(symbols),
		Limit:      maxCodes,
		Truncated:  truncated,
		Symbols:    symbols,
		Sample:     append([]string{}, sample...),
		Sources:    sources,
	}
}

func setKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	return keys
}
