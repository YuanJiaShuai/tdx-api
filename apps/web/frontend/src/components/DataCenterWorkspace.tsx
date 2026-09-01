import { Button, Card, Checkbox, Form, Input, InputNumber, Select, Space, Statistic, Table, Typography, message } from 'antd';
import { DatabaseOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { DynamicTable } from './DynamicTable';
import { JsonPane } from './JsonPane';

const { Text } = Typography;

function rowsFrom(value: unknown): Array<Record<string, unknown>> {
  if (Array.isArray(value)) return value as Array<Record<string, unknown>>;
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    if (Array.isArray(record.codes)) return record.codes as Array<Record<string, unknown>>;
    if (Array.isArray(record.list)) return record.list.map((item) => (typeof item === 'object' ? item as Record<string, unknown> : { value: item }));
    if (Array.isArray(record.List)) return record.List as Array<Record<string, unknown>>;
  }
  return [];
}

function directoryRowKey(record: Record<string, unknown>) {
  return String(record.code || record.Code || record.value || record.name || record.Name || JSON.stringify(record));
}

export function DataCenterWorkspace() {
  const [overview, setOverview] = useState<Record<string, unknown>>({});
  const [directoryRows, setDirectoryRows] = useState<Array<Record<string, unknown>>>([]);
  const [directoryLoading, setDirectoryLoading] = useState(false);
  const [exchange, setExchange] = useState('all');
  const [limit, setLimit] = useState(80);
  const [profileCode, setProfileCode] = useState('000001');
  const [profileOutput, setProfileOutput] = useState<unknown>('尚未查询');
  const [historyOutput, setHistoryOutput] = useState<unknown>('尚未查询');
  const [historyCode, setHistoryCode] = useState('000001');
  const [historyType, setHistoryType] = useState('day');
  const [historyLimit, setHistoryLimit] = useState(120);
  const [blockOutput, setBlockOutput] = useState<unknown>('尚未查询');
  const [blockFile, setBlockFile] = useState('gn');
  const [blockWithIndex, setBlockWithIndex] = useState(true);
  const [hikyuuStatus, setHikyuuStatus] = useState<unknown>('尚未查询');

  const loadOverview = useCallback(async () => {
    try {
      const [status, stats, count] = await Promise.allSettled([
        apiFetch<Record<string, unknown>>('/api/server-status'),
        apiFetch<Record<string, unknown>>('/api/market-stats'),
        apiFetch<Record<string, unknown>>('/api/market-count')
      ]);
      setOverview({
        status: status.status === 'fulfilled' ? status.value : {},
        stats: stats.status === 'fulfilled' ? stats.value : {},
        count: count.status === 'fulfilled' ? count.value : {}
      });
    } catch (error) {
      message.warning(error instanceof Error ? error.message : '市场总览加载失败');
    }
  }, []);

  const loadDirectory = useCallback(async (kind: 'codes' | 'etf' = 'codes') => {
    setDirectoryLoading(true);
    try {
      const url =
        kind === 'codes'
          ? `/api/codes?exchange=${encodeURIComponent(exchange)}`
          : `/api/etf?exchange=${encodeURIComponent(exchange)}&limit=${encodeURIComponent(limit)}`;
      const data = await apiFetch<unknown>(url);
      setDirectoryRows(rowsFrom(data).slice(0, limit));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '代码目录加载失败');
    } finally {
      setDirectoryLoading(false);
    }
  }, [exchange, limit]);

  useEffect(() => {
    loadOverview();
    loadDirectory();
  }, []);

  async function loadProfile() {
    try {
      const [info, finance] = await Promise.allSettled([
        apiFetch(`/api/stock-info?code=${encodeURIComponent(profileCode)}`),
        apiFetch(`/api/finance?code=${encodeURIComponent(profileCode)}`)
      ]);
      setProfileOutput({ info: info.status === 'fulfilled' ? info.value : String(info.reason), finance: finance.status === 'fulfilled' ? finance.value : String(finance.reason) });
    } catch (error) {
      setProfileOutput(error instanceof Error ? error.message : '查询失败');
    }
  }

  async function loadHistory(mode: 'tdx' | 'ths' | 'history' | 'trade' | 'minuteTrade') {
    try {
      const query = `code=${encodeURIComponent(historyCode)}&type=${encodeURIComponent(historyType)}&limit=${encodeURIComponent(historyLimit)}`;
      const path =
        mode === 'tdx'
          ? `/api/kline-all/tdx?${query}`
          : mode === 'ths'
            ? `/api/kline-all/ths?${query}`
            : mode === 'history'
              ? `/api/kline-history?${query}`
              : mode === 'trade'
                ? `/api/trade-history?code=${encodeURIComponent(historyCode)}`
                : `/api/minute-trade-all?code=${encodeURIComponent(historyCode)}`;
      setHistoryOutput(await apiFetch(path));
    } catch (error) {
      setHistoryOutput(error instanceof Error ? error.message : '加载失败');
    }
  }

  async function loadBlock(mode: 'block' | 'hy' | 'stat' | 'stat2' | 'xgsg') {
    try {
      const path =
        mode === 'block'
          ? `/api/block?file=${encodeURIComponent(blockFile)}&with_index=${blockWithIndex ? '1' : '0'}`
          : mode === 'hy'
            ? '/api/tdx-hy'
            : mode === 'stat'
              ? '/api/tdx-stat'
              : mode === 'stat2'
                ? '/api/tdx-stat2'
                : '/api/xgsg';
      setBlockOutput(await apiFetch(path));
    } catch (error) {
      setBlockOutput(error instanceof Error ? error.message : '查询失败');
    }
  }

  async function loadHikyuu(path = '/api/hikyuu/health') {
    try {
      setHikyuuStatus(await apiFetch(path));
    } catch (error) {
      setHikyuuStatus(error instanceof Error ? error.message : '数据服务不可用');
    }
  }

  const status = overview.status as Record<string, unknown> | undefined;
  const count = overview.count as Record<string, unknown> | undefined;
  const stats = overview.stats as Record<string, unknown> | undefined;

  return (
    <div className="data-center-grid">
      <Card className="work-card" title="市场总览" extra={<Button icon={<ReloadOutlined />} onClick={loadOverview}>刷新</Button>}>
        <div className="metric-strip">
          <Statistic title="服务状态" value={String(status?.status || '--')} />
          <Statistic title="市场证券数" value={Number(count?.total || 0)} />
          <Statistic title="沪市股票" value={Number((stats?.sh as Record<string, unknown> | undefined)?.total || 0)} />
          <Statistic title="深市股票" value={Number((stats?.sz as Record<string, unknown> | undefined)?.total || 0)} />
        </div>
        <Space wrap className="toolbar-row">
          <Select value={exchange} onChange={setExchange} options={[
            { value: 'all', label: '全部市场' },
            { value: 'sh', label: '沪市' },
            { value: 'sz', label: '深市' },
            { value: 'bj', label: '北交所' }
          ]} />
          <InputNumber min={1} max={500} value={limit} onChange={(value) => setLimit(Number(value || 80))} />
          <Button type="primary" icon={<DatabaseOutlined />} loading={directoryLoading} onClick={() => loadDirectory('codes')}>代码目录</Button>
          <Button onClick={() => loadDirectory('etf')}>ETF</Button>
        </Space>
        <Table
          size="small"
          rowKey={directoryRowKey}
          loading={directoryLoading}
          dataSource={directoryRows}
          pagination={{ pageSize: 12, size: 'small' }}
          columns={[
            { title: '代码', dataIndex: 'code', render: (value, record) => value || record.Code || record.value },
            { title: '名称', dataIndex: 'name', render: (value, record) => value || record.Name || '--' },
            { title: '市场', dataIndex: 'exchange', render: (value, record) => value || record.Exchange || '--' }
          ]}
        />
      </Card>

      <Card className="work-card" title="Hikyuu 行情数据" extra={<Button onClick={() => loadHikyuu()}>刷新</Button>}>
        <Space wrap className="toolbar-row">
          <Button type="primary" onClick={() => loadHikyuu('/api/hikyuu/tasks/full-sync')}>全量同步</Button>
          <Button onClick={() => loadHikyuu('/api/hikyuu/tasks/after-close-sync')}>盘后同步</Button>
          <Button onClick={() => loadHikyuu('/api/hikyuu/tasks')}>刷新任务</Button>
        </Space>
        <JsonPane value={hikyuuStatus} />
      </Card>

      <Card className="work-card" title="个股资料" extra={<Button icon={<SearchOutlined />} onClick={loadProfile}>查询</Button>}>
        <Space wrap className="toolbar-row">
          <Input value={profileCode} onChange={(event) => setProfileCode(event.target.value)} />
          <Button type="primary" onClick={loadProfile}>财务/F10/股本</Button>
        </Space>
        <JsonPane value={profileOutput} />
      </Card>

      <Card className="work-card" title="历史数据">
        <Form layout="inline" className="chart-control-form">
          <Form.Item label="代码"><Input value={historyCode} onChange={(event) => setHistoryCode(event.target.value)} /></Form.Item>
          <Form.Item label="周期"><Select value={historyType} onChange={setHistoryType} options={[
            { value: 'day', label: '日K' },
            { value: 'week', label: '周K' },
            { value: 'month', label: '月K' },
            { value: 'minute30', label: '30分钟' },
            { value: 'minute5', label: '5分钟' }
          ]} /></Form.Item>
          <Form.Item label="数量"><InputNumber min={1} max={2000} value={historyLimit} onChange={(value) => setHistoryLimit(Number(value || 120))} /></Form.Item>
        </Form>
        <Space wrap className="toolbar-row">
          <Button type="primary" onClick={() => loadHistory('tdx')}>TDX K线</Button>
          <Button type="primary" onClick={() => loadHistory('ths')}>THS前复权</Button>
          <Button onClick={() => loadHistory('history')}>分页K线</Button>
          <Button onClick={() => loadHistory('trade')}>历史成交</Button>
          <Button onClick={() => loadHistory('minuteTrade')}>全天分时成交</Button>
        </Space>
        <DynamicTable rows={rowsFrom(historyOutput)} scrollY={320} />
        {!rowsFrom(historyOutput).length ? <JsonPane value={historyOutput} /> : null}
      </Card>

      <Card className="work-card" title="板块行业">
        <Space wrap className="toolbar-row">
          <Select value={blockFile} onChange={setBlockFile} options={[
            { value: 'gn', label: '概念板块' },
            { value: 'fg', label: '地域风格' },
            { value: 'zs', label: '指数板块' },
            { value: 'hy', label: '行业板块' }
          ]} />
          <Checkbox checked={blockWithIndex} onChange={(event) => setBlockWithIndex(event.target.checked)}>含指数代码</Checkbox>
          <Button type="primary" onClick={() => loadBlock('block')}>板块成分</Button>
          <Button onClick={() => loadBlock('hy')}>行业归属</Button>
          <Button onClick={() => loadBlock('stat')}>个股统计</Button>
          <Button onClick={() => loadBlock('stat2')}>资金流向</Button>
          <Button onClick={() => loadBlock('xgsg')}>新股申购</Button>
        </Space>
        <DynamicTable rows={rowsFrom(blockOutput)} scrollY={320} />
        {!rowsFrom(blockOutput).length ? <JsonPane value={blockOutput} /> : null}
      </Card>
    </div>
  );
}
