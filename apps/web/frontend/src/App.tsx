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
import { ProChartWorkspace } from './components/ProChartWorkspace';
import { WatchlistTable } from './components/WatchlistTable';
import { apiFetch } from './lib/api';
import type { ServiceStatusResult } from './types';

const workspaces = [
  { key: 'market', label: '自选' },
  { key: 'proChart', label: '专业行情' },
  { key: 'dataCenter', label: '数据中心' },
  { key: 'selectionResults', label: '选股结果' },
  { key: 'dailyReview', label: '每日复盘' },
  { key: 'tradingSystem', label: '交易系统' },
  { key: 'strategies', label: '策略中心' },
  { key: 'automations', label: '自动化' },
  { key: 'aiConfigs', label: 'AI 模型' },
  { key: 'webhooks', label: 'Webhook' }
];

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
          <ProChartWorkspace />
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
        ) : workspace.key === 'webhooks' ? (
          <WebhooksWorkspace />
        ) : (
          <Placeholder name={workspace.label} />
        )
      })),
    []
  );

  return (
    <Layout className="page-shell">
      <div className="page-container">
        <AppHeader status={serviceStatus} loading={serviceLoading} error={serviceError} />
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
