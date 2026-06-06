package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	appStore         *AppStore
	formulaWorker    *FormulaWorkerClient
	automationRunner *AutomationRunner
)

func initAutomationServices() error {
	store, err := OpenAppStore()
	if err != nil {
		return err
	}
	appStore = store
	formulaWorker = NewFormulaWorkerClient()
	automationRunner = NewAutomationRunner(appStore, formulaWorker)
	if err := automationRunner.Start(); err != nil {
		return err
	}
	return nil
}

func handleFormulaHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	info, err := formulaWorker.HealthInfo(ctx)
	if info == nil {
		info = map[string]interface{}{}
	}
	info["ok"] = err == nil
	if err != nil {
		info["error"] = err.Error()
	} else {
		info["error"] = ""
	}
	successResponse(w, info)
}

func handleFormulas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListFormulas()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var req Formula
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		if req.ArgsJSON == "" {
			var raw struct {
				Args json.RawMessage `json:"args"`
			}
			_ = json.NewDecoder(strings.NewReader("{}")).Decode(&raw)
		}
		item, err := appStore.UpsertFormula(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleFormulaOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/formulas/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "test" {
		handleFormulaTest(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := appStore.GetFormula(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "公式不存在"))
			return
		}
		successResponse(w, item)
	case http.MethodPut:
		var req Formula
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertFormula(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteFormula(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleFormulaTest(w http.ResponseWriter, r *http.Request, formulaID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	formula, err := appStore.GetFormula(formulaID)
	if err != nil {
		errorResponse(w, notFoundMessage(err, "公式不存在"))
		return
	}
	var req struct {
		Symbol    string `json:"symbol"`
		Period    string `json:"period"`
		Right     int    `json:"right"`
		OutCount  int    `json:"out_count"`
		CalcCount int    `json:"calc_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	if req.Symbol == "" {
		errorResponse(w, "symbol不能为空")
		return
	}
	period := req.Period
	if period == "" {
		period = formula.Period
	}
	right := req.Right
	if right == 0 {
		right = formula.Right
	}
	resp, err := formulaWorker.Run(r.Context(), FormulaRunRequest{
		Symbol:    req.Symbol,
		Script:    formula.Script,
		Args:      json.RawMessage(formula.ArgsJSON),
		Period:    period,
		Right:     right,
		OutCount:  req.OutCount,
		CalcCount: req.CalcCount,
	})
	if err != nil {
		errorResponse(w, "公式执行失败: "+err.Error())
		return
	}
	successResponse(w, resp)
}

func handleFormulaRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req FormulaRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	resp, err := formulaWorker.Run(r.Context(), req)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func handleStockPools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListStockPools()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var req StockPool
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertStockPool(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleStockPoolOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/stock-pools/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	switch r.Method {
	case http.MethodGet:
		item, err := appStore.GetStockPool(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "股票池不存在"))
			return
		}
		successResponse(w, item)
	case http.MethodPut:
		var req StockPool
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertStockPool(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteStockPool(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleAutomationTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListAutomationTasks()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var req AutomationTask
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertAutomationTask(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		if err := automationRunner.Reload(); err != nil {
			errorResponse(w, "任务已保存，但调度重载失败: "+err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleAutomationTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	task, err := buildAutomationTemplate(req.Template)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	item, err := appStore.UpsertAutomationTask(task)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if err := automationRunner.Reload(); err != nil {
		errorResponse(w, "模板已保存，但调度重载失败: "+err.Error())
		return
	}
	successResponse(w, item)
}

func buildAutomationTemplate(name string) (AutomationTask, error) {
	switch name {
	case "morning_sync":
		return AutomationTask{
			Name:        "早盘基础数据同步",
			Type:        "system_sync",
			Cron:        "0 0 8 * * 1-5",
			Enabled:     false,
			PayloadJSON: `{"scope":"basic"}`,
			WebhookIDs:  "[]",
		}, nil
	case "evening_kline":
		return AutomationTask{
			Name:        "晚盘日K同步",
			Type:        "system_sync",
			Cron:        "0 30 18 * * 1-5",
			Enabled:     false,
			PayloadJSON: `{"scope":"kline","tables":["day"],"limit":4}`,
			WebhookIDs:  "[]",
		}, nil
	case "evening_full":
		return AutomationTask{
			Name:        "晚盘完整数据同步",
			Type:        "system_sync",
			Cron:        "0 0 21 * * 1-5",
			Enabled:     false,
			PayloadJSON: `{"scope":"all","tables":["day"],"limit":4,"max_codes":200,"block_files":["gn","fg","zs","hy","block"],"with_index":true,"continue_on_error":true}`,
			WebhookIDs:  "[]",
		}, nil
	default:
		return AutomationTask{}, fmt.Errorf("未知任务模板: %s", name)
	}
}

func handleAutomationOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/automations/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "run" {
		if r.Method != http.MethodPost {
			errorResponse(w, "只支持POST请求")
			return
		}
		run, err := automationRunner.RunTask(r.Context(), id)
		if err != nil {
			errorResponse(w, "任务执行失败: "+err.Error())
			return
		}
		successResponse(w, run)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := appStore.GetAutomationTask(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "任务不存在"))
			return
		}
		successResponse(w, item)
	case http.MethodPut:
		var req AutomationTask
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertAutomationTask(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		if err := automationRunner.Reload(); err != nil {
			errorResponse(w, "任务已保存，但调度重载失败: "+err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteAutomationTask(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		if err := automationRunner.Reload(); err != nil {
			errorResponse(w, "任务已删除，但调度重载失败: "+err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleAutomationRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	taskID := r.URL.Query().Get("task_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := appStore.ListAutomationRuns(taskID, limit)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if items == nil {
		items = []AutomationRun{}
	}
	successResponse(w, items)
}

func handleSelectionResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	onlyLatest := r.URL.Query().Get("latest") == "true" || r.URL.Query().Get("latest") == "1"
	items, err := appStore.ListSelectionResults(
		r.URL.Query().Get("task_id"),
		r.URL.Query().Get("formula_id"),
		r.URL.Query().Get("symbol"),
		onlyLatest,
		limit,
	)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if items == nil {
		items = []SelectionResult{}
	}
	successResponse(w, items)
}

func handleWebhooks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListWebhooks()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var req Webhook
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertWebhook(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleWebhookOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/webhooks/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "test" {
		if r.Method != http.MethodPost {
			errorResponse(w, "只支持POST请求")
			return
		}
		hook, err := appStore.GetWebhook(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "Webhook不存在"))
			return
		}
		logs := sendWebhooks(r.Context(), []Webhook{hook}, WebhookEvent{
			Event:   "webhook.test",
			Status:  "success",
			Message: "tdx-api webhook测试",
		})
		successResponse(w, logs)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := appStore.GetWebhook(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "Webhook不存在"))
			return
		}
		successResponse(w, item)
	case http.MethodPut:
		var req Webhook
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertWebhook(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteWebhook(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleHQChartKline(w http.ResponseWriter, r *http.Request) {
	code := normalizeStockCode(r.URL.Query().Get("symbol"))
	if code == "" {
		code = normalizeStockCode(r.URL.Query().Get("code"))
	}
	if code == "" {
		errorResponse(w, "symbol不能为空")
		return
	}
	period := r.URL.Query().Get("period")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 800
	}
	data, err := loadFormulaKline(code, period, limit)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, map[string]interface{}{
		"symbol": strings.ToUpper(code),
		"period": formulaPeriodToKlineType(period),
		"data":   data,
	})
}

func handleHQChartHistory(w http.ResponseWriter, r *http.Request) {
	code := normalizeStockCode(r.URL.Query().Get("symbol"))
	if code == "" {
		code = normalizeStockCode(r.URL.Query().Get("code"))
	}
	if code == "" {
		errorResponse(w, "symbol不能为空")
		return
	}
	period := r.URL.Query().Get("period")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 800
	}
	rows, err := loadFormulaKline(code, period, limit)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	chartRows := make([][]interface{}, 0, len(rows))
	for _, item := range rows {
		row := []interface{}{
			item.Date,
			item.YClose,
			item.Open,
			item.High,
			item.Low,
			item.Close,
			item.Vol,
			item.Amount,
		}
		if item.Time > 0 {
			row = append(row, item.Time)
		}
		chartRows = append(chartRows, row)
	}

	symbol := toHQChartSymbol(code)
	successResponse(w, map[string]interface{}{
		"symbol": symbol,
		"name":   symbol,
		"period": formulaPeriodToKlineType(period),
		"data":   chartRows,
		"ver":    2,
	})
}

func normalizeStockCode(symbol string) string {
	s := strings.TrimSpace(strings.ToLower(symbol))
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "."); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimLeft(s, " ")
}

func toHQChartSymbol(code string) string {
	c := normalizeStockCode(code)
	if c == "" {
		return ""
	}
	market := "sz"
	if strings.HasPrefix(c, "6") || strings.HasPrefix(c, "9") {
		market = "sh"
	}
	return c + "." + market
}

func pathParts(path, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func notFoundMessage(err error, fallback string) string {
	if err == sql.ErrNoRows {
		return fallback
	}
	return fmt.Sprintf("%s: %v", fallback, err)
}
