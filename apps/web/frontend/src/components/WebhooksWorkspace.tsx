import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { ApiOutlined, BellOutlined, CheckCircleOutlined, CloseOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { Webhook } from '../types';

const { Text } = Typography;

const methodOptions = [
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' }
];

function parseEventCount(value?: string) {
  try {
    const events = JSON.parse(value || '[]');
    return Array.isArray(events) ? events.length : 0;
  } catch {
    return 0;
  }
}

function endpointKind(url?: string) {
  const text = (url || '').toLowerCase();
  if (text.includes('open.feishu.cn') || text.includes('open.larksuite.com')) return '飞书';
  if (text.startsWith('https://')) return 'HTTPS';
  if (text.startsWith('http://')) return 'HTTP';
  return '未识别';
}

export function WebhooksWorkspace() {
  const [items, setItems] = useState<Webhook[]>([]);
  const [selected, setSelected] = useState<Webhook | null>(null);
  const [testOutput, setTestOutput] = useState<unknown>('尚未测试');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Webhook | null>(null);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<Webhook>();

  const load = async () => {
    setLoading(true);
    try {
      const data = await apiFetch<Webhook[]>('/api/webhooks');
      setItems(data || []);
      setSelected((current) => current || data[0] || null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Webhook 加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const enabledCount = useMemo(() => items.filter((item) => item.enabled).length, [items]);
  const eventCount = useMemo(() => items.reduce((sum, item) => sum + parseEventCount(item.events), 0), [items]);

  function openDialog(item?: Webhook) {
    const next = item || {
      id: '',
      name: '',
      url: '',
      method: 'POST',
      headers_json: '{}',
      events: '["automation.failed","automation.finished","stock_selection.finished","macro_event.alert_due","macro_event.window_started"]',
      enabled: true
    };
    setEditing(item || null);
    form.resetFields();
    form.setFieldsValue(next);
    setDialogOpen(true);
  }

  async function save(values: Webhook) {
    try {
      await apiFetch(values.id ? `/api/webhooks/${values.id}` : '/api/webhooks', {
        method: values.id ? 'PUT' : 'POST',
        body: JSON.stringify(values)
      });
      message.success(values.id ? 'Webhook 已更新' : 'Webhook 已添加');
      setDialogOpen(false);
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Webhook 保存失败');
    }
  }

  async function test(id: string) {
    const item = items.find((value) => value.id === id);
    setSelected(item || null);
    try {
      const data = await apiFetch(`/api/webhooks/${id}/test`, { method: 'POST' });
      setTestOutput(data);
      message.success('Webhook 测试完成');
    } catch (error) {
      const text = error instanceof Error ? error.message : 'Webhook 测试失败';
      setTestOutput(text);
      message.error(text);
    }
  }

  async function remove(id: string) {
    try {
      await apiFetch(`/api/webhooks/${id}`, { method: 'DELETE' });
      message.success('Webhook 已删除');
      if (selected?.id === id) setSelected(null);
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Webhook 删除失败');
    }
  }

  return (
    <div className="webhook-workspace">
      <Card
        className="work-card webhook-overview"
        title={
          <div className="webhook-panel-title">
            <div><span>Webhook</span><Text type="secondary">自动化通知通道</Text></div>
            <small>DELIVERY BUS</small>
          </div>
        }
        extra={
          <Space>
            <Button aria-label="刷新 Webhook" icon={<ReloadOutlined />} onClick={load} loading={loading} />
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openDialog()}>添加通道</Button>
          </Space>
        }
      >
        <div className="webhook-overview-grid">
          <div className="webhook-lead">
            <span>当前通道</span>
            <strong>{selected?.name || items[0]?.name || '尚未配置'}</strong>
            <small>{selected ? `${endpointKind(selected.url)} · ${selected.method || 'POST'}` : '添加一个通知通道后接收任务结果'}</small>
          </div>
          <div><span>通道数</span><strong>{items.length}</strong><small>个配置</small></div>
          <div><span>启用数</span><strong>{enabledCount}</strong><small>可发送</small></div>
          <div><span>事件数</span><strong>{eventCount}</strong><small>已订阅</small></div>
          <div className="webhook-overview-status">
            <CheckCircleOutlined />
            <div><strong>通知总线就绪</strong><small>按事件白名单发送</small></div>
          </div>
        </div>
      </Card>

      <div className="webhook-main-grid">
        <Card
          className="work-card webhook-list-card"
          title={
            <div className="webhook-panel-title">
              <div><span>通知通道</span><Text type="secondary">{items.length} 个端点</Text></div>
              <small>ENDPOINTS</small>
            </div>
          }
        >
          <Table
            size="middle"
            rowKey="id"
            dataSource={items}
            loading={loading}
            pagination={{ pageSize: 8, size: 'small', showSizeChanger: false }}
            rowClassName={(record) => (record.id === selected?.id ? 'row-active' : '')}
            locale={{ emptyText: '暂无 Webhook，先添加一个通知通道' }}
            onRow={(record) => ({ onClick: () => setSelected(record) })}
            columns={[
              {
                title: '通道名称',
                dataIndex: 'name',
                width: 156,
                render: (value: string, record: Webhook) => (
                  <div className="webhook-name-cell"><strong>{value || '--'}</strong><span>{record.id || '--'}</span></div>
                )
              },
              { title: '类型', dataIndex: 'url', width: 86, render: (value: string) => <span className="webhook-kind">{endpointKind(value)}</span> },
              { title: '方法', dataIndex: 'method', width: 82, render: (value: string) => <span className="webhook-code">{value || 'POST'}</span> },
              { title: '事件', dataIndex: 'events', width: 92, render: (value: string) => `${parseEventCount(value)} 个` },
              {
                title: '状态',
                width: 88,
                render: (_value, record) => <Tag className={`webhook-status-tag ${record.enabled ? 'is-enabled' : ''}`}>{record.enabled ? '已启用' : '已停用'}</Tag>
              },
              { title: 'URL', dataIndex: 'url', width: 230, render: (value: string) => <span className="webhook-url">{value || '--'}</span> },
              {
                title: '操作',
                width: 194,
                render: (_value, record) => (
                  <Space size={4}>
                    <Button size="small" icon={<ApiOutlined />} onClick={(event) => { event.stopPropagation(); test(record.id); }}>测试</Button>
                    <Button size="small" icon={<EditOutlined />} onClick={(event) => { event.stopPropagation(); openDialog(record); }}>编辑</Button>
                    <Popconfirm
                      title="删除 Webhook"
                      description="删除后自动化任务将不再使用这个通道。"
                      okText="删除"
                      cancelText="取消"
                      onConfirm={(event) => { event?.stopPropagation(); remove(record.id); }}
                    >
                      <Button size="small" danger icon={<DeleteOutlined />} onClick={(event) => event.stopPropagation()} />
                    </Popconfirm>
                  </Space>
                )
              }
            ]}
            scroll={{ x: 928 }}
          />
        </Card>

        <Card
          className="work-card webhook-test-card"
          title={
            <div className="webhook-panel-title">
              <div><span>发送测试</span><Text type="secondary">{selected?.name || '未选择通道'}</Text></div>
              <small>PROBE</small>
            </div>
          }
          extra={selected ? <Button size="small" icon={<ApiOutlined />} onClick={() => test(selected.id)}>重新测试</Button> : null}
        >
          <div className="webhook-test-intro">
            <BellOutlined />
            <div><strong>{selected ? '发送一条本地测试通知' : '选择一个通道查看测试结果'}</strong><span>测试事件为 webhook.test</span></div>
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
        className="webhook-edit-modal"
        title={
          <div className="quote-dialog-title webhook-dialog-title">
            <div>
              <strong>{editing ? '编辑 Webhook' : '添加 Webhook'}</strong>
              <span>{editing?.name || '通知通道'}</span>
            </div>
            <small>事件通知 · 结果投递</small>
          </div>
        }
      >
        <Form form={form} layout="vertical" onFinish={save} className="webhook-form">
          <Form.Item name="id" hidden><Input /></Form.Item>
          <section className="webhook-form-section">
            <div className="webhook-form-section-head">
              <div><strong>端点信息</strong><span>定义通知发往哪里、用什么方法发送</span></div>
              <small>ENDPOINT</small>
            </div>
            <div className="webhook-form-grid webhook-form-grid-endpoint">
              <Form.Item name="name" label="通道名称" rules={[{ required: true, message: '请输入通道名称' }]}><Input placeholder="例如：飞书交易提醒" /></Form.Item>
              <Form.Item name="method" label="请求方法" rules={[{ required: true, message: '请选择请求方法' }]}><Select options={methodOptions} /></Form.Item>
              <Form.Item name="enabled" label="启用通道" valuePropName="checked"><Switch /></Form.Item>
              <Form.Item name="url" label="Webhook URL" rules={[{ required: true, message: '请输入 Webhook URL' }]}><Input prefix={<LinkOutlined />} placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..." /></Form.Item>
            </div>
          </section>

          <section className="webhook-form-section">
            <div className="webhook-form-section-head">
              <div><strong>事件白名单</strong><span>空数组表示接收所有事件，也可以填写具体事件名</span></div>
              <small>EVENTS</small>
            </div>
            <div className="webhook-form-grid webhook-form-grid-events">
              <Form.Item name="events" label="事件 JSON"><Input.TextArea rows={4} placeholder={'例如：["automation.failed","stock_selection.finished","macro_event.alert_due"]'} /></Form.Item>
            </div>
          </section>

          <section className="webhook-form-section">
            <div className="webhook-form-section-head">
              <div><strong>请求头</strong><span>用于兼容自建网关或签名认证</span></div>
              <small>HEADERS</small>
            </div>
            <div className="webhook-form-grid webhook-form-grid-headers">
              <Form.Item name="headers_json" label="请求头 JSON"><Input.TextArea rows={4} placeholder={'例如：{"Authorization": "Bearer token"}'} /></Form.Item>
            </div>
          </section>

          <div className="webhook-form-actions">
            <Text type="secondary">保存后可在自动化任务中选择这个通知通道</Text>
            <Space>
              <Button onClick={() => setDialogOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存通道</Button>
            </Space>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
