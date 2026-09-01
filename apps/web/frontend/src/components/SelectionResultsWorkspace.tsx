import { Button, Card, Input, Select, Space, Table, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { HQChartPanel } from './HQChartPanel';
import { JsonPane } from './JsonPane';
import type { DailyReviewResponse, SelectionResult } from '../types';

const { Text } = Typography;

export function SelectionResultsWorkspace() {
  const [results, setResults] = useState<SelectionResult[]>([]);
  const [review, setReview] = useState<DailyReviewResponse>({});
  const [selected, setSelected] = useState<SelectionResult | null>(null);
  const [formulaID, setFormulaID] = useState('');
  const [symbol, setSymbol] = useState('');
  const [latest, setLatest] = useState('');
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: '200' });
      if (formulaID) params.set('formula_id', formulaID);
      if (symbol) params.set('symbol', symbol);
      if (latest) params.set('latest', latest);
      const [items, daily] = await Promise.all([
        apiFetch<SelectionResult[]>(`/api/selection-results?${params.toString()}`),
        apiFetch<DailyReviewResponse>('/api/daily-review?limit=200')
      ]);
      setResults(items || []);
      setReview(daily || {});
      setSelected((current) => (current ? items.find((item) => item.id === current.id) || items[0] || null : items[0] || null));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '选股结果加载失败');
    } finally {
      setLoading(false);
    }
  }, [formulaID, latest, symbol]);

  useEffect(() => {
    load();
  }, []);

  const active = selected || results[0] || null;
  const reviewItem = useMemo(() => review.items?.find((item) => item.result.id === active?.id), [active, review.items]);

  return (
    <div className="selection-layout">
      <Card className="work-card selection-list-card" title="选股结果中心" extra={<Button onClick={load}>刷新</Button>}>
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
          <Button type="primary" onClick={load}>查询</Button>
        </Space>
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
            { title: '最新值', dataIndex: 'latest' },
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
            {reviewItem ? <JsonPane value={reviewItem} /> : null}
          </>
        ) : (
          <Text type="secondary">暂无命中结果</Text>
        )}
      </Card>
    </div>
  );
}
