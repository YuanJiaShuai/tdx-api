import { Button, Card, Form, Input, Space, Table, Typography, message, Switch } from 'antd';
import { useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { Webhook } from '../types';

export function WebhooksWorkspace() {
  const [items, setItems] = useState<Webhook[]>([]);
  const [testOutput, setTestOutput] = useState<unknown>('尚未测试');
  const [form] = Form.useForm<Webhook>();

  const load = async () => {
    try {
      setItems(await apiFetch<Webhook[]>('/api/webhooks'));
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Webhook 加载失败');
    }
  };

  useEffect(() => { load(); }, []);

  async function save(values: Webhook) {
    await apiFetch(values.id ? `/api/webhooks/${values.id}` : '/api/webhooks', {
      method: values.id ? 'PUT' : 'POST',
      body: JSON.stringify(values)
    });
    await load();
  }

  async function test(id: string) {
    setTestOutput(await apiFetch(`/api/webhooks/${id}/test`, { method: 'POST' }));
  }

  return (
    <div className="webhook-layout">
      <Card className="work-card" title="Webhook 通知">
        <Form form={form} layout="vertical" onFinish={save}>
          <div className="form-grid two">
            <Form.Item name="name" label="名称"><Input /></Form.Item>
            <Form.Item name="url" label="URL"><Input /></Form.Item>
            <Form.Item name="method" label="方法"><Input /></Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
          </div>
          <Form.Item name="headers_json" label="请求头 JSON"><Input.TextArea rows={4} /></Form.Item>
          <Form.Item name="events" label="事件 JSON"><Input.TextArea rows={4} /></Form.Item>
          <Button type="primary" htmlType="submit">保存 Webhook</Button>
        </Form>
      </Card>
      <Card className="work-card" title="Webhook 列表">
        <Table size="small" rowKey="id" dataSource={items} pagination={{ pageSize: 8, size: 'small' }}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: 'URL', dataIndex: 'url' },
            { title: '启用', render: (_value, record) => record.enabled ? '是' : '否' },
            { title: '操作', render: (_value, record) => <Space><Button onClick={() => test(record.id)}>测试</Button></Space> }
          ]}
          onRow={(record) => ({ onClick: () => form.setFieldsValue(record) })}
        />
      </Card>
      <Card className="work-card" title="测试结果">
        <JsonPane value={testOutput} />
      </Card>
    </div>
  );
}
