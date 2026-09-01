import { Button, Card, Checkbox, Form, Input, Select, Space, Table, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { AutomationRun, AutomationTask, Formula, StockPool, Strategy, Webhook } from '../types';

const { Text } = Typography;

export function AutomationsWorkspace() {
  const [tasks, setTasks] = useState<AutomationTask[]>([]);
  const [runs, setRuns] = useState<AutomationRun[]>([]);
  const [pools, setPools] = useState<StockPool[]>([]);
  const [formulas, setFormulas] = useState<Formula[]>([]);
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [payloadOutput, setPayloadOutput] = useState<unknown>('暂无运行');
  const [form] = Form.useForm<AutomationTask>();

  const load = async () => {
    try {
      const [taskList, runList, poolList, formulaList, strategyList, webhookList] = await Promise.all([
        apiFetch<AutomationTask[]>('/api/automations'),
        apiFetch<AutomationRun[]>('/api/automations/runs?limit=50'),
        apiFetch<StockPool[]>('/api/stock-pools'),
        apiFetch<Formula[]>('/api/formulas'),
        apiFetch<Strategy[]>('/api/strategies'),
        apiFetch<Webhook[]>('/api/webhooks')
      ]);
      setTasks(taskList || []);
      setRuns(runList || []);
      setPools(poolList || []);
      setFormulas(formulaList || []);
      setStrategies(strategyList || []);
      setWebhooks(webhookList || []);
      if (taskList[0]) form.setFieldsValue(taskList[0]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '自动化加载失败');
    }
  };

  useEffect(() => { load(); }, []);

  async function saveTask(values: AutomationTask) {
    await apiFetch(values.id ? `/api/automations/${values.id}` : '/api/automations', {
      method: values.id ? 'PUT' : 'POST',
      body: JSON.stringify(values)
    });
    message.success('任务已保存');
    await load();
  }

  async function runTask(id: string) {
    const run = await apiFetch(`/api/automations/${id}/run`, { method: 'POST' });
    setPayloadOutput(run);
    await load();
  }

  async function toggleTask(id: string, enabled: boolean) {
    await apiFetch(`/api/automations/${id}/enabled`, { method: 'POST', body: JSON.stringify({ enabled }) });
    await load();
  }

  async function deleteTask(id: string) {
    await apiFetch(`/api/automations/${id}`, { method: 'DELETE' });
    await load();
  }

  async function createTemplate(template: string) {
    await apiFetch('/api/automations/templates', { method: 'POST', body: JSON.stringify({ template }) });
    await load();
  }

  return (
    <div className="automation-layout">
      <Card className="work-card" title="股票池">
        <Table size="small" rowKey="id" dataSource={pools} pagination={{ pageSize: 8, size: 'small' }}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '代码数', render: (_value, record) => record.symbols.length },
            { title: '说明', dataIndex: 'description' }
          ]} />
      </Card>
      <Card className="work-card" title="自动化任务">
        <Form form={form} layout="vertical" onFinish={saveTask}>
          <div className="form-grid two">
            <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="type" label="类型"><Select options={[
              { value: 'stock_selection', label: '选股任务' },
              { value: 'strategy_selection', label: '策略选股' },
              { value: 'system_sync', label: '系统同步' },
              { value: 'custom', label: '自定义任务' }
            ]} /></Form.Item>
            <Form.Item name="cron" label="Cron" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked"><Checkbox /></Form.Item>
          </div>
          <Form.Item name="payload_json" label="Payload JSON"><Input.TextArea rows={6} className="formula-script-input" /></Form.Item>
          <Form.Item name="webhook_ids" label="Webhook ID 列表"><Input.TextArea rows={2} className="formula-script-input" /></Form.Item>
          <Space wrap>
            <Button type="primary" htmlType="submit">保存任务</Button>
            <Button onClick={() => createTemplate('morning_sync')}>早盘基础同步</Button>
            <Button onClick={() => createTemplate('evening_kline')}>晚盘日K同步</Button>
            <Button onClick={() => createTemplate('evening_full')}>晚盘完整同步</Button>
          </Space>
        </Form>
      </Card>
      <Card className="work-card" title="任务列表">
        <Table size="small" rowKey="id" dataSource={tasks} pagination={{ pageSize: 8, size: 'small' }}
          onRow={(record) => ({ onClick: () => form.setFieldsValue(record) })}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'type' },
            { title: 'Cron', dataIndex: 'cron' },
            { title: '状态', render: (_value, record) => record.enabled ? '启用' : '停用' },
            {
              title: '操作',
              render: (_value, record) => (
                <Space>
                  <Button onClick={() => runTask(record.id)}>运行</Button>
                  <Button onClick={() => toggleTask(record.id, !record.enabled)}>{record.enabled ? '关闭' : '开启'}</Button>
                  {!record.readonly ? <Button danger onClick={() => deleteTask(record.id)}>删除</Button> : null}
                </Space>
              )
            }
          ]} />
      </Card>
      <Card className="work-card" title="运行记录">
        <Table size="small" rowKey="id" dataSource={runs} pagination={{ pageSize: 10, size: 'small' }}
          columns={[
            { title: '任务', dataIndex: 'task_name' },
            { title: '状态', dataIndex: 'status' },
            { title: '命中', dataIndex: 'matched_count' },
            { title: '开始', dataIndex: 'started_at' }
          ]} />
        <JsonPane value={payloadOutput} />
      </Card>
    </div>
  );
}
