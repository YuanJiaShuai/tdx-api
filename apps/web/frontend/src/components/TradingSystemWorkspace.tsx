import { Button, Card, Checkbox, Form, Input, InputNumber, Modal, Select, Space, Statistic, Table, Typography, message } from 'antd';
import { PlusOutlined, SaveOutlined } from '@ant-design/icons';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { DynamicTable } from './DynamicTable';
import type { TradingSystemState, TradingTrade } from '../types';

const { Text } = Typography;

export function TradingSystemWorkspace() {
  const [state, setState] = useState<TradingSystemState | null>(null);
  const [loading, setLoading] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<TradingTrade | null>(null);
  const [form] = Form.useForm<TradingTrade>();

  const load = async () => {
    setLoading(true);
    try {
      setState(await apiFetch<TradingSystemState>('/api/trading-system'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '交易系统加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const trades = state?.trades || [];

  const stats = useMemo(() => {
    const account = state?.account || { principal: 0, totalAssets: 0, marketValue: 0, dailyProfit: 0, maxTradeRisk: 0, maxPositionWeight: 0 };
    return account;
  }, [state]);

  function openTradeDialog(trade?: TradingTrade) {
    setEditing(trade || null);
    form.setFieldsValue(trade || {
      id: '',
      stockName: '',
      stockCode: '',
      status: 'active',
      entryDate: '',
      entryPrice: 0,
      currentPrice: 0,
      shares: 0,
      invalidPrice: 0,
      positionLabel: '试错仓',
      targetOne: '',
      targetTwo: '',
      tradeMode: '',
      buyReason: '',
      exitRules: '',
      review: ''
    });
    setDialogOpen(true);
  }

  async function saveTrade(values: TradingTrade) {
    if (!state) return;
    const nextTrades = [...trades];
    const id = values.id || `trade-${Date.now()}`;
    const trade = { ...values, id };
    const index = nextTrades.findIndex((item) => item.id === id);
    if (index >= 0) nextTrades[index] = trade;
    else nextTrades.unshift(trade);
    const payload = { ...state, trades: nextTrades };
    await apiFetch('/api/trading-system', { method: 'PUT', body: JSON.stringify(payload) });
    message.success('交易已保存');
    setDialogOpen(false);
    await load();
  }

  return (
    <div className="trading-layout">
      <Card className="work-card trading-side" title="账户与纪律" extra={<Button onClick={load}>刷新</Button>}>
        <div className="metric-strip">
          <Statistic title="本金" value={stats.principal} precision={2} />
          <Statistic title="总资产" value={stats.totalAssets} precision={2} />
          <Statistic title="总市值" value={stats.marketValue} precision={2} />
          <Statistic title="当日盈亏" value={stats.dailyProfit} precision={2} />
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openTradeDialog()}>新建交易</Button>
        <Table
          size="small"
          loading={loading}
          rowKey="id"
          dataSource={trades}
          pagination={{ pageSize: 8, size: 'small' }}
          columns={[
            { title: '名称', dataIndex: 'stockName' },
            { title: '代码', dataIndex: 'stockCode' },
            { title: '状态', dataIndex: 'status' },
            { title: '买入价', dataIndex: 'entryPrice' },
            { title: '现价', dataIndex: 'currentPrice' },
            {
              title: '操作',
              render: (_value, record) => <Button onClick={() => openTradeDialog(record)}>编辑</Button>
            }
          ]}
        />
      </Card>

      <Card className="work-card trading-main" title="交易卡">
        <DynamicTable<TradingTrade> rows={trades} scrollY={520} />
      </Card>

      <Modal open={dialogOpen} title={editing ? '编辑交易' : '新建交易'} onCancel={() => setDialogOpen(false)} footer={null} width={960}>
        <Form form={form} layout="vertical" onFinish={saveTrade}>
          <div className="form-grid two">
            <Form.Item name="stockName" label="股票名称" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="stockCode" label="代码" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="status" label="状态"><Select options={[{ value: 'active', label: '持仓' }, { value: 'closed', label: '已清仓' }]} /></Form.Item>
            <Form.Item name="entryDate" label="买入日期"><Input type="date" /></Form.Item>
            <Form.Item name="entryPrice" label="买入价"><InputNumber style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="shares" label="股数"><InputNumber style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="currentPrice" label="当前价"><InputNumber style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="invalidPrice" label="技术无效点"><InputNumber style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="positionLabel" label="仓位标签"><Select options={[{ value: '试错仓' }, { value: '确认仓' }, { value: '趋势仓' }, { value: '观察仓' }]} /></Form.Item>
            <Form.Item name="targetOne" label="第一观察/压力位"><Input /></Form.Item>
            <Form.Item name="targetTwo" label="强压力/止盈区"><Input /></Form.Item>
          </div>
          <Form.Item name="tradeMode" label="交易模式"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="buyReason" label="买入理由"><Input.TextArea rows={4} /></Form.Item>
          <Form.Item name="exitRules" label="退出/加仓规则"><Input.TextArea rows={4} /></Form.Item>
          <Form.Item name="review" label="盘后复盘"><Input.TextArea rows={4} /></Form.Item>
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存交易</Button>
        </Form>
      </Modal>
    </div>
  );
}
