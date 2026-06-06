package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type AutomationRunner struct {
	store   *AppStore
	worker  *FormulaWorkerClient
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[string]cron.EntryID
}

type StockSelectionPayload struct {
	FormulaID string   `json:"formula_id"`
	PoolID    string   `json:"pool_id"`
	Symbols   []string `json:"symbols"`
	Period    string   `json:"period"`
	Right     int      `json:"right"`
	OutCount  int      `json:"out_count"`
	CalcCount int      `json:"calc_count"`
}

type SystemSyncPayload struct {
	Scope     string   `json:"scope"`
	Codes     []string `json:"codes"`
	Tables    []string `json:"tables"`
	Limit     int      `json:"limit"`
	StartDate string   `json:"start_date"`
}

func NewAutomationRunner(store *AppStore, worker *FormulaWorkerClient) *AutomationRunner {
	return &AutomationRunner{
		store:   store,
		worker:  worker,
		cron:    cron.New(cron.WithSeconds()),
		entries: map[string]cron.EntryID{},
	}
}

func (r *AutomationRunner) Start() error {
	if err := r.Reload(); err != nil {
		return err
	}
	r.cron.Start()
	return nil
}

func (r *AutomationRunner) Stop() {
	if r == nil || r.cron == nil {
		return
	}
	ctx := r.cron.Stop()
	<-ctx.Done()
}

func (r *AutomationRunner) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for taskID, entryID := range r.entries {
		r.cron.Remove(entryID)
		delete(r.entries, taskID)
	}

	tasks, err := r.store.ListAutomationTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if !task.Enabled {
			continue
		}
		entryID, err := r.cron.AddFunc(task.Cron, func(taskID string) func() {
			return func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
				defer cancel()
				if _, err := r.RunTask(ctx, taskID); err != nil {
					log.Printf("自动化任务执行失败: %s %v", taskID, err)
				}
			}
		}(task.ID))
		if err != nil {
			return fmt.Errorf("%s cron配置错误: %w", task.Name, err)
		}
		r.entries[task.ID] = entryID
		next := r.cron.Entry(entryID).Next
		if !next.IsZero() {
			_ = r.store.UpdateTaskNextRun(task.ID, next.Format(time.RFC3339))
		}
	}
	return nil
}

func (r *AutomationRunner) RunTask(ctx context.Context, taskID string) (AutomationRun, error) {
	task, err := r.store.GetAutomationTask(taskID)
	if err != nil {
		return AutomationRun{}, err
	}
	return r.runTask(ctx, task)
}

func (r *AutomationRunner) runTask(ctx context.Context, task AutomationTask) (AutomationRun, error) {
	run, err := r.store.CreateAutomationRun(task)
	if err != nil {
		return run, err
	}

	status := "success"
	logText := ""
	resultJSON := "{}"
	matchedCount := 0
	var result interface{}

	switch task.Type {
	case "stock_selection":
		result, matchedCount, err = r.runStockSelection(ctx, task)
	case "system_sync":
		result, matchedCount, err = r.runSystemSync(ctx, task)
	default:
		err = fmt.Errorf("暂不支持的任务类型: %s", task.Type)
	}

	if err != nil {
		status = "failed"
		logText = err.Error()
	} else {
		resultJSON = mustJSON(result)
	}

	if finishErr := r.store.FinishAutomationRun(run.ID, status, logText, resultJSON, matchedCount); finishErr != nil && err == nil {
		err = finishErr
	}
	message := logText
	if message == "" {
		message = fmt.Sprintf("完成，命中 %d 条", matchedCount)
	}
	_ = r.store.UpdateTaskRunState(task.ID, status, message)

	hooks, _ := r.store.ResolveWebhooks(task.WebhookIDs)
	eventName := "automation.finished"
	if task.Type == "stock_selection" && status == "success" {
		eventName = "stock_selection.finished"
	}
	if status == "failed" {
		eventName = "automation.failed"
	}
	hookLogs := sendWebhooks(ctx, hooks, WebhookEvent{
		Event:        eventName,
		TaskID:       task.ID,
		TaskName:     task.Name,
		TaskType:     task.Type,
		RunID:        run.ID,
		Status:       status,
		Message:      message,
		MatchedCount: matchedCount,
		Result:       result,
	})
	if len(hookLogs) > 0 {
		log.Printf("Webhook通知: %s", strings.Join(hookLogs, "; "))
	}

	if latest, latestErr := r.store.GetAutomationRun(run.ID); latestErr == nil {
		run = latest
	}
	return run, err
}

func (r *AutomationRunner) runStockSelection(ctx context.Context, task AutomationTask) (interface{}, int, error) {
	var payload StockSelectionPayload
	if err := json.Unmarshal([]byte(task.PayloadJSON), &payload); err != nil {
		return nil, 0, err
	}
	if payload.FormulaID == "" {
		return nil, 0, errors.New("选股任务缺少formula_id")
	}
	formula, err := r.store.GetFormula(payload.FormulaID)
	if err != nil {
		return nil, 0, err
	}

	symbols := payload.Symbols
	if len(symbols) == 0 && payload.PoolID != "" {
		pool, err := r.store.GetStockPool(payload.PoolID)
		if err != nil {
			return nil, 0, err
		}
		symbols = pool.Symbols
	}
	symbols = normalizeSymbols(symbols)
	if len(symbols) == 0 {
		return nil, 0, errors.New("选股任务股票池为空")
	}
	period := payload.Period
	if period == "" {
		period = formula.Period
	}
	right := payload.Right
	if right == 0 {
		right = formula.Right
	}
	outCount := payload.OutCount
	if outCount == 0 {
		outCount = 1
	}
	calcCount := payload.CalcCount
	if calcCount == 0 {
		calcCount = 240
	}

	resp, err := r.worker.Run(ctx, FormulaRunRequest{
		Symbols:   symbols,
		Script:    formula.Script,
		Args:      json.RawMessage(formula.ArgsJSON),
		Period:    period,
		Right:     right,
		OutCount:  outCount,
		CalcCount: calcCount,
	})
	if err != nil {
		return nil, 0, err
	}
	matched := countFormulaMatches(resp.Data)
	result := map[string]interface{}{
		"formula":  formula,
		"symbols":  symbols,
		"response": resp,
	}
	return result, matched, nil
}

func (r *AutomationRunner) runSystemSync(ctx context.Context, task AutomationTask) (interface{}, int, error) {
	var payload SystemSyncPayload
	if strings.TrimSpace(task.PayloadJSON) != "" {
		_ = json.Unmarshal([]byte(task.PayloadJSON), &payload)
	}
	switch payload.Scope {
	case "", "codes":
		if manager == nil {
			return nil, 0, errors.New("数据管理器未初始化")
		}
		err := manager.Codes.Update()
		return map[string]interface{}{"scope": "codes"}, 0, err
	case "workday":
		if manager == nil {
			return nil, 0, errors.New("数据管理器未初始化")
		}
		err := manager.Workday.Update()
		return map[string]interface{}{"scope": "workday"}, 0, err
	case "kline":
		if manager == nil {
			return nil, 0, errors.New("数据管理器未初始化")
		}
		if len(payload.Tables) == 0 {
			payload.Tables = []string{"day"}
		}
		if payload.Limit == 0 {
			payload.Limit = 800
		}
		req := struct {
			Codes     []string `json:"codes"`
			Tables    []string `json:"tables"`
			Dir       string   `json:"dir"`
			Limit     int      `json:"limit"`
			StartDate string   `json:"start_date"`
		}{
			Codes:     payload.Codes,
			Tables:    payload.Tables,
			Limit:     payload.Limit,
			StartDate: payload.StartDate,
		}
		return map[string]interface{}{"scope": "kline", "config": req}, 0, errors.New("系统K线同步已预留，第一版请使用任务页的拉取K线或选股前自动取数")
	default:
		return nil, 0, fmt.Errorf("未知系统同步scope: %s", payload.Scope)
	}
}

func countFormulaMatches(data interface{}) int {
	raw, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0
	}
	return countTruthyLeaves(decoded)
}

func countTruthyLeaves(v interface{}) int {
	switch x := v.(type) {
	case map[string]interface{}:
		if hit, ok := x["hit"].(bool); ok && hit {
			return 1
		}
		if matched, ok := x["matched"].(bool); ok && matched {
			return 1
		}
		total := 0
		for _, item := range x {
			total += countTruthyLeaves(item)
		}
		return total
	case []interface{}:
		total := 0
		for _, item := range x {
			total += countTruthyLeaves(item)
		}
		return total
	case bool:
		if x {
			return 1
		}
	case float64:
		if x > 0 {
			return 1
		}
	}
	return 0
}
