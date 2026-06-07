package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
	"github.com/injoyai/tdx"
)

type AppStore struct {
	db *sql.DB
}

type Formula struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Script      string `json:"script"`
	ArgsJSON    string `json:"args_json"`
	Period      string `json:"period"`
	Right       int    `json:"right"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type StockPool struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Symbols     []string `json:"symbols"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	System      bool     `json:"system"`
	Readonly    bool     `json:"readonly"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type Strategy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ConfigJSON  string `json:"config_json"`
	Enabled     bool   `json:"enabled"`
	Readonly    bool   `json:"readonly"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

const (
	DecisionWatchPoolID   = "watchlist"
	DecisionExcludePoolID = "exclude"
	FixedCloseSyncTaskID  = "fixed-close-sync"
)

type AutomationTask struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Cron        string `json:"cron"`
	Enabled     bool   `json:"enabled"`
	PayloadJSON string `json:"payload_json"`
	WebhookIDs  string `json:"webhook_ids"`
	LastRunAt   string `json:"last_run_at"`
	NextRunAt   string `json:"next_run_at"`
	LastStatus  string `json:"last_status"`
	LastMessage string `json:"last_message"`
	CronEntryID int    `json:"-"`
	Readonly    bool   `json:"readonly"`
	System      bool   `json:"system"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AutomationRun struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	TaskName     string `json:"task_name"`
	TaskType     string `json:"task_type"`
	Status       string `json:"status"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	Log          string `json:"log"`
	ResultJSON   string `json:"result_json"`
	MatchedCount int    `json:"matched_count"`
}

type SelectionResult struct {
	ID          string  `json:"id"`
	RunID       string  `json:"run_id"`
	TaskID      string  `json:"task_id"`
	TaskName    string  `json:"task_name"`
	FormulaID   string  `json:"formula_id"`
	FormulaName string  `json:"formula_name"`
	Symbol      string  `json:"symbol"`
	Latest      float64 `json:"latest"`
	DetailJSON  string  `json:"detail_json"`
	CreatedAt   string  `json:"created_at"`
}

type DecisionNote struct {
	Symbol          string  `json:"symbol"`
	Status          string  `json:"status"`
	AddedPrice      float64 `json:"added_price"`
	AddReason       string  `json:"add_reason"`
	PlanBuy         float64 `json:"plan_buy"`
	StopLoss        float64 `json:"stop_loss"`
	ReviewNote      string  `json:"review_note"`
	ExcludeCategory string  `json:"exclude_category"`
	ExcludeReason   string  `json:"exclude_reason"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type Webhook struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Method      string `json:"method"`
	HeadersJSON string `json:"headers_json"`
	Events      string `json:"events"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func OpenAppStore() (*AppStore, error) {
	if err := os.MkdirAll(tdx.DefaultDatabaseDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(tdx.DefaultDatabaseDir, "automation.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &AppStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.seedDefaults(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *AppStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *AppStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS formulas (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'indicator',
			script TEXT NOT NULL,
			args_json TEXT NOT NULL DEFAULT '[]',
			period TEXT NOT NULL DEFAULT 'day',
			right INTEGER NOT NULL DEFAULT 1,
			enabled INTEGER NOT NULL DEFAULT 1,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stock_pools (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			symbols_json TEXT NOT NULL DEFAULT '[]',
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS automation_tasks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			cron TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 0,
			payload_json TEXT NOT NULL DEFAULT '{}',
			webhook_ids TEXT NOT NULL DEFAULT '[]',
			last_run_at TEXT NOT NULL DEFAULT '',
			next_run_at TEXT NOT NULL DEFAULT '',
			last_status TEXT NOT NULL DEFAULT '',
			last_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			config_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			readonly INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS automation_runs (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			task_name TEXT NOT NULL,
			task_type TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT '',
			log TEXT NOT NULL DEFAULT '',
			result_json TEXT NOT NULL DEFAULT '{}',
			matched_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS selection_results (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			task_name TEXT NOT NULL,
			formula_id TEXT NOT NULL,
			formula_name TEXT NOT NULL,
			symbol TEXT NOT NULL,
			latest REAL NOT NULL DEFAULT 0,
			detail_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_selection_results_created_at ON selection_results(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_selection_results_symbol ON selection_results(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_selection_results_formula ON selection_results(formula_id)`,
		`CREATE TABLE IF NOT EXISTS decision_notes (
			symbol TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT '',
			added_price REAL NOT NULL DEFAULT 0,
			add_reason TEXT NOT NULL DEFAULT '',
			plan_buy REAL NOT NULL DEFAULT 0,
			stop_loss REAL NOT NULL DEFAULT 0,
			review_note TEXT NOT NULL DEFAULT '',
			exclude_category TEXT NOT NULL DEFAULT '',
			exclude_reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_decision_notes_status ON decision_notes(status)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			method TEXT NOT NULL DEFAULT 'POST',
			headers_json TEXT NOT NULL DEFAULT '{}',
			events TEXT NOT NULL DEFAULT '["automation.failed","stock_selection.finished"]',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *AppStore) seedDefaults() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM formulas`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		now := nowText()
		_, err := s.db.Exec(`INSERT INTO formulas
			(id,name,type,script,args_json,period,right,enabled,description,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			uuid.NewString(), "5日上穿20日均线", "selection",
			"CROSS(MA(C,5),MA(C,20));", "[]", "day", 1, 1,
			"示例选股公式：短期均线上穿长期均线。", now, now)
		if err != nil {
			return err
		}
	}
	now := nowText()
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO formulas
		(id,name,type,script,args_json,period,right,enabled,description,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		"default-ma-overlay", "MA均线叠加", "indicator",
		"MA5:MA(C,5);\nMA10:MA(C,10);\nMA20:MA(C,20);", "[]", "day", 1, 1,
		"默认图表指标：在专业行情K线上叠加5/10/20日均线。", now, now); err != nil {
		return err
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM stock_pools`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		pool := StockPool{
			Name:        "示例股票池",
			Symbols:     []string{"000001", "600000", "000858", "002202"},
			Description: "第一版内置的小股票池，可在页面里改成你的自选列表。",
		}
		if _, err := s.UpsertStockPool(pool); err != nil {
			return err
		}
	}

	if err := s.ensureDecisionPools(); err != nil {
		return err
	}
	if err := s.ensureStrategyTemplates(); err != nil {
		return err
	}
	return s.ensureFixedAutomationTasks()
}

func defaultStrategyTemplates() []Strategy {
	return []Strategy{
		{
			ID:          "template-a-share-v3",
			Name:        "A股V3强势启动",
			Description: "内置模板：硬过滤成交额/排除池，使用趋势、放量、突破和主力拉升公式进行加权评分。",
			ConfigJSON:  `{"universe":"market","pool_id":"market-all-a","calc_count":260,"batch_size":50,"continue_on_error":true,"filters":[{"id":"exclude_pool","factor":"pool_exclude","params":{"pool_id":"exclude"}},{"id":"min_amount","factor":"min_amount","params":{"value":100000000}}],"scores":[{"id":"ma_trend","factor":"ma_trend","weight":20,"params":{"short":5,"mid":10,"long":20}},{"id":"volume_up","factor":"volume_up","weight":15,"params":{"days":5,"ratio":1.3}},{"id":"break_high","factor":"break_high","weight":15,"params":{"days":20}},{"id":"main_force","factor":"formula","weight":30,"params":{"formula_name":"主力拉升"}}],"pass":{"min_score":60,"top_n":50}}`,
			Enabled:     true,
			Readonly:    true,
		},
	}
}

func (s *AppStore) ensureStrategyTemplates() error {
	now := nowText()
	for _, item := range defaultStrategyTemplates() {
		if strings.TrimSpace(item.ConfigJSON) == "" {
			item.ConfigJSON = "{}"
		}
		_, err := s.db.Exec(`INSERT OR IGNORE INTO strategies
			(id,name,description,config_json,enabled,readonly,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			item.ID, item.Name, item.Description, item.ConfigJSON, boolInt(item.Enabled), boolInt(item.Readonly), now, now)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(`UPDATE strategies
			SET name=?,description=?,config_json=?,enabled=?,readonly=1,updated_at=?
			WHERE id=? AND readonly=1`,
			item.Name, item.Description, item.ConfigJSON, boolInt(item.Enabled), now, item.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func fixedCloseSyncAutomationTask() AutomationTask {
	return AutomationTask{
		ID:          FixedCloseSyncTaskID,
		Name:        "收盘作业：更新当天行情",
		Type:        "system_sync",
		Cron:        "0 0 16 * * 1-5",
		Enabled:     false,
		PayloadJSON: `{"scope":"kline","tables":["day"],"limit":4,"continue_on_error":true}`,
		WebhookIDs:  "[]",
		Readonly:    true,
		System:      true,
	}
}

func isFixedAutomationTaskID(id string) bool {
	return id == FixedCloseSyncTaskID
}

func decorateAutomationTask(t AutomationTask) AutomationTask {
	if isFixedAutomationTaskID(t.ID) {
		t.Readonly = true
		t.System = true
	}
	return t
}

func (s *AppStore) ensureFixedAutomationTasks() error {
	task := fixedCloseSyncAutomationTask()
	now := nowText()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO automation_tasks
		(id,name,type,cron,enabled,payload_json,webhook_ids,last_run_at,next_run_at,last_status,last_message,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		task.ID, task.Name, task.Type, task.Cron, boolInt(task.Enabled), task.PayloadJSON, task.WebhookIDs,
		"", "", "", "", now, now)
	return err
}

func (s *AppStore) ensureDecisionPools() error {
	defaults := []StockPool{
		{
			ID:          DecisionWatchPoolID,
			Name:        "观察池",
			Description: "首页决策工作台使用的观察列表，适合放入今日命中后准备继续跟踪的股票。",
		},
		{
			ID:          DecisionExcludePoolID,
			Name:        "排除池",
			Description: "首页决策工作台使用的排除列表，适合放入暂时不想再处理的股票。",
		},
	}
	for _, pool := range defaults {
		now := nowText()
		_, err := s.db.Exec(`INSERT OR IGNORE INTO stock_pools
			(id,name,symbols_json,description,created_at,updated_at)
			VALUES (?,?,?,?,?,?)`,
			pool.ID, pool.Name, "[]", pool.Description, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

func nowText() string {
	return time.Now().Format(time.RFC3339)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intBool(v int) bool {
	return v != 0
}

func normalizeFormula(f Formula) Formula {
	f.Name = strings.TrimSpace(f.Name)
	f.Type = strings.TrimSpace(f.Type)
	if f.Type == "" {
		f.Type = "indicator"
	}
	f.Script = strings.TrimSpace(f.Script)
	if strings.TrimSpace(f.ArgsJSON) == "" {
		f.ArgsJSON = "[]"
	}
	f.Period = strings.TrimSpace(f.Period)
	if f.Period == "" {
		f.Period = "day"
	}
	if f.Right < 0 || f.Right > 2 {
		f.Right = 1
	}
	return f
}

func (s *AppStore) ListFormulas() ([]Formula, error) {
	rows, err := s.db.Query(`SELECT id,name,type,script,args_json,period,right,enabled,description,created_at,updated_at FROM formulas ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Formula
	for rows.Next() {
		var f Formula
		var enabled int
		if err := rows.Scan(&f.ID, &f.Name, &f.Type, &f.Script, &f.ArgsJSON, &f.Period, &f.Right, &enabled, &f.Description, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		f.Enabled = intBool(enabled)
		list = append(list, f)
	}
	if list == nil {
		list = []Formula{}
	}
	return list, rows.Err()
}

func (s *AppStore) GetFormula(id string) (Formula, error) {
	var f Formula
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,type,script,args_json,period,right,enabled,description,created_at,updated_at FROM formulas WHERE id=?`, id).
		Scan(&f.ID, &f.Name, &f.Type, &f.Script, &f.ArgsJSON, &f.Period, &f.Right, &enabled, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	f.Enabled = intBool(enabled)
	return f, err
}

func (s *AppStore) UpsertFormula(f Formula) (Formula, error) {
	f = normalizeFormula(f)
	if f.Name == "" || f.Script == "" {
		return f, errors.New("公式名称和内容不能为空")
	}
	if !json.Valid([]byte(f.ArgsJSON)) {
		return f, errors.New("args_json不是有效JSON")
	}

	now := nowText()
	if f.ID == "" {
		f.ID = uuid.NewString()
		f.CreatedAt = now
	} else if f.CreatedAt == "" {
		old, err := s.GetFormula(f.ID)
		if err == nil {
			f.CreatedAt = old.CreatedAt
		} else {
			f.CreatedAt = now
		}
	}
	f.UpdatedAt = now

	_, err := s.db.Exec(`INSERT INTO formulas
		(id,name,type,script,args_json,period,right,enabled,description,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,type=excluded.type,script=excluded.script,args_json=excluded.args_json,
			period=excluded.period,right=excluded.right,enabled=excluded.enabled,description=excluded.description,
			updated_at=excluded.updated_at`,
		f.ID, f.Name, f.Type, f.Script, f.ArgsJSON, f.Period, f.Right, boolInt(f.Enabled), f.Description, f.CreatedAt, f.UpdatedAt)
	return f, err
}

func (s *AppStore) DeleteFormula(id string) error {
	_, err := s.db.Exec(`DELETE FROM formulas WHERE id=?`, id)
	return err
}

func (s *AppStore) ListStockPools() ([]StockPool, error) {
	systemPools := listMarketStockPools()
	rows, err := s.db.Query(`SELECT id,name,symbols_json,description,created_at,updated_at FROM stock_pools ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := append([]StockPool{}, systemPools...)
	for rows.Next() {
		var pool StockPool
		var symbols string
		if err := rows.Scan(&pool.ID, &pool.Name, &symbols, &pool.Description, &pool.CreatedAt, &pool.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(symbols), &pool.Symbols)
		pool.Category = "custom"
		if pool.ID == DecisionWatchPoolID || pool.ID == DecisionExcludePoolID {
			pool.Category = "decision"
			pool.System = true
		}
		list = append(list, pool)
	}
	if list == nil {
		list = []StockPool{}
	}
	return list, rows.Err()
}

func (s *AppStore) GetStockPool(id string) (StockPool, error) {
	if pool, ok := getMarketStockPool(id); ok {
		return pool, nil
	}
	var pool StockPool
	var symbols string
	err := s.db.QueryRow(`SELECT id,name,symbols_json,description,created_at,updated_at FROM stock_pools WHERE id=?`, id).
		Scan(&pool.ID, &pool.Name, &symbols, &pool.Description, &pool.CreatedAt, &pool.UpdatedAt)
	_ = json.Unmarshal([]byte(symbols), &pool.Symbols)
	pool.Category = "custom"
	if pool.ID == DecisionWatchPoolID || pool.ID == DecisionExcludePoolID {
		pool.Category = "decision"
		pool.System = true
	}
	return pool, err
}

func (s *AppStore) UpsertStockPool(pool StockPool) (StockPool, error) {
	if isMarketPoolID(pool.ID) {
		return pool, errors.New("系统市场分组不能编辑")
	}
	pool.Name = strings.TrimSpace(pool.Name)
	if pool.Name == "" {
		return pool, errors.New("股票池名称不能为空")
	}
	pool.Symbols = normalizeSymbols(pool.Symbols)
	raw, err := json.Marshal(pool.Symbols)
	if err != nil {
		return pool, err
	}

	now := nowText()
	if pool.ID == "" {
		pool.ID = uuid.NewString()
		pool.CreatedAt = now
	} else if pool.CreatedAt == "" {
		old, err := s.GetStockPool(pool.ID)
		if err == nil {
			pool.CreatedAt = old.CreatedAt
		} else {
			pool.CreatedAt = now
		}
	}
	pool.UpdatedAt = now

	_, err = s.db.Exec(`INSERT INTO stock_pools
		(id,name,symbols_json,description,created_at,updated_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,symbols_json=excluded.symbols_json,description=excluded.description,updated_at=excluded.updated_at`,
		pool.ID, pool.Name, string(raw), pool.Description, pool.CreatedAt, pool.UpdatedAt)
	return pool, err
}

func (s *AppStore) DeleteStockPool(id string) error {
	if isMarketPoolID(id) {
		return errors.New("系统市场分组不能删除")
	}
	if id == DecisionWatchPoolID || id == DecisionExcludePoolID {
		return errors.New("系统股票池不能删除")
	}
	_, err := s.db.Exec(`DELETE FROM stock_pools WHERE id=?`, id)
	return err
}

func normalizeStrategy(item Strategy) Strategy {
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	if strings.TrimSpace(item.ConfigJSON) == "" {
		item.ConfigJSON = "{}"
	}
	return item
}

func (s *AppStore) ListStrategies() ([]Strategy, error) {
	rows, err := s.db.Query(`SELECT id,name,description,config_json,enabled,readonly,created_at,updated_at FROM strategies ORDER BY readonly DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Strategy{}
	for rows.Next() {
		var item Strategy
		var enabled, readonly int
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.ConfigJSON, &enabled, &readonly, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = intBool(enabled)
		item.Readonly = intBool(readonly)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *AppStore) GetStrategy(id string) (Strategy, error) {
	var item Strategy
	var enabled, readonly int
	err := s.db.QueryRow(`SELECT id,name,description,config_json,enabled,readonly,created_at,updated_at FROM strategies WHERE id=?`, id).
		Scan(&item.ID, &item.Name, &item.Description, &item.ConfigJSON, &enabled, &readonly, &item.CreatedAt, &item.UpdatedAt)
	item.Enabled = intBool(enabled)
	item.Readonly = intBool(readonly)
	return item, err
}

func (s *AppStore) UpsertStrategy(item Strategy) (Strategy, error) {
	item = normalizeStrategy(item)
	if item.Name == "" {
		return item, errors.New("策略名称不能为空")
	}
	if !json.Valid([]byte(item.ConfigJSON)) {
		return item, errors.New("config_json不是有效JSON")
	}
	if item.ID != "" {
		old, err := s.GetStrategy(item.ID)
		if err == nil && old.Readonly {
			return item, errors.New("系统策略模板不能编辑，请复制后修改")
		}
	}
	now := nowText()
	if item.ID == "" {
		item.ID = uuid.NewString()
		item.CreatedAt = now
	} else if item.CreatedAt == "" {
		old, err := s.GetStrategy(item.ID)
		if err == nil {
			item.CreatedAt = old.CreatedAt
		} else {
			item.CreatedAt = now
		}
	}
	item.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO strategies
		(id,name,description,config_json,enabled,readonly,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,description=excluded.description,config_json=excluded.config_json,
			enabled=excluded.enabled,updated_at=excluded.updated_at`,
		item.ID, item.Name, item.Description, item.ConfigJSON, boolInt(item.Enabled), boolInt(item.Readonly), item.CreatedAt, item.UpdatedAt)
	return item, err
}

func (s *AppStore) DeleteStrategy(id string) error {
	item, err := s.GetStrategy(id)
	if err != nil {
		return err
	}
	if item.Readonly {
		return errors.New("系统策略模板不能删除")
	}
	_, err = s.db.Exec(`DELETE FROM strategies WHERE id=?`, id)
	return err
}

func (s *AppStore) CloneStrategy(id string) (Strategy, error) {
	item, err := s.GetStrategy(id)
	if err != nil {
		return Strategy{}, err
	}
	item.ID = ""
	item.Name = item.Name + " 副本"
	item.Readonly = false
	return s.UpsertStrategy(item)
}

func (s *AppStore) AddStockPoolSymbol(id, symbol string) (StockPool, error) {
	if isMarketPoolID(id) {
		return StockPool{}, errors.New("系统市场分组不能手动增删股票")
	}
	pool, err := s.GetStockPool(id)
	if err != nil {
		return pool, err
	}
	symbols := append(pool.Symbols, symbol)
	pool.Symbols = normalizeSymbols(symbols)
	return s.UpsertStockPool(pool)
}

func (s *AppStore) RemoveStockPoolSymbol(id, symbol string) (StockPool, error) {
	if isMarketPoolID(id) {
		return StockPool{}, errors.New("系统市场分组不能手动增删股票")
	}
	pool, err := s.GetStockPool(id)
	if err != nil {
		return pool, err
	}
	targets := normalizeSymbols([]string{symbol})
	if len(targets) == 0 {
		return pool, errors.New("股票代码不能为空")
	}
	target := targets[0]
	next := make([]string, 0, len(pool.Symbols))
	for _, item := range normalizeSymbols(pool.Symbols) {
		if item != target {
			next = append(next, item)
		}
	}
	pool.Symbols = next
	return s.UpsertStockPool(pool)
}

func (s *AppStore) GetDecisionNote(symbol string) (DecisionNote, error) {
	symbols := normalizeSymbols([]string{symbol})
	if len(symbols) == 0 {
		return DecisionNote{}, errors.New("股票代码不能为空")
	}
	var note DecisionNote
	err := s.db.QueryRow(`SELECT symbol,status,added_price,add_reason,plan_buy,stop_loss,review_note,exclude_category,exclude_reason,created_at,updated_at
		FROM decision_notes WHERE symbol=?`, symbols[0]).
		Scan(&note.Symbol, &note.Status, &note.AddedPrice, &note.AddReason, &note.PlanBuy, &note.StopLoss, &note.ReviewNote, &note.ExcludeCategory, &note.ExcludeReason, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}

func (s *AppStore) UpsertDecisionNote(note DecisionNote) (DecisionNote, error) {
	symbols := normalizeSymbols([]string{note.Symbol})
	if len(symbols) == 0 {
		return note, errors.New("股票代码不能为空")
	}
	note.Symbol = symbols[0]
	note.Status = strings.TrimSpace(note.Status)
	note.AddReason = strings.TrimSpace(note.AddReason)
	note.ReviewNote = strings.TrimSpace(note.ReviewNote)
	note.ExcludeCategory = strings.TrimSpace(note.ExcludeCategory)
	note.ExcludeReason = strings.TrimSpace(note.ExcludeReason)
	now := nowText()
	if note.CreatedAt == "" {
		old, err := s.GetDecisionNote(note.Symbol)
		if err == nil {
			note.CreatedAt = old.CreatedAt
		} else {
			note.CreatedAt = now
		}
	}
	note.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO decision_notes
		(symbol,status,added_price,add_reason,plan_buy,stop_loss,review_note,exclude_category,exclude_reason,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(symbol) DO UPDATE SET
			status=excluded.status,added_price=excluded.added_price,add_reason=excluded.add_reason,
			plan_buy=excluded.plan_buy,stop_loss=excluded.stop_loss,review_note=excluded.review_note,
			exclude_category=excluded.exclude_category,exclude_reason=excluded.exclude_reason,updated_at=excluded.updated_at`,
		note.Symbol, note.Status, note.AddedPrice, note.AddReason, note.PlanBuy, note.StopLoss, note.ReviewNote, note.ExcludeCategory, note.ExcludeReason, note.CreatedAt, note.UpdatedAt)
	return note, err
}

func (s *AppStore) SetDecisionStatus(symbol, status string) error {
	symbols := normalizeSymbols([]string{symbol})
	if len(symbols) == 0 {
		return errors.New("股票代码不能为空")
	}
	note, err := s.GetDecisionNote(symbols[0])
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		note = DecisionNote{Symbol: symbols[0]}
	}
	note.Status = strings.TrimSpace(status)
	_, err = s.UpsertDecisionNote(note)
	return err
}

func (s *AppStore) ListDecisionNotes(status string, limit int) ([]DecisionNote, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query := `SELECT symbol,status,added_price,add_reason,plan_buy,stop_loss,review_note,exclude_category,exclude_reason,created_at,updated_at FROM decision_notes`
	args := []interface{}{}
	if strings.TrimSpace(status) != "" {
		query += ` WHERE status=?`
		args = append(args, strings.TrimSpace(status))
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []DecisionNote
	for rows.Next() {
		var note DecisionNote
		if err := rows.Scan(&note.Symbol, &note.Status, &note.AddedPrice, &note.AddReason, &note.PlanBuy, &note.StopLoss, &note.ReviewNote, &note.ExcludeCategory, &note.ExcludeReason, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, note)
	}
	if list == nil {
		list = []DecisionNote{}
	}
	return list, rows.Err()
}

func normalizeSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		s := strings.TrimSpace(strings.ToUpper(symbol))
		s = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(s, ".SH"), ".SZ"), ".BJ")
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		result = append(result, s)
	}
	return result
}

func normalizeTask(t AutomationTask) AutomationTask {
	t.Name = strings.TrimSpace(t.Name)
	t.Type = strings.TrimSpace(t.Type)
	if t.Type == "" {
		t.Type = "custom"
	}
	t.Cron = strings.TrimSpace(t.Cron)
	if strings.TrimSpace(t.PayloadJSON) == "" {
		t.PayloadJSON = "{}"
	}
	if strings.TrimSpace(t.WebhookIDs) == "" {
		t.WebhookIDs = "[]"
	}
	return t
}

func (s *AppStore) ListAutomationTasks() ([]AutomationTask, error) {
	rows, err := s.db.Query(`SELECT id,name,type,cron,enabled,payload_json,webhook_ids,last_run_at,next_run_at,last_status,last_message,created_at,updated_at FROM automation_tasks ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AutomationTask
	for rows.Next() {
		var t AutomationTask
		var enabled int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Cron, &enabled, &t.PayloadJSON, &t.WebhookIDs, &t.LastRunAt, &t.NextRunAt, &t.LastStatus, &t.LastMessage, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Enabled = intBool(enabled)
		t = decorateAutomationTask(t)
		list = append(list, t)
	}
	if list == nil {
		list = []AutomationTask{}
	}
	return list, rows.Err()
}

func (s *AppStore) GetAutomationTask(id string) (AutomationTask, error) {
	var t AutomationTask
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,type,cron,enabled,payload_json,webhook_ids,last_run_at,next_run_at,last_status,last_message,created_at,updated_at FROM automation_tasks WHERE id=?`, id).
		Scan(&t.ID, &t.Name, &t.Type, &t.Cron, &enabled, &t.PayloadJSON, &t.WebhookIDs, &t.LastRunAt, &t.NextRunAt, &t.LastStatus, &t.LastMessage, &t.CreatedAt, &t.UpdatedAt)
	t.Enabled = intBool(enabled)
	return decorateAutomationTask(t), err
}

func (s *AppStore) UpsertAutomationTask(t AutomationTask) (AutomationTask, error) {
	t = normalizeTask(t)
	if t.Name == "" || t.Cron == "" {
		return t, errors.New("任务名称和cron不能为空")
	}
	if !json.Valid([]byte(t.PayloadJSON)) {
		return t, errors.New("payload_json不是有效JSON")
	}
	if !json.Valid([]byte(t.WebhookIDs)) {
		return t, errors.New("webhook_ids不是有效JSON数组")
	}

	now := nowText()
	if t.ID == "" {
		t.ID = uuid.NewString()
		t.CreatedAt = now
	} else if t.CreatedAt == "" {
		old, err := s.GetAutomationTask(t.ID)
		if err == nil {
			t.CreatedAt = old.CreatedAt
		} else {
			t.CreatedAt = now
		}
	}
	t.UpdatedAt = now

	_, err := s.db.Exec(`INSERT INTO automation_tasks
		(id,name,type,cron,enabled,payload_json,webhook_ids,last_run_at,next_run_at,last_status,last_message,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,type=excluded.type,cron=excluded.cron,enabled=excluded.enabled,payload_json=excluded.payload_json,
			webhook_ids=excluded.webhook_ids,next_run_at=excluded.next_run_at,updated_at=excluded.updated_at`,
		t.ID, t.Name, t.Type, t.Cron, boolInt(t.Enabled), t.PayloadJSON, t.WebhookIDs, t.LastRunAt, t.NextRunAt, t.LastStatus, t.LastMessage, t.CreatedAt, t.UpdatedAt)
	return t, err
}

func (s *AppStore) SetAutomationTaskEnabled(id string, enabled bool) (AutomationTask, error) {
	now := nowText()
	nextRun := ""
	if enabled {
		current, err := s.GetAutomationTask(id)
		if err == nil {
			nextRun = current.NextRunAt
		}
	}
	result, err := s.db.Exec(`UPDATE automation_tasks SET enabled=?, next_run_at=?, updated_at=? WHERE id=?`, boolInt(enabled), nextRun, now, id)
	if err != nil {
		return AutomationTask{}, err
	}
	if count, err := result.RowsAffected(); err == nil && count == 0 {
		return AutomationTask{}, sql.ErrNoRows
	}
	return s.GetAutomationTask(id)
}

func (s *AppStore) DeleteAutomationTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM automation_tasks WHERE id=?`, id)
	return err
}

func (s *AppStore) CreateAutomationRun(task AutomationTask) (AutomationRun, error) {
	run := AutomationRun{
		ID:         uuid.NewString(),
		TaskID:     task.ID,
		TaskName:   task.Name,
		TaskType:   task.Type,
		Status:     "running",
		StartedAt:  nowText(),
		ResultJSON: "{}",
	}
	_, err := s.db.Exec(`INSERT INTO automation_runs
		(id,task_id,task_name,task_type,status,started_at,finished_at,log,result_json,matched_count)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		run.ID, run.TaskID, run.TaskName, run.TaskType, run.Status, run.StartedAt, run.FinishedAt, run.Log, run.ResultJSON, run.MatchedCount)
	return run, err
}

func (s *AppStore) FinishAutomationRun(runID, status, logText, resultJSON string, matchedCount int) error {
	if strings.TrimSpace(resultJSON) == "" {
		resultJSON = "{}"
	}
	_, err := s.db.Exec(`UPDATE automation_runs SET status=?,finished_at=?,log=?,result_json=?,matched_count=? WHERE id=?`,
		status, nowText(), logText, resultJSON, matchedCount, runID)
	return err
}

func (s *AppStore) SaveSelectionResults(run AutomationRun, formula Formula, items []SelectionResult) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM selection_results WHERE run_id=?`, run.ID); err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == "" {
			item.ID = uuid.NewString()
		}
		item.RunID = run.ID
		item.TaskID = run.TaskID
		item.TaskName = run.TaskName
		item.FormulaID = formula.ID
		item.FormulaName = formula.Name
		if item.CreatedAt == "" {
			item.CreatedAt = nowText()
		}
		if strings.TrimSpace(item.DetailJSON) == "" {
			item.DetailJSON = "{}"
		}
		if _, err := tx.Exec(`INSERT INTO selection_results
			(id,run_id,task_id,task_name,formula_id,formula_name,symbol,latest,detail_json,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			item.ID, item.RunID, item.TaskID, item.TaskName, item.FormulaID, item.FormulaName, item.Symbol, item.Latest, item.DetailJSON, item.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *AppStore) GetAutomationRun(runID string) (AutomationRun, error) {
	var run AutomationRun
	err := s.db.QueryRow(`SELECT id,task_id,task_name,task_type,status,started_at,finished_at,log,result_json,matched_count FROM automation_runs WHERE id=?`, runID).
		Scan(&run.ID, &run.TaskID, &run.TaskName, &run.TaskType, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Log, &run.ResultJSON, &run.MatchedCount)
	return run, err
}

func (s *AppStore) UpdateTaskRunState(taskID, status, message string) error {
	_, err := s.db.Exec(`UPDATE automation_tasks SET last_run_at=?,last_status=?,last_message=?,updated_at=? WHERE id=?`,
		nowText(), status, message, nowText(), taskID)
	return err
}

func (s *AppStore) UpdateTaskNextRun(taskID, nextRun string) error {
	_, err := s.db.Exec(`UPDATE automation_tasks SET next_run_at=?,updated_at=? WHERE id=?`, nextRun, nowText(), taskID)
	return err
}

func (s *AppStore) ListAutomationRuns(taskID string, limit int) ([]AutomationRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id,task_id,task_name,task_type,status,started_at,finished_at,log,result_json,matched_count FROM automation_runs`
	args := []interface{}{}
	if taskID != "" {
		query += ` WHERE task_id=?`
		args = append(args, taskID)
	}
	query += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AutomationRun
	for rows.Next() {
		var run AutomationRun
		if err := rows.Scan(&run.ID, &run.TaskID, &run.TaskName, &run.TaskType, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Log, &run.ResultJSON, &run.MatchedCount); err != nil {
			return nil, err
		}
		list = append(list, run)
	}
	if list == nil {
		list = []AutomationRun{}
	}
	return list, rows.Err()
}

func (s *AppStore) ListSelectionResults(taskID, formulaID, symbol string, onlyLatest bool, limit int) ([]SelectionResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id,run_id,task_id,task_name,formula_id,formula_name,symbol,latest,detail_json,created_at FROM selection_results`
	args := []interface{}{}
	conds := []string{}
	if taskID != "" {
		conds = append(conds, "task_id=?")
		args = append(args, taskID)
	}
	if formulaID != "" {
		conds = append(conds, "formula_id=?")
		args = append(args, formulaID)
	}
	if symbol != "" {
		conds = append(conds, "symbol=?")
		args = append(args, strings.ToUpper(symbol))
	}
	if onlyLatest {
		conds = append(conds, `run_id=(SELECT run_id FROM selection_results ORDER BY created_at DESC LIMIT 1)`)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SelectionResult
	for rows.Next() {
		var item SelectionResult
		if err := rows.Scan(&item.ID, &item.RunID, &item.TaskID, &item.TaskName, &item.FormulaID, &item.FormulaName, &item.Symbol, &item.Latest, &item.DetailJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = []SelectionResult{}
	}
	return list, rows.Err()
}

func normalizeWebhook(h Webhook) Webhook {
	h.Name = strings.TrimSpace(h.Name)
	h.URL = strings.TrimSpace(h.URL)
	h.Method = strings.ToUpper(strings.TrimSpace(h.Method))
	if h.Method == "" {
		h.Method = "POST"
	}
	if strings.TrimSpace(h.HeadersJSON) == "" {
		h.HeadersJSON = "{}"
	}
	if strings.TrimSpace(h.Events) == "" {
		h.Events = `["automation.failed","automation.finished","stock_selection.finished","strategy_selection.finished"]`
	}
	return h
}

func (s *AppStore) ListWebhooks() ([]Webhook, error) {
	rows, err := s.db.Query(`SELECT id,name,url,method,headers_json,events,enabled,created_at,updated_at FROM webhooks ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Webhook
	for rows.Next() {
		var h Webhook
		var enabled int
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &h.Method, &h.HeadersJSON, &h.Events, &enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		h.Enabled = intBool(enabled)
		list = append(list, h)
	}
	if list == nil {
		list = []Webhook{}
	}
	return list, rows.Err()
}

func (s *AppStore) ListEnabledWebhooks() ([]Webhook, error) {
	items, err := s.ListWebhooks()
	if err != nil {
		return nil, err
	}
	enabled := make([]Webhook, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			enabled = append(enabled, item)
		}
	}
	return enabled, nil
}

func (s *AppStore) GetWebhook(id string) (Webhook, error) {
	var h Webhook
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,url,method,headers_json,events,enabled,created_at,updated_at FROM webhooks WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.URL, &h.Method, &h.HeadersJSON, &h.Events, &enabled, &h.CreatedAt, &h.UpdatedAt)
	h.Enabled = intBool(enabled)
	return h, err
}

func (s *AppStore) UpsertWebhook(h Webhook) (Webhook, error) {
	h = normalizeWebhook(h)
	if h.Name == "" || h.URL == "" {
		return h, errors.New("Webhook名称和URL不能为空")
	}
	if !json.Valid([]byte(h.HeadersJSON)) {
		return h, errors.New("headers_json不是有效JSON")
	}
	if !json.Valid([]byte(h.Events)) {
		return h, errors.New("events不是有效JSON数组")
	}

	now := nowText()
	if h.ID == "" {
		h.ID = uuid.NewString()
		h.CreatedAt = now
	} else if h.CreatedAt == "" {
		old, err := s.GetWebhook(h.ID)
		if err == nil {
			h.CreatedAt = old.CreatedAt
		} else {
			h.CreatedAt = now
		}
	}
	h.UpdatedAt = now

	_, err := s.db.Exec(`INSERT INTO webhooks
		(id,name,url,method,headers_json,events,enabled,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,url=excluded.url,method=excluded.method,headers_json=excluded.headers_json,
			events=excluded.events,enabled=excluded.enabled,updated_at=excluded.updated_at`,
		h.ID, h.Name, h.URL, h.Method, h.HeadersJSON, h.Events, boolInt(h.Enabled), h.CreatedAt, h.UpdatedAt)
	return h, err
}

func (s *AppStore) DeleteWebhook(id string) error {
	_, err := s.db.Exec(`DELETE FROM webhooks WHERE id=?`, id)
	return err
}

func (s *AppStore) ResolveWebhooks(idsJSON string) ([]Webhook, error) {
	var ids []string
	if strings.TrimSpace(idsJSON) == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	hooks := make([]Webhook, 0, len(ids))
	for _, id := range ids {
		h, err := s.GetWebhook(id)
		if err != nil {
			continue
		}
		if h.Enabled {
			hooks = append(hooks, h)
		}
	}
	return hooks, nil
}

func mustJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(raw)
}
