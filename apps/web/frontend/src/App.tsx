import { Empty, Layout, Tabs } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { AppHeader } from './components/AppHeader';
import { AIConfigsWorkspace } from './components/AIConfigsWorkspace';
import { AutomationsWorkspace } from './components/AutomationsWorkspace';
import { DataCenterWorkspace } from './components/DataCenterWorkspace';
import { DailyReviewWorkspace } from './components/DailyReviewWorkspace';
import { SelectionResultsWorkspace } from './components/SelectionResultsWorkspace';
import { StrategiesWorkspace } from './components/StrategiesWorkspace';
import { TradingSystemWorkspace } from './components/TradingSystemWorkspace';
import { WebhooksWorkspace } from './components/WebhooksWorkspace';
import { MarketWorkspace } from './components/MarketWorkspace';
import { KlineAnalysisWorkspace } from './components/KlineAnalysisWorkspace';
import { AIAssistantWorkspace } from './components/AIAssistantWorkspace';
import { AlertsWorkspace } from './components/AlertsWorkspace';
import { WatchlistTable } from './components/WatchlistTable';
import { apiFetch } from './lib/api';
import type { ServiceStatusResult } from './types';

const workspaces = [
  { key: 'market', label: '自选' },
  { key: 'proChart', label: '市场行情' },
  { key: 'klineAnalysis', label: 'K线分析' },
  { key: 'dataCenter', label: '数据中心' },
  { key: 'selectionResults', label: '选股结果' },
  { key: 'dailyReview', label: '每日复盘' },
  { key: 'tradingSystem', label: '交易系统' },
  { key: 'strategies', label: '策略中心' },
  { key: 'automations', label: '自动化' },
  { key: 'aiConfigs', label: 'AI 模型' },
  { key: 'aiAssistant', label: 'AI 助手' },
  { key: 'alerts', label: '预警中心' },
  { key: 'webhooks', label: 'Webhook' }
];

const workspaceCodes: Record<string, string> = {
  market: 'WATCHLIST',
  proChart: 'MARKET_DATA',
  klineAnalysis: 'KLINE_ANALYSIS',
  dataCenter: 'DATA_CENTER',
  selectionResults: 'SIGNAL_RESULTS',
  dailyReview: 'DAILY_REVIEW',
  tradingSystem: 'TRADING_PLAN',
  strategies: 'STRATEGY_LAB',
  automations: 'RUN_SCHEDULER',
  aiConfigs: 'MODEL_ROUTER',
  aiAssistant: 'AI_ASSISTANT',
  alerts: 'RISK_CALENDAR',
  webhooks: 'WEBHOOKS'
};

function Placeholder({ name }: { name: string }) {
  return (
    <div className="placeholder-panel">
      <Empty description={`${name} 会在下一阶段迁移`} />
    </div>
  );
}

export default function App() {
  const [activeWorkspace, setActiveWorkspace] = useState('market');
  const [serviceStatus, setServiceStatus] = useState<ServiceStatusResult>();
  const [serviceLoading, setServiceLoading] = useState(false);
  const [serviceError, setServiceError] = useState('');

  useEffect(() => {
    let disposed = false;
    async function refreshStatus() {
      setServiceLoading(true);
      try {
        const data = await apiFetch<ServiceStatusResult>('/api/services/status');
        if (disposed) return;
        setServiceStatus(data);
        setServiceError('');
      } catch (error) {
        if (disposed) return;
        setServiceError(error instanceof Error ? error.message : '服务状态获取失败');
      } finally {
        if (!disposed) setServiceLoading(false);
      }
    }
    refreshStatus();
    const timer = window.setInterval(refreshStatus, 30000);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, []);

  const tabItems = useMemo(
    () =>
      workspaces.map((workspace) => ({
        key: workspace.key,
        label: workspace.label,
        children: workspace.key === 'market' ? (
          <WatchlistTable />
        ) : workspace.key === 'proChart' ? (
          <MarketWorkspace />
        ) : workspace.key === 'klineAnalysis' ? (
          <KlineAnalysisWorkspace />
        ) : workspace.key === 'dataCenter' ? (
          <DataCenterWorkspace />
        ) : workspace.key === 'selectionResults' ? (
          <SelectionResultsWorkspace />
        ) : workspace.key === 'dailyReview' ? (
          <DailyReviewWorkspace />
        ) : workspace.key === 'tradingSystem' ? (
          <TradingSystemWorkspace />
        ) : workspace.key === 'strategies' ? (
          <StrategiesWorkspace />
        ) : workspace.key === 'automations' ? (
          <AutomationsWorkspace />
        ) : workspace.key === 'aiConfigs' ? (
          <AIConfigsWorkspace />
        ) : workspace.key === 'aiAssistant' ? (
          <AIAssistantWorkspace />
        ) : workspace.key === 'alerts' ? (
          <AlertsWorkspace />
        ) : workspace.key === 'webhooks' ? (
          <WebhooksWorkspace />
        ) : (
          <Placeholder name={workspace.label} />
        )
      })),
    []
  );

  const activeWorkspaceLabel = workspaces.find((workspace) => workspace.key === activeWorkspace)?.label || '工作台';

  return (
    <Layout className="page-shell">
      <div className="page-container">
        <AppHeader status={serviceStatus} loading={serviceLoading} error={serviceError} />
        <div className="terminal-bar" aria-label="当前工作区">
          <span className="terminal-path">TDX://LOCAL/{workspaceCodes[activeWorkspace] || activeWorkspace.toUpperCase()}</span>
          <strong>{activeWorkspaceLabel}</strong>
          <span>{serviceStatus?.checked_at ? `CHECKED ${new Date(serviceStatus.checked_at).toLocaleTimeString('zh-CN', { hour12: false })}` : 'CHECKING'}</span>
        </div>
        <Tabs
          className="workspace-tabs"
          activeKey={activeWorkspace}
          items={tabItems}
          onChange={setActiveWorkspace}
          destroyOnHidden={false}
        />
      </div>
    </Layout>
  );
}
