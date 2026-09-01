import { Button, Card, Table, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { HQChartPanel } from './HQChartPanel';
import { JsonPane } from './JsonPane';
import type { DailyReviewResponse } from '../types';

const { Text } = Typography;

export function DailyReviewWorkspace() {
  const [review, setReview] = useState<DailyReviewResponse>({});
  const [selectedSymbol, setSelectedSymbol] = useState('');
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setReview(await apiFetch<DailyReviewResponse>('/api/daily-review?limit=200'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '每日复盘加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, []);

  const items = review.items || [];
  const selected = items.find((item) => item.result.symbol === selectedSymbol) || items[0];

  return (
    <Card className="work-card" title="每日复盘" extra={<Button onClick={load}>刷新</Button>}>
      <div className="metric-strip">
        <Text>日期：{review.date || '--'}</Text>
        <Text>命中：{review.summary?.hits || 0}</Text>
        <Text>观察池：{review.summary?.watch_count || 0}</Text>
        <Text>排除池：{review.summary?.exclude_count || 0}</Text>
      </div>
      <Table
        size="small"
        loading={loading}
        rowKey={(record) => record.result.id}
        dataSource={items}
        pagination={{ pageSize: 12, size: 'small' }}
        onRow={(record) => ({ onClick: () => setSelectedSymbol(record.result.symbol) })}
        columns={[
          { title: '代码', dataIndex: ['result', 'symbol'] },
          { title: '公式', dataIndex: ['result', 'formula_name'] },
          { title: '评分', dataIndex: ['score', 'total'] },
          { title: '跟踪', dataIndex: ['track', 'summary'] },
          { title: '状态', dataIndex: 'status' }
        ]}
      />
      {selected ? (
        <div className="detail-grid">
          <HQChartPanel symbol={selected.result.symbol} period="day" count={260} pageSize={80} />
          <JsonPane value={selected} />
        </div>
      ) : null}
    </Card>
  );
}
