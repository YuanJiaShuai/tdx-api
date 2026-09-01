import { Button, Card, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { BarChartOutlined, ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import {
  formatAmount,
  formatPrice,
  formatSigned,
  normalizeSymbol,
  priceFromMilli
} from '../lib/format';
import type { Quote, WatchlistRow } from '../types';

const { Text } = Typography;

const initialRows: WatchlistRow[] = [
  { key: '601899', code: '601899', name: '紫金矿业' },
  { key: '603171', code: '603171', name: '税友股份' },
  { key: '002202', code: '002202', name: '金风科技' },
  { key: '000630', code: '000630', name: '铜陵有色' }
];

function changeClassName(value?: string) {
  const number = Number.parseFloat(String(value || '').replace('%', ''));
  if (!Number.isFinite(number) || number === 0) return '';
  return number > 0 ? 'market-up' : 'market-down';
}

function bidAskRatio(quote: Quote): string {
  const buyTotal = (quote.BuyLevel || []).reduce((sum, item) => sum + Number(item?.Number || 0), 0);
  const sellTotal = (quote.SellLevel || []).reduce((sum, item) => sum + Number(item?.Number || 0), 0);
  const total = buyTotal + sellTotal;
  if (total <= 0) return '--';
  return formatSigned(((buyTotal - sellTotal) / total) * 100, '%');
}

function mergeQuote(row: WatchlistRow, quote?: Quote): WatchlistRow {
  if (!quote?.K) return row;
  const previousClose = priceFromMilli(quote.K.Last);
  const currentPrice = priceFromMilli(quote.K.Close);
  if (currentPrice <= 0) return row;
  const change = previousClose > 0 ? currentPrice - previousClose : 0;
  const changePercent = previousClose > 0 ? (change / previousClose) * 100 : 0;
  return {
    ...row,
    price: formatPrice(currentPrice),
    change: formatSigned(change),
    changePercent: formatSigned(changePercent, '%'),
    volume: quote.TotalHand && quote.TotalHand > 0 ? formatAmount(quote.TotalHand * 100) : '--',
    amount: formatAmount(quote.Amount),
    speed: Number.isFinite(Number(quote.Rate)) && Number(quote.Rate) !== 0 ? formatSigned(quote.Rate, '%') : '--',
    entrust: bidAskRatio(quote)
  };
}

export function WatchlistTable() {
  const [rows, setRows] = useState<WatchlistRow[]>(initialRows);
  const [loading, setLoading] = useState(false);
  const [updatedAt, setUpdatedAt] = useState('');

  const refreshQuotes = useCallback(async () => {
    const codes = rows.map((row) => normalizeSymbol(row.code)).filter(Boolean);
    if (!codes.length) return;
    setLoading(true);
    try {
      const quotes = await apiFetch<Quote[]>(`/api/quote?code=${encodeURIComponent(codes.join(','))}`);
      const quoteMap = new Map((Array.isArray(quotes) ? quotes : []).map((quote) => [normalizeSymbol(quote.Code), quote]));
      setRows((current) =>
        current.map((row) => {
          const code = normalizeSymbol(row.code);
          return mergeQuote(row, quoteMap.get(code));
        })
      );
      setUpdatedAt(new Date().toLocaleTimeString('zh-CN', { hour12: false }));
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '行情刷新失败');
    } finally {
      setLoading(false);
    }
  }, [rows]);

  useEffect(() => {
    refreshQuotes();
  }, []);

  const columns = useMemo<ColumnsType<WatchlistRow>>(
    () => [
      {
        title: '序号',
        dataIndex: 'index',
        width: 70,
        fixed: 'left',
        render: (_value, _record, index) => index + 1
      },
      {
        title: '代码',
        dataIndex: 'code',
        width: 104,
        fixed: 'left',
        render: (value: string, record) => (
          <Button type="link" className="code-link" onClick={() => message.info(`${record.name} 行情详情将在下一阶段迁移`)}>
            {value}
          </Button>
        )
      },
      {
        title: '名称',
        dataIndex: 'name',
        width: 130,
        fixed: 'left',
        render: (value: string) => <strong>{value}</strong>
      },
      {
        title: '涨幅 %',
        dataIndex: 'changePercent',
        width: 110,
        sorter: (a, b) => Number.parseFloat(a.changePercent || '0') - Number.parseFloat(b.changePercent || '0'),
        render: (value?: string) => <span className={changeClassName(value)}>{value || '--'}</span>
      },
      {
        title: '现价',
        dataIndex: 'price',
        width: 96,
        render: (value: string | undefined, record: WatchlistRow) => (
          <span className={changeClassName(record.changePercent)}>{value || '--'}</span>
        )
      },
      { title: '成交量', dataIndex: 'volume', width: 116 },
      { title: '成交额', dataIndex: 'amount', width: 116 },
      { title: '涨速 %', dataIndex: 'speed', width: 100 },
      {
        title: '涨跌',
        dataIndex: 'change',
        width: 96,
        render: (value: string | undefined, record: WatchlistRow) => (
          <span className={changeClassName(record.changePercent)}>{value || '--'}</span>
        )
      },
      { title: '主力净量', dataIndex: 'mainNetVolume', width: 112, render: (value?: string) => value || '--' },
      { title: '主力净流入', dataIndex: 'mainNetInflow', width: 128, render: (value?: string) => value || '--' },
      { title: '所属行业', dataIndex: 'industry', width: 150, render: (value?: string) => value || '--' },
      { title: '换手 %', dataIndex: 'turnover', width: 100, render: (value?: string) => value || '--' },
      {
        title: '分类标示',
        dataIndex: 'category',
        width: 112,
        render: (value?: string) => (value ? <Tag color="blue">{value}</Tag> : '--')
      },
      { title: '委比 %', dataIndex: 'entrust', width: 100, render: (value?: string) => value || '--' },
      { title: '量比', dataIndex: 'volumeRatio', width: 96, render: (value?: string) => value || '--' }
    ],
    []
  );

  return (
    <Card
      className="work-card"
      title={
        <div>
          <span>自选列表</span>
          <Text type="secondary" className="card-subtitle">
            共 {rows.length} 只{updatedAt ? ` · 更新于 ${updatedAt}` : ' · 等待行情'}
          </Text>
        </div>
      }
      extra={
        <Space>
          <Button icon={<BarChartOutlined />}>自选分析</Button>
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={refreshQuotes}>
            刷新
          </Button>
        </Space>
      }
    >
      <Table
        className="watchlist-table"
        rowKey="key"
        size="middle"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={false}
        scroll={{ x: 1740, y: 520 }}
      />
    </Card>
  );
}
