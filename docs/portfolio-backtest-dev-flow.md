# 组合级回测(Portfolio Backtest)开发流程(详细版)

> 状态:**已实施(落地位置有变更)**。本文档整理自已批准的《组合级回测设计方案》,描述开发流程、数据结构草案、算法细节与验收标准。
>
> ⚠️ **重要变更**:实际落地在 `HistoricalBacktestWorkspace.tsx` 历史回放页,而非 4.2 原规划的 `StrategiesWorkspace.tsx` 策略回测页——详见 0.4 实施记录。

## 0. 背景速览

### 0.1 现状(精确到代码)

| 能力 | 位置 | 现状 |
|---|---|---|
| 回测入口 | `apps/web/server_strategy.go` 271 行 `handleStrategyBacktest` | `POST /api/strategies/{id}/backtest`,30 分钟超时 |
| 引擎分发 | `apps/web/strategy_backtest.go` 96 行 `runStrategyBacktest` | engine=hikyuu 走参考实现,否则 Go 引擎 |
| Go 引擎 | 同文件 `backtestSymbol`(356 行) | **逐标的独立账户**:每只股票各配一份初始资金独立跑 |
| 聚合方式 | 同文件 `aggregateBacktestResults`(654 行) | 各标的曲线**加总**(totalInitial = initialCash × 标的数) |
| 信号计算 | 同文件 `backtestStrategySignal`(526 行) | 因子过滤 → 评分 → minScore,与实时选股逻辑一致 |
| 成交模型 | 同文件 476/438 行 | 买=信号次日开盘;卖=触发次日开盘;含 buy/sell cost |
| 退出规则 | 同文件 427-434 行 | switch 顺序:stop → trail → ma → time |
| 缺失 | — | 涨跌停、100 股整手、单账户资金池、月度收益 |

### 0.2 前端现状(精确到代码)

| 能力 | 位置 | 现状 |
|---|---|---|
| 回测按钮 | `apps/web/frontend/src/components/StrategiesWorkspace.tsx` 342-343 行 | "回测"(go)、"Hikyuu 校验"两个按钮 |
| 请求体 | 同文件 `runStrategy`(188 行) | 仅 `{ strategy_id: id, engine }`,**未传任何回测参数**(全用后端默认值) |
| 结果展示 | 同文件 396 行 | `<JsonPane value={runOutput} />`,原始 JSON 展示 |
| 类型定义 | `apps/web/frontend/src/types.ts` | **无** StrategyBacktest 类型,用 `apiFetch<unknown>` |
| 构建 | `apps/web/frontend/vite.config.ts` | `npm run build` → 输出 `../static-react`;dev 5173 代理 8080 |

### 0.3 目标与边界

- 目标:新增**组合模式(portfolio)**——单账户资金池按信号轮动,模拟涨停买不进、跌停卖不出、100 股整手、T+1 等 A 股约束,输出账户净值曲线、最大回撤与月度收益
- 边界:复用现有入口(默认 mode=symbol 行为完全不变);前端基于 React 版;不做 ST 差异化限幅、滑点模型、自动下单

### 0.4 实施记录(2026-09-14,与原文规划的关键差异)

> 实际开发受后续需求演进驱动,落地位置与原文 2/3/4 章的规划不同。**以本节为准**,后续章节中与本节冲突的描述均已加标注。

| 规划项 | 原文规划 | 实际落地 |
|---|---|---|
| 落地入口 | `StrategiesWorkspace.tsx` 策略回测(`/api/strategies/{id}/backtest`,mode=symbol/portfolio) | `HistoricalBacktestWorkspace.tsx` 历史选股回放(`/api/historical-backtests`),无需 mode 开关 |
| 需求动因 | 扩展策略回测为组合模式 | 用户需求演进:"保留策略信号列表,新开交易记录列表"(信号=候选,交易=账户实际买卖) |
| 撮合引擎位置 | apps/web 新增 strategy_backtest_portfolio.go | `services/selection-worker/portfolio_simulator.go`,挂入历史回放主循环(T 日信号 → T+1 开盘撮合) |
| 交易流水存储 | 无(原规划随回测结果返回) | workbench-core 新增 `historical_backtest_trades` 表 + `/api/historical-backtests/{id}/trades` 分页 API |
| 结果展示 | PortfolioResultCard:指标卡 + SVG 净值曲线 + 月度收益表 | 交易记录 Tab + 7 格绩效汇总条(期末权益/总收益/回撤/胜率等);SVG 净值曲线与月度收益表**未实施** |
| 信号列表 | 无提及 | 原样保留(信号明细 Tab),交易列表为新增视角 |
| 单标的策略回测 | mode=symbol 行为不变 | `StrategiesWorkspace` 策略回测**完全未动**,老功能无回归风险 |
| 单元测试(5.1) | apps/web 下 10 个 Given/When/Then 用例 | 已补核心用例:`services/selection-worker/portfolio_simulator_test.go`(合成 K 线,12 个用例:涨停/跌停/整手/T+1/退出优先级/等权/重复信号/持仓上限/限幅分板块/信号合并/现金降级/摘要统计),与本节日录表不完全一一对应 |

---

## 1. 阶段 0:前置验证 —— 数据复权核查(约 0.5 天)

**目的**:在写引擎前确认数据源质量。不复权价格在除权日会产生 >10% 的假暴跌,直接扭曲回撤与信号,一切回测结论作废。

### 1.1 验证步骤

1. 启动服务(`./start.sh`),确认 market-service 可用
2. 拉取高分红股日线:`GET /api/kline?code=600519&type=day`(或 market-service 对应接口),取近 3 年
3. 检查已知除权除息日(如贵州茅台 2022-06-30 每 10 股派 216.75 元、2023-06-30 派 259.11 元)前后:价格是否出现 >10% 的单日跳空
   - 前复权:除权日及之前的价格整体下移,**不会**出现单日跳空
   - 不复权:除权日会看到明显跳空(假暴跌)
4. 交叉验证:`packages/tdx-core` 的 `Gbbq.QFQ`(gbbq.go 205 行)对比同区间,确认数据源价格与 QFQ 结果同量级

### 1.2 判定与处理

- 通过 → 在下方"结论栏"记录,进入阶段 1
- 不通过 → **暂停本流程**,先解决数据源复权(优先在 market-service/Hikyuu 导入侧处理,不动回测代码)

> 阶段 0 结论:(待填写)

---

## 2. 阶段 1:后端组合引擎(约 2~3 天)

> ⚠️ **已实施,但位置变更**:撮合规则与退出优先级已在 `services/selection-worker/portfolio_simulator.go` 落地(挂入历史回放主循环),而非本节规划的 apps/web。本节数据结构草案仅作参照,实际结构为 `HistoricalBacktestTrade`(见 0.4)。

**文件**:`apps/web/strategy_backtest_portfolio.go`(新建)。现有文件零改动。

### 2.1 数据结构草案

```go
// 持仓
type portfolioPosition struct {
    Symbol         string
    Shares         int                   // 股数,始终为 100 的整数倍
    EntryPrice     float64
    EntryDate      int                   // 买入日 yyyymmdd
    HighSinceEntry float64               // 买入后最高价(移动止盈用)
    EntrySignal    StrategySelectionItem
    PendingSell    bool                  // 已触发退出,等待开盘成交
    SellReason     string                // stop/trail/ma/time/final
}

// 命中信号(Pass1 产物)
type portfolioSignal struct {
    Date   int                   // 信号日 yyyymmdd
    Symbol string
    Score  float64
    Item   StrategySelectionItem // 因子明细,复用现有结构
}

// 请求参数(组合模式专属,默认值在 runStrategyBacktest 填充后传入)
type portfolioBacktestParams struct {
    InitialCash   float64
    BuyCost       float64
    SellCost      float64
    StopLoss      float64
    ProfitTrigger float64
    TrailingStop  float64
    MaxHold       int
    ExitMA        int
    MaxPositions  int     // 同时持仓上限,默认 5
    TopN          int     // 每日最多买入新信号数,0=不限
    StartDate     int     // yyyymmdd
    EndDate       int
}

// 跳过统计
type portfolioSkippedStats struct {
    LimitUp int `json:"limit_up"` // 涨停买不进
    Cash    int `json:"cash"`     // 资金不足
    Suspend int `json:"suspend"`  // 买入日停牌无数据
}

// 月度收益
type portfolioMonthlyReturn struct {
    Month      string  `json:"month"`       // "2024-03"
    ReturnPct  float64 `json:"return_pct"`  // 月末净值/上月末净值-1
}
```

### 2.2 Pass 1:信号预计算

**函数**:`precomputePortfolioSignals(r *AutomationRunner, cfg StrategyConfig, symbols []string, klines map[string][]FormulaKline, startDate, endDate int) map[int]map[string]*portfolioSignal`

**流程**(与现有 `backtestSymbol` 的信号部分逐行同构,保证结果一致):

```go
for _, symbol := range symbols {
    rows := klines[symbol]
    // warmup、startIdx、endIdx 计算与 backtestSymbol 相同(backtestWarmupBars)
    signalRunner := &StrategyRunResult{
        Config: cfg,
        PoolCache: map[string]map[string]bool{},
        FormulaCache: map[string]map[string]bool{},
    }
    for i := warmupStart; i < endIdx; i++ {
        if rows[i].Date < startDate { continue }
        item, ok := r.backtestStrategySignal(signalRunner, cfg, symbol, rows[:i+1])
        if ok {
            signals[rows[i].Date][symbol] = &portfolioSignal{Date: rows[i].Date, Symbol: symbol, Score: item.Score, Item: item}
        }
    }
}
```

**要点**:
- 只存命中信号(minScore 已在 `backtestStrategySignal` 内判定),内存占用 = 命中信号数(远小于 80×520)
- 计算量与现有单标的回测同量级(现有已可在 30 分钟超时内完成)
- 必须复用 `backtestStrategySignal`,不得重写因子逻辑——阶段 4 的对照一致性测试依赖这一点
- 已知限制:`evaluateBacktestFactor` 不支持 market_momentum 等大盘因子(与现有回测一致),见附录 B

### 2.3 Pass 2:组合撮合引擎

**函数**:`simulatePortfolio(params portfolioBacktestParams, signals map[int]map[string]*portfolioSignal, klines map[string][]FormulaKline, dates []int) (*portfolioSimResult, error)`

**交易日历**:`dates` 为所有 symbol K 线日期的并集(升序),某 symbol 某日无 K 线 = 停牌/缺数据。

**主循环伪代码**:

```go
positions := map[string]*portfolioPosition{}
pendingBuys := []*portfolioSignal{}   // 等待次日开盘买入
pendingSells := []*portfolioPosition{} // 等待次日开盘卖出
cash := params.InitialCash
curve := []StrategyBacktestEquityPoint{}
trades := []StrategyBacktestTrade{}
skipped := portfolioSkippedStats{}

for idx, date := range dates {
    // —— 阶段 A:开盘撮合(先卖后买,卖出资金当日可再买)——
    // A1 卖出
    for _, pos := range pendingSells {
        row := klineRow(klines, pos.Symbol, date)
        if row == nil { continue } // 停牌,继续顺延
        openPrice := row.Open
        limitDown := limitPrice(row.YClose, -limitRate(pos.Symbol))
        if openPrice <= limitDown+eps { continue } // 跌停卖不出,顺延
        proceeds := float64(pos.Shares) * openPrice * (1 - params.SellCost)
        cash += proceeds
        trades = append(trades, ...) // reason=pos.SellReason
        delete(positions, pos.Symbol)
    }
    pendingSells = []*portfolioPosition{}
    // A2 买入(score 降序;TopN 限制每日数量)
    for _, sig := range orderedPendingBuys {
        if count(positions) >= params.MaxPositions { break }
        if topN 已达 { break }
        row := klineRow(klines, sig.Symbol, date)
        if row == nil { skipped.Suspend++; continue }
        if _, held := positions[sig.Symbol]; held { continue } // 已有持仓不重复买
        limitUp := limitPrice(row.YClose, +limitRate(sig.Symbol))
        if row.Open >= limitUp-eps { skipped.LimitUp++; continue } // 涨停买不进
        target := equity(cash, positions, klines, date) / float64(params.MaxPositions)
        shares := floor(target / row.Open / 100) * 100
        cost := float64(shares) * row.Open * (1 + params.BuyCost)
        if cost > cash { // 下调到可用现金
            shares = floor(cash / (row.Open * (1 + params.BuyCost)) / 100) * 100
            cost = float64(shares) * row.Open * (1 + params.BuyCost)
        }
        if shares < 100 { skipped.Cash++; continue }
        cash -= cost
        positions[sig.Symbol] = &portfolioPosition{...EntryDate: date...}
        entrySignal 记录
    }
    pendingBuys = []*portfolioSignal{}

    // —— 阶段 B:收盘评估(用当日收盘价判断退出;T+1:买入日跳过)——
    for _, pos := range positions {
        if pos.EntryDate == date { continue } // T+1
        row := klineRow(klines, pos.Symbol, date)
        if row == nil { continue } // 停牌,不评估
        if row.High > pos.HighSinceEntry { pos.HighSinceEntry = row.High }
        reason := exitReason(pos, row, params) // stop → trail → ma → time,与现有 switch 顺序一致
        if reason != "" { pos.PendingSell = true; pos.SellReason = reason; pendingSells = append(pendingSells, pos) }
    }

    // —— 阶段 C:收盘后接收当日新信号 → 进入明日买入队列 ——
    for _, sig := range signals[date] { 排序后 append 到 pendingBuys(score 降序) }

    // —— 阶段 D:收盘结算 ——
    equityValue := cash + Σ(pos.Shares × 最近收盘价)
    curve = append(curve, {Date: date, Equity: equityValue})
}

// 期末强平:最后一日所有持仓按最后收盘价卖出,reason="final"
```

**关键函数**:

```go
// 限幅:60/00 → 0.10;30/68 → 0.20;8/4 → 0.30(北交所)
func limitRate(symbol string) float64
// 涨停/跌停价 = round(昨收 × (1±rate), 2),四舍五入到分
func limitPrice(prevClose, rate float64) float64
// 退出判断,switch 顺序:stop → trail → ma → time(与 backtestSymbol 427-434 行一致)
func exitReason(pos *portfolioPosition, row FormulaKline, params portfolioBacktestParams) string
```

### 2.4 撮合规则边界情况(逐条明确)

| 场景 | 处理 | 理由 |
|---|---|---|
| 开盘价 ≥ 涨停价(一字板或高开涨停) | 放弃买入,`skipped.LimitUp++` | 实盘买不进;日K无法判断盘中是否开板,取保守 |
| 开盘价 ≤ 跌停价(卖出日) | 顺延到下一交易日,顺延期间**不再重复评估**其他退出条件 | 实盘卖不出;避免状态混乱 |
| 顺延至区间结束仍未卖出 | 期末按最后收盘价强平,reason="final" | 与现有实现一致 |
| 目标金额买不起一手 | 下调到可用现金计算,仍不足 100 股则跳过,`skipped.Cash++` | 资金约束 |
| 已持仓股票再次出信号 | 跳过(不重复买入) | 避免重复持仓 |
| 买入日停牌(无K线) | 放弃信号,`skipped.Suspend++` | 简单可解释,不做挂单顺延 |
| 信号日与买入日之间停牌 | 同上(买入日在信号次日) | — |
| 持仓股当日无K线(停牌) | 不评估退出、不卖出、估值沿用最近收盘价 | 停牌不可交易 |
| 新信号数超过空余仓位 | 按 score 降序,先买分数高的 | 优先级 |
| TopN > 0 | 每日最多买入 TopN 个新信号 | 控制换手节奏 |
| T+1 | `pos.EntryDate == date` 时跳过退出评估 | A股规则 |
| 卖出资金再买 | 阶段 A 先卖后买,现金即时可用 | A股:卖出资金当日可用于买入 |

### 2.5 指标算法

- `equity_curve`:每个交易日一个点(阶段 D)
- `metrics`:复用现有 `backtestCurveStats`(total_return/cagr/max_drawdown/exposure)与 `backtestTradeStats`(win_rate/profit_factor/avg_trade/avg_hold_days),initialCash 传单份本金
- `monthly_returns`:取每月最后一个交易日的 equity,`return_pct = 月末/上月末 - 1`(首月与初始资金比)
- `cash_utilization`:日均(持仓市值 / 总权益),度量资金使用效率
- `skipped_signals`:limit_up / cash / suspend 三类计数

### 2.6 验收

- `cd apps/web && go build ./...` 通过
- 阶段 4 的单元测试全部通过

---

## 3. 阶段 2:API 接入(约 0.5 天)

> ⚠️ **已实施,但接口变更**:实际为历史回放接口——启动请求体新增 `initial_cash`/`max_positions`,新增 `/api/historical-backtests/{id}/trades` 分页查询,结果 `portfolio` 摘要挂在 `result_json` 内。本节 mode 开关方案未实施(见 0.4)。

**文件**:`apps/web/strategy_backtest.go`(结构体扩展 + 分支)、`apps/web/server_strategy.go`(预期无需改动)

### 3.1 结构体扩展草案

```go
// StrategyBacktestRequest 增加三个字段(全 omitempty,向后兼容)
Mode         string `json:"mode,omitempty"`          // "symbol"(默认) | "portfolio"
MaxPositions int    `json:"max_positions,omitempty"` // 组合模式持仓上限,默认 5
TopN         int    `json:"top_n,omitempty"`         // 组合模式每日买入上限,0=不限

// StrategyBacktestResult 增加
Portfolio *PortfolioBacktestSummary `json:"portfolio,omitempty"` // 仅组合模式返回

type PortfolioBacktestSummary struct {
    MonthlyReturns   []PortfolioMonthlyReturn `json:"monthly_returns"`
    SkippedSignals   PortfolioSkippedStats    `json:"skipped_signals"`
    CashUtilization  float64                  `json:"cash_utilization"`
}
```

### 3.2 分支接入

`runStrategyBacktest` 在现有参数默认值填充(96-148 行)之后:

```go
if strings.EqualFold(strings.TrimSpace(req.Mode), "portfolio") {
    return r.runPortfolioBacktest(ctx, strategy, cfg, req)
}
```

组合模式复用同一套默认值(initial_cash=100000、buy=0.0005、sell=0.001、stop=0.08、profit=0.12、trail=0.10、max_hold=40、exit_ma=20),`MaxPositions <= 0` 时取 5。

### 3.3 请求/响应示例

请求:

```json
POST /api/strategies/{id}/backtest
{ "mode": "portfolio", "max_positions": 5, "top_n": 2,
  "start_date": "2023-01-01", "end_date": "2025-12-31", "initial_cash": 200000 }
```

响应(在现有结构上追加 portfolio 块,equity_curve/trades/metrics 语义不变):

```json
{
  "strategy": {...}, "config": {...}, "request": {...},
  "symbols": 62, "signals": 214, "trades": [...],
  "equity_curve": [{"date": 20230103, "equity": 200000.0}, ...],
  "metrics": {"total_return": 0.23, "cagr": 0.071, "max_drawdown": -0.11, ...},
  "portfolio": {
    "monthly_returns": [{"month": "2023-01", "return_pct": 0.021}, ...],
    "skipped_signals": {"limit_up": 9, "cash": 2, "suspend": 1},
    "cash_utilization": 0.62
  }
}
```

### 3.4 回归验收

- [ ] 不传 mode / mode=symbol:行为与改动前完全一致(逐标的加总)
- [ ] mode=portfolio:返回 equity_curve / metrics / portfolio 三块
- [ ] 参数错误(mode 传其他值):按 symbol 处理或返回明确错误(建议:未知 mode 报错,避免静默)

---

## 4. 阶段 3:前端 React(约 1~2 天)

**文件**:`apps/web/frontend/src/components/StrategiesWorkspace.tsx`、`apps/web/frontend/src/types.ts`、`apps/web/frontend/src/styles.css`(少量样式)

### 4.1 types.ts 新增

```ts
export interface StrategyBacktestRequest {
  strategy_id?: string;
  engine?: string;
  mode?: 'symbol' | 'portfolio';
  max_positions?: number;
  top_n?: number;
  // ...其余回测参数按后端字段补齐
}

export interface PortfolioMonthlyReturn { month: string; return_pct: number; }
export interface PortfolioSkippedStats { limit_up: number; cash: number; suspend: number; }
export interface PortfolioBacktestSummary {
  monthly_returns: PortfolioMonthlyReturn[];
  skipped_signals: PortfolioSkippedStats;
  cash_utilization: number;
}
export interface StrategyBacktestResult {
  // equity_curve / metrics / trades / portfolio ...
}
```

### 4.2 StrategiesWorkspace.tsx 改动点

> ⚠️ **未按本节实施**。实际落地在 `HistoricalBacktestWorkspace.tsx` 历史回放页:参数区新增初始资金/最大持仓,结果区新增"交易记录"Tab。见 0.4 实施记录。本节保留仅供对照。

1. **新增状态**(组件内 useState):
   - `backtestMode: 'symbol' | 'portfolio'`(默认 'symbol')
   - `maxPositions: number`(默认 5)
   - `backtestResult: StrategyBacktestResult | null`(替代现在的 unknown runOutput 中的回测部分,或并存)
2. **参数控件**:回测按钮旁(策略检查卡 action 区,342 行附近)加内联控件:组合模式 Switch + 最大持仓 InputNumber(仅组合模式显示)。样式类名沿用 `.strategy-*` 前缀
3. **runStrategy 扩展**:Go 回测请求体改为
   `{ strategy_id: id, engine, mode: backtestMode, max_positions: backtestMode === 'portfolio' ? maxPositions : undefined }`
   (hikyuu 校验按钮行为不变)
4. **结果区改造**(运行结果卡,396 行附近):
   - 单标的模式:维持 `<JsonPane value={runOutput} />` 不变
   - 组合模式:JsonPane 上方渲染 `<PortfolioResultCard result={...} />`,包含:
     - 指标卡行:总收益 / 年化 / 最大回撤 / 资金利用率 / 涨停跳过数(复用现有卡片样式)
     - 净值曲线:内联 SVG 折线组件 `EquityCurve`(见 4.3)
     - 月度收益表:antd Table,columns = 月份 / 收益%(红涨绿跌)
5. **JsonPane 仍保留**,供查看完整原始返回

### 4.3 SVG 净值曲线组件(零新依赖,约 50 行)

> ⚠️ **未实施**。绩效由 7 格汇总条呈现,净值曲线与月度收益表留待后续迭代。

`EquityCurve({ points, width = 640, height = 220 })`:

- 输入:`{date, equity}[]`
- 归一化:y 轴映射 equity min~max,留 8% 上下边距;x 轴按点数等分
- `<svg viewBox>` + `<polyline points="...">` 主线 + 渐变填充区域(可选)+ 首末端点标记
- 初始资金水平线(虚线)+ 图例"初始资金"
- 收益为负时曲线在基线下方,自然呈现
- 建议放新文件 `apps/web/frontend/src/components/EquityCurve.tsx`

### 4.4 构建与联调

```
cd apps/web/frontend
npm run build    # tsc --noEmit && vite build → ../static-react
npm run dev      # 5173,/api 代理 localhost:8080
```

### 4.5 验收

- [ ] `npm run build` 无 tsc 错误
- [ ] dev 联调:组合模式跑一次,看到指标卡 + 净值曲线 + 月度收益表;单标的模式界面无变化

---

## 5. 阶段 4:测试与端到端验证(约 1~2 天)

**文件**:`apps/web/strategy_backtest_portfolio_test.go`(新建)

### 5.1 单元测试用例(Given/When/Then)

> ✅ 核心用例已落地在 `services/selection-worker/portfolio_simulator_test.go`(合成 K 线,不依赖行情库),与本表不完全一一对应。`TestPortfolioConsistency`(组合与单标的逐位对照)未写,留给真实数据端到端验证。

| 用例 | Given | When | Then |
|---|---|---|---|
| `TestPortfolioLimitUpSkip` | 昨收 10.00(主板),信号命中;次日开盘 11.00(=涨停价) | 组合引擎运行买入 | 未买入;`skipped.limit_up=1` |
| `TestPortfolioLimitDownDelay` | 持仓触发止损,次日开盘 9.00(=跌停价),再下一日开盘 9.50 | 组合引擎运行 | 跌停日未卖出;次日照常卖出,reason=stop |
| `TestPortfolioRoundLot` | 目标金额 15000,开盘 10.50 | 计算股数 | shares = floor(15000/10.5/100)×100 = 1400 |
| `TestPortfolioInsufficientCash` | 现金只够买 50 股 | 组合引擎运行买入 | 跳过;`skipped.cash=1` |
| `TestPortfolioTPlusOne` | 昨日买入(entryDate=昨日),今日大跌触发止损价 | 退出评估 | 今日不产生卖出(买入日不可卖) |
| `TestPortfolioEqualWeight` | 同日两个信号,equity=100000,max_positions=2 | 次日买入 | 两笔目标金额各 ≈50000 |
| `TestPortfolioDuplicateSignal` | 已持仓股票再次出信号 | 组合引擎运行 | 不重复买入,持仓数不变 |
| `TestPortfolioExitPriority` | 同时满足 stop 与 ma 条件 | 退出评估 | reason=stop(switch 顺序优先) |
| `TestPortfolioConsistency`(对照) | 同一策略、同参数,单票场景 | 分别跑 portfolio 与 symbol 模式 | 该票单笔交易:买价/卖价/收益/持有天数/reason 完全一致 |
| `TestLimitPrice` | 昨收 10.05,主板 | 计算涨跌停价 | 涨停=11.06(round(11.055,2)),跌停=9.05(round(9.045,2)) |

> `TestPortfolioConsistency` 是最关键测试:组合引擎的信号计算与退出语义必须与现有引擎逐位一致,任何偏差都说明新引擎改变了语义。

### 5.2 端到端手工验证

1. 启动:`./start.sh`,确认 8080 与 market-service 正常
2. 构造请求:

```bash
curl -s -X POST http://localhost:8080/api/strategies/<id>/backtest \
  -H 'Content-Type: application/json' \
  -d '{"mode":"portfolio","max_positions":5,"top_n":2,"start_date":"2023-01-01","end_date":"2026-09-01","initial_cash":200000}'
```

3. 检查点:
   - [ ] 净值曲线无异常跳变(单日 ±30% 以上需解释:除权?数据错误?)
   - [ ] 月度收益求和 ≈ 总收益
   - [ ] 小盘策略应能看到 `skipped.limit_up > 0`(涨停买不进真实发生)
   - [ ] 对照:同策略 symbol 模式下单笔交易与 portfolio 模式中该票交易数据可对上
   - [ ] 最大回撤、年化等指标与手工估算同量级
4. 记录结果到下方"验证记录"栏

### 5.3 验收

- `cd apps/web && go test ./...` 全绿
- 手工验证记录完整

> 验证记录:(待填写)

---

## 6. 阶段 5:文档收尾(约 0.5 天)

- [ ] `docs/api-reference.md` 补充组合回测一节:新字段说明、请求/响应示例、模式语义
- [ ] 已知限制随文档记录:
  - ST 股 5% 限幅不区分(按 10% 处理,漏判)
  - 开盘价=涨停价但盘中开板的情况无法用日K判断(取保守:视为买不进)
  - 滑点用固定 buy/sell cost 系数近似
  - 停牌股按"不可交易"处理(买入日停牌则放弃信号)
  - market_momentum 等大盘因子不在回测支持列表

---

## 7. 开发纪律与回滚

- 每个阶段完成后独立提交(commit):
  - 阶段 1:`git add apps/web/strategy_backtest_portfolio.go`
  - 阶段 2:`git add apps/web/strategy_backtest.go apps/web/server_strategy.go`
  - 阶段 3:`git add apps/web/frontend/src`(构建产物 static-react 按现有惯例处理)
  - 阶段 4:`git add apps/web/strategy_backtest_portfolio_test.go`
- 回滚:任一阶段失败,`git checkout -- <该阶段文件>` 即恢复,不影响其他阶段
- 实施过程中本文档与设计方案如有出入,以**代码审查共识**为准,并在附录 B 记录

---

## 附录 A:关键代码引用表

| 引用 | 位置 | 用途 |
|---|---|---|
| `handleStrategyBacktest` | apps/web/server_strategy.go:271 | 回测入口 handler |
| `runStrategyBacktest` | apps/web/strategy_backtest.go:96 | 引擎分发 + 默认值填充 |
| `backtestSymbol` | apps/web/strategy_backtest.go:356 | 现有逐标的引擎(对照基准) |
| `backtestStrategySignal` | apps/web/strategy_backtest.go:526 | 信号计算(**组合模式必须复用**) |
| `evaluateBacktestFactor` | apps/web/strategy_backtest.go:559 | 回测因子评估 |
| 退出规则 switch | apps/web/strategy_backtest.go:427-434 | stop/trail/ma/time 顺序与阈值 |
| `aggregateBacktestResults` | apps/web/strategy_backtest.go:654 | 现有聚合(组合模式不沿用) |
| `backtestCurveStats` / `backtestTradeStats` | apps/web/strategy_backtest.go:692/739 | 指标计算(组合模式复用) |
| `backtestWarmupBars` | apps/web/strategy_backtest.go(附近) | warmup 计算 |
| `loadFormulaKline` | apps/web/formula_worker.go:186 | K线加载(market-service → TDX fallback) |
| `StrategyConfig` / `StrategyPassConfig` | apps/web/strategy_runner.go:13/35 | 策略配置(TopN/MinScore) |
| `Gbbq.QFQ` | packages/tdx-core/gbbq.go:205 | 阶段 0 复权交叉验证 |
| `runStrategy` | frontend/src/components/StrategiesWorkspace.tsx:188 | React 回测请求 |
| `JsonPane` | frontend/src/components/JsonPane.tsx | 现有结果展示组件 |
| vite 构建配置 | apps/web/frontend/vite.config.ts | outDir=../static-react |

## 附录 B:决策记录

**已定决策**:

| 决策 | 结论 | 理由 |
|---|---|---|
| 实现位置 | apps/web 新增 strategy_backtest_portfolio.go | ~~已变更~~:实际为 selection-worker/portfolio_simulator.go,挂入历史回放主循环(见 0.4 实施记录);复用信号/退出语义一致,历史回放定位为信号质量统计+账户模拟 |
| 落地入口变更 | — | 历史回放页(HistoricalBacktestWorkspace)而非策略回测(StrategiesWorkspace):用户需求"保留信号列表+新开交易列表",历史回放已有日期范围+策略选择参数,天然匹配组合回测语义 |
| 两遍法 | Pass1 信号预计算 + Pass2 撮合 | 避免日期主循环内重复计算因子,性能与现回测同量级 |
| 复用 backtestStrategySignal | 是 | 保证对照一致性 |
| 退出判断时机 | 收盘判断、次日开盘成交 | 与现有引擎一致 |
| 卖出资金再买 | 允许(先卖后买) | 符合 A 股资金规则 |
| 涨停判定 | 开盘价 ≥ 涨停价(round 到分)即放弃 | 保守;日K无法判断盘中开板 |
| 跌停顺延 | 顺延至可卖或区间结束(期末强平) | 不设顺延天数上限,简单可解释 |
| 买入日停牌 | 放弃信号,skipped.suspend++ | 不做挂单顺延,保持简单 |
| 已知 mode 之外的值 | 报错 | 避免静默降级 |

**待定问题(实施时确认)**:

1. 等权金额的权益基准时刻:建议"信号日收盘结算后的总权益 / max_positions"(阶段 D 之后、阶段 C 排队的下一日开盘使用)
2. 组合模式是否支持 hikyuu 引擎:第一版仅 Go 引擎;hikyuu 校验按钮保持原样
3. 组合模式是否复用 `top_n` 之外的策略 `Pass.TopN`:建议独立 `req.TopN`(回测参数)与策略配置解耦
4. equity_curve 是否包含起始日(初始资金点):建议在第一个交易日前插入 `{date: startDate-1, equity: initialCash}` 锚点,便于前端画基线

## 风险清单

| 风险 | 影响 | 应对 |
|---|---|---|
| 数据源不复权 | 回测结果整体失真 | 阶段 0 前置拦截 |
| ST 股限幅误判(5% 按 10%) | 少量漏判涨停 | 第一版接受,记录为已知限制 |
| 开盘涨停但盘中开板 | 保守放弃(少买) | 已知限制,后续可用分钟数据优化 |
| 信号预计算性能超时 | 30 分钟超时 | 与现有回测同量级;若超时,再优化缓存/并发 |
| 破坏现有单标的回测 | 老功能回归 | mode 默认值不变;阶段 2 回归验收 |
| 对照不一致 | 新引擎悄悄改变语义 | 阶段 4 `TestPortfolioConsistency` 兜底 |
| 期末强平价格失真 | 回测尾部收益虚高 | 期末强平与现有引擎一致(close 价),记录即可 |
