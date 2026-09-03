import { Button, Card, DatePicker, Empty, Form, Input, InputNumber, Modal, Popconfirm, Select, Segmented, Space, Switch, Tag, Tooltip, Typography, message } from 'antd';
import { CalendarOutlined, CheckOutlined, ClockCircleOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, ReloadOutlined, SettingOutlined, SyncOutlined, WarningOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import { apiFetch } from '../lib/api';
import type { MacroAlertSettings, MacroEvent, MacroEventOverview, MacroEventSyncState } from '../types';

const { Text } = Typography;

const categoryLabels: Record<string, string> = {
  inflation: '通胀',
  employment: '就业',
  central_bank: '央行',
  growth: '增长',
  china_macro: '中国宏观',
  policy: '政策',
  earnings: '财报',
  unlock: '解禁',
  macro: '宏观'
};

const impactMeta: Record<string, { label: string; className: string }> = {
  low: { label: '低', className: 'is-low' },
  medium: { label: '中', className: 'is-medium' },
  high: { label: '高', className: 'is-high' },
  critical: { label: '极高', className: 'is-critical' }
};

const syncStatusMeta: Record<string, { label: string; className: string }> = {
  idle: { label: '未同步', className: 'is-idle' },
  success: { label: '已同步', className: 'is-success' },
  partial: { label: '部分成功', className: 'is-partial' },
  failed: { label: '同步失败', className: 'is-failed' }
};

function eventDate(value?: string) {
  const date = value ? dayjs(value) : null;
  return date?.isValid() ? date : null;
}

function formatTime(value?: string) {
  const date = eventDate(value);
  return date ? date.format('MM月DD日 HH:mm') : '--';
}

function formatRelative(value?: string) {
  const date = eventDate(value);
  if (!date) return '--';
  const hours = date.diff(dayjs(), 'hour', true);
  if (hours < 0) return '已发布';
  if (hours < 24) return `${Math.max(1, Math.round(hours))} 小时后`;
  return `${Math.ceil(hours / 24)} 天后`;
}

function toRFC3339(value: string) {
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('YYYY-MM-DDTHH:mm:ssZ') : value;
}

function formatSyncTime(value?: string) {
  const date = eventDate(value);
  return date ? date.format('MM月DD日 HH:mm') : '从未同步';
}

function alertWindowState(value: string, impactValue: string, settings: MacroAlertSettings) {
  if (!settings.enabled || (settings.critical_only && !['high', 'critical'].includes(impactValue))) return 'muted';
  const date = eventDate(value);
  if (!date) return 'unknown';
  const now = dayjs();
  const start = date.subtract(settings.window_before_minutes, 'minute');
  const end = date.add(settings.window_after_minutes, 'minute');
  if (!now.isBefore(start) && now.isBefore(end)) return 'window_active';
  if (now.isBefore(date.subtract(settings.lead_minutes, 'minute'))) return 'scheduled';
  if (now.isBefore(date)) return 'alert_due';
  return 'released';
}

function minutesLabel(value: number) {
  if (value >= 60 && value % 60 === 0) return `${value / 60} 小时`;
  return `${value} 分钟`;
}

interface EventFormValues {
  code: string;
  name: string;
  category: string;
  impact: string;
  starts_at: Dayjs;
  a_share_date?: Dayjs;
  source?: string;
  source_url?: string;
  description?: string;
  country?: string;
}

interface AlertSettingsFormValues extends MacroAlertSettings {}

const defaultAlertSettings: MacroAlertSettings = {
  id: 'default',
  enabled: true,
  lead_minutes: 240,
  window_before_minutes: 240,
  window_after_minutes: 120,
  critical_only: false,
  notify_webhooks: false,
  webhook_ids: '[]'
};

export function AlertsWorkspace() {
  const [events, setEvents] = useState<MacroEvent[]>([]);
  const [syncStates, setSyncStates] = useState<MacroEventSyncState[]>([]);
  const [alertSettings, setAlertSettings] = useState<MacroAlertSettings>(defaultAlertSettings);
  const [selected, setSelected] = useState<MacroEvent | null>(null);
  const [overview, setOverview] = useState<MacroEventOverview | null>(null);
  const [category, setCategory] = useState('all');
  const [impact, setImpact] = useState('all');
  const [view, setView] = useState<'timeline' | 'calendar'>('timeline');
  const [loading, setLoading] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<MacroEvent | null>(null);
  const [form] = Form.useForm<EventFormValues>();
  const [settingsForm] = Form.useForm<AlertSettingsFormValues>();

  async function load() {
    setLoading(true);
    try {
      const items = await apiFetch<MacroEvent[]>('/api/macro-events');
      let sync: { states: MacroEventSyncState[] } | undefined;
      try {
        sync = await apiFetch<{ states: MacroEventSyncState[] }>('/api/macro-events/sync');
      } catch {
        // Older gateways can still serve the calendar while the sync endpoint is unavailable.
      }
      try {
        const settings = await apiFetch<MacroAlertSettings>('/api/macro-events/settings');
        setAlertSettings(settings);
      } catch {
        // Calendar remains usable when the gateway predates alert settings.
      }
      try {
        setOverview(await apiFetch<MacroEventOverview>('/api/macro-events/overview'));
      } catch {
        setOverview(null);
      }
      const next = (items || []).sort((a, b) => (a.starts_at || '').localeCompare(b.starts_at || ''));
      setEvents(next);
      setSyncStates(sync?.states || []);
      setSelected((current) => next.find((item) => item.id === current?.id) || next.find((item) => !item.acknowledged) || next[0] || null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '预警事件加载失败');
    } finally {
      setLoading(false);
    }
  }

  async function syncOfficialCalendars() {
    setSyncLoading(true);
    try {
      const result = await apiFetch<{ states: MacroEventSyncState[] }>('/api/macro-events/sync', { method: 'POST' });
      setSyncStates(result?.states || []);
      message[result?.states?.some((state) => state.status === 'failed') ? 'warning' : 'success'](
        result?.states?.some((state) => state.status === 'failed') ? '部分官方日程同步失败，已保留本地日历' : '官方日程同步完成'
      );
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '官方日程同步失败');
    } finally {
      setSyncLoading(false);
    }
  }

  function openSettings() {
    settingsForm.setFieldsValue(alertSettings);
    setSettingsOpen(true);
  }

  async function saveSettings(values: AlertSettingsFormValues) {
    setSettingsLoading(true);
    try {
      const settings = await apiFetch<MacroAlertSettings>('/api/macro-events/settings', { method: 'PUT', body: JSON.stringify(values) });
      setAlertSettings(settings);
      setSettingsOpen(false);
      message.success('提醒规则已保存');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '提醒规则保存失败');
    } finally {
      setSettingsLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 60000);
    return () => window.clearInterval(timer);
  }, []);

  const filtered = useMemo(() => events.filter((event) => (category === 'all' || event.category === category) && (impact === 'all' || event.impact === impact)), [category, events, impact]);
  const upcoming = useMemo(() => events.filter((event) => eventDate(event.starts_at)?.isAfter(dayjs().subtract(1, 'hour'))), [events]);
  const nextEvent = upcoming[0];
  const highRiskCount = upcoming.filter((event) => event.impact === 'high' || event.impact === 'critical').length;
  const acknowledgedCount = events.filter((event) => event.acknowledged).length;
  const grouped = useMemo(() => {
    const groups = new Map<string, MacroEvent[]>();
    filtered.forEach((event) => {
      const key = eventDate(event.starts_at)?.format('YYYY-MM-DD') || 'unknown';
      groups.set(key, [...(groups.get(key) || []), event]);
    });
    return [...groups.entries()];
  }, [filtered]);

  function openDialog(event?: MacroEvent) {
    setEditing(event || null);
    form.resetFields();
    if (event) {
      form.setFieldsValue({
        ...event,
        starts_at: eventDate(event.starts_at) || dayjs(),
        a_share_date: eventDate(event.a_share_date) || undefined
      });
    } else {
      form.setFieldsValue({ code: '', name: '', category: 'inflation', country: 'US', impact: 'high', starts_at: dayjs().add(1, 'day').hour(20).minute(30), source: '', source_url: '', description: '' });
    }
    setDialogOpen(true);
  }

  async function saveEvent(values: EventFormValues) {
    const payload = {
      ...(editing || {}),
      code: values.code,
      name: values.name,
      category: values.category,
      country: values.country || editing?.country || 'US',
      impact: values.impact,
      starts_at: toRFC3339(values.starts_at.toISOString()),
      a_share_date: values.a_share_date?.format('YYYY-MM-DD') || '',
      source: values.source || '',
      source_url: values.source_url || '',
      description: values.description || '',
      status: 'scheduled'
    };
    try {
      await apiFetch(editing ? `/api/macro-events/${editing.id}` : '/api/macro-events', { method: editing ? 'PUT' : 'POST', body: JSON.stringify(payload) });
      message.success(editing ? '事件已更新' : '事件已添加');
      setDialogOpen(false);
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '事件保存失败');
    }
  }

  async function acknowledge(event: MacroEvent, value: boolean) {
    try {
      await apiFetch(`/api/macro-events/${event.id}/acknowledge`, { method: 'POST', body: JSON.stringify({ acknowledged: value }) });
      message.success(value ? '已标记为已处理' : '已恢复为待处理');
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '事件状态更新失败');
    }
  }

  async function deleteEvent(event: MacroEvent) {
    try {
      await apiFetch(`/api/macro-events/${event.id}`, { method: 'DELETE' });
      message.success('事件已删除');
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '事件删除失败');
    }
  }

  return (
    <div className="alerts-workspace">
      <Card className="work-card alerts-overview" title={<div className="alerts-panel-title"><div><span>预警中心</span><Text type="secondary">宏观事件与交易前检查</Text></div><small>RISK CALENDAR</small></div>} extra={<Space><Tooltip title="刷新事件"><Button aria-label="刷新预警事件" icon={<ReloadOutlined />} onClick={load} loading={loading} /></Tooltip><Tooltip title="同步官方日程"><Button aria-label="同步官方日程" icon={<SyncOutlined />} onClick={syncOfficialCalendars} loading={syncLoading} /></Tooltip><Tooltip title="提醒规则"><Button aria-label="提醒规则" icon={<SettingOutlined />} onClick={openSettings} /></Tooltip><Button type="primary" icon={<PlusOutlined />} onClick={() => openDialog()}>添加事件</Button></Space>}>
        <div className="alerts-overview-grid">
          <div className="alerts-overview-lead"><span>下一事件</span><strong>{nextEvent?.name || '暂无待发布事件'}</strong><small>{nextEvent ? `${formatTime(nextEvent.starts_at)} · ${formatRelative(nextEvent.starts_at)} · ${alertSettings.enabled ? `提前 ${minutesLabel(alertSettings.lead_minutes)} 检查` : '提醒规则已停用'}` : '添加一条事件开始建立日历'}</small></div>
          <div><span>高影响</span><strong>{highRiskCount}</strong><small>未来事件</small></div>
          <div><span>待处理</span><strong>{Math.max(0, events.length - acknowledgedCount)}</strong><small>个提醒</small></div>
          <div className="alerts-overview-status"><WarningOutlined /><div><strong>交易前检查</strong><small>{alertSettings.enabled ? `提前 ${minutesLabel(alertSettings.lead_minutes)} 提醒` : '提醒规则已停用'}</small></div></div>
        </div>
        <div className="alerts-risk-banner"><WarningOutlined /><span>{overview?.active_risk_events?.length ? `当前有 ${overview.active_risk_events.length} 个风险窗口${overview.holding_risk_events ? `，持仓相关 ${overview.holding_risk_events} 个` : ''}${overview.watchlist_risk_events ? `，自选相关 ${overview.watchlist_risk_events} 个` : ''}。` : '当前没有处于风险窗口的高影响事件。'} 高影响事件前后，建议复核交易计划；不会自动阻止交易。</span></div>
        <div className="alerts-sync-strip">
          <div className="alerts-sync-heading"><SyncOutlined /><span>官方日程同步</span><Text type="secondary">失败时保留本地参考日历</Text></div>
          <div className="alerts-sync-providers">{syncStates.map((state) => { const meta = syncStatusMeta[state.status] || syncStatusMeta.idle; return <span className={`alerts-sync-provider ${meta.className}`} key={state.id}><i />{state.provider}<strong>{meta.label}</strong><small>{formatSyncTime(state.last_success_at || state.last_attempt_at)}</small></span>; })}</div>
        </div>
      </Card>

      <div className="alerts-toolbar">
        <Space wrap>
          <Select aria-label="事件分类" value={category} onChange={setCategory} options={[{ value: 'all', label: '全部分类' }, ...Object.entries(categoryLabels).filter(([key]) => key !== 'macro').map(([value, label]) => ({ value, label }))]} />
          <Select aria-label="影响级别" value={impact} onChange={setImpact} options={[{ value: 'all', label: '全部级别' }, ...Object.entries(impactMeta).map(([value, meta]) => ({ value, label: `${meta.label}影响` }))]} />
        </Space>
        <Segmented value={view} onChange={(value) => setView(value as 'timeline' | 'calendar')} options={[{ value: 'timeline', label: '时间线', icon: <ClockCircleOutlined /> }, { value: 'calendar', label: '日历', icon: <CalendarOutlined /> }]} />
      </div>

      {view === 'timeline' ? (
        <div className="alerts-timeline-layout">
          <Card className="work-card alerts-event-card" title={<div className="alerts-panel-title"><div><span>美国宏观日历</span><Text type="secondary">{filtered.length} 个事件</Text></div><small>US MACRO</small></div>}>
            {grouped.length ? <div className="alerts-timeline">{grouped.map(([date, items]) => <div className="alerts-day" key={date}><div className="alerts-day-marker"><strong>{date === 'unknown' ? '--' : dayjs(date).format('DD')}</strong><span>{date === 'unknown' ? '未知日期' : dayjs(date).format('MM月')}</span></div><div className="alerts-day-events">{items.map((event) => { const meta = impactMeta[event.impact] || impactMeta.medium; const windowState = alertWindowState(event.starts_at, event.impact, alertSettings); return <article className={`alerts-event-row ${event.id === selected?.id ? 'is-selected' : ''} ${event.acknowledged ? 'is-acknowledged' : ''}`} key={event.id} onClick={() => setSelected(event)}><div className="alerts-event-time">{formatTime(event.starts_at).split(' ')[1]}<small>{formatRelative(event.starts_at)}</small></div><div className="alerts-event-main"><div className="alerts-event-heading"><Tag className={`alerts-impact-tag ${meta.className}`}>{meta.label}影响</Tag><strong>{event.name}</strong><span>{categoryLabels[event.category] || event.category}</span></div><p>{event.description || '暂无事件说明。'}</p><div className="alerts-event-meta"><span>{event.source || '自定义来源'}</span><span>影响 A 股：{event.a_share_date || '--'}</span>{windowState === 'window_active' || windowState === 'alert_due' ? <span className="alerts-window-active"><WarningOutlined /> 风险窗口</span> : null}{event.acknowledged ? <span className="is-done"><CheckOutlined /> 已处理</span> : null}</div></div></article>; })}</div></div>)}</div> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前筛选没有事件" />}
          </Card>
          <Card className="work-card alerts-inspector-card" title={<div className="alerts-panel-title"><div><span>事件检查</span><Text type="secondary">{selected?.code || '未选择事件'}</Text></div><small>INSPECTOR</small></div>}>
            {selected ? <div className="alerts-inspector"><div className="alerts-inspector-head"><div><span>{selected.source || '自定义来源'}</span><strong>{selected.name}</strong></div><Tag className={`alerts-impact-tag ${(impactMeta[selected.impact] || impactMeta.medium).className}`}>{(impactMeta[selected.impact] || impactMeta.medium).label}影响</Tag></div><div className="alerts-inspector-facts"><div><span>美国发布时间</span><strong>{formatTime(selected.starts_at)}</strong></div><div><span>对应 A 股交易日</span><strong>{selected.a_share_date || '--'}</strong></div><div><span>事件分类</span><strong>{categoryLabels[selected.category] || selected.category}</strong></div><div><span>状态</span><strong>{selected.acknowledged ? '已处理' : '待处理'}</strong></div></div><p>{selected.description || '暂无事件说明。'}</p><div className="alerts-inspector-actions"><Button type={selected.acknowledged ? 'default' : 'primary'} icon={<CheckOutlined />} onClick={() => acknowledge(selected, !selected.acknowledged)}>{selected.acknowledged ? '恢复待处理' : '标记已处理'}</Button><Button icon={<EditOutlined />} onClick={() => openDialog(selected)}>编辑</Button><Popconfirm title="确认删除这条事件？" description="删除后无法从预警中心恢复。" okText="删除" cancelText="取消" onConfirm={() => deleteEvent(selected)}><Button danger icon={<DeleteOutlined />}>删除</Button></Popconfirm></div>{selected.source_url ? <a className="alerts-source-link" href={selected.source_url} target="_blank" rel="noreferrer"><LinkOutlined /> 查看官方日程</a> : null}</div> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="选择事件查看风险信息" />}
          </Card>
        </div>
      ) : (
        <Card className="work-card alerts-calendar-card" title={<div className="alerts-panel-title"><div><span>事件日历</span><Text type="secondary">按日期查看交易窗口</Text></div><small>CALENDAR</small></div>}>
          <div className="alerts-calendar-grid">{filtered.map((event) => { const meta = impactMeta[event.impact] || impactMeta.medium; return <button type="button" className={`alerts-calendar-item ${meta.className}`} key={event.id} onClick={() => { setSelected(event); setView('timeline'); }}><span>{eventDate(event.starts_at)?.format('MM/DD') || '--'}</span><strong>{event.code}</strong><small>{formatTime(event.starts_at).split(' ')[1]}</small><em>{event.name}</em></button>; })}</div>
        </Card>
      )}

      <Modal open={dialogOpen} onCancel={() => setDialogOpen(false)} footer={null} width={760} centered destroyOnHidden className="app-themed-modal alerts-edit-modal" title={<div className="quote-dialog-title alerts-dialog-title"><div><strong>{editing ? '编辑事件' : '添加宏观事件'}</strong><span>{editing?.name || '交易前风险日历'}</span></div><small>EVENT SCHEDULE</small></div>}>
        <Form form={form} layout="vertical" onFinish={saveEvent} className="alerts-form"><div className="alerts-form-grid"><Form.Item name="code" label="事件代码" rules={[{ required: true, message: '请输入事件代码' }]}><Input placeholder="例如：CPI" /></Form.Item><Form.Item name="name" label="事件名称" rules={[{ required: true, message: '请输入事件名称' }]}><Input placeholder="例如：美国 CPI 通胀" /></Form.Item><Form.Item name="category" label="事件分类"><Select options={Object.entries(categoryLabels).map(([value, label]) => ({ value, label }))} /></Form.Item><Form.Item name="country" label="国家/地区"><Select options={[{ value: 'US', label: '美国' }, { value: 'CN', label: '中国' }, { value: 'GLOBAL', label: '全球' }]} /></Form.Item><Form.Item name="impact" label="影响级别"><Select options={Object.entries(impactMeta).map(([value, meta]) => ({ value, label: `${meta.label}影响` }))} /></Form.Item><Form.Item name="starts_at" label="发布时间（按本地时区展示）" rules={[{ required: true, message: '请选择发布时间' }]}><DatePicker showTime format="YYYY-MM-DD HH:mm" style={{ width: '100%' }} /></Form.Item><Form.Item name="a_share_date" label="对应 A 股交易日"><DatePicker format="YYYY-MM-DD" style={{ width: '100%' }} /></Form.Item><Form.Item name="source" label="来源"><Input placeholder="例如：BLS / Federal Reserve" /></Form.Item><Form.Item name="source_url" label="来源链接"><Input placeholder="https://..." /></Form.Item></div><Form.Item name="description" label="风险说明"><Input.TextArea rows={4} placeholder="写清楚关注什么，以及为什么需要在事件前复核交易计划" /></Form.Item><div className="alerts-form-actions"><Text type="secondary">官方日程可能调整，保存前请核对来源链接。</Text><Space><Button onClick={() => setDialogOpen(false)}>取消</Button><Button type="primary" htmlType="submit">保存事件</Button></Space></div></Form>
      </Modal>

      <Modal open={settingsOpen} onCancel={() => setSettingsOpen(false)} footer={null} width={560} centered destroyOnHidden className="app-themed-modal alerts-settings-modal" title={<div className="quote-dialog-title alerts-dialog-title"><div><strong>提醒规则</strong><span>交易前风险窗口</span></div><small>ALERT POLICY</small></div>}>
        <Form form={settingsForm} layout="vertical" onFinish={saveSettings} className="alerts-form">
          <Form.Item name="enabled" label="启用预警窗口" valuePropName="checked"><Switch checkedChildren="启用" unCheckedChildren="停用" /></Form.Item>
          <div className="alerts-form-grid"><Form.Item name="lead_minutes" label="提前提醒（分钟）"><InputNumber min={0} max={10080} step={15} style={{ width: '100%' }} /></Form.Item><Form.Item name="window_before_minutes" label="事件前窗口（分钟）"><InputNumber min={0} max={10080} step={15} style={{ width: '100%' }} /></Form.Item><Form.Item name="window_after_minutes" label="事件后窗口（分钟）"><InputNumber min={0} max={10080} step={15} style={{ width: '100%' }} /></Form.Item></div>
          <Form.Item name="critical_only" label="只关注高影响和极高影响" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_webhooks" label="通过 Webhook 发送提醒" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="webhook_ids" label="提醒渠道 ID（JSON 数组）"><Input placeholder='例如：["feishu-risk"]，留空不发送' /></Form.Item>
          <div className="alerts-settings-note"><WarningOutlined /> 规则用于页面风险提示和后续通知，不会自动阻止交易。</div>
          <div className="alerts-form-actions"><Text type="secondary">最多可设置 7 天窗口</Text><Space><Button onClick={() => setSettingsOpen(false)}>取消</Button><Button type="primary" htmlType="submit" loading={settingsLoading}>保存规则</Button></Space></div>
        </Form>
      </Modal>
    </div>
  );
}
