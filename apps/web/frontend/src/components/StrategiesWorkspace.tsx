import { Button, Card, Checkbox, Form, Input, InputNumber, Select, Space, Table, Typography, message } from 'antd';
import { useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { Strategy } from '../types';

const { Text } = Typography;

export function StrategiesWorkspace() {
  const [items, setItems] = useState<Strategy[]>([]);
  const [factors, setFactors] = useState<Array<Record<string, unknown>>>([]);
  const [selected, setSelected] = useState<Strategy | null>(null);
  const [backtestOutput, setBacktestOutput] = useState<unknown>('暂无回测');
  const [form] = Form.useForm<Strategy>();

  const load = async () => {
    try {
      const [strategies, factorDefs] = await Promise.all([
        apiFetch<Strategy[]>('/api/strategies'),
        apiFetch<Array<Record<string, unknown>>>('/api/factors')
      ]);
      setItems(strategies || []);
      setFactors(factorDefs || []);
      setSelected((current) => current || strategies[0] || null);
      if (strategies[0]) form.setFieldsValue(strategies[0]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '策略加载失败');
    }
  };

  useEffect(() => { load(); }, []);

  async function saveStrategy(values: Strategy) {
    await apiFetch(values.id ? `/api/strategies/${values.id}` : '/api/strategies', {
      method: values.id ? 'PUT' : 'POST',
      body: JSON.stringify(values)
    });
    message.success('策略已保存');
    await load();
  }

  async function cloneStrategy(id: string) {
    await apiFetch(`/api/strategies/${id}/clone`, { method: 'POST' });
    await load();
  }

  async function runStrategy(id: string) {
    const run = await apiFetch(`/api/strategies/${id}/run`, { method: 'POST' });
    setBacktestOutput(run);
  }

  async function backtestStrategy(id: string) {
    const run = await apiFetch(`/api/strategies/${id}/backtest`, {
      method: 'POST',
      body: JSON.stringify({ strategy_id: id })
    });
    setBacktestOutput(run);
  }

  return (
    <div className="strategy-layout">
      <Card className="work-card strategy-list-card" title="策略列表">
        <Table
          size="small"
          rowKey="id"
          dataSource={items}
          pagination={{ pageSize: 8, size: 'small' }}
          onRow={(record) => ({ onClick: () => { setSelected(record); form.setFieldsValue(record); } })}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '说明', dataIndex: 'description' },
            { title: '启用', dataIndex: 'enabled', render: (value) => (value ? '是' : '否') }
          ]}
        />
      </Card>

      <Card className="work-card strategy-editor-card" title="策略编辑">
        <Form form={form} layout="vertical" onFinish={saveStrategy}>
          <div className="form-grid two">
            <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked"><Checkbox /></Form.Item>
          </div>
          <Form.Item name="description" label="说明"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="config_json" label="JSON 配置" rules={[{ required: true }]}><Input.TextArea rows={14} /></Form.Item>
          <Space wrap>
            <Button type="primary" htmlType="submit">保存</Button>
            <Button onClick={() => selected?.id && cloneStrategy(selected.id)}>复制</Button>
            <Button onClick={() => selected?.id && runStrategy(selected.id)}>运行</Button>
            <Button onClick={() => selected?.id && backtestStrategy(selected.id)}>回测</Button>
          </Space>
        </Form>
      </Card>

      <Card className="work-card strategy-side" title="因子库">
        <Table size="small" rowKey={(record) => String(record.id)} dataSource={factors} pagination={{ pageSize: 8, size: 'small' }}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'kind' },
            { title: '说明', dataIndex: 'description' }
          ]} />
      </Card>

      <Card className="work-card" title="运行结果">
        <JsonPane value={backtestOutput} />
      </Card>
    </div>
  );
}
