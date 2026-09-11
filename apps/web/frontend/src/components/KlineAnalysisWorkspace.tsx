import { Button, Card, Checkbox, Empty, Input, Select, Space, Tag, Typography, message } from 'antd';
import { CheckCircleOutlined, ClearOutlined, EditOutlined, ExperimentOutlined, LineChartOutlined, ReloadOutlined, SearchOutlined, SettingOutlined, StarOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatFormulaArgs, parseFormulaArgs } from '../lib/formula';
import {
  formatAmount,
  formatPercent,
  formatPrice,
  formatSigned,
  normalizeSymbol,
  priceFromMilli,
  quoteAmountYuan,
  quoteKline,
  quoteVolumeShares
} from '../lib/format';
import type { Formula, FormulaArg, FormulaRunResponse, Quote } from '../types';
import { FormulaManager } from './FormulaManager';
import { HQChartPanel } from './HQChartPanel';

const { Text } = Typography;

type Period = 'day' | 'week' | 'month' | 'minute5' | 'minute15' | 'minute30' | 'hour';
type IndicatorKey = 'ma' | 'ema' | 'boll' | 'macd' | 'kdj' | 'rsi' | 'obv';
type Signal = 'bullish' | 'bearish' | 'neutral' | 'oscillating';
type ApplyMode = 'overlay' | 'change' | 'new-window';

interface AppliedFormulaOperation {
  formulaID: string;
  formulaName: string;
  script: string;
  args: FormulaArg[];
  mode: ApplyMode;
  windowIndex: number;
  independentY: boolean;
  excludeY: boolean;
}

interface SearchResult {
  code: string;
  name: string;
  exchange?: string;
  type?: string;
}

interface HistoryBar {
  date: string;
  yclose: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  amount: number;
}

const periodOptions: Array<{ value: Period; label: string }> = [
  { value: 'minute5', label: '5 分' },
  { value: 'minute15', label: '15 分' },
  { value: 'minute30', label: '30 分' },
  { value: 'hour', label: '60 分' },
  { value: 'day', label: '日 K' },
  { value: 'week', label: '周 K' },
  { value: 'month', label: '月 K' }
];

const indicatorGroups: Array<{
  title: string;
  tone: string;
  items: Array<{ key: IndicatorKey; label: string; window: string; subPane?: boolean }>;
}> = [
  {
    title: '主图',
    tone: 'trend',
    items: [
      { key: 'ma', label: 'MA', window: 'MA' },
      { key: 'ema', label: 'EMA', window: 'EMA' },
      { key: 'boll', label: 'BOLL', window: 'BOLL' }
    ]
  },
  {
    title: '副图',
    tone: 'momentum',
    items: [
      { key: 'macd', label: 'MACD', window: 'MACD', subPane: true },
      { key: 'kdj', label: 'KDJ', window: 'KDJ', subPane: true },
      { key: 'rsi', label: 'RSI', window: 'RSI', subPane: true },
      { key: 'obv', label: 'OBV', window: 'OBV', subPane: true }
    ]
  }
];

const defaultIndicators: Record<IndicatorKey, boolean> = {
  ma: true,
  ema: false,
  boll: false,
  macd: true,
  kdj: false,
  rsi: false,
  obv: false
};

function finiteNumber(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function asPrice(value: unknown): number {
  const number = finiteNumber(value);
  return number > 100000 ? number / 1000 : number;
}

function dateLabel(value: unknown): string {
  const raw = String(value || '');
  if (/^\d{8}$/.test(raw)) return `${raw.slice(0, 4)}-${raw.slice(4, 6)}-${raw.slice(6, 8)}`;
  if (/^\d{6}$/.test(raw)) return `${raw.slice(0, 4)}-${raw.slice(4, 6)}-01`;
  return raw.slice(0, 10) || '--';
}

function parseHistory(payload: unknown): HistoryBar[] {
  const record = payload && typeof payload === 'object' ? payload as { data?: unknown; list?: unknown[] } : {};
  const rows = Array.isArray(record.data)
    ? record.data.map((row) => {
      const item = row as unknown[];
      return {
        date: dateLabel(item[0]),
        yclose: asPrice(item[1]),
        open: asPrice(item[2]),
        high: asPrice(item[3]),
        low: asPrice(item[4]),
        close: asPrice(item[5]),
        volume: finiteNumber(item[6]),
        amount: finiteNumber(item[7])
      };
    })
    : Array.isArray(record.list)
      ? record.list.map((row) => {
        const item = row as Record<string, unknown>;
        return {
          date: dateLabel(item.Time),
          yclose: priceFromMilli(item.Last),
          open: priceFromMilli(item.Open),
          high: priceFromMilli(item.High),
          low: priceFromMilli(item.Low),
          close: priceFromMilli(item.Close),
          volume: finiteNumber(item.Volume),
          amount: finiteNumber(item.Amount) / 1000
        };
      })
      : [];

  return rows
    .filter((bar) => bar.close > 0);
}

function sma(values: number[], period: number): number | null {
  if (values.length < period) return null;
  return values.slice(-period).reduce((sum, value) => sum + value, 0) / period;
}

function ema(values: number[], period: number): number | null {
  if (values.length < period) return null;
  let result = values.slice(0, period).reduce((sum, value) => sum + value, 0) / period;
  const alpha = 2 / (period + 1);
  for (let index = period; index < values.length; index += 1) {
    result = (values[index] - result) * alpha + result;
  }
  return result;
}

function indicatorSignal(indicators: Record<IndicatorKey, boolean>, bars: HistoryBar[]) {
  if (!bars.length) return [];
  const closes = bars.map((bar) => bar.close);
  const latest = closes[closes.length - 1];
  const previous = closes[closes.length - 2] || latest;
  const signals: Array<{ label: string; signal: Signal }> = [];

  if (indicators.ma) {
    const ma5 = sma(closes, 5);
    const ma20 = sma(closes, 20);
    signals.push({ label: 'MA', signal: ma5 == null || ma20 == null ? 'neutral' : ma5 > ma20 ? 'bullish' : ma5 < ma20 ? 'bearish' : 'neutral' });
  }
  if (indicators.ema) {
    const ema12 = ema(closes, 12);
    const ema26 = ema(closes, 26);
    signals.push({ label: 'EMA', signal: ema12 == null || ema26 == null ? 'neutral' : ema12 > ema26 ? 'bullish' : ema12 < ema26 ? 'bearish' : 'neutral' });
  }
  if (indicators.boll) {
    const middle = sma(closes, 20);
    signals.push({ label: 'BOLL', signal: middle == null ? 'neutral' : latest > middle ? 'bullish' : latest < middle ? 'bearish' : 'neutral' });
  }
  if (indicators.macd) {
    const fast = ema(closes, 12);
    const slow = ema(closes, 26);
    signals.push({ label: 'MACD', signal: fast == null || slow == null ? 'neutral' : fast > slow ? 'bullish' : 'bearish' });
  }
  if (indicators.kdj) {
    signals.push({ label: 'KDJ', signal: latest >= previous ? 'bullish' : 'bearish' });
  }
  if (indicators.rsi) {
    const change = latest - previous;
    signals.push({ label: 'RSI', signal: Math.abs(change) < latest * 0.005 ? 'neutral' : change > 0 ? 'bullish' : 'bearish' });
  }
  if (indicators.obv) {
    const latestBar = bars[bars.length - 1];
    const previousBar = bars[bars.length - 2];
    signals.push({ label: 'OBV', signal: !previousBar || latestBar.volume >= previousBar.volume ? 'bullish' : 'bearish' });
  }
  return signals;
}

function signalClass(signal: Signal) {
  return `kline-signal kline-signal--${signal}`;
}

function chartWindows(indicators: Record<IndicatorKey, boolean>) {
  const mainWindow = indicators.boll ? 'BOLL' : indicators.ema ? 'EMA' : 'MA';
  const subWindows = indicatorGroups[1].items
    .filter((item) => indicators[item.key])
    .map((item) => ({ Index: item.window }));
  return [{ Index: mainWindow }, { Index: 'VOL' }, ...subWindows];
}

function getChartWindowCount(chart: Record<string, unknown>, fallback: number) {
  const nested = chart.JSChartContainer as { Frame?: { SubFrame?: unknown[] } } | undefined;
  const direct = chart.Frame as { SubFrame?: unknown[] } | undefined;
  return nested?.Frame?.SubFrame?.length || direct?.SubFrame?.length || fallback;
}

function formulaErrorText(error: unknown) {
  if (typeof error === 'string') return error;
  if (error && typeof error === 'object') {
    const record = error as Record<string, unknown>;
    if (record.Description || record.message) return String(record.Description || record.message);
    try {
      return JSON.stringify(record);
    } catch {
      return String(error);
    }
  }
  return '未知错误';
}

function performFormulaOperation(chart: Record<string, unknown>, operation: AppliedFormulaOperation) {
  const indexInfo = {
    Name: operation.formulaName || '自定义公式',
    Script: operation.script,
    Args: operation.args,
    YAxis: { ExcludeValue: operation.excludeY }
  };

  if (operation.mode === 'change') {
    const changeScriptIndex = chart.ChangeScriptIndex as undefined | ((index: number, info: unknown) => void);
    if (!changeScriptIndex) throw new Error('当前 HQChart 版本不支持 ChangeScriptIndex');
    changeScriptIndex.call(chart, operation.windowIndex, indexInfo);
    return;
  }

  if (operation.mode === 'new-window') {
    const addScriptIndexWindow = chart.AddScriptIndexWindow as undefined | ((info: unknown, options: unknown) => void);
    if (!addScriptIndexWindow) throw new Error('当前 HQChart 版本不支持 AddScriptIndexWindow');
    addScriptIndexWindow.call(chart, indexInfo, { Draw: true });
    return;
  }

  const addOverlayIndex = chart.AddOverlayIndex as undefined | ((options: unknown) => void);
  if (!addOverlayIndex) throw new Error('当前 HQChart 版本不支持 AddOverlayIndex');
  addOverlayIndex.call(chart, {
    Script: indexInfo.Script,
    WindowIndex: operation.windowIndex,
    Name: indexInfo.Name,
    Args: indexInfo.Args,
    IsShareY: !operation.independentY,
    YAxis: operation.independentY ? undefined : indexInfo.YAxis
  });
}

export function KlineAnalysisWorkspace() {
  const chartContainerRef = useRef<HTMLDivElement | null>(null);
  const appliedFormulaOperationsRef = useRef<AppliedFormulaOperation[]>([]);
  const [symbol, setSymbol] = useState('sh000001');
  const [symbolName, setSymbolName] = useState('上证指数');
  const [searchValue, setSearchValue] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [period, setPeriod] = useState<Period>('day');
  const [history, setHistory] = useState<HistoryBar[]>([]);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [loading, setLoading] = useState(false);
  const [indicators, setIndicators] = useState(defaultIndicators);
  const [chartVersion, setChartVersion] = useState(0);
  const [hikyuuIndicator, setHikyuuIndicator] = useState<Record<string, unknown> | null>(null);
  const [hikyuuIndicatorLoading, setHikyuuIndicatorLoading] = useState(false);
  const [hikyuuIndicatorName, setHikyuuIndicatorName] = useState('macd');
  const [formulas, setFormulas] = useState<Formula[]>([]);
  const [selectedFormulaID, setSelectedFormulaID] = useState('');
  const [loadingFormulas, setLoadingFormulas] = useState(false);
  const [formulaTesting, setFormulaTesting] = useState(false);
  const [formulaApplying, setFormulaApplying] = useState(false);
  const [formulaStatus, setFormulaStatus] = useState('等待图表就绪');
  const [testOutput, setTestOutput] = useState('');
  const [formulaDrawerOpen, setFormulaDrawerOpen] = useState(false);
  const [editingFormula, setEditingFormula] = useState<Formula>();
  const [applyMode, setApplyMode] = useState<ApplyMode>('overlay');
  const [windowIndex, setWindowIndex] = useState(0);
  const [chartWindowCount, setChartWindowCount] = useState(2);
  const [independentY, setIndependentY] = useState(false);
  const [excludeY, setExcludeY] = useState(false);
  const [appliedFormulaCount, setAppliedFormulaCount] = useState(0);

  const selectedFormula = useMemo(
    () => formulas.find((item) => item.id === selectedFormulaID),
    [formulas, selectedFormulaID]
  );

  const loadFormulas = useCallback(async () => {
    setLoadingFormulas(true);
    try {
      const items = await apiFetch<Formula[]>('/api/formulas');
      setFormulas(items);
      setSelectedFormulaID((current) => items.some((item) => item.id === current) ? current : items[0]?.id || '');
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '公式列表加载失败');
    } finally {
      setLoadingFormulas(false);
    }
  }, []);

  const loadData = useCallback(async (nextSymbol = symbol, nextPeriod = period) => {
    const normalized = normalizeSymbol(nextSymbol);
    if (!normalized) {
      message.warning('请输入有效股票代码');
      return;
    }
    setLoading(true);
    try {
      const [historyResult, quoteResult] = await Promise.all([
        apiFetch<unknown>(`/api/kline-all/tdx?code=${encodeURIComponent(normalized)}&type=${encodeURIComponent(nextPeriod)}&limit=800`),
        apiFetch<Quote[]>(`/api/quote?code=${encodeURIComponent(normalized)}`)
      ]);
      setHistory(parseHistory(historyResult));
      setQuote(Array.isArray(quoteResult) ? quoteResult[0] || null : null);
      setSymbol(normalized);
      setChartVersion((value) => value + 1);
    } catch (error) {
      setHistory([]);
      setQuote(null);
      message.error(error instanceof Error ? error.message : 'K 线数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [period, symbol]);

  useEffect(() => {
    void loadData('sh000001', 'day');
  }, []);

  useEffect(() => {
    void loadFormulas();
  }, [loadFormulas]);

  useEffect(() => {
    const value = searchValue.trim();
    if (!value) {
      setSearchResults([]);
      return;
    }
    const timer = window.setTimeout(async () => {
      try {
        setSearchResults(await apiFetch<SearchResult[]>(`/api/search?keyword=${encodeURIComponent(value)}`));
      } catch {
        setSearchResults([]);
      }
    }, 240);
    return () => window.clearTimeout(timer);
  }, [searchValue]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') void loadData(symbol, period);
    }, 60000);
    return () => window.clearInterval(timer);
  }, [loadData, period, symbol]);

  async function calculateHikyuuIndicator(indicator: string) {
    setHikyuuIndicatorLoading(true);
    try {
      const result = await apiFetch<Record<string, unknown>>('/api/hikyuu/indicators', {
        method: 'POST',
        body: JSON.stringify({ code: symbol, type: period, indicator, limit: 800, recover: 'none' })
      });
      setHikyuuIndicator(result);
      message.success(`${indicator.toUpperCase()} 已由 Hikyuu 计算`);
    } catch (error) {
      message.warning(error instanceof Error ? error.message : 'Hikyuu 指标不可用');
    } finally {
      setHikyuuIndicatorLoading(false);
    }
  }

  const latestBar = history[history.length - 1];
  const quoteBar = quoteKline(quote);
  const latestClose = priceFromMilli(quoteBar?.Close) || latestBar?.close || 0;
  const previousClose = priceFromMilli(quoteBar?.Last) || latestBar?.yclose || 0;
  const change = latestClose - previousClose;
  const changePercent = previousClose > 0 ? (change / previousClose) * 100 : 0;
  const signals = useMemo(() => indicatorSignal(indicators, history), [history, indicators]);
  const windows = useMemo(() => chartWindows(indicators), [indicators]);
  const priceTone = change > 0 ? 'market-up' : change < 0 ? 'market-down' : 'market-muted';
  const formulaWindowOptions = useMemo(
    () => Array.from({ length: Math.max(2, chartWindowCount) }, (_, index) => ({
      value: index,
      label: index === 0 ? '主图' : `副图${index}`
    })),
    [chartWindowCount]
  );

  useEffect(() => {
    setWindowIndex((current) => Math.min(current, Math.max(0, chartWindowCount - 1)));
  }, [chartWindowCount]);

  const handleChartReady = useCallback((container: HTMLDivElement | null) => {
    chartContainerRef.current = container;
    if (!container) return;
    const chart = window.TDXHQChart?.getChart?.(container) as Record<string, unknown> | null;
    if (!chart) {
      setFormulaStatus('图表实例未就绪');
      return;
    }

    let latestScriptError = '';
    chart.ScriptErrorCallback = (error: unknown) => {
      const detail = formulaErrorText(error);
      latestScriptError = detail;
      setFormulaStatus(`公式执行失败：${detail}`);
      message.error(`公式执行失败：${detail}`);
    };

    try {
      appliedFormulaOperationsRef.current.forEach((operation) => {
        const availableWindows = getChartWindowCount(chart, 2);
        const resolvedOperation = operation.mode === 'new-window' ? operation : {
          ...operation,
          windowIndex: Math.min(operation.windowIndex, Math.max(0, availableWindows - 1))
        };
        performFormulaOperation(chart, resolvedOperation);
      });
      setChartWindowCount(getChartWindowCount(chart, 2));
      if (!latestScriptError) {
        setFormulaStatus(appliedFormulaOperationsRef.current.length
          ? `已恢复 ${appliedFormulaOperationsRef.current.length} 个公式操作`
          : '图表已就绪');
      }
    } catch (error) {
      setFormulaStatus(`公式恢复失败：${formulaErrorText(error)}`);
    }
  }, []);

  async function testFormula(formula = selectedFormula, showSuccess = true) {
    if (!formula) {
      message.warning('请先选择一个公式');
      return undefined;
    }
    setFormulaTesting(true);
    try {
      const data = await apiFetch<FormulaRunResponse>(`/api/formulas/${formula.id}/test`, {
        method: 'POST',
        body: JSON.stringify({
          symbol,
          period,
          calc_count: 500,
          out_count: 20
        })
      });
      setTestOutput(JSON.stringify(data, null, 2));
      if (showSuccess) message.success(`测试完成 · ${data.engine || 'engine'} ${data.tick_ms || 0}ms`);
      return data;
    } catch (error) {
      const text = error instanceof Error ? error.message : '公式测试失败';
      setTestOutput(text);
      message.error(text);
      return undefined;
    } finally {
      setFormulaTesting(false);
    }
  }

  async function applyFormula() {
    if (!selectedFormula) {
      message.warning('请先选择一个公式');
      return;
    }
    if (!selectedFormula.script.trim()) {
      message.warning('公式脚本为空，无法应用');
      return;
    }
    const operation: AppliedFormulaOperation = {
      formulaID: selectedFormula.id,
      formulaName: selectedFormula.name,
      script: selectedFormula.script,
      args: parseFormulaArgs(selectedFormula),
      mode: applyMode,
      windowIndex,
      independentY,
      excludeY
    };

    setFormulaApplying(true);
    try {
      const testResult = await testFormula(selectedFormula, false);
      if (!testResult) {
        setFormulaStatus('公式测试失败，未应用到图表');
        return;
      }
      const container = chartContainerRef.current;
      const chart = container ? window.TDXHQChart?.getChart?.(container) as Record<string, unknown> | null : null;
      if (!chart) {
        setChartVersion((value) => value + 1);
        message.warning('图表初始化中，请稍后再应用公式');
        return;
      }
      performFormulaOperation(chart, operation);
      appliedFormulaOperationsRef.current = [...appliedFormulaOperationsRef.current, operation];
      setAppliedFormulaCount(appliedFormulaOperationsRef.current.length);
      setChartWindowCount(getChartWindowCount(chart, windows.length));
      const modeLabel = applyMode === 'change' ? '切换窗口' : applyMode === 'new-window' ? '新建副图' : '叠加指标';
      setFormulaStatus(`已${modeLabel}：${selectedFormula.name}`);
      message.success(`已${modeLabel}：${selectedFormula.name}`);
    } catch (error) {
      const text = formulaErrorText(error);
      setFormulaStatus(`公式应用失败：${text}`);
      message.error(text);
    } finally {
      setFormulaApplying(false);
    }
  }

  function clearOverlay() {
    appliedFormulaOperationsRef.current = [];
    setAppliedFormulaCount(0);
    setTestOutput('');
    setFormulaStatus('未叠加公式');
    setChartWindowCount(windows.length);
    setWindowIndex(0);
    setChartVersion((value) => value + 1);
  }

  const selectResult = (result: SearchResult) => {
    setSymbolName(result.name);
    setSearchValue('');
    setSearchResults([]);
    void loadData(result.code, period);
  };

  return (
    <div className="kline-analysis-workspace">
      <Card className="work-card kline-analysis-header-card">
        <div className="kline-toolbar">
          <div className="kline-toolbar-heading">
            <span className="kline-toolbar-icon"><LineChartOutlined /></span>
            <div>
              <div className="kline-toolbar-title">K 线分析</div>
              <Text type="secondary">{symbolName || symbol} · {symbol}</Text>
            </div>
          </div>
          <div className="kline-symbol-search">
            <Input
              value={searchValue}
              prefix={<SearchOutlined />}
              placeholder="搜索股票名称或代码"
              allowClear
              onChange={(event) => setSearchValue(event.target.value)}
              onPressEnter={() => searchResults[0] && selectResult(searchResults[0])}
            />
            {searchResults.length ? (
              <div className="kline-search-results">
                {searchResults.slice(0, 8).map((result) => (
                  <button type="button" key={`${result.exchange}-${result.code}`} onClick={() => selectResult(result)}>
                    <strong>{result.name}</strong>
                    <span>{result.code} · {result.exchange?.toUpperCase() || 'A 股'}</span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          <div className="kline-period-switcher" aria-label="K 线周期">
            {periodOptions.map((item) => (
              <Button
                key={item.value}
                size="small"
                type={period === item.value ? 'primary' : 'default'}
                onClick={() => {
                  setPeriod(item.value);
                  void loadData(symbol, item.value);
                }}
              >
                {item.label}
              </Button>
            ))}
          </div>
          <Space className="kline-toolbar-actions">
            <Tag color="blue">HQChart</Tag>
            <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void loadData()}>
              刷新
            </Button>
            <Button icon={<StarOutlined />} onClick={() => message.info('关注入口沿用市场行情工作区')}>
              关注
            </Button>
          </Space>
        </div>
      </Card>

      <div className="kline-analysis-main">
        <Card className="work-card kline-indicator-card" title="技术指标">
          {indicatorGroups.map((group) => (
            <section className={`kline-indicator-group kline-indicator-group--${group.tone}`} key={group.title}>
              <div className="kline-indicator-group-title">{group.title}</div>
              <div className="kline-indicator-buttons">
                {group.items.map((item) => (
                  <Button
                    key={item.key}
                    size="small"
                    type={indicators[item.key] ? 'primary' : 'default'}
                    onClick={() => setIndicators((current) => ({ ...current, [item.key]: !current[item.key] }))}
                  >
                    {item.label}
                    {item.subPane ? <small>副图</small> : null}
                  </Button>
                ))}
              </div>
            </section>
          ))}
          <div className="kline-indicator-note">
            <span>图表由 HQChart 绘制；研究值可切换到 Hikyuu 计算并保留数据修订号。</span>
            <div className="kline-indicator-note-controls">
              <Select size="small" value={hikyuuIndicatorName} onChange={setHikyuuIndicatorName} options={['ma', 'ema', 'macd', 'boll', 'atr'].map((value) => ({ value, label: value.toUpperCase() }))} />
              <Button size="small" icon={<CheckCircleOutlined />} loading={hikyuuIndicatorLoading} onClick={() => void calculateHikyuuIndicator(hikyuuIndicatorName)}>Hikyuu 校验</Button>
            </div>
          </div>
        </Card>

        <Card className="work-card kline-chart-card">
          <div className="kline-chart-heading">
            <div>
              <span className="kline-chart-kicker">MARKET / ANALYSIS</span>
              <strong>{symbolName || symbol}</strong>
              <Text type="secondary">{symbol} · {periodOptions.find((item) => item.value === period)?.label}</Text>
            </div>
            <div className={`kline-chart-last ${priceTone}`}>
              <b>{latestClose > 0 ? formatPrice(latestClose) : '--'}</b>
              <span>{previousClose > 0 ? `${formatSigned(change)} · ${formatPercent(changePercent)}` : '等待行情'}</span>
            </div>
          </div>

          <div className="kline-metric-grid">
            <div><span>开盘</span><strong>{latestBar ? formatPrice(latestBar.open) : formatPrice(quoteBar?.Open ? priceFromMilli(quoteBar.Open) : 0)}</strong></div>
            <div><span>最高</span><strong>{latestBar ? formatPrice(latestBar.high) : formatPrice(quoteBar?.High ? priceFromMilli(quoteBar.High) : 0)}</strong></div>
            <div><span>最低</span><strong>{latestBar ? formatPrice(latestBar.low) : formatPrice(quoteBar?.Low ? priceFromMilli(quoteBar.Low) : 0)}</strong></div>
            <div><span>成交量</span><strong>{latestBar ? formatAmount(latestBar.volume) : formatAmount(quoteVolumeShares(quote))}</strong></div>
            <div><span>成交额</span><strong>{latestBar ? formatAmount(latestBar.amount) : formatAmount(quoteAmountYuan(quote))}</strong></div>
            <div><span>涨速</span><strong>{quote?.Rate != null ? formatSigned(quote.Rate, '%') : '--'}</strong></div>
          </div>

          {signals.length ? (
            <div className="kline-signal-summary">
              <div className="kline-signal-summary-head">
                <strong>指标信号</strong>
                <Text type="secondary">当前启用 {signals.length} 项</Text>
              </div>
              <div className="kline-signal-summary-tags">
                {signals.map((item) => <span className={signalClass(item.signal)} key={item.label}>{item.label}</span>)}
              </div>
            </div>
          ) : null}

          <div className="kline-analysis-chart-shell">
            {history.length ? (
              <HQChartPanel
                key={`${symbol}-${period}-${chartVersion}-${JSON.stringify(windows)}`}
                symbol={symbol}
                period={period}
                count={800}
                pageSize={80}
                windows={windows}
                className="kline-analysis-chart"
                onReady={handleChartReady}
              />
            ) : (
              <div className="kline-chart-empty">
                {loading ? <Text type="secondary">正在加载 K 线...</Text> : <Empty description="暂无 K 线数据" />}
              </div>
            )}
          </div>
          <div className="kline-analysis-footer">
            <Text type="secondary">
              {history.length ? `已加载 ${history.length} 根 · 每 60 秒刷新 · HQChart 统一交互` : '等待行情数据'}
            </Text>
            {quote?.Name ? <Text type="secondary">实时行情：{quote.Name}</Text> : null}
            {hikyuuIndicator ? <Text type="secondary">研究引擎：{String(hikyuuIndicator.meta && typeof hikyuuIndicator.meta === 'object' ? (hikyuuIndicator.meta as Record<string, unknown>).calculation_engine : '--')} · 修订 {String(hikyuuIndicator.meta && typeof hikyuuIndicator.meta === 'object' ? (hikyuuIndicator.meta as Record<string, unknown>).data_revision : '--')}</Text> : null}
          </div>
        </Card>

        <Card
          className="work-card kline-formula-card"
          title={
            <Space size={8}>
              <span>公式叠加</span>
              {appliedFormulaCount ? <Tag color="gold">{appliedFormulaCount}</Tag> : null}
            </Space>
          }
          extra={
            <Button
              type="text"
              size="small"
              icon={<SettingOutlined />}
              onClick={() => {
                setEditingFormula(undefined);
                setFormulaDrawerOpen(true);
              }}
            >
              管理
            </Button>
          }
        >
          <Space direction="vertical" size={12} className="kline-formula-panel">
            <Select
              value={selectedFormulaID || undefined}
              loading={loadingFormulas}
              placeholder="选择公式"
              options={formulas.map((formula) => ({ value: formula.id, label: formula.name }))}
              onChange={setSelectedFormulaID}
            />
            <Button
              block
              icon={<EditOutlined />}
              disabled={!selectedFormula}
              onClick={() => {
                setEditingFormula(selectedFormula);
                setFormulaDrawerOpen(true);
              }}
            >
              编辑所选
            </Button>
            <div className="kline-formula-selects">
              <Select
                value={applyMode}
                onChange={setApplyMode}
                options={[
                  { value: 'overlay', label: '叠加指标' },
                  { value: 'change', label: '切换当前窗口' },
                  { value: 'new-window', label: '新建副图' }
                ]}
              />
              <Select
                value={windowIndex}
                disabled={applyMode === 'new-window'}
                onChange={setWindowIndex}
                options={formulaWindowOptions}
              />
            </div>
            <Checkbox checked={independentY} onChange={(event) => setIndependentY(event.target.checked)}>
              使用独立坐标
            </Checkbox>
            <Checkbox checked={excludeY} onChange={(event) => setExcludeY(event.target.checked)}>
              不参与 Y 轴计算
            </Checkbox>
            <div className="kline-formula-meta">
              <span>参数</span>
              <code>{formatFormulaArgs(selectedFormula)}</code>
            </div>
            <div className="kline-formula-actions">
              <Button icon={<ExperimentOutlined />} loading={formulaTesting} onClick={() => void testFormula()}>
                测试
              </Button>
              <Button type="primary" icon={<LineChartOutlined />} loading={formulaApplying} onClick={() => void applyFormula()}>
                应用
              </Button>
              <Button icon={<ClearOutlined />} disabled={!appliedFormulaCount} onClick={clearOverlay}>
                清除
              </Button>
            </div>
            <div className="kline-formula-status" aria-live="polite">{formulaStatus}</div>
            <pre className="kline-formula-output">{testOutput || '测试结果'}</pre>
          </Space>
        </Card>
      </div>

      <FormulaManager
        open={formulaDrawerOpen}
        formulas={formulas}
        loading={loadingFormulas}
        editingFormula={editingFormula}
        onClose={() => setFormulaDrawerOpen(false)}
        onEdit={setEditingFormula}
        onReload={loadFormulas}
      />
    </div>
  );
}
