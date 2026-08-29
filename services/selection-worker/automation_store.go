package main

import workbench "workbench-core"

type AppStore = workbench.AppStore
type Formula = workbench.Formula
type StockPool = workbench.StockPool
type Strategy = workbench.Strategy
type AutomationTask = workbench.AutomationTask
type AutomationRun = workbench.AutomationRun
type SelectionResult = workbench.SelectionResult
type DecisionNote = workbench.DecisionNote
type Webhook = workbench.Webhook
type TradingAccount = workbench.TradingAccount
type TradingDiscipline = workbench.TradingDiscipline
type TradingFeeConfig = workbench.TradingFeeConfig
type TradingTrade = workbench.TradingTrade
type TradingSystemState = workbench.TradingSystemState

const (
	DecisionWatchPoolID   = workbench.DecisionWatchPoolID
	DecisionExcludePoolID = workbench.DecisionExcludePoolID
	FixedCloseSyncTaskID  = workbench.FixedCloseSyncTaskID
	TradingSystemStateID  = workbench.TradingSystemStateID
)

func OpenAppStore() (*AppStore, error) {
	workbench.SetCodeModelProvider(getAllCodeModels)
	return workbench.OpenAppStore()
}

func nowText() string {
	return workbench.NowText()
}

func normalizeSymbols(symbols []string) []string {
	return workbench.NormalizeSymbols(symbols)
}

func defaultTradingSystemState() TradingSystemState {
	return workbench.DefaultTradingSystemState()
}

func isFixedAutomationTaskID(id string) bool {
	return workbench.IsFixedAutomationTaskID(id)
}

func mustJSON(v interface{}) string {
	return workbench.MustJSON(v)
}

func limitedMarketPoolSymbols(poolID string, maxCodes int) []string {
	return workbench.LimitedMarketPoolSymbols(poolID, maxCodes)
}
