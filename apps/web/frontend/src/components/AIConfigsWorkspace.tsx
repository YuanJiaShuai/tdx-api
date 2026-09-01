import { Button, Card, Form, Input, Modal, Select, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { ApiOutlined, CheckCircleOutlined, CloseOutlined, EditOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
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
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<AICredential | null>(null);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<AICredential>();

  const load = async () => {
    setLoading(true);
    try {
      const [providerList, credentialList] = await Promise.all([
        apiFetch<AIProvider[]>('/api/ai/providers'),
        apiFetch<AICredential[]>('/api/ai/credentials')
      ]);
      setProviders(providerList || []);
      setCredentials(credentialList || []);
      setSelected((current) => current || credentialList[0] || null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'AI 配置加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const providerOptions = useMemo(() => providers.map((item) => ({ value: item.id, label: item.name || item.id })), [providers]);

  function openDialog(credential?: AICredential) {
    const next = credential || {
      id: '',
      name: '',
      provider: providerOptions[0]?.value || '',
      base_url: '',
      model: '',
      api_key: '',
      api_secret: '',
      enabled: true,
      extra_json: ''
    };
    setEditing(credential || null);
    form.resetFields();
    form.setFieldsValue(next);
    setDialogOpen(true);
  }

  async function save(values: AICredential) {
    try {
      await apiFetch(values.id ? `/api/ai/credentials/${values.id}` : '/api/ai/credentials', {
        method: values.id ? 'PUT' : 'POST',
        body: JSON.stringify(values)
      });
      message.success(values.id ? '模型配置已更新' : '模型配置已添加');
      setDialogOpen(false);
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '模型配置保存失败');
    }
  }

  async function test(id: string) {
    const item = credentials.find((value) => value.id === id);
    setSelected(item || null);
    try {
      const data = await apiFetch(`/api/ai/credentials/${id}/test`, {
        method: 'POST',
        body: JSON.stringify({ provider: item?.provider, model: item?.model })
      });
      setTestOutput(data);
      message.success('连接测试完成');
    } catch (error) {
      const text = error instanceof Error ? error.message : '连接测试失败';
      setTestOutput(text);
      message.error(text);
    }
  }

  return (
    <div className="ai-config-workspace">
      <Card
        className="work-card ai-config-overview"
        title={
          <div className="ai-config-panel-title">
            <div><span>AI 模型</span><Text type="secondary">连接与调用配置</Text></div>
            <small>MODEL ROUTER</small>
          </div>
        }
        extra={
          <Space>
            <Button aria-label="刷新模型配置" icon={<ReloadOutlined />} onClick={load} loading={loading} />
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openDialog()}>添加模型</Button>
          </Space>
        }
      >
        <div className="ai-config-overview-grid">
          <div className="ai-config-lead">
            <span>当前路由</span>
            <strong>{selected?.name || credentials[0]?.name || '尚未配置'}</strong>
            <small>{selected ? `${selected.provider || '--'} · ${selected.model || '--'}` : '添加一个可用模型后开始调用'}</small>
          </div>
          <div><span>连接数</span><strong>{credentials.length}</strong><small>个配置</small></div>
          <div><span>启用数</span><strong>{credentials.filter((item) => item.enabled).length}</strong><small>可调用</small></div>
          <div><span>供应商</span><strong>{new Set(credentials.map((item) => item.provider).filter(Boolean)).size}</strong><small>已接入</small></div>
          <div className="ai-config-overview-status">
            <CheckCircleOutlined />
            <div><strong>配置中心正常</strong><small>凭据仅显示掩码</small></div>
          </div>
        </div>
      </Card>

      <div className="ai-config-main-grid">
        <Card
          className="work-card ai-config-list-card"
          title={
            <div className="ai-config-panel-title">
              <div><span>已配置模型</span><Text type="secondary">{credentials.length} 个连接</Text></div>
              <small>ROUTES</small>
            </div>
          }
        >
          <Table
            size="middle"
            rowKey="id"
            dataSource={credentials}
            loading={loading}
            pagination={{ pageSize: 8, size: 'small', showSizeChanger: false }}
            rowClassName={(record) => (record.id === selected?.id ? 'row-active' : '')}
            onRow={(record) => ({ onClick: () => setSelected(record) })}
            locale={{ emptyText: '暂无模型配置，先添加一个连接' }}
            columns={[
              {
                title: '配置名称',
                dataIndex: 'name',
                width: 156,
                render: (value: string, record: AICredential) => (
                  <div className="ai-model-name-cell"><strong>{value || '--'}</strong><span>{record.id || '--'}</span></div>
                )
              },
              { title: '供应商', dataIndex: 'provider', width: 108, render: (value: string) => value || '--' },
              { title: '模型', dataIndex: 'model', width: 156, render: (value: string) => <span className="ai-model-code">{value || '--'}</span> },
              {
                title: '状态',
                width: 88,
                render: (_value, record) => <Tag className={`ai-model-status-tag ${record.enabled ? 'is-enabled' : ''}`}>{record.enabled ? '已启用' : '已停用'}</Tag>
              },
              { title: 'API Key', dataIndex: 'api_key_masked', width: 132, render: (value: string) => <span className="ai-model-code">{value || '未设置'}</span> },
              {
                title: '操作',
                width: 138,
                render: (_value, record) => (
                  <Space size={4}>
                    <Button size="small" icon={<ApiOutlined />} onClick={(event) => { event.stopPropagation(); test(record.id); }}>测试</Button>
                    <Button size="small" icon={<EditOutlined />} onClick={(event) => { event.stopPropagation(); openDialog(record); }}>编辑</Button>
                  </Space>
                )
              }
            ]}
            scroll={{ x: 778 }}
          />
        </Card>

        <Card
          className="work-card ai-config-test-card"
          title={
            <div className="ai-config-panel-title">
              <div><span>连接测试</span><Text type="secondary">{selected?.name || '未选择模型'}</Text></div>
              <small>PROBE</small>
            </div>
          }
          extra={selected ? <Button size="small" icon={<ApiOutlined />} onClick={() => test(selected.id)}>重新测试</Button> : null}
        >
          <div className="ai-config-test-intro">
            <ApiOutlined />
            <div><strong>{selected ? '检查供应商连接与模型响应' : '选择一个模型查看测试结果'}</strong><span>服务端探针，不展示完整凭据</span></div>
          </div>
          <JsonPane value={testOutput} />
        </Card>
      </div>

      <Modal
        open={dialogOpen}
        onCancel={() => setDialogOpen(false)}
        footer={null}
        width={940}
        centered
        destroyOnHidden
        closeIcon={<CloseOutlined />}
        className="ai-model-edit-modal"
        title={
          <div className="quote-dialog-title ai-model-dialog-title">
            <div>
              <strong>{editing ? '编辑 AI 模型' : '添加 AI 模型'}</strong>
              <span>{editing?.name || '连接配置'}</span>
            </div>
            <small>供应商连接 · 凭据管理</small>
          </div>
        }
      >
        <Form form={form} layout="vertical" onFinish={save} className="ai-model-form">
          <section className="ai-model-form-section">
            <div className="ai-model-form-section-head">
              <div><strong>路由信息</strong><span>定义模型由谁提供、从哪里调用</span></div>
              <small>ROUTE</small>
            </div>
            <div className="ai-model-form-grid ai-model-form-grid-connection">
              <Form.Item name="name" label="配置名称" rules={[{ required: true, message: '请输入配置名称' }]}><Input placeholder="例如：DeepSeek 主模型" /></Form.Item>
              <Form.Item name="provider" label="供应商" rules={[{ required: true, message: '请选择供应商' }]}><Select options={providerOptions} placeholder="选择供应商" /></Form.Item>
              <Form.Item name="model" label="模型标识" rules={[{ required: true, message: '请输入模型标识' }]}><Input placeholder="例如：deepseek-chat" /></Form.Item>
              <Form.Item name="base_url" label="接口地址"><Input placeholder="https://api.example.com" /></Form.Item>
            </div>
          </section>

          <section className="ai-model-form-section">
            <div className="ai-model-form-section-head">
              <div><strong>访问凭据</strong><span>密钥只用于服务端请求，列表中始终掩码显示</span></div>
              <small>AUTH</small>
            </div>
            <div className="ai-model-form-grid ai-model-form-grid-credentials">
              <Form.Item name="api_key" label="API Key"><Input.Password placeholder={editing ? '留空则保持原密钥' : '输入 API Key'} /></Form.Item>
              <Form.Item name="api_secret" label="API Secret"><Input.Password placeholder={editing ? '留空则保持原密钥' : '可选' } /></Form.Item>
              <Form.Item name="enabled" label="启用模型" valuePropName="checked"><Switch /></Form.Item>
            </div>
          </section>

          <section className="ai-model-form-section">
            <div className="ai-model-form-section-head">
              <div><strong>调用参数</strong><span>兼容供应商额外请求字段</span></div>
              <small>PARAMS</small>
            </div>
            <div className="ai-model-form-grid ai-model-form-grid-extra">
              <Form.Item name="extra_json" label="额外 JSON"><Input.TextArea rows={4} placeholder={'例如：{"temperature": 0.2}'} /></Form.Item>
            </div>
          </section>

          <div className="ai-model-form-actions">
            <Text type="secondary">保存后会更新模型路由列表</Text>
            <Space>
              <Button onClick={() => setDialogOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存配置</Button>
            </Space>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
