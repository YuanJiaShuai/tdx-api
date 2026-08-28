package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type StrategyFactorDefinition struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Kind        string                   `json:"kind"`
	Description string                   `json:"description"`
	Params      []StrategyFactorParamDef `json:"params"`
}

type StrategyFactorParamDef struct {
	Name    string      `json:"name"`
	Label   string      `json:"label"`
	Type    string      `json:"type"`
	Default interface{} `json:"default"`
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListStrategies()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var req Strategy
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertStrategy(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleStrategyOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/strategies/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "clone" {
		if r.Method != http.MethodPost {
			errorResponse(w, "只支持POST请求")
			return
		}
		item, err := appStore.CloneStrategy(id)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
		return
	}
	if len(parts) == 2 && parts[1] == "run" {
		if r.Method != http.MethodPost {
			errorResponse(w, "只支持POST请求")
			return
		}
		handleStrategyRun(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := appStore.GetStrategy(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "策略不存在"))
			return
		}
		successResponse(w, item)
	case http.MethodPut:
		var req Strategy
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertStrategy(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteStrategy(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleStrategyRun(w http.ResponseWriter, r *http.Request, id string) {
	strategy, err := appStore.GetStrategy(id)
	if err != nil {
		errorResponse(w, notFoundMessage(err, "策略不存在"))
		return
	}
	task := AutomationTask{
		ID:          "manual-strategy-" + strategy.ID,
		Name:        strategy.Name,
		Type:        "strategy_selection",
		Cron:        "0 0 0 * * *",
		PayloadJSON: mustJSON(map[string]string{"strategy_id": strategy.ID}),
		WebhookIDs:  "[]",
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()
	run, err := automationRunner.runTask(ctx, task)
	if err != nil {
		errorResponse(w, "策略运行失败: "+err.Error())
		return
	}
	successResponse(w, run)
}

func handleStrategyFactors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	successResponse(w, []StrategyFactorDefinition{
		{
			ID:          "pool_exclude",
			Name:        "排除股票池",
			Kind:        "filter",
			Description: "过滤掉已经放入指定股票池的标的。",
			Params: []StrategyFactorParamDef{
				{Name: "pool_id", Label: "股票池ID", Type: "string", Default: DecisionExcludePoolID},
			},
		},
		{
			ID:          "min_amount",
			Name:        "最低成交额",
			Kind:        "filter",
			Description: "要求最新K线成交额达到指定金额。",
			Params: []StrategyFactorParamDef{
				{Name: "value", Label: "最低成交额", Type: "number", Default: 100000000},
			},
		},
		{
			ID:          "price_range",
			Name:        "价格区间",
			Kind:        "filter",
			Description: "要求最新收盘价落在指定区间。",
			Params: []StrategyFactorParamDef{
				{Name: "min", Label: "最低价", Type: "number", Default: 0},
				{Name: "max", Label: "最高价", Type: "number", Default: 9999},
			},
		},
		{
			ID:          "change_range",
			Name:        "涨跌幅区间",
			Kind:        "filter",
			Description: "要求最新K线涨跌幅落在指定百分比区间。",
			Params: []StrategyFactorParamDef{
				{Name: "min", Label: "最小涨跌幅", Type: "number", Default: -10},
				{Name: "max", Label: "最大涨跌幅", Type: "number", Default: 10},
			},
		},
		{
			ID:          "ma_trend",
			Name:        "均线多头",
			Kind:        "score",
			Description: "收盘价、短中长期均线呈多头排列时加分。",
			Params: []StrategyFactorParamDef{
				{Name: "short", Label: "短均线", Type: "number", Default: 5},
				{Name: "mid", Label: "中均线", Type: "number", Default: 10},
				{Name: "long", Label: "长均线", Type: "number", Default: 20},
			},
		},
		{
			ID:          "volume_up",
			Name:        "阶段放量",
			Kind:        "score",
			Description: "最新成交量达到前N日均量的一定倍数时加分。",
			Params: []StrategyFactorParamDef{
				{Name: "days", Label: "对比天数", Type: "number", Default: 5},
				{Name: "ratio", Label: "放量倍数", Type: "number", Default: 1.3},
			},
		},
		{
			ID:          "break_high",
			Name:        "突破新高",
			Kind:        "score",
			Description: "最新收盘价突破前N日高点时加分。",
			Params: []StrategyFactorParamDef{
				{Name: "days", Label: "回看天数", Type: "number", Default: 20},
			},
		},
		{
			ID:          "formula",
			Name:        "公式因子",
			Kind:        "score",
			Description: "调用公式中心里的公式，命中则按权重加分。",
			Params: []StrategyFactorParamDef{
				{Name: "formula_id", Label: "公式ID", Type: "string", Default: ""},
				{Name: "formula_name", Label: "公式名称", Type: "string", Default: ""},
			},
		},
	})
}
