import { Button, Card, Empty, Input, Select, Space, Table, Typography, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatPrice, formatSigned } from '../lib/format';
import type { LongTigerRank, LongTigerResponse } from '../types';
import { StockQuoteModal } from './StockQuoteModal';

const { Text } = Typography;

function localDate() {
  const now = new Date();
  const offset = now.getTimezoneOffset();
  return new Date(now.getTime() - offset * 60 * 1000).toISOString().slice(0, 10);
}

function numberValue(value?: number) {
  return Number.isFinite(Number(value)) ? Number(value) : 0;
}

function wan(value?: number) {
  const number = numberValue(value) / 10000;
  return number === 0 ? '--' : number.toFixed(2);
}

function yi(value?: number) {
  const number = numberValue(value) / 100000000;
  return number === 0 ? '--' : number.toFixed(2);
}

function percent(value?: number) {
  return Number.isFinite(Number(value)) ? `${numberValue(value).toFixed(2)}%` : '--';
}

function rankKey(item: LongTigerRank, index?: number) {
  return `${item.SECUCODE || item.SECURITY_CODE || 'rank'}-${item.TRADE_DATE || index || 0}`;
}

export function LongTigerWorkspace() {
  const [date, setDate] = useState(localDate);
  const [items, setItems] = useState<LongTigerRank[]>([]);
  const [selectedExplanation, setSelectedExplanation] = useState('');
  const [loading, setLoading] = useState(false);
  const [source, setSource] = useState('');
  const [actualDate, setActualDate] = useState('');
  const [quoteTarget, setQuoteTarget] = useState<{ code: string; name?: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    let currentDate = date;
    try {
      for (let retry = 0; retry <= 7; retry += 1) {
        const data = await apiFetch<LongTigerResponse>(`/api/long-tiger?date=${encodeURIComponent(currentDate)}`);
        const nextItems = Array.isArray(data?.items) ? data.items : [];
        if (nextItems.length) {
          setItems(nextItems);
          setSource(data?.source || '');
          setActualDate(data?.trade_date || currentDate);
          setSelectedExplanation('');
          return;
        }
        const previous = new Date(`${currentDate}T00:00:00`);
        previous.setDate(previous.getDate() - 1);
        const nextDate = previous.toISOString().slice(0, 10);
        if (nextDate === currentDate) break;
        message.info(`当前日期 ${currentDate} 暂无数据，尝试 ${nextDate}`);
        currentDate = nextDate;
        setDate(nextDate);
      }
      setItems([]);
      setSource('');
      setActualDate(currentDate);
      setSelectedExplanation('');
      message.info('暂无龙虎榜数据');
    } catch (error) {
      setItems([]);
      setSource('');
      setActualDate(currentDate);
      message.error(error instanceof Error ? error.message : '龙虎榜加载失败');
    } finally {
      setLoading(false);
    }
  }, [date]);

  useEffect(() => {
    load();
  }, []);

  const explanationOptions = useMemo(() => {
    const values = Array.from(new Set(items.map((item) => item.EXPLANATION).filter(Boolean)));
    return [{ value: '', label: '全部上榜原因' }, ...values.map((value) => ({ value, label: value }))];
  }, [items]);

  const filteredItems = useMemo(
    () => (selectedExplanation ? items.filter((item) => item.EXPLANATION === selectedExplanation) : items),
    [items, selectedExplanation]
  );

  return (
    <Card
      className="work-card long-tiger-card"
      title={
        <div className="market-panel-title">
          <strong>龙虎榜</strong>
          <Text type="secondary">
            {actualDate || date} · {items.length ? `${filteredItems.length}/${items.length} 条` : '暂无数据'}
          </Text>
        </div>
      }
      extra={
        <Space wrap>
          <Input
            type="date"
            value={date}
            onChange={(event) => setDate(event.target.value)}
            className="market-date-input"
          />
          <Select
            value={selectedExplanation}
            options={explanationOptions}
            onChange={setSelectedExplanation}
            className="long-tiger-filter"
          />
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={load}>
            查询
          </Button>
        </Space>
      }
    >
      <div className="long-tiger-meta">
        <Text type="secondary">数据源：东方财富数据中心</Text>
        <Text type="secondary">
          {source === 'database' ? '本地数据库' : source === 'stale-database' ? '数据库兜底' : source === 'cache' ? '本地缓存' : '刚刚更新'}
        </Text>
      </div>
      <Table
        className="long-tiger-table"
        size="small"
        loading={loading}
        rowKey={(item, index) => rankKey(item, index)}
        dataSource={filteredItems}
        pagination={{ pageSize: 20, size: 'small', showSizeChanger: false }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该日期暂无龙虎榜数据" /> }}
        scroll={{ x: 1320 }}
        columns={[
          {
            title: '代码',
            dataIndex: 'SECURITY_CODE',
            fixed: 'left',
            width: 90,
            render: (value: string, record) => {
              const code = value || record.SECUCODE || '';
              const name = record.SECURITY_NAME_ABBR || '';
              return code ? (
                <Button type="link" className="code-link" onClick={() => setQuoteTarget({ code, name })}>
                  {code.replace(/\.[A-Z]+$/, '')}
                </Button>
              ) : (
                '--'
              );
            }
          },
          {
            title: '名称',
            dataIndex: 'SECURITY_NAME_ABBR',
            fixed: 'left',
            width: 110,
            render: (value: string, record) => (
              <strong className={numberValue(record.CHANGE_RATE) >= 0 ? 'market-up' : 'market-down'}>{value || '--'}</strong>
            )
          },
          {
            title: '收盘价',
            dataIndex: 'CLOSE_PRICE',
            width: 96,
            render: (value: number) => formatPrice(value)
          },
          {
            title: '涨跌幅',
            dataIndex: 'CHANGE_RATE',
            width: 100,
            render: (value: number) => (
              <span className={numberValue(value) >= 0 ? 'market-up' : 'market-down'}>{percent(value)}</span>
            )
          },
          {
            title: '净买额(万)',
            dataIndex: 'BILLBOARD_NET_AMT',
            width: 120,
            render: (value: number) => (
              <span className={numberValue(value) >= 0 ? 'market-up' : 'market-down'}>{wan(value)}</span>
            )
          },
          { title: '买入额(万)', dataIndex: 'BILLBOARD_BUY_AMT', width: 120, render: wan },
          { title: '卖出额(万)', dataIndex: 'BILLBOARD_SELL_AMT', width: 120, render: wan },
          { title: '成交额(万)', dataIndex: 'BILLBOARD_DEAL_AMT', width: 120, render: wan },
          { title: '换手率', dataIndex: 'TURNOVERRATE', width: 90, render: percent },
          { title: '流通市值(亿)', dataIndex: 'FREE_MARKET_CAP', width: 120, render: yi },
          { title: '净买占比', dataIndex: 'DEAL_NET_RATIO', width: 100, render: percent },
          { title: '上榜原因', dataIndex: 'EXPLANATION', width: 260, ellipsis: true, render: (value: string) => value || '--' },
          {
            title: '近5日涨跌',
            dataIndex: 'D5_CLOSE_ADJCHRATE',
            width: 110,
            render: (value: number) => formatSigned(value, '%')
          }
        ]}
      />
      <StockQuoteModal target={quoteTarget} onClose={() => setQuoteTarget(null)} />
    </Card>
  );
}
