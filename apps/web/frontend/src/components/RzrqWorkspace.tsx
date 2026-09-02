import { Button, Card, DatePicker, Empty, Select, Space, Statistic, Table, Tag, Typography, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatPrice, formatSigned } from '../lib/format';
import type { RzrqRankItem, RzrqRankResponse, RzrqTrendResponse, RzrqTrendItem } from '../types';

const { Text } = Typography;

const typeOptions = [
  { value: 'hyList', label: '行业' },
  { value: 'gnList', label: '概念' },
  { value: 'ggList', label: '个股' }
];

const sortKeyOptions = [
  { value: 'jmr', label: '净买入额' },
  { value: 'rzye', label: '融资余额' },
  { value: 'rqye', label: '融券余额' },
  { value: 'rzmre', label: '融资买入额' },
  { value: 'rzjmce', label: '融资净买入额' },
  { value: 'lrye', label: '融资融券余额' },
  { value: 'yezf', label: '余额增幅' },
  { value: 'close_profit', label: '涨跌幅' }
];

const sortTypeOptions = [
  { value: 'desc', label: '降序' },
  { value: 'asc', label: '升序' }
];

const lengthOptions = [10, 20, 50, 100].map((value) => ({ value, label: `${value}条` }));

function numberValue(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function amount(value?: string | number) {
  const number = numberValue(value) * 1000;
  if (!Number.isFinite(number)) return '--';
  if (Math.abs(number) >= 100000000) return `${(number / 100000000).toFixed(2)}亿`;
  if (Math.abs(number) >= 10000) return `${(number / 10000).toFixed(2)}万`;
  return number.toFixed(2);
}

function percentage(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? `${number.toFixed(2)}%` : '--';
}

function signedClass(value?: string | number) {
  const number = numberValue(value);
  if (number === 0) return '';
  return number > 0 ? 'market-up' : 'market-down';
}

function fetchedTime(value?: string) {
  return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--';
}

function dateFromTimestamp(value?: number) {
  if (!value) return '';
  const date = new Date(value > 1e12 ? value : value * 1000);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 10);
}

function TrendChart({ data }: { data: RzrqTrendResponse | null }) {
  const items = data?.items || [];
  const width = 840;
  const height = 250;
  const padding = { top: 18, right: 18, bottom: 32, left: 54 };
  const values = items.flatMap((item) => [numberValue(item.rzye), numberValue(item.rzjlr)]);
  const min = values.length ? Math.min(...values) : 0;
  const max = values.length ? Math.max(...values) : 1;
  const range = max - min || 1;
  const x = (index: number) =>
    padding.left + (index / Math.max(items.length - 1, 1)) * (width - padding.left - padding.right);
  const y = (value: number) =>
    padding.top + (1 - (value - min) / range) * (height - padding.top - padding.bottom);
  const line = (selector: (item: RzrqTrendItem) => string | undefined) =>
    items.map((item, index) => `${x(index).toFixed(1)},${y(numberValue(selector(item))).toFixed(1)}`).join(' ');

  return (
    <div className="rzrq-trend-wrap">
      {items.length ? (
        <>
          <svg className="rzrq-trend-chart" viewBox={`0 0 ${width} ${height}`} role="img" aria-label="融资融券走势">
            {[0, 0.5, 1].map((ratio) => {
              const gridY = padding.top + ratio * (height - padding.top - padding.bottom);
              return (
                <line
                  key={ratio}
                  x1={padding.left}
                  x2={width - padding.right}
                  y1={gridY}
                  y2={gridY}
                  className="rzrq-grid-line"
                />
              );
            })}
            <polyline points={line((item) => item.rzye)} className="rzrq-line rzrq-line-balance" />
            <polyline points={line((item) => item.rzjlr)} className="rzrq-line rzrq-line-net" />
            <text x={padding.left - 8} y={padding.top + 4} textAnchor="end" className="rzrq-axis-label">
              {max.toFixed(2)}
            </text>
            <text x={padding.left - 8} y={height - padding.bottom} textAnchor="end" className="rzrq-axis-label">
              {min.toFixed(2)}
            </text>
            <text x={padding.left} y={height - 8} className="rzrq-axis-label">
              {items[0]?.date || ''}
            </text>
            <text x={width - padding.right} y={height - 8} textAnchor="end" className="rzrq-axis-label">
              {items[items.length - 1]?.date || ''}
            </text>
          </svg>
          <div className="rzrq-legend">
            <span><i className="rzrq-legend-dot balance" />融资余额 ({data?.rzye_unit || '亿'})</span>
            <span><i className="rzrq-legend-dot net" />融资净买入 ({data?.rzjlr_unit || '亿'})</span>
          </div>
        </>
      ) : (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无融资融券走势数据" />
      )}
    </div>
  );
}

function RankTable({ items, loading }: { items: RzrqRankItem[]; loading: boolean }) {
  return (
    <Table<RzrqRankItem>
      className="rzrq-rank-table"
      size="small"
      loading={loading}
      dataSource={items}
      rowKey={(item, index) => `${item.marketId || item.stockCode || item.stockName || 'rzrq'}-${index}`}
      pagination={{ pageSize: 20, size: 'small', showSizeChanger: false }}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无融资融券排名数据" /> }}
      scroll={{ x: 1680 }}
      columns={[
        { title: '排名', width: 60, fixed: 'left', render: (_value, _item, index) => index + 1 },
        { title: '代码', dataIndex: 'stockCode', width: 95, fixed: 'left', render: (value) => value || '--' },
        { title: '名称', dataIndex: 'stockName', width: 115, fixed: 'left', render: (value) => <Tag color="blue">{value || '--'}</Tag> },
        { title: '收盘价', dataIndex: 'close_price', width: 85, align: 'right', render: formatPrice },
        {
          title: '涨跌幅',
          dataIndex: 'close_profit',
          width: 90,
          align: 'right',
          render: (value) => <span className={signedClass(value)}>{formatSigned(value, '%')}</span>
        },
        {
          title: '净买入额',
          dataIndex: 'jmr',
          width: 115,
          align: 'right',
          render: (value) => <span className={signedClass(value)}>{amount(value)}</span>
        },
        { title: '净买入占比', dataIndex: 'jmrRate', width: 105, align: 'right', render: percentage },
        { title: '融资余额', dataIndex: 'rzye', width: 115, align: 'right', render: amount },
        { title: '融资占比', dataIndex: 'rzyeRate', width: 95, align: 'right', render: percentage },
        { title: '融资买入额', dataIndex: 'rzmre', width: 115, align: 'right', render: amount },
        { title: '融资偿还额', dataIndex: 'rzche', width: 115, align: 'right', render: amount },
        { title: '融资净买入', dataIndex: 'rzjmce', width: 115, align: 'right', render: amount },
        { title: '融券余额', dataIndex: 'rqye', width: 115, align: 'right', render: amount },
        { title: '融券占比', dataIndex: 'rqyeRate', width: 95, align: 'right', render: percentage },
        { title: '两融余额', dataIndex: 'lrye', width: 115, align: 'right', render: amount },
        { title: '余额占比', dataIndex: 'lryeRate', width: 95, align: 'right', render: percentage },
        {
          title: '余额增幅',
          dataIndex: 'yezf',
          width: 95,
          align: 'right',
          render: (value) => <span className={signedClass(value)}>{percentage(value)}</span>
        }
      ]}
    />
  );
}

export function RzrqWorkspace() {
  const [type, setType] = useState('hyList');
  const [sortKey, setSortKey] = useState('jmr');
  const [sortType, setSortType] = useState('desc');
  const [length, setLength] = useState(20);
  const [date, setDate] = useState('');
  const [rank, setRank] = useState<RzrqRankResponse>();
  const [trend, setTrend] = useState<RzrqTrendResponse | null>(null);
  const [rankLoading, setRankLoading] = useState(false);
  const [trendLoading, setTrendLoading] = useState(false);
  const initialLoad = useRef(false);

  const loadRank = useCallback(async () => {
    setRankLoading(true);
    try {
      const params = new URLSearchParams({
        type,
        sort_key: sortKey,
        sort_type: sortType,
        length: String(length),
        offset: '0'
      });
      if (date) params.set('date', date);
      const data = await apiFetch<RzrqRankResponse>(`/api/market/rzrq/rank?${params.toString()}`);
      setRank(data);
    } catch (error) {
      setRank(undefined);
      message.warning(error instanceof Error ? error.message : '融资融券排名加载失败');
    } finally {
      setRankLoading(false);
    }
  }, [date, length, sortKey, sortType, type]);

  const loadTrend = useCallback(async () => {
    setTrendLoading(true);
    try {
      const data = await apiFetch<RzrqTrendResponse>('/api/market/rzrq/trend');
      setTrend(data);
    } catch (error) {
      setTrend(null);
      message.warning(error instanceof Error ? error.message : '融资融券走势加载失败');
    } finally {
      setTrendLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!initialLoad.current) {
      void loadRank();
      return;
    }
    initialLoad.current = false;
    void loadRank();
    void loadTrend();
  }, [loadRank, loadTrend]);

  const dataDate = useMemo(
    () => rank?.data_date || dateFromTimestamp(rank?.list?.[0]?.date),
    [rank]
  );
  const summary = useMemo(() => {
    const items = rank?.list || [];
    return {
      net: items.reduce((sum, item) => sum + numberValue(item.jmr), 0),
      margin: items.reduce((sum, item) => sum + numberValue(item.rzye), 0),
      short: items.reduce((sum, item) => sum + numberValue(item.rqye), 0),
      up: items.filter((item) => numberValue(item.close_profit) > 0).length,
      down: items.filter((item) => numberValue(item.close_profit) < 0).length
    };
  }, [rank]);

  return (
    <div className="rzrq-layout">
      <Card
        className="work-card rzrq-trend-card"
        title={
          <div className="market-panel-title">
            <strong>融资融券走势</strong>
            <Text type="secondary">
              {trend?.update_time ? `更新于 ${trend.update_time}` : '全市场汇总'}
            </Text>
          </div>
        }
        extra={
          <Space>
            <Text type="secondary">{trend?.source || '同花顺'}</Text>
            <Button type="primary" icon={<ReloadOutlined />} loading={trendLoading} onClick={() => void loadTrend()}>
              刷新走势
            </Button>
          </Space>
        }
      >
        <TrendChart data={trend} />
      </Card>

      <Card
        className="work-card rzrq-rank-card"
        title={
          <div className="market-panel-title">
            <strong>融资融券余额排名</strong>
            <Text type="secondary">
              {dataDate ? `${typeOptions.find((item) => item.value === type)?.label || ''} · ${dataDate}` : '行业 / 概念 / 个股'}
            </Text>
          </div>
        }
        extra={
          <Space>
            <Text type="secondary">{rank?.source || '同花顺'}</Text>
            <Button type="primary" icon={<ReloadOutlined />} loading={rankLoading} onClick={() => void loadRank()}>
              刷新排名
            </Button>
          </Space>
        }
      >
        <Space wrap className="toolbar-row rzrq-toolbar">
          <Select value={type} options={typeOptions} onChange={setType} />
          <Select value={sortKey} options={sortKeyOptions} onChange={setSortKey} />
          <Select value={sortType} options={sortTypeOptions} onChange={setSortType} />
          <Select value={length} options={lengthOptions} onChange={setLength} />
          <DatePicker
            placeholder="最新日期"
            onChange={(_value, dateString) => setDate(typeof dateString === 'string' ? dateString : '')}
          />
        </Space>

        <div className="metric-strip rzrq-summary">
          <Statistic title="净买入合计" value={amount(summary.net)} />
          <Statistic title="融资余额合计" value={amount(summary.margin)} />
          <Statistic title="融券余额合计" value={amount(summary.short)} />
          <Statistic title="上涨" value={summary.up} valueStyle={{ color: 'var(--danger)' }} />
          <Statistic title="下跌" value={summary.down} valueStyle={{ color: 'var(--success)' }} />
        </div>

        <RankTable items={rank?.list || []} loading={rankLoading} />
      </Card>
    </div>
  );
}
