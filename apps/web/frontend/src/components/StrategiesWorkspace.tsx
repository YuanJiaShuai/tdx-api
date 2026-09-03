import { Button, Card, Form, Input, Modal, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { BarChartOutlined, CheckCircleOutlined, CloseOutlined, CodeOutlined, CopyOutlined, DatabaseOutlined, EditOutlined, ExperimentOutlined, PlusOutlined, ReloadOutlined, SaveOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import { JsonPane } from './JsonPane';
import type { AutomationRun, Strategy } from '../types';

const { Text } = Typography;
type StrategyAction = 'run' | 'backtest' | 'hikyuu';

function formatConfig(value?: string) {
  try {
    return JSON.stringify(JSON.parse(value || '{}'), null, 2);
  } catch {
    return value || '{}';
  }
}

function isAutomationRun(value: unknown): value is AutomationRun {
  return Boolean(
    value &&
    typeof value === 'object' &&
    'id' in value &&
    'status' in value &&
    'task_type' in value
  );
}

function formatAutomationRun(run: AutomationRun) {
  if (run.status === 'running') {
    return '策略已提交，正在读取行情并计算因子，请稍候。';
  }
  if (run.result_json) {
    try {
      return JSON.parse(run.result_json);
    } catch {
      return run.result_json;
    }
  }
  return run.log || run;
}

export function StrategiesWorkspace() {
  const [items, setItems] = useState<Strategy[]>([]);
  const [factors, setFactors] = useState<Array<Record<string, unknown>>>([]);
  const [selected, setSelected] = useState<Strategy | null>(null);
  const [runOutput, setRunOutput] = useState<unknown>('暂无运行');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Strategy | null>(null);
  const [loading, setLoading] = useState(false);
  const [runningAction, setRunningAction] = useState<StrategyAction | null>(null);
  const [pendingRunId, setPendingRunId] = useState<string | null>(null);
  const [form] = Form.useForm<Strategy>();

  const load = async () => {
    setLoading(true);
    try {
      const [strategies, factorDefs] = await Promise.all([
        apiFetch<Strategy[]>('/api/strategies'),
        apiFetch<Array<Record<string, unknown>>>('/api/factors')
      ]);
      const nextItems = strategies || [];
      setItems(nextItems);
      setFactors(factorDefs || []);
      setSelected((current) => nextItems.find((item) => item.id === current?.id) || nextItems[0] || null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '策略加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  useEffect(() => {
    if (!pendingRunId) return;
    let active = true;
    const checkRun = async () => {
      try {
        const runs = await apiFetch<AutomationRun[]>('/api/automations/runs?limit=50');
        const run = runs.find((item) => item.id === pendingRunId);
        if (!active || !run || run.status === 'running') return;
        setRunOutput(formatAutomationRun(run));
        setPendingRunId(null);
        setRunningAction(null);
        if (run.status === 'success') {
          message.success('策略运行已完成');
        } else {
          message.error(run.log || '策略运行失败');
        }
      } catch {
        // Keep polling so a temporary status request failure does not lose the run.
      }
    };
    void checkRun();
    const timer = window.setInterval(() => { void checkRun(); }, 2000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [pendingRunId]);

  const enabledCount = useMemo(() => items.filter((item) => item.enabled).length, [items]);
  const templateCount = useMemo(() => items.filter((item) => item.readonly).length, [items]);

  function openDialog(strategy?: Strategy) {
    const next = strategy || {
      id: '',
      name: '',
      description: '',
      config_json: '{}',
      enabled: true,
      readonly: false
    };
    setEditing(strategy || null);
    form.resetFields();
    form.setFieldsValue(next);
    setDialogOpen(true);
  }

  async function saveStrategy(values: Strategy) {
    try {
      await apiFetch(values.id ? `/api/strategies/${values.id}` : '/api/strategies', {
        method: values.id ? 'PUT' : 'POST',
        body: JSON.stringify(values)
      });
      message.success(values.id ? '策略已更新' : '策略已添加');
      setDialogOpen(false);
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '策略保存失败');
    }
  }

  async function cloneStrategy(id: string) {
    try {
      await apiFetch(`/api/strategies/${id}/clone`, { method: 'POST' });
      message.success('策略副本已创建');
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '策略复制失败');
    }
  }

  async function runStrategy(id: string, backtest = false, engine = 'go') {
    if (runningAction) return;
    const action: StrategyAction = backtest ? (engine === 'hikyuu' ? 'hikyuu' : 'backtest') : 'run';
    let waitingForRun = false;
    setRunningAction(action);
    setRunOutput(backtest ? '正在执行回测，请稍候。' : '策略已提交，正在读取行情并计算因子，请稍候。');
    try {
      const run = await apiFetch<unknown>(
        `/api/strategies/${id}/${backtest ? 'backtest' : 'run'}`,
        { method: 'POST', body: backtest ? JSON.stringify({ strategy_id: id, engine }) : undefined }
      );
      if (!backtest && isAutomationRun(run)) {
        waitingForRun = true;
        setPendingRunId(run.id);
        setRunOutput(formatAutomationRun(run));
        message.info('策略运行已提交，后台计算中');
        return;
      }
      setRunOutput(run);
      message.success(backtest ? `${engine === 'hikyuu' ? 'Hikyuu 校验' : 'Go'} 回测已完成` : '策略运行已提交');
    } catch (error) {
      const text = error instanceof Error ? error.message : (backtest ? '策略回测失败' : '策略运行失败');
      setRunOutput(text);
      message.error(text);
    } finally {
      if (!waitingForRun) setRunningAction(null);
    }
  }

  return (
    <div className="strategy-workspace">
      <Card
        className="work-card strategy-overview"
        title={
          <div className="strategy-panel-title">
            <div><span>策略中心</span><Text type="secondary">研究规则与执行信号</Text></div>
            <small>STRATEGY LAB</small>
          </div>
        }
        extra={
          <Space>
            <Button aria-label="刷新策略" icon={<ReloadOutlined />} onClick={load} loading={loading} />
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openDialog()}>新建策略</Button>
          </Space>
        }
      >
        <div className="strategy-overview-grid">
          <div className="strategy-overview-lead">
            <span>当前策略</span>
            <strong>{selected?.name || '尚未选择'}</strong>
            <small>{selected?.description || '选择一个策略查看规则和运行状态'}</small>
          </div>
          <div><span>策略数</span><strong>{items.length}</strong><small>个配置</small></div>
          <div><span>启用数</span><strong>{enabledCount}</strong><small>可执行</small></div>
          <div><span>因子数</span><strong>{factors.length}</strong><small>可引用</small></div>
          <div className="strategy-overview-status">
            <CheckCircleOutlined />
            <div><strong>研究环境就绪</strong><small>{templateCount} 个系统模板</small></div>
          </div>
        </div>
      </Card>

      <div className="strategy-main-grid">
        <Card
          className="work-card strategy-list-card"
          title={
            <div className="strategy-panel-title">
              <div><span>策略列表</span><Text type="secondary">{items.length} 个规则</Text></div>
              <small>RULES</small>
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
            onRow={(record) => ({ onClick: () => setSelected(record) })}
            locale={{ emptyText: '暂无策略，先建立一条研究规则' }}
            columns={[
              {
                title: '策略名称',
                dataIndex: 'name',
                width: 190,
                render: (value: string, record: Strategy) => (
                  <div className="strategy-name-cell"><strong>{value || '--'}</strong><span>{record.id || '--'}</span></div>
                )
              },
              {
                title: '说明',
                dataIndex: 'description',
                width: 270,
                render: (value: string) => <span className="strategy-description">{value || '未填写说明'}</span>
              },
              {
                title: '来源',
                width: 82,
                render: (_value, record) => <Tag className={`strategy-source-tag ${record.readonly ? 'is-template' : ''}`}>{record.readonly ? '系统' : '自建'}</Tag>
              },
              {
                title: '状态',
                width: 82,
                render: (_value, record) => <Tag className={`strategy-status-tag ${record.enabled ? 'is-enabled' : ''}`}>{record.enabled ? '已启用' : '已停用'}</Tag>
              },
              {
                title: '更新时间',
                dataIndex: 'updated_at',
                width: 156,
                render: (value: string) => <span className="strategy-time">{value || '--'}</span>
              }
            ]}
            scroll={{ x: 780 }}
          />
        </Card>

        <Card
          className="work-card strategy-inspector-card"
          title={
            <div className="strategy-panel-title">
              <div><span>策略检查</span><Text type="secondary">{selected?.name || '未选择策略'}</Text></div>
              <small>INSPECTOR</small>
            </div>
          }
        >
          {selected ? (
            <div className="strategy-inspector">
              <div className="strategy-inspector-heading">
                <div>
                  <span>当前规则</span>
                  <strong>{selected.name}</strong>
                </div>
                <Tag className={`strategy-status-tag ${selected.enabled ? 'is-enabled' : ''}`}>{selected.enabled ? '已启用' : '已停用'}</Tag>
              </div>
              <p className="strategy-inspector-description">{selected.description || '未填写策略说明。'}</p>
              <div className="strategy-inspector-meta">
                <div><span>策略来源</span><strong>{selected.readonly ? '系统模板' : '自定义'}</strong></div>
                <div><span>配置状态</span><strong>{selected.config_json ? 'JSON 已载入' : '空配置'}</strong></div>
              </div>
              <div className="strategy-inspector-actions">
                {!selected.readonly ? <Button disabled={Boolean(runningAction)} icon={<EditOutlined />} onClick={() => openDialog(selected)}>编辑策略</Button> : null}
                <Button disabled={Boolean(runningAction)} icon={<CopyOutlined />} onClick={() => cloneStrategy(selected.id)}>复制副本</Button>
                <Button type="primary" loading={runningAction === 'run'} disabled={Boolean(runningAction) && runningAction !== 'run'} icon={<ThunderboltOutlined />} onClick={() => { void runStrategy(selected.id); }}>立即运行</Button>
                <Button loading={runningAction === 'backtest'} disabled={Boolean(runningAction) && runningAction !== 'backtest'} icon={<ExperimentOutlined />} onClick={() => { void runStrategy(selected.id, true); }}>回测</Button>
                <Button loading={runningAction === 'hikyuu'} disabled={Boolean(runningAction) && runningAction !== 'hikyuu'} icon={<DatabaseOutlined />} onClick={() => { void runStrategy(selected.id, true, 'hikyuu'); }}>Hikyuu 校验</Button>
              </div>
              <div className="strategy-config-preview">
                <div><span>配置预览</span><CodeOutlined /></div>
                <pre>{formatConfig(selected.config_json)}</pre>
              </div>
            </div>
          ) : (
            <div className="strategy-empty-inspector">选择一个策略查看规则。</div>
          )}
        </Card>
      </div>

      <div className="strategy-support-grid">
        <Card
          className="work-card strategy-factor-card"
          title={
            <div className="strategy-panel-title">
              <div><span>因子库</span><Text type="secondary">{factors.length} 个可用因子</Text></div>
              <small>FACTORS</small>
            </div>
          }
        >
          <Table
            size="small"
            rowKey={(record) => String(record.id)}
            dataSource={factors}
            pagination={{ pageSize: 6, size: 'small', showSizeChanger: false }}
            locale={{ emptyText: '暂无因子定义' }}
            columns={[
              { title: '名称', dataIndex: 'name', width: 150 },
              { title: '类型', dataIndex: 'kind', width: 90, render: (value: string) => <span className="strategy-factor-kind">{value || '--'}</span> },
              { title: '说明', dataIndex: 'description', render: (value: string) => <span className="strategy-description">{value || '--'}</span> }
            ]}
          />
        </Card>

        <Card
          className="work-card strategy-result-card"
          title={
            <div className="strategy-panel-title">
              <div><span>运行结果</span><Text type="secondary">{selected?.name || '等待运行'}</Text></div>
              <small>OUTPUT</small>
            </div>
          }
        >
          <div className="strategy-result-intro">
            <BarChartOutlined />
            <div>
              <strong>{runningAction ? '策略正在运行' : (selected ? '查看运行或回测返回值' : '选择策略查看结果')}</strong>
              <span>{runningAction ? '正在读取行情并计算，请稍候' : '运行不会修改策略配置'}</span>
            </div>
          </div>
          <JsonPane value={runOutput} />
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
        className="app-themed-modal strategy-edit-modal"
        title={
          <div className="quote-dialog-title strategy-dialog-title">
            <div>
              <strong>{editing ? '编辑策略' : '新建策略'}</strong>
              <span>{editing?.name || '研究规则'}</span>
            </div>
            <small>策略配置 · 执行参数</small>
          </div>
        }
      >
        <Form form={form} layout="vertical" onFinish={saveStrategy} className="strategy-form">
          <Form.Item name="id" hidden><Input /></Form.Item>
          <section className="strategy-form-section">
            <div className="strategy-form-section-head">
              <div><strong>规则信息</strong><span>为策略命名并说明它解决什么问题</span></div>
              <small>RULE</small>
            </div>
            <div className="strategy-form-grid strategy-form-grid-basic">
              <Form.Item name="name" label="策略名称" rules={[{ required: true, message: '请输入策略名称' }]}><Input placeholder="例如：放量突破选股" /></Form.Item>
              <Form.Item name="enabled" label="启用策略" valuePropName="checked"><Switch /></Form.Item>
              <Form.Item name="description" label="策略说明"><Input.TextArea rows={3} placeholder="写清楚策略关注的市场行为和使用场景" /></Form.Item>
            </div>
          </section>
          <section className="strategy-form-section">
            <div className="strategy-form-section-head">
              <div><strong>策略配置</strong><span>使用 JSON 定义参数、条件和输出</span></div>
              <small>CONFIG</small>
            </div>
            <div className="strategy-form-grid strategy-form-grid-config">
              <Form.Item name="config_json" label="JSON 配置" rules={[{ required: true, message: '请输入 JSON 配置' }]}><Input.TextArea rows={14} placeholder={'例如：{"factors":["volume_ratio"],"threshold":1.8}'} /></Form.Item>
            </div>
          </section>
          <div className="strategy-form-actions">
            <Text type="secondary">{editing?.readonly ? '系统模板不可直接修改，请复制后编辑' : '保存后可在自动化任务中调用这条策略'}</Text>
            <Space>
              {editing?.readonly ? <Button icon={<CopyOutlined />} onClick={() => { setDialogOpen(false); cloneStrategy(editing.id); }}>复制副本</Button> : null}
              <Button onClick={() => setDialogOpen(false)}>取消</Button>
              {!editing?.readonly ? <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存策略</Button> : null}
            </Space>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
