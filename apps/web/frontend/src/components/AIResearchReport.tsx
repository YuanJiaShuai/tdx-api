import { Alert, Button, Descriptions, Empty, List, Modal, Space, Tag, Typography, message } from 'antd';
import { FileSearchOutlined, ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { apiFetch } from '../lib/api';
import type { AICredential, AIResearchReport, AIResearchResponse } from '../types';

const { Text, Paragraph } = Typography;

interface AIResearchReportProps {
  symbol: string;
  name?: string;
}

function tone(value?: string) {
  if (value === 'high' || value === 'down' || value === 'warn' || value === 'mismatch') return 'warning';
  if (value === 'low' || value === 'up' || value === 'pass' || value === 'match') return 'success';
  return 'default';
}

function label(value?: string) {
  return ({ up: '上行', down: '下行', sideways: '震荡', unknown: '未知', low: '低', medium: '中', high: '高', pass: '通过', warn: '需留意', match: '匹配', mismatch: '不匹配' } as Record<string, string>)[value || ''] || value || '未知';
}

function stringList(value?: unknown) {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0) : [];
}

export function AIResearchReport({ symbol, name }: AIResearchReportProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<AIResearchResponse | null>(null);

  async function loadReport() {
    setLoading(true);
    try {
      const credentials = await apiFetch<AICredential[]>('/api/ai/credentials');
      const credential = (credentials || []).find((item) => item.enabled !== false);
      if (!credential) throw new Error('请先在“AI 模型”中配置并启用一个模型');
      const result = await apiFetch<AIResearchResponse>('/api/ai/research/stock', {
        method: 'POST',
        body: JSON.stringify({
          symbol,
          provider: credential.provider,
          model: credential.model,
          credential_id: credential.id,
          input: { name, source: 'watchlist' },
          options: { max_tokens: 1800 }
        })
      });
      setResponse(result);
      setOpen(true);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '研究报告生成失败');
    } finally {
      setLoading(false);
    }
  }

  const report: AIResearchReport = response?.result || {};
  const technicalSignals = report.technical?.signals || [];
  const evidence = report.evidence || [];

  return (
    <>
      <Button size="small" icon={<FileSearchOutlined />} loading={loading} onClick={() => void loadReport()}>
        AI 研究报告
      </Button>
      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={920}
        centered
        destroyOnHidden
        className="app-themed-modal ai-research-modal"
        title={<div className="quote-dialog-title"><div><strong>{name || symbol}</strong><span>{symbol}</span></div><small>AI RESEARCH DOSSIER</small></div>}
      >
        {!response ? <Empty description="暂无研究报告" /> : (
          <div className="ai-research-report">
            <div className="ai-research-headline">
              <div><Text type="secondary">核心结论</Text><h3>{report.summary || '模型未给出明确结论'}</h3></div>
              <Space wrap>
                <Tag color={tone(report.confidence)}>置信度 {label(report.confidence)}</Tag>
                <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={() => void loadReport()}>重新生成</Button>
              </Space>
            </div>
            <Descriptions size="small" column={{ xs: 1, sm: 3 }} bordered>
              <Descriptions.Item label="数据修订">{response.data_revision || '未提供'}</Descriptions.Item>
              <Descriptions.Item label="Prompt">{response.prompt_version || 'v1'}</Descriptions.Item>
              <Descriptions.Item label="运行记录">{response.run_id || '未记录'}</Descriptions.Item>
            </Descriptions>
            <div className="ai-research-grid">
              <section><div className="ai-research-section-title">技术信号 <Tag color={tone(report.technical?.trend)}>{label(report.technical?.trend)}</Tag></div>
                {technicalSignals.length ? <List size="small" dataSource={technicalSignals} renderItem={(item) => <List.Item><div><strong>{item.name || '指标'}</strong><div>{item.value || 'unknown'}</div><Text type="secondary">证据：{stringList(item.evidence).join('；') || '未提供'}</Text></div></List.Item>} /> : <Text type="secondary">暂无技术信号</Text>}
              </section>
              <section><div className="ai-research-section-title">基本面与策略适配</div><Paragraph>{report.fundamental?.summary || '基本面数据未提供'}</Paragraph><div className="ai-research-meta-row"><Tag color={tone(report.strategy_fit?.status)}>策略 {label(report.strategy_fit?.status)}</Tag><Text type="secondary">{report.strategy_fit?.reason || '未提供适配说明'}</Text></div></section>
              <section><div className="ai-research-section-title">宏观风险 <Tag color={tone(report.macro_risk?.level)}>{label(report.macro_risk?.level)}</Tag></div><List size="small" dataSource={report.macro_risk?.events || []} locale={{ emptyText: '暂无宏观事件数据' }} renderItem={(item) => <List.Item>{String(item.name || item.title || item.event || '宏观事件')} {item.date ? `· ${String(item.date)}` : ''}</List.Item>} /></section>
              <section><div className="ai-research-section-title">数据质量 <Tag color={tone(report.data_quality?.status)}>{label(report.data_quality?.status)}</Tag></div><List size="small" dataSource={stringList(report.data_quality?.notes)} locale={{ emptyText: '未提供质量备注' }} renderItem={(item) => <List.Item>{item}</List.Item>} /></section>
            </div>
            <section className="ai-research-evidence"><div className="ai-research-section-title">证据链</div>{evidence.length ? evidence.map((item, index) => <div className="ai-research-evidence-item" key={`${item.claim}-${index}`}><strong>{item.claim || '未命名判断'}</strong><span>{stringList(item.evidence).join('；') || '未提供证据'} · {item.source || 'unknown'}</span></div>) : <Text type="secondary">模型未返回可引用证据</Text>}</section>
            <div className="ai-research-next"><div><strong>下一步检查</strong><List size="small" dataSource={stringList(report.next_checks)} locale={{ emptyText: '暂无' }} renderItem={(item) => <List.Item>{item}</List.Item>} /></div><div><strong>纪律提醒</strong><List size="small" dataSource={stringList(report.discipline_notes)} locale={{ emptyText: '暂无' }} renderItem={(item) => <List.Item>{item}</List.Item>} /></div></div>
            {report.disclaimer ? <Alert type="warning" showIcon message={report.disclaimer} /> : null}
            <details className="ai-research-raw"><summary>查看原始 JSON</summary><pre>{JSON.stringify(response, null, 2)}</pre></details>
          </div>
        )}
      </Modal>
    </>
  );
}
