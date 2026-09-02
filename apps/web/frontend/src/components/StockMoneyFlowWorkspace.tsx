import { Button, Card, Empty, Space, Table, Tabs, Typography, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatPrice, formatSigned, normalizeSymbol } from '../lib/format';
import type { StockMoneyRankItem, StockMoneyRankResponse } from '../types';
import { StockQuoteModal } from './StockQuoteModal';

const { Text } = Typography;

const refreshInterval = 30 * 60 * 1000;

const rankTabs = [
  { key: 'netamount', label: '净流入额排名' },
  { key: 'outamount', label: '流出资金排名' },
  { key: 'ratioamount', label: '净流入率排名' },
  { key: 'r0_net', label: '主力净流入额排名' },
  { key: 'r0_out', label: '主力流出排名' },
  { key: 'r0_ratio', label: '主力净流入率排名' },
  { key: 'r3_net', label: '散户净流入额排名' },
  { key: 'r3_out', label: '散户流出排名' },
  { key: 'r3_ratio', label: '散户净流入率排名' }
] as const;

type RankKey = (typeof rankTabs)[number]['key'];

function numberValue(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function percent(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? `${(number * 100).toFixed(2)}%` : '--';
}

function amountWan(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? (number / 10000).toFixed(2) : '--';
}

function signedAmountWan(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? formatSigned(number / 10000) : '--';
}

function signedClass(value?: string | number, ratio = false) {
  const number = numberValue(value) * (ratio ? 100 : 1);
  if (number === 0) return '';
  return number > 0 ? 'market-up' : 'market-down';
}

function displayName(value?: string | boolean) {
  return typeof value === 'string' && value.trim() ? value : '--';
}

function displayCode(value?: string) {
  return normalizeSymbol(value || '');
}

function fetchedTime(value?: string) {
  return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--';
}

function sourceText(value?: string) {
  if (value === 'cache') return '共享缓存';
  if (value === 'database') return '本地数据库';
  return value ? '实时接口' : '暂无来源';
}

function baseColumns(onStockClick: (item: StockMoneyRankItem) => void) {
  return [
    {
      title: '代码',
      dataIndex: 'symbol',
      fixed: 'left' as const,
      width: 90,
      render: (value: string, item: StockMoneyRankItem) => (
        <Button
          type="link"
          className="code-link"
          onClick={() => onStockClick(item)}
        >
          {displayCode(value) || '--'}
        </Button>
      )
    },
    {
      title: '名称',
      dataIndex: 'name',
      fixed: 'left' as const,
      width: 125,
      render: (value: string | boolean, item: StockMoneyRankItem) => (
        <Button type="link" className="stock-money-name-link" onClick={() => onStockClick(item)}>
          {displayName(value)}
        </Button>
      )
    },
    { title: '最新价', dataIndex: 'trade', width: 95, render: formatPrice },
    {
      title: '涨跌幅',
      dataIndex: 'changeratio',
      width: 95,
      render: (value: string) => <span className={signedClass(value, true)}>{percent(value)}</span>
    },
    {
      title: '换手率',
      dataIndex: 'turnover',
      width: 95,
      render: (value: string) => <span>{numberValue(value) ? `${(numberValue(value) / 100).toFixed(2)}%` : '--'}</span>
    },
    { title: '成交额(万)', dataIndex: 'amount', width: 120, render: amountWan },
    { title: '流出资金(万)', dataIndex: 'outamount', width: 125, render: amountWan },
    { title: '流入资金(万)', dataIndex: 'inamount', width: 125, render: amountWan },
    {
      title: '净流入(万)',
      dataIndex: 'netamount',
      width: 125,
      render: (value: string) => <span className={signedClass(value)}>{signedAmountWan(value)}</span>
    },
    {
      title: '净流入率',
      dataIndex: 'ratioamount',
      width: 105,
      render: (value: string) => <span className={signedClass(value, true)}>{percent(value)}</span>
    },
    {
      title: '主力净流入率',
      dataIndex: 'r0_ratio',
      width: 115,
      render: (value: string) => <span className={signedClass(value, true)}>{percent(value)}</span>
    },
    {
      title: '散户净流入率',
      dataIndex: 'r3_ratio',
      width: 115,
      render: (value: string) => <span className={signedClass(value, true)}>{percent(value)}</span>
    }
  ];
}

function extraColumns(sort: RankKey) {
  const columns = [];
  if (sort === 'r0_net' || sort === 'r0_out') {
    columns.push({ title: '主力流出(万)', dataIndex: 'r0_out', width: 125, render: amountWan });
  }
  if (sort === 'r0_net') {
    columns.push(
      { title: '主力流入(万)', dataIndex: 'r0_in', width: 125, render: amountWan },
      {
        title: '主力净流入(万)',
        dataIndex: 'r0_net',
        width: 135,
        render: (value: string) => <span className={signedClass(value)}>{signedAmountWan(value)}</span>
      }
    );
  }
  if (sort === 'r3_net' || sort === 'r3_out') {
    columns.push({ title: '散户流出(万)', dataIndex: 'r3_out', width: 125, render: amountWan });
  }
  if (sort === 'r3_net') {
    columns.push(
      { title: '散户流入(万)', dataIndex: 'r3_in', width: 125, render: amountWan },
      {
        title: '散户净流入(万)',
        dataIndex: 'r3_net',
        width: 135,
        render: (value: string) => <span className={signedClass(value)}>{signedAmountWan(value)}</span>
      }
    );
  }
  return columns;
}

export function StockMoneyFlowWorkspace() {
  const [items, setItems] = useState<Record<string, StockMoneyRankItem[]>>({});
  const [loading, setLoading] = useState(false);
  const [source, setSource] = useState('');
  const [fetchedAt, setFetchedAt] = useState('');
  const [activeTab, setActiveTab] = useState<RankKey>('netamount');
  const [quoteTarget, setQuoteTarget] = useState<{ code: string; name?: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const results = await Promise.all(
        rankTabs.map(async (tab) => {
          const data = await apiFetch<StockMoneyRankResponse>(
            `/api/market/stock-money?sort=${encodeURIComponent(tab.key)}`
          );
          return { key: tab.key, data };
        })
      );
      setItems(Object.fromEntries(results.map(({ key, data }) => [key, Array.isArray(data.items) ? data.items : []])));
      setSource(results.map(({ data }) => data.source).find(Boolean) || '');
      setFetchedAt(results.map(({ data }) => data.fetched_at).find(Boolean) || '');
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '个股资金流向加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), refreshInterval);
    return () => window.clearInterval(timer);
  }, [load]);

  function openStock(item: StockMoneyRankItem) {
    if (!item.symbol) return;
    setQuoteTarget({ code: item.symbol, name: displayName(item.name) });
  }

  return (
    <Card
      className="work-card stock-money-card"
      title={
        <div className="market-panel-title">
          <strong>个股资金流向</strong>
          <Text type="secondary">每 30 分钟刷新 · 更新于 {fetchedTime(fetchedAt)}</Text>
        </div>
      }
      extra={
        <Space>
          <Text type="secondary">{sourceText(source)}</Text>
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>
            刷新
          </Button>
        </Space>
      }
    >
      <Tabs
        className="stock-money-tabs"
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key as RankKey)}
        items={rankTabs.map((tab) => ({
          key: tab.key,
          label: tab.label,
          children: (
            <Table<StockMoneyRankItem>
              className="stock-money-table"
              size="small"
              loading={loading}
              dataSource={items[tab.key] || []}
              rowKey={(item, index) => `${item.symbol || 'stock'}-${index}`}
              pagination={{ pageSize: 20, size: 'small', showSizeChanger: false }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无个股资金流向数据" /> }}
              scroll={{ x: 1660 }}
              columns={[...baseColumns(openStock), ...extraColumns(tab.key)]}
            />
          )
        }))}
      />
      <StockQuoteModal target={quoteTarget} onClose={() => setQuoteTarget(null)} />
    </Card>
  );
}
