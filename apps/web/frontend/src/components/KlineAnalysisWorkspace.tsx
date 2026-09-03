import { Button, Card, Empty, Input, Select, Space, Tag, Typography, message } from 'antd';
import { CheckCircleOutlined, LineChartOutlined, ReloadOutlined, SearchOutlined, StarOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatAmount, formatPercent, formatPrice, formatSigned, normalizeSymbol, priceFromMilli } from '../lib/format';
import type { Quote } from '../types';
import { HQChartPanel } from './HQChartPanel';

const { Text } = Typography;

type Period = 'day' | 'week' | 'month' | 'minute5' | 'minute15' | 'minute30' | 'hour';
type IndicatorKey = 'ma' | 'ema' | 'boll' | 'macd' | 'kdj' | 'rsi' | 'obv';
type Signal = 'bullish' | 'bearish' | 'neutral' | 'oscillating';

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

export function KlineAnalysisWorkspace() {
  const [symbol, setSymbol] = useState('000001');
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
    void loadData('000001', 'day');
  }, []);

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
  const latestClose = priceFromMilli(quote?.K?.Close) || latestBar?.close || 0;
  const previousClose = priceFromMilli(quote?.K?.Last) || latestBar?.yclose || 0;
  const change = latestClose - previousClose;
  const changePercent = previousClose > 0 ? (change / previousClose) * 100 : 0;
  const signals = useMemo(() => indicatorSignal(indicators, history), [history, indicators]);
  const windows = useMemo(() => chartWindows(indicators), [indicators]);
  const priceTone = change > 0 ? 'market-up' : change < 0 ? 'market-down' : 'market-muted';

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
            <div><span>开盘</span><strong>{latestBar ? formatPrice(latestBar.open) : formatPrice(quote?.K?.Open ? priceFromMilli(quote.K.Open) : 0)}</strong></div>
            <div><span>最高</span><strong>{latestBar ? formatPrice(latestBar.high) : formatPrice(quote?.K?.High ? priceFromMilli(quote.K.High) : 0)}</strong></div>
            <div><span>最低</span><strong>{latestBar ? formatPrice(latestBar.low) : formatPrice(quote?.K?.Low ? priceFromMilli(quote.K.Low) : 0)}</strong></div>
            <div><span>成交量</span><strong>{latestBar ? formatAmount(latestBar.volume) : quote?.TotalHand ? formatAmount(quote.TotalHand * 100) : '--'}</strong></div>
            <div><span>成交额</span><strong>{latestBar ? formatAmount(latestBar.amount) : quote?.Amount ? formatAmount(quote.Amount) : '--'}</strong></div>
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
      </div>
    </div>
  );
}
