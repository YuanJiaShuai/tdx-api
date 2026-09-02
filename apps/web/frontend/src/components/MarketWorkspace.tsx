import { Tabs } from 'antd';
import { useState } from 'react';
import { LongTigerWorkspace } from './LongTigerWorkspace';
import { MarketInfoWorkspace } from './MarketInfoWorkspace';
import { ProChartWorkspace } from './ProChartWorkspace';
import { IndustryRankWorkspace } from './IndustryRankWorkspace';
import { StockMoneyFlowWorkspace } from './StockMoneyFlowWorkspace';

export function MarketWorkspace() {
  const [activeTab, setActiveTab] = useState('chart');

  return (
    <div className="market-workspace">
      <Tabs
        className="market-inner-tabs"
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'chart',
            label: '行情图表',
            children: <ProChartWorkspace />
          },
          {
            key: 'industry-rank',
            label: '行业排名',
            children: <IndustryRankWorkspace />
          },
          {
            key: 'stock-money',
            label: '个股资金流向',
            children: <StockMoneyFlowWorkspace />
          },
          {
            key: 'long-tiger',
            label: '龙虎榜',
            children: <LongTigerWorkspace />
          },
          {
            key: 'hot-money',
            label: '游资动向',
            children: <MarketInfoWorkspace kind="hot-money" />
          },
          {
            key: 'research',
            label: '个股研报',
            children: <MarketInfoWorkspace kind="research" />
          },
          {
            key: 'notice',
            label: '公司公告',
            children: <MarketInfoWorkspace kind="notice" />
          }
        ]}
      />
    </div>
  );
}
