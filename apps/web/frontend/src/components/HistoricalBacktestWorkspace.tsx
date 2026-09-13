import {
  Alert, Button, Card, DatePicker, Empty, Input, InputNumber, Progress, Segmented, Select, Space, Table, Tag, Typography, message
} from 'antd';
import {
  BarChartOutlined, ClockCircleOutlined, ExperimentOutlined, PlayCircleOutlined, ReloadOutlined, StopOutlined, TeamOutlined
} from '@ant-design/icons';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { apiFetch } from '../lib/api';
import { normalizeSymbol } from '../lib/format';
import type {
  HistoricalBacktestResult, HistoricalBacktestRun, HistoricalBacktestSignal, HistoricalBacktestSignalPage,
  HistoricalConsensusSummary, HistoricalHorizonSummary, HistoricalStrategySummary, SelectionTracking, Strategy
} from '../types';
import { StockQuoteModal, type StockQuoteTarget } from './StockQuoteModal';

const { Text } = Typography;
type ResultView = 'ranking' | 'signals' | 'consensus';
interface StockDirectoryItem { code?: string; name?: string; }
interface StockDirectory { codes?: StockDirectoryItem[]; }

function parseJSON<T>(value?: string, fallback?: T): T {
  try { return JSON.parse(value || '') as T; } catch { return fallback as T; }
}

function statusMeta(status?: string) {
  if (status === 'success') return { label: '已完成', color: 'success' } as const;
  if (status === 'failed') return { label: '失败', color: 'error' } as const;
  if (status === 'cancelled') return { label: '已取消', color: 'default' } as const;
  return { label: '执行中', color: 'processing' } as const;
}

function pct(value?: number) {
  if (!Number.isFinite(value)) return '--';
  const number = Number(value);
  return `${number >= 0 ? '+' : ''}${number.toFixed(2)}%`;
}

function horizonMetric(raw: string | undefined, horizon: number) {
  return parseJSON<SelectionTracking>(raw, { horizons: {} }).horizons?.[`d${horizon}`];
}

function horizonCell(raw: string | undefined, horizon: number) {
  const value = horizonMetric(raw, horizon);
  if (!value || value.status !== 'complete') return <Tag>待观察</Tag>;
  return <Tag color={value.success ? 'success' : 'error'}>{pct(value.close_return)}</Tag>;
}

function compareNullableNumber(left?: number, right?: number) {
  const leftValid = Number.isFinite(left);
  const rightValid = Number.isFinite(right);
  if (!leftValid && !rightValid) return 0;
  if (!leftValid) return 1;
  if (!rightValid) return -1;
  return Number(left) - Number(right);
}

function horizonReturn(raw: string | undefined, horizon: number) {
  const value = horizonMetric(raw, horizon);
  return value?.status === 'complete' && Number.isFinite(value.close_return) ? value.close_return : undefined;
}

export function HistoricalBacktestWorkspace() {
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [runs, setRuns] = useState<HistoricalBacktestRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<HistoricalBacktestRun | null>(null);
  const [startDate, setStartDate] = useState<Dayjs | null>(dayjs().startOf('year'));
  const [endDate, setEndDate] = useState<Dayjs | null>(dayjs());
  const [strategyIDs, setStrategyIDs] = useState<string[]>([]);
  const [targetReturn, setTargetReturn] = useState(3);
  const [drawdownLimit, setDrawdownLimit] = useState(5);
  const [view, setView] = useState<ResultView>('ranking');
  const [signals, setSignals] = useState<HistoricalBacktestSignalPage>({ items: [], total: 0, limit: 100, offset: 0 });
  const [signalStrategy, setSignalStrategy] = useState('all');
  const [signalKeyword, setSignalKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [starting, setStarting] = useState(false);
  const [stockNames, setStockNames] = useState<Record<string, string>>({});
  const [quoteTarget, setQuoteTarget] = useState<StockQuoteTarget | null>(null);
  const initializedStrategies = useRef(false);

  const loadRuns = useCallback(async (preferID?: string) => {
    const response = await apiFetch<{ items: HistoricalBacktestRun[] }>('/api/historical-backtests?limit=30');
    const items = response.items || [];
    setRuns(items);
    setSelectedRun((current) => {
      const targetID = preferID || current?.id;
      return (targetID ? items.find((item) => item.id === targetID) : null) || items[0] || null;
    });
    return items;
  }, []);

  const loadInitial = useCallback(async () => {
    setLoading(true);
    try {
      const [strategyList, directory] = await Promise.all([
        apiFetch<Strategy[]>('/api/strategies'),
        apiFetch<StockDirectory>('/api/codes').catch(() => ({ codes: [] }))
      ]);
      const available = (strategyList || []).filter((item) => item.enabled);
      setStrategies(available);
      if (!initializedStrategies.current) {
        const systemIDs = available.filter((item) => item.readonly).map((item) => item.id);
        setStrategyIDs(systemIDs.length ? systemIDs : available.map((item) => item.id));
        initializedStrategies.current = true;
      }
      setStockNames(Object.fromEntries((directory.codes || []).flatMap((item) => {
        const code = normalizeSymbol(item.code);
        return code && item.name?.trim() ? [[code, item.name.trim()]] : [];
      })));
      await loadRuns();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '历史回测数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [loadRuns]);

  useEffect(() => { void loadInitial(); }, [loadInitial]);

  useEffect(() => {
    if (!runs.some((item) => item.status === 'running')) return;
    const timer = window.setInterval(() => { void loadRuns(); }, 2500);
    return () => window.clearInterval(timer);
  }, [loadRuns, runs]);

  const loadSignals = useCallback(async (run: HistoricalBacktestRun, nextPage = page) => {
    const params = new URLSearchParams({ limit: '100', offset: String((nextPage - 1) * 100) });
    if (signalStrategy !== 'all') params.set('strategy_id', signalStrategy);
    if (signalKeyword.trim()) params.set('symbol', normalizeSymbol(signalKeyword));
    try {
      setSignals(await apiFetch<HistoricalBacktestSignalPage>(`/api/historical-backtests/${run.id}/signals?${params}`));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '历史信号加载失败');
    }
  }, [page, signalKeyword, signalStrategy]);

  useEffect(() => {
    if (selectedRun && view === 'signals') void loadSignals(selectedRun);
  }, [loadSignals, selectedRun, view]);

  async function startBacktest() {
    if (!startDate || !endDate) return message.warning('请选择开始和结束日期');
    if (!strategyIDs.length) return message.warning('至少选择一个策略');
    setStarting(true);
    try {
      const run = await apiFetch<HistoricalBacktestRun>('/api/historical-backtests', {
        method: 'POST',
        body: JSON.stringify({
          start_date: startDate.format('YYYY-MM-DD'), end_date: endDate.format('YYYY-MM-DD'), strategy_ids: strategyIDs,
          horizons: [3, 5, 10], target_return: targetReturn, drawdown_limit: drawdownLimit
        })
      });
      await loadRuns(run.id);
      setView('ranking');
      message.success('历史回测已开始');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '历史回测启动失败');
    } finally {
      setStarting(false);
    }
  }

  async function cancelRun() {
    if (!selectedRun) return;
    try {
      await apiFetch(`/api/historical-backtests/${selectedRun.id}/cancel`, { method: 'POST' });
      await loadRuns(selectedRun.id);
      message.info('已提交取消请求');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '取消失败');
    }
  }

  function stockName(symbol: string) { return stockNames[normalizeSymbol(symbol)] || ''; }
  function openQuote(symbol: string) { setQuoteTarget({ code: symbol, name: stockName(symbol) || undefined }); }

  const result = useMemo(() => parseJSON<HistoricalBacktestResult>(selectedRun?.result_json, {}), [selectedRun?.result_json]);
  const summaries = result.strategy_summaries || [];
  const consensus = result.consensus || [];
  const progress = selectedRun?.total_dates ? Math.round(selectedRun.processed_dates * 100 / selectedRun.total_dates) : 0;
  const runStrategyOptions = strategies.filter((item) => selectedRun?.strategy_ids.includes(item.id)).map((item) => ({ value: item.id, label: item.name }));

  const rankingColumns = [
    { title: '策略', dataIndex: 'strategy_name', width: 190, render: (value: string) => <strong>{value}</strong> },
    { title: '信号', dataIndex: 'signal_count', width: 82 },
    ...[3, 5, 10].flatMap((days) => [
      { title: `D${days}成功率`, key: `d${days}-rate`, width: 105, sorter: (a: HistoricalStrategySummary, b: HistoricalStrategySummary) => (a.horizons[`d${days}`]?.success_rate || 0) - (b.horizons[`d${days}`]?.success_rate || 0), render: (_: unknown, row: HistoricalStrategySummary) => <strong>{(row.horizons[`d${days}`]?.success_rate || 0).toFixed(1)}%</strong> },
      { title: `D${days}均收`, key: `d${days}-return`, width: 95, render: (_: unknown, row: HistoricalStrategySummary) => pct(row.horizons[`d${days}`]?.average_return) }
    ]),
    { title: 'D10回撤', key: 'drawdown', width: 100, render: (_: unknown, row: HistoricalStrategySummary) => pct(row.horizons.d10?.average_max_drawdown) }
  ];

  const signalColumns = [
    { title: '信号日', dataIndex: 'signal_date', width: 108 },
    { title: '代码', dataIndex: 'symbol', width: 92, render: (value: string) => <Button type="link" className="code-link" onClick={() => openQuote(value)}>{value}</Button> },
    { title: '名称', dataIndex: 'symbol', width: 118, render: (value: string) => <strong>{stockName(value) || '--'}</strong> },
    { title: '策略', dataIndex: 'strategy_name', width: 170 },
    { title: '信号价', dataIndex: 'latest', width: 88, render: (value: number) => value?.toFixed(2) || '--' },
    { title: '得分', dataIndex: 'score', width: 72 },
    ...[3, 5, 10].map((days) => ({ title: `D${days}`, key: `d${days}`, width: 92, render: (_: unknown, row: HistoricalBacktestSignal) => horizonCell(row.tracking_json, days) }))
  ];

  const consensusColumns = [
    { title: '信号日', dataIndex: 'signal_date', width: 108, sorter: (left: HistoricalConsensusSummary, right: HistoricalConsensusSummary) => left.signal_date.localeCompare(right.signal_date) },
    { title: '代码', dataIndex: 'symbol', width: 92, render: (value: string) => <Button type="link" className="code-link" onClick={() => openQuote(value)}>{value}</Button> },
    { title: '名称', dataIndex: 'symbol', width: 118, render: (value: string) => <strong>{stockName(value) || '--'}</strong> },
    { title: '共振', dataIndex: 'strategy_count', width: 78, sorter: (left: HistoricalConsensusSummary, right: HistoricalConsensusSummary) => left.strategy_count - right.strategy_count, render: (value: number) => <Tag color="blue">{value} 策略</Tag> },
    { title: '命中策略', dataIndex: 'strategies', width: 310, render: (value: string[]) => value.join(' · ') },
    ...[3, 5, 10].map((days) => ({
      title: `D${days}`,
      key: `d${days}`,
      width: 92,
      sorter: (left: HistoricalConsensusSummary, right: HistoricalConsensusSummary) => compareNullableNumber(horizonReturn(left.tracking_json, days), horizonReturn(right.tracking_json, days)),
      render: (_: unknown, row: HistoricalConsensusSummary) => horizonCell(row.tracking_json, days)
    }))
  ];

  return (
    <div className="historical-backtest-workspace">
      <Card className="work-card historical-backtest-launch" title={<div className="historical-backtest-title"><div><span>历史选股回放</span><Text type="secondary">按交易日重放策略并验证后续表现</Text></div><small>POINT-IN-TIME REPLAY</small></div>} extra={<Button aria-label="刷新历史回测" icon={<ReloadOutlined />} loading={loading} onClick={() => void loadInitial()} />}>
        <div className="historical-backtest-form">
          <label><span>开始日期</span><DatePicker value={startDate} onChange={setStartDate} allowClear={false} /></label>
          <label><span>结束日期</span><DatePicker value={endDate} onChange={setEndDate} allowClear={false} disabledDate={(date) => Boolean(startDate && date.isBefore(startDate, 'day'))} /></label>
          <label className="historical-strategy-picker"><span>执行策略</span><Select mode="multiple" maxTagCount="responsive" value={strategyIDs} onChange={setStrategyIDs} options={strategies.map((item) => ({ value: item.id, label: item.name }))} /></label>
          <label><span>目标收益</span><InputNumber min={0.1} max={100} step={0.5} addonAfter="%" value={targetReturn} onChange={(value) => setTargetReturn(value || 3)} /></label>
          <label><span>回撤上限</span><InputNumber min={0.1} max={100} step={0.5} addonAfter="%" value={drawdownLimit} onChange={(value) => setDrawdownLimit(value || 5)} /></label>
          <Button className="historical-start-button" type="primary" icon={<PlayCircleOutlined />} loading={starting} disabled={runs.some((item) => item.status === 'running')} onClick={() => void startBacktest()}>开始回测</Button>
        </div>
      </Card>

      <div className="historical-backtest-main">
        <Card className="work-card historical-run-list" title={<div className="historical-backtest-title"><div><span>回测批次</span><Text type="secondary">{runs.length} 条记录</Text></div><small>RUNS</small></div>}>
          {runs.length ? <div className="historical-run-stack">{runs.map((run) => { const meta = statusMeta(run.status); return <button type="button" key={run.id} className={run.id === selectedRun?.id ? 'is-active' : ''} onClick={() => { setSelectedRun(run); setPage(1); }}><span>{run.start_date} → {run.end_date}</span><strong>{run.strategy_ids.length} 个策略 · {run.signal_count} 条信号</strong><small><Tag color={meta.color}>{meta.label}</Tag>{run.processed_dates}/{run.total_dates || '--'} 交易日</small></button>; })}</div> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无历史回测" />}
        </Card>

        <Card className="work-card historical-result-card" title={<div className="historical-backtest-title"><div><span>回测结果</span><Text type="secondary">{selectedRun ? `${selectedRun.start_date} 至 ${selectedRun.end_date}` : '等待任务'}</Text></div><small>RESULT</small></div>} extra={selectedRun?.status === 'running' ? <Button danger icon={<StopOutlined />} onClick={() => void cancelRun()}>取消</Button> : null}>
          {!selectedRun ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="创建回测后查看结果" /> : <>
            <div className="historical-progress-head">
              <div><ClockCircleOutlined /><span>当前交易日</span><strong>{selectedRun.current_date || '正在加载K线'}</strong></div>
              <Progress percent={selectedRun.status === 'success' ? 100 : progress} status={selectedRun.status === 'failed' ? 'exception' : selectedRun.status === 'success' ? 'success' : 'active'} />
              <Tag color={statusMeta(selectedRun.status).color}>{statusMeta(selectedRun.status).label}</Tag>
            </div>
            <div className="historical-facts">
              <div><span>交易日</span><strong>{selectedRun.processed_dates}/{selectedRun.total_dates || '--'}</strong></div>
              <div><span>候选股票</span><strong>{selectedRun.candidate_symbols || '--'}</strong></div>
              <div><span>历史信号</span><strong>{selectedRun.signal_count}</strong></div>
              <div><span>成功标准</span><strong>收益 ≥ {selectedRun.target_return}%</strong><small>回撤不低于 -{selectedRun.drawdown_limit}%</small></div>
            </div>
            {selectedRun.error ? <Alert type="error" showIcon message={selectedRun.error} /> : null}
            <div className="historical-result-toolbar">
              <Segmented value={view} onChange={(value) => setView(value as ResultView)} options={[
                { value: 'ranking', label: '策略排行', icon: <BarChartOutlined /> },
                { value: 'signals', label: '信号明细', icon: <ExperimentOutlined /> },
                { value: 'consensus', label: '多策略共振', icon: <TeamOutlined /> }
              ]} />
              {view === 'signals' ? <Space wrap><Select value={signalStrategy} onChange={(value) => { setSignalStrategy(value); setPage(1); }} options={[{ value: 'all', label: '全部策略' }, ...runStrategyOptions]} /><Input allowClear placeholder="股票代码" value={signalKeyword} onChange={(event) => { setSignalKeyword(event.target.value); setPage(1); }} /></Space> : null}
            </div>
            {view === 'ranking' ? <Table<HistoricalStrategySummary> size="small" rowKey="strategy_id" dataSource={summaries} columns={rankingColumns} pagination={false} scroll={{ x: 1050 }} locale={{ emptyText: selectedRun.status === 'running' ? '完成后生成策略统计' : '暂无策略统计' }} /> : null}
            {view === 'signals' ? <Table<HistoricalBacktestSignal> size="small" rowKey="id" dataSource={signals.items} columns={signalColumns} scroll={{ x: 1000 }} pagination={{ current: page, pageSize: 100, total: signals.total, showSizeChanger: false, onChange: (next) => setPage(next) }} locale={{ emptyText: '暂无历史信号' }} /> : null}
            {view === 'consensus' ? <Table<HistoricalConsensusSummary> size="small" rowKey={(row) => `${row.signal_date}-${row.symbol}`} dataSource={consensus} columns={consensusColumns} sortDirections={['ascend', 'descend']} pagination={{ pageSize: 50, showSizeChanger: false }} scroll={{ x: 1050 }} locale={{ emptyText: selectedRun.status === 'running' ? '完成后生成共振统计' : '暂无多策略共振信号' }} /> : null}
          </>}
        </Card>
      </div>
      <StockQuoteModal target={quoteTarget} onClose={() => setQuoteTarget(null)} />
    </div>
  );
}
