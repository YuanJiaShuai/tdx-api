import { Button, Card, Empty, Space, Table, Tabs, Tag, Typography, message } from 'antd';
import { ReloadOutlined, SwapOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatPrice, formatSigned, normalizeSymbol } from '../lib/format';
import type {
  IndustryMoneyRankItem,
  IndustryMoneyRankResponse,
  IndustryRankItem,
  IndustryRankResponse,
  MarketTradingStatus
} from '../types';
import { StockQuoteModal } from './StockQuoteModal';

const { Text } = Typography;

const moneyRankTabs = [
  { key: '0', label: '行业资金排名' },
  { key: '2', label: '证监会行业资金排名' },
  { key: '1', label: '概念板块资金排名' }
];

function numberValue(value?: string | number) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function percentValue(value?: string | number, ratio = false) {
  const number = numberValue(value) * (ratio ? 100 : 1);
  return Number.isFinite(number) ? `${number.toFixed(2)}%` : '--';
}

function amountWan(value?: string | number) {
  const number = numberValue(value);
  return Number.isFinite(number) ? (number / 10000).toFixed(2) : '--';
}

function signedClass(value?: string | number, ratio = false) {
  const number = numberValue(value) * (ratio ? 100 : 1);
  if (number === 0) return '';
  return number > 0 ? 'market-up' : 'market-down';
}

function sourceText(source?: string) {
  switch (source) {
    case 'cache':
      return '共享缓存';
    case 'database':
      return '本地数据库';
    default:
      return source ? '实时接口' : '暂无来源';
  }
}

function displayLeaderCode(value?: string) {
  return normalizeSymbol(value || '');
}

function IndustryChangeTable({
  items,
  loading,
  sort,
  onSort,
  onLeaderClick
}: {
  items: IndustryRankItem[];
  loading: boolean;
  sort: string;
  onSort: () => void;
  onLeaderClick: (item: IndustryRankItem) => void;
}) {
  return (
    <Table<IndustryRankItem>
      className="industry-rank-table"
      size="small"
      loading={loading}
      dataSource={items}
      rowKey={(item, index) => item.bd_code || `${item.bd_name || 'industry'}-${index}`}
      pagination={{ pageSize: 30, size: 'small', showSizeChanger: false }}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无行业涨幅数据" /> }}
      scroll={{ x: 980 }}
      columns={[
        { title: '行业名称', dataIndex: 'bd_name', width: 170, fixed: 'left', render: (value) => <Tag color="blue">{value || '--'}</Tag> },
        {
          title: (
            <Button type="text" size="small" className="industry-sort-button" icon={<SwapOutlined />} onClick={onSort}>
              行业涨幅 {sort === '0' ? '降序' : '升序'}
            </Button>
          ),
          dataIndex: 'bd_zdf',
          width: 130,
          render: (value) => <span className={signedClass(value)}>{value ? `${value}%` : '--'}</span>
        },
        {
          title: '行业5日涨幅',
          dataIndex: 'bd_zdf5',
          width: 130,
          render: (value) => <span className={signedClass(value)}>{value ? `${value}%` : '--'}</span>
        },
        {
          title: '行业20日涨幅',
          dataIndex: 'bd_zdf20',
          width: 140,
          render: (value) => <span className={signedClass(value)}>{value ? `${value}%` : '--'}</span>
        },
        {
          title: '领涨股',
          width: 190,
          render: (_value, item) =>
            item.nzg_code ? (
              <Button type="link" className="industry-leader-link" onClick={() => onLeaderClick(item)}>
                <strong>{item.nzg_name || '--'}</strong>
                <span>{displayLeaderCode(item.nzg_code)}</span>
              </Button>
            ) : (
              '--'
            )
        },
        {
          title: '领涨股涨幅',
          dataIndex: 'nzg_zdf',
          width: 130,
          render: (value) => <span className={signedClass(value)}>{value ? `${value}%` : '--'}</span>
        },
        { title: '行业最新价', dataIndex: 'bd_zxj', width: 130, render: (value) => value || '--' },
        { title: '领涨股最新价', dataIndex: 'nzg_zxj', width: 130, render: (value) => formatPrice(value) }
      ]}
    />
  );
}

function IndustryMoneyTable({
  items,
  loading,
  onLeaderClick
}: {
  items: IndustryMoneyRankItem[];
  loading: boolean;
  onLeaderClick: (item: IndustryMoneyRankItem) => void;
}) {
  return (
    <Table<IndustryMoneyRankItem>
      className="industry-rank-table industry-money-table"
      size="small"
      loading={loading}
      dataSource={items}
      rowKey={(item, index) => item.category || `${item.name || 'category'}-${index}`}
      pagination={{ pageSize: 20, size: 'small', showSizeChanger: false }}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无板块资金数据" /> }}
      scroll={{ x: 1240 }}
      columns={[
        { title: '板块名称', dataIndex: 'name', width: 170, fixed: 'left', render: (value) => <Tag color="blue">{value || '--'}</Tag> },
        {
          title: '涨跌幅',
          dataIndex: 'avg_changeratio',
          width: 110,
          render: (value) => <span className={signedClass(value, true)}>{percentValue(value, true)}</span>
        },
        { title: '流入资金(万)', dataIndex: 'inamount', width: 130, render: amountWan },
        { title: '流出资金(万)', dataIndex: 'outamount', width: 130, render: amountWan },
        {
          title: '净流入(万)',
          dataIndex: 'netamount',
          width: 130,
          render: (value) => <span className={signedClass(value)}>{formatSigned(numberValue(value) / 10000)}</span>
        },
        {
          title: '净流入率',
          dataIndex: 'ratioamount',
          width: 110,
          render: (value) => <span className={signedClass(value, true)}>{percentValue(value, true)}</span>
        },
        {
          title: '领涨股',
          width: 190,
          render: (_value, item) =>
            item.ts_symbol ? (
              <Button type="link" className="industry-leader-link" onClick={() => onLeaderClick(item)}>
                <strong>{item.ts_name || '--'}</strong>
                <span>{displayLeaderCode(item.ts_symbol)}</span>
              </Button>
            ) : (
              '--'
            )
        },
        {
          title: '领涨股涨幅',
          dataIndex: 'ts_changeratio',
          width: 130,
          render: (value) => <span className={signedClass(value, true)}>{percentValue(value, true)}</span>
        },
        { title: '领涨股最新价', dataIndex: 'ts_trade', width: 130, render: (value) => formatPrice(value) },
        {
          title: '领涨股净流入率',
          dataIndex: 'ts_ratioamount',
          width: 150,
          render: (value) => <span className={signedClass(value, true)}>{percentValue(value, true)}</span>
        }
      ]}
    />
  );
}

export function IndustryRankWorkspace() {
  const [rankItems, setRankItems] = useState<IndustryRankItem[]>([]);
  const [moneyItems, setMoneyItems] = useState<Record<string, IndustryMoneyRankItem[]>>({});
  const [rankSort, setRankSort] = useState('0');
  const [rankLoading, setRankLoading] = useState(false);
  const [moneyLoading, setMoneyLoading] = useState(false);
  const [trading, setTrading] = useState(false);
  const [rankSource, setRankSource] = useState('');
  const [moneySource, setMoneySource] = useState('');
  const [rankFetchedAt, setRankFetchedAt] = useState('');
  const [moneyFetchedAt, setMoneyFetchedAt] = useState('');
  const [quoteTarget, setQuoteTarget] = useState<{ code: string; name?: string } | null>(null);
  const tradingRef = useRef(false);

  const loadRank = useCallback(async () => {
    setRankLoading(true);
    try {
      const data = await apiFetch<IndustryRankResponse>(`/api/market/industry/rank?sort=${rankSort}&limit=150`);
      setRankItems(Array.isArray(data.items) ? data.items : []);
      setRankSource(data.source);
      setRankFetchedAt(data.fetched_at);
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '行业涨幅排名加载失败');
    } finally {
      setRankLoading(false);
    }
  }, [rankSort]);

  const loadMoney = useCallback(async () => {
    setMoneyLoading(true);
    try {
      const results = await Promise.all(
        moneyRankTabs.map(async (tab) => {
          const data = await apiFetch<IndustryMoneyRankResponse>(
            `/api/market/industry/money?category=${tab.key}&sort=netamount`
          );
          return { key: tab.key, data };
        })
      );
      setMoneyItems(Object.fromEntries(results.map(({ key, data }) => [key, Array.isArray(data.items) ? data.items : []])));
      setMoneySource(results.map(({ data }) => data.source).find(Boolean) || '');
      setMoneyFetchedAt(results.map(({ data }) => data.fetched_at).find(Boolean) || '');
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '行业资金排名加载失败');
    } finally {
      setMoneyLoading(false);
    }
  }, []);

  const refreshTradingStatus = useCallback(async () => {
    try {
      const data = await apiFetch<MarketTradingStatus>('/api/market/trading-status');
      tradingRef.current = Boolean(data.is_trading);
      setTrading(tradingRef.current);
    } catch {
      tradingRef.current = false;
      setTrading(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadRank(), loadMoney(), refreshTradingStatus()]);
  }, [loadMoney, loadRank, refreshTradingStatus]);

  useEffect(() => {
    void refreshAll();
    const statusTimer = window.setInterval(() => void refreshTradingStatus(), 60000);
    const rankTimer = window.setInterval(() => {
      if (tradingRef.current) void loadRank();
    }, 10000);
    const moneyTimer = window.setInterval(() => void loadMoney(), 60000);
    return () => {
      window.clearInterval(statusTimer);
      window.clearInterval(rankTimer);
      window.clearInterval(moneyTimer);
    };
  }, [loadMoney, loadRank, refreshAll, refreshTradingStatus]);

  const formatFetchedAt = (value: string) => (value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '--');

  return (
    <Card
      className="work-card industry-rank-card"
      title={
        <div className="market-panel-title">
          <strong>行业排名</strong>
          <Text type="secondary">
            {trading ? '交易时段' : '非交易时段'} · 涨幅 {formatFetchedAt(rankFetchedAt)} · 资金 {formatFetchedAt(moneyFetchedAt)}
          </Text>
        </div>
      }
      extra={
        <Space>
          <Text type="secondary">{sourceText(rankSource)} / {sourceText(moneySource)}</Text>
          <Button type="primary" icon={<ReloadOutlined />} loading={rankLoading || moneyLoading} onClick={() => void refreshAll()}>
            刷新
          </Button>
        </Space>
      }
    >
      <Tabs
        className="industry-rank-tabs"
        items={[
          {
            key: 'change',
            label: '行业涨幅排名',
            children: (
              <IndustryChangeTable
                items={rankItems}
                loading={rankLoading}
                sort={rankSort}
                onSort={() => setRankSort((value) => (value === '0' ? '1' : '0'))}
                onLeaderClick={(item) => setQuoteTarget({ code: item.nzg_code || '', name: item.nzg_name })}
              />
            )
          },
          ...moneyRankTabs.map((tab) => ({
            key: `money-${tab.key}`,
            label: tab.label,
            children: (
              <IndustryMoneyTable
                items={moneyItems[tab.key] || []}
                loading={moneyLoading}
                onLeaderClick={(item) => setQuoteTarget({ code: item.ts_symbol || '', name: item.ts_name })}
              />
            )
          }))
        ]}
      />
      <StockQuoteModal target={quoteTarget} onClose={() => setQuoteTarget(null)} />
    </Card>
  );
}
