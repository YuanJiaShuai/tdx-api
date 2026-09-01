import { Button, Card, Form, Input, Select, Space, Table, Typography, message, Switch } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { AICredential, AIProvider } from '../types';

const { Text } = Typography;

export function AIConfigsWorkspace() {
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const [credentials, setCredentials] = useState<AICredential[]>([]);
  const [selected, setSelected] = useState<AICredential | null>(null);
  const [testOutput, setTestOutput] = useState<unknown>('尚未测试');
  const [form] = Form.useForm<AICredential>();

  const load = async () => {
    try {
      const [providerList, credentialList] = await Promise.all([
        apiFetch<AIProvider[]>('/api/ai/providers'),
        apiFetch<AICredential[]>('/api/ai/credentials')
      ]);
      setProviders(providerList || []);
      setCredentials(credentialList || []);
      setSelected((current) => current || credentialList[0] || null);
      if (credentialList[0]) form.setFieldsValue(credentialList[0]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'AI 配置加载失败');
    }
  };

  useEffect(() => { load(); }, []);

  const providerOptions = useMemo(() => providers.map((item) => ({ value: item.id, label: item.name || item.id })), [providers]);

  async function save(values: AICredential) {
    await apiFetch(values.id ? `/api/ai/credentials/${values.id}` : '/api/ai/credentials', {
      method: values.id ? 'PUT' : 'POST',
      body: JSON.stringify(values)
    });
    await load();
  }

  async function test(id: string) {
    const item = credentials.find((value) => value.id === id);
    const data = await apiFetch(`/api/ai/credentials/${id}/test`, {
      method: 'POST',
      body: JSON.stringify({ provider: item?.provider, model: item?.model })
    });
    setTestOutput(data);
  }

  return (
    <div className="ai-config-layout">
      <Card className="work-card" title="添加 AI 模型">
        <Form form={form} layout="vertical" onFinish={save}>
          <div className="form-grid two">
            <Form.Item name="name" label="配置名称"><Input /></Form.Item>
            <Form.Item name="provider" label="供应商"><Select options={providerOptions} /></Form.Item>
            <Form.Item name="base_url" label="接口地址"><Input /></Form.Item>
            <Form.Item name="model" label="模型"><Input /></Form.Item>
            <Form.Item name="api_key" label="API Key"><Input.Password /></Form.Item>
            <Form.Item name="api_secret" label="API Secret"><Input.Password /></Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
          </div>
          <Form.Item name="extra_json" label="额外 JSON"><Input.TextArea rows={4} /></Form.Item>
          <Button type="primary" htmlType="submit">保存配置</Button>
        </Form>
      </Card>
      <Card className="work-card" title="已配置模型">
        <Table size="small" rowKey="id" dataSource={credentials} pagination={{ pageSize: 8, size: 'small' }}
          onRow={(record) => ({ onClick: () => { setSelected(record); form.setFieldsValue(record); } })}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '供应商', dataIndex: 'provider' },
            { title: '模型', dataIndex: 'model' },
            { title: '启用', render: (_value, record) => record.enabled ? '是' : '否' },
            { title: 'Key', dataIndex: 'api_key_masked' },
            { title: '操作', render: (_value, record) => <Space><Button onClick={() => test(record.id)}>测试</Button></Space> }
          ]} />
      </Card>
      <Card className="work-card" title="连接测试结果">
        <JsonPane value={testOutput} />
      </Card>
    </div>
  );
}
