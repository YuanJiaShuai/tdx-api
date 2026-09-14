package main

import (
	"sort"
	"strings"
)

// 组合模拟撮合引擎:在历史回测逐日信号的基础上,模拟单账户资金池的买卖流水。
// 信号于 T 日收盘产生,撮合在 T+1 日开盘执行;退出条件由 T 日收盘判定,次日开盘执行。
// 约束:涨停买不进、跌停卖不出、100股整手、T+1、最大持仓数、等权分配。

type portfolioSimConfig struct {
	InitialCash  float64 `json:"initial_cash"`
	MaxPositions int     `json:"max_positions"`
}

type portfolioSimPosition struct {
	symbol         string
	shares         int
	entryPrice     float64
	entryDate      int
	signalDate     int
	strategyName   string
	lastClose      float64 // 最近可用收盘价(权益估值用)
	peakClose      float64 // 持仓期间最高收盘价(回撤止盈基准)
	stopActive     bool    // 浮盈是否曾触及 +12%,激活回撤止盈
	holdDays       int     // 持有交易日数
	pendingExit    string  // 昨日收盘判定、今日开盘执行的退出原因
	pendingExitSet bool
}

type portfolioPendingBuy struct {
	symbol       string
	score        float64
	signalDate   int
	strategyName string
}

type portfolioSimulator struct {
	cfg         portfolioSimConfig
	cash        float64
	positions   map[string]*portfolioSimPosition
	trades      []HistoricalBacktestTrade
	equityPeak  float64
	maxDrawdown float64
}

func newPortfolioSimulator(cfg portfolioSimConfig) *portfolioSimulator {
	return &portfolioSimulator{
		cfg:         cfg,
		cash:        cfg.InitialCash,
		positions:   map[string]*portfolioSimPosition{},
		trades:      []HistoricalBacktestTrade{},
		equityPeak:  cfg.InitialCash,
		maxDrawdown: 0,
	}
}

func (sim *portfolioSimulator) equity() float64 {
	total := sim.cash
	for _, pos := range sim.positions {
		total += float64(pos.shares) * pos.lastClose
	}
	return total
}

func portfolioPriceLimitRate(symbol string) float64 {
	switch {
	case strings.HasPrefix(symbol, "30"), strings.HasPrefix(symbol, "68"):
		return 0.20
	case strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"), strings.HasPrefix(symbol, "92"):
		return 0.30
	default:
		return 0.10
	}
}

func portfolioKlineOnDate(rows []FormulaKline, date int) (FormulaKline, bool) {
	index := sort.Search(len(rows), func(i int) bool { return rows[i].Date >= date })
	if index >= len(rows) || rows[index].Date != date {
		return FormulaKline{}, false
	}
	return rows[index], true
}

func portfolioMA20(rows []FormulaKline, date int) (float64, bool) {
	end := sort.Search(len(rows), func(i int) bool { return rows[i].Date > date })
	if end < 20 {
		return 0, false
	}
	window := rows[end-20 : end]
	if window[len(window)-1].Date != date {
		return 0, false
	}
	sum := 0.0
	for _, row := range window {
		sum += row.Close
	}
	return sum / 20, true
}

// onDayOpen 处理当日开盘:先执行昨日收盘触发的卖出,再执行昨日信号的买入。
func (sim *portfolioSimulator) onDayOpen(date int, dateText string, klines map[string][]FormulaKline, buys []portfolioPendingBuy) {
	// 1) 卖出(先卖释放现金)
	for _, pos := range sim.positions {
		if !pos.pendingExitSet {
			continue
		}
		row, ok := portfolioKlineOnDate(klines[pos.symbol], date)
		if !ok {
			continue // 停牌缺K线,顺延到下一交易日
		}
		if row.Open <= row.YClose*(1-portfolioPriceLimitRate(pos.symbol)) {
			continue // 跌停开盘卖不出,顺延
		}
		sim.executeSell(pos, row.Open, date, dateText)
	}

	// 2) 买入(按得分降序,先买高分)
	sorted := append([]portfolioPendingBuy{}, buys...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].score == sorted[j].score {
			return sorted[i].symbol < sorted[j].symbol
		}
		return sorted[i].score > sorted[j].score
	})
	for _, buy := range sorted {
		sim.tryBuy(buy, date, dateText, klines)
	}
}

func (sim *portfolioSimulator) tryBuy(buy portfolioPendingBuy, date int, dateText string, klines map[string][]FormulaKline) {
	if _, exists := sim.positions[buy.symbol]; exists {
		sim.appendSkip(buy, dateText, "已有持仓")
		return
	}
	if len(sim.positions) >= sim.cfg.MaxPositions {
		sim.appendSkip(buy, dateText, "持仓已满")
		return
	}
	row, ok := portfolioKlineOnDate(klines[buy.symbol], date)
	if !ok {
		sim.appendSkip(buy, dateText, "停牌或缺少K线")
		return
	}
	if row.Open <= 0 {
		sim.appendSkip(buy, dateText, "开盘价无效")
		return
	}
	if row.Open >= row.YClose*(1+portfolioPriceLimitRate(buy.symbol)) {
		sim.appendSkip(buy, dateText, "涨停无法买入")
		return
	}
	target := sim.equity() / float64(sim.cfg.MaxPositions)
	shares := int(target / row.Open / 100) * 100
	if shares < 100 {
		sim.appendSkip(buy, dateText, "资金不足一手")
		return
	}
	cost := float64(shares) * row.Open
	if cost > sim.cash {
		shares = int(sim.cash / row.Open / 100) * 100
		if shares < 100 {
			sim.appendSkip(buy, dateText, "现金不足")
			return
		}
		cost = float64(shares) * row.Open
	}
	sim.cash -= cost
	sim.positions[buy.symbol] = &portfolioSimPosition{
		symbol:       buy.symbol,
		shares:       shares,
		entryPrice:   row.Open,
		entryDate:    date,
		signalDate:   buy.signalDate,
		strategyName: buy.strategyName,
		lastClose:    row.Open,
		peakClose:    row.Open,
	}
}

func (sim *portfolioSimulator) executeSell(pos *portfolioSimPosition, price float64, date int, dateText string) {
	sim.cash += float64(pos.shares) * price
	pnl := (price - pos.entryPrice) * float64(pos.shares)
	pnlRate := 0.0
	if pos.entryPrice > 0 {
		pnlRate = (price - pos.entryPrice) / pos.entryPrice * 100
	}
	sim.trades = append(sim.trades, HistoricalBacktestTrade{
		Symbol:       pos.symbol,
		Status:       "closed",
		EntryDate:    historicalDateText(pos.entryDate),
		EntryPrice:   pos.entryPrice,
		Shares:       pos.shares,
		ExitDate:     dateText,
		ExitPrice:    price,
		Pnl:          pnl,
		PnlRate:      pnlRate,
		HoldDays:     pos.holdDays,
		StrategyName: pos.strategyName,
		SignalDate:   historicalDateText(pos.signalDate),
		Reason:       pos.pendingExit,
	})
	delete(sim.positions, pos.symbol)
}

func (sim *portfolioSimulator) appendSkip(buy portfolioPendingBuy, attemptDate, reason string) {
	sim.trades = append(sim.trades, HistoricalBacktestTrade{
		Symbol:       buy.symbol,
		Status:       "skipped",
		EntryDate:    attemptDate,
		StrategyName: buy.strategyName,
		SignalDate:   historicalDateText(buy.signalDate),
		Reason:       reason,
	})
}

// onDayClose 收盘结算:更新持仓估值与峰值,判定次日开盘执行的退出条件,记录最大回撤。
func (sim *portfolioSimulator) onDayClose(date int, klines map[string][]FormulaKline) {
	for _, pos := range sim.positions {
		row, ok := portfolioKlineOnDate(klines[pos.symbol], date)
		if !ok {
			continue // 停牌,沿用上次收盘
		}
		pos.lastClose = row.Close
		pos.holdDays++
		if row.Close > pos.peakClose {
			pos.peakClose = row.Close
		}
		if row.Close >= pos.entryPrice*1.12 {
			pos.stopActive = true
		}
		switch {
		case row.Close <= pos.entryPrice*0.92:
			pos.pendingExit, pos.pendingExitSet = "止损(收盘≤-8%)", true
		case pos.stopActive && row.Close <= pos.peakClose*0.90:
			pos.pendingExit, pos.pendingExitSet = "回撤止盈(高点回撤10%)", true
		default:
			if ma, ok := portfolioMA20(klines[pos.symbol], date); ok && row.Close < ma {
				pos.pendingExit, pos.pendingExitSet = "收盘跌破20日线", true
			} else if pos.holdDays >= 40 {
				pos.pendingExit, pos.pendingExitSet = "持有超过40个交易日", true
			}
		}
	}
	equity := sim.equity()
	if equity > sim.equityPeak {
		sim.equityPeak = equity
	}
	drawdown := 0.0
	if sim.equityPeak > 0 {
		drawdown = (sim.equityPeak - equity) / sim.equityPeak * 100
	}
	if drawdown > sim.maxDrawdown {
		sim.maxDrawdown = drawdown
	}
}

// finalize 回测结束时,把未平仓持仓转为 open 记录并排序返回全部交易。
func (sim *portfolioSimulator) finalize(runID string) []HistoricalBacktestTrade {
	for _, pos := range sim.positions {
		sim.trades = append(sim.trades, HistoricalBacktestTrade{
			Symbol:       pos.symbol,
			Status:       "open",
			EntryDate:    historicalDateText(pos.entryDate),
			EntryPrice:   pos.entryPrice,
			Shares:       pos.shares,
			HoldDays:     pos.holdDays,
			StrategyName: pos.strategyName,
			SignalDate:   historicalDateText(pos.signalDate),
		})
	}
	sort.SliceStable(sim.trades, func(i, j int) bool {
		left, right := sim.trades[i], sim.trades[j]
		if left.Status == "skipped" || right.Status == "skipped" {
			if left.Status == "skipped" && right.Status != "skipped" {
				return false
			}
			if right.Status == "skipped" && left.Status != "skipped" {
				return true
			}
			return left.SignalDate > right.SignalDate
		}
		return left.EntryDate > right.EntryDate
	})
	for i := range sim.trades {
		sim.trades[i].RunID = runID
	}
	return sim.trades
}

func (sim *portfolioSimulator) summary() map[string]interface{} {
	closedCount, openCount, skippedCount, winCount := 0, 0, 0, 0
	for _, trade := range sim.trades {
		switch trade.Status {
		case "closed":
			closedCount++
			if trade.Pnl > 0 {
				winCount++
			}
		case "open":
			openCount++
		case "skipped":
			skippedCount++
		}
	}
	winRate := 0.0
	if closedCount > 0 {
		winRate = float64(winCount) / float64(closedCount) * 100
	}
	finalEquity := sim.equity()
	totalReturn := 0.0
	if sim.cfg.InitialCash > 0 {
		totalReturn = (finalEquity - sim.cfg.InitialCash) / sim.cfg.InitialCash * 100
	}
	return map[string]interface{}{
		"initial_cash":  sim.cfg.InitialCash,
		"max_positions": sim.cfg.MaxPositions,
		"final_equity":  round2(finalEquity),
		"total_return":  round2(totalReturn),
		"max_drawdown":  round2(sim.maxDrawdown),
		"closed_count":  closedCount,
		"open_count":    openCount,
		"skipped_count": skippedCount,
		"win_count":     winCount,
		"win_rate":      round2(winRate),
	}
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// mergePendingBuy 把当日新信号合并进待买列表：同一股票多条策略信号只买一次，
// 保留最高得分并拼接触发策略名。
func mergePendingBuy(buys []portfolioPendingBuy, symbol string, score float64, signalDate int, strategyName string) []portfolioPendingBuy {
	for i := range buys {
		if buys[i].symbol != symbol {
			continue
		}
		if score > buys[i].score {
			buys[i].score = score
		}
		if !strings.Contains(buys[i].strategyName, strategyName) {
			if buys[i].strategyName != "" {
				buys[i].strategyName += "·"
			}
			buys[i].strategyName += strategyName
		}
		return buys
	}
	return append(buys, portfolioPendingBuy{symbol: symbol, score: score, signalDate: signalDate, strategyName: strategyName})
}
