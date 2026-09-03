import { Button, Card, Input, InputNumber, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { HQChartPanel } from './HQChartPanel';
import { JsonPane } from './JsonPane';
import type { DailyReviewResponse, SelectionHorizon, SelectionResult, SelectionTrackingResponse } from '../types';

const { Text } = Typography;

export function SelectionResultsWorkspace() {
  const [results, setResults] = useState<SelectionResult[]>([]);
  const [review, setReview] = useState<DailyReviewResponse>({});
  const [selected, setSelected] = useState<SelectionResult | null>(null);
  const [formulaID, setFormulaID] = useState('');
  const [symbol, setSymbol] = useState('');
  const [latest, setLatest] = useState('');
  const [loading, setLoading] = useState(false);
  const [tracking, setTracking] = useState<SelectionTrackingResponse>({ items: [] });
  const [targetReturn, setTargetReturn] = useState(3);
  const [drawdownLimit, setDrawdownLimit] = useState(5);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: '200' });
      if (formulaID) params.set('formula_id', formulaID);
      if (symbol) params.set('symbol', symbol);
      if (latest) params.set('latest', latest);
      const trackingParams = new URLSearchParams(params);
      trackingParams.set('target_return', String(targetReturn));
      trackingParams.set('drawdown_limit', String(drawdownLimit));
      trackingParams.set('latest', latest || '0');
      const [items, daily, tracked] = await Promise.all([
        apiFetch<SelectionResult[]>(`/api/selection-results?${params.toString()}`),
        apiFetch<DailyReviewResponse>('/api/daily-review?limit=200'),
        apiFetch<SelectionTrackingResponse>(`/api/selection-results/tracking?${trackingParams.toString()}`)
      ]);
      setResults(items || []);
      setReview(daily || {});
      setTracking(tracked || { items: [] });
      setSelected((current) => (current ? items.find((item) => item.id === current.id) || items[0] || null : items[0] || null));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '选股结果加载失败');
    } finally {
      setLoading(false);
    }
  }, [drawdownLimit, formulaID, latest, symbol, targetReturn]);

  useEffect(() => {
    load();
  }, []);

  const active = selected || results[0] || null;
  const reviewItem = useMemo(() => review.items?.find((item) => item.result.id === active?.id), [active, review.items]);
  const trackingMap = useMemo(() => new Map((tracking.items || []).map((item) => [item.result.id, item.tracking])), [tracking.items]);
  const formatPct = (value?: number) => value === undefined ? '--' : `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
  const horizon = (item: SelectionResult | null, key: string): SelectionHorizon | undefined => item ? trackingMap.get(item.id)?.horizons?.[key] : undefined;
  const horizonLabel = (value?: SelectionHorizon) => !value || value.status !== 'complete' ? '待观察' : value.success ? '达标' : '未达标';
  const horizonColor = (value?: SelectionHorizon) => !value || value.status !== 'complete' ? 'default' : value.success ? 'success' : 'error';
  const summary = tracking.summary?.horizons || {};

  return (
    <div className="selection-layout">
      <Card className="work-card selection-list-card" title="选股结果中心" extra={<Button onClick={load}>刷新跟踪</Button>}>
        <Space wrap className="toolbar-row">
          <Input value={formulaID} onChange={(event) => setFormulaID(event.target.value)} placeholder="formula_id" />
          <Input value={symbol} onChange={(event) => setSymbol(event.target.value)} placeholder="按股票代码筛选" />
          <Select
            value={latest || undefined}
            onChange={setLatest}
            options={[
              { value: '', label: '全部结果' },
              { value: '1', label: '最近一次运行' }
            ]}
            style={{ width: 150 }}
          />
          <InputNumber min={0.1} max={100} step={0.5} value={targetReturn} onChange={(value) => setTargetReturn(value || 3)} addonBefore="目标收益" addonAfter="%" />
          <InputNumber min={0.1} max={100} step={0.5} value={drawdownLimit} onChange={(value) => setDrawdownLimit(value || 5)} addonBefore="最大回撤" addonAfter="%" />
          <Button type="primary" onClick={load}>查询</Button>
        </Space>
        <div className="selection-tracking-strip">
          <div><Text type="secondary">历史验证样本</Text><strong>{tracking.summary?.total ?? results.length}</strong></div>
          {[1, 5, 10].map((days) => {
            const item = summary[`d${days}`];
            return <div key={days}><Text type="secondary">D{days} 达标率</Text><strong>{item?.completed ? `${item.success_rate?.toFixed(2)}%` : '--'}</strong><span>{item?.completed ? `平均 ${formatPct(item.average_close_return)}` : '样本不足'}</span></div>;
          })}
        </div>
        <Table
          size="small"
          loading={loading}
          rowKey="id"
          dataSource={results}
          pagination={{ pageSize: 12, size: 'small' }}
          onRow={(record) => ({ onClick: () => setSelected(record) })}
          rowClassName={(record) => (record.id === active?.id ? 'row-active' : '')}
          columns={[
            { title: '代码', dataIndex: 'symbol' },
            { title: '公式', dataIndex: 'formula_name' },
            { title: '任务', dataIndex: 'task_name' },
            { title: '最新值', dataIndex: 'latest', render: (value: number) => value ? value.toFixed(2) : '--' },
            { title: 'D1', render: (_: unknown, record: SelectionResult) => { const value = horizon(record, 'd1'); return <Tag color={horizonColor(value)}>{horizonLabel(value)}{value?.status === 'complete' ? ` ${formatPct(value.close_return)}` : ''}</Tag>; } },
            { title: 'D5', render: (_: unknown, record: SelectionResult) => { const value = horizon(record, 'd5'); return <Tag color={horizonColor(value)}>{horizonLabel(value)}{value?.status === 'complete' ? ` ${formatPct(value.close_return)}` : ''}</Tag>; } },
            { title: 'D10', render: (_: unknown, record: SelectionResult) => { const value = horizon(record, 'd10'); return <Tag color={horizonColor(value)}>{horizonLabel(value)}{value?.status === 'complete' ? ` ${formatPct(value.close_return)}` : ''}</Tag>; } },
            { title: '时间', dataIndex: 'created_at' }
          ]}
        />
      </Card>

      <Card className="work-card selection-detail-card" title={active ? `${active.symbol} 命中详情` : '命中详情'}>
        {active ? (
          <>
            <Space wrap className="toolbar-row">
              <Button
                type="primary"
                onClick={async () => {
                  await navigator.clipboard.writeText(active.symbol);
                  message.success(`已复制 ${active.symbol}`);
                }}
              >
                复制代码
              </Button>
            </Space>
            <div className="detail-grid">
              <HQChartPanel symbol={active.symbol} period="day" count={260} pageSize={80} />
              <JsonPane value={JSON.parse(active.detail_json || '{}')} />
            </div>
            <Text type="secondary">
              {active.formula_name} · {active.task_name} · {active.created_at}
            </Text>
            <div className="selection-tracking-detail">
              <Text strong>历史前向表现</Text>
              <Text type="secondary">基于信号后真实交易日K线，非未来成功概率</Text>
              <div className="tracking-horizon-grid">
                {[1, 5, 10].map((days) => {
                  const value = horizon(active, `d${days}`);
                  return <div key={days}><Text type="secondary">D{days}</Text><Tag color={horizonColor(value)}>{horizonLabel(value)}</Tag><span>收盘 {formatPct(value?.close_return)}</span><span>最高 {formatPct(value?.max_gain)}</span><span>回撤 {formatPct(value?.max_drawdown)}</span></div>;
                })}
              </div>
            </div>
            {reviewItem ? <JsonPane value={reviewItem} /> : null}
          </>
        ) : (
          <Text type="secondary">暂无命中结果</Text>
        )}
      </Card>
    </div>
  );
}
