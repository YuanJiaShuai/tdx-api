import { Button, Card, Empty, Select, Space, Table, Tag, Typography, message } from 'antd';
import { LinkOutlined, ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch } from '../lib/api';
import type {
  IndustryDictItem,
  IndustryDictResponse,
  IndustryResearchReport,
  IndustryResearchResponse
} from '../types';

const { Text } = Typography;

const dayOptions = [
  { value: 7, label: '近 7 天' },
  { value: 30, label: '近 30 天' },
  { value: 90, label: '近 90 天' },
  { value: 365, label: '近一年' }
];

function sourceText(source?: string) {
  switch (source) {
    case 'cache':
      return '共享缓存';
    case 'database':
      return '本地数据库';
    default:
      return source ? '东方财富接口' : '暂无来源';
  }
}

function displayDate(value?: string) {
  return String(value || '').slice(0, 10) || '--';
}

function ratingChangeName(value?: number) {
  switch (Number(value)) {
    case 0:
      return '调高';
    case 1:
      return '调低';
    case 2:
      return '首次';
    case 3:
      return '维持';
    case 4:
      return '无变化';
    default:
      return '--';
  }
}

function openReport(item: IndustryResearchReport) {
  if (!item.infoCode) {
    message.warning('这条研报没有可用的报告地址');
    return;
  }
  window.open(`https://pdf.dfcfw.com/pdf/H3_${item.infoCode}_1.pdf`, '_blank', 'noopener,noreferrer');
}

export function IndustryResearchWorkspace() {
  const [items, setItems] = useState<IndustryResearchReport[]>([]);
  const [options, setOptions] = useState<IndustryDictItem[]>([]);
  const [industryCode, setIndustryCode] = useState('');
  const [days, setDays] = useState(7);
  const [loading, setLoading] = useState(false);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [source, setSource] = useState('');
  const [fetchedAt, setFetchedAt] = useState('');
  const searchTimer = useRef<number | undefined>(undefined);

  const load = useCallback(async (nextCode = industryCode, nextDays = days) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        industry_code: nextCode,
        days: String(nextDays),
        limit: '50'
      });
      const data = await apiFetch<IndustryResearchResponse>(`/api/market/industry/research?${params.toString()}`);
      setItems(Array.isArray(data.items) ? data.items : []);
      setSource(data.source || '');
      setFetchedAt(data.fetched_at || '');
    } catch (error) {
      setItems([]);
      message.error(error instanceof Error ? error.message : '行业研究加载失败');
    } finally {
      setLoading(false);
    }
  }, [days, industryCode]);

  const loadOptions = useCallback(async (keyword: string) => {
    if (searchTimer.current) {
      window.clearTimeout(searchTimer.current);
    }
    if (!keyword.trim()) {
      setOptions([]);
      return;
    }
    searchTimer.current = window.setTimeout(async () => {
      setOptionsLoading(true);
      try {
        const data = await apiFetch<IndustryDictResponse>('/api/market/industry/options');
        const normalized = keyword.trim().toLowerCase();
        setOptions(
          (data.items || []).filter((item) => {
            const name = String(item.bkName || '').toLowerCase();
            const code = String(item.bkCode || '').toLowerCase();
            const letter = String(item.firstLetter || '').toLowerCase();
            return name.includes(normalized) || code.includes(normalized) || letter.includes(normalized);
          }).slice(0, 30)
        );
      } catch (error) {
        message.error(error instanceof Error ? error.message : '行业字典加载失败');
      } finally {
        setOptionsLoading(false);
      }
    }, 180);
  }, []);

  useEffect(() => {
    void load();
    return () => {
      if (searchTimer.current) {
        window.clearTimeout(searchTimer.current);
      }
    };
  }, []);

  const selectedOption = options.find((item) => item.bkCode === industryCode);
  const titleSuffix = selectedOption?.bkName || (industryCode ? `行业 ${industryCode}` : '全部行业');

  return (
    <Card
      className="work-card industry-research-card"
      title={
        <div className="market-panel-title">
          <strong>行业研究</strong>
          <Text type="secondary">
            {titleSuffix} · {items.length} 条
          </Text>
        </div>
      }
      extra={
        <Space className="industry-research-toolbar">
          <Select
            showSearch
            allowClear
            value={industryCode || undefined}
            placeholder="搜索行业名称或代码"
            className="industry-research-select"
            filterOption={false}
            loading={optionsLoading}
            options={options.map((item) => ({
              value: item.bkCode || '',
              label: `${item.bkName || '--'} · ${item.bkCode || '--'}`
            }))}
            onSearch={(value) => void loadOptions(value)}
            onChange={(value) => {
              const nextCode = String(value || '');
              setIndustryCode(nextCode);
              void load(nextCode, days);
            }}
            onClear={() => {
              setIndustryCode('');
              void load('', days);
            }}
          />
          <Select
            value={days}
            options={dayOptions}
            className="industry-research-days"
            onChange={(value) => {
              setDays(value);
              void load(industryCode, value);
            }}
          />
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>
            刷新
          </Button>
        </Space>
      }
    >
      <div className="market-info-meta">
        <Text type="secondary">数据源：东方财富行业研报接口</Text>
        <Text type="secondary">
          {sourceText(source)} · 更新于 {fetchedAt ? new Date(fetchedAt).toLocaleString('zh-CN', { hour12: false }) : '--'}
        </Text>
      </div>

      <Table<IndustryResearchReport>
        className="market-info-table industry-research-table"
        size="small"
        rowKey={(item, index) => item.infoCode || `${item.industryCode}-${item.publishDate}-${index}`}
        dataSource={items}
        loading={loading}
        pagination={{ pageSize: 15, size: 'small', showSizeChanger: false }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无行业研究报告" /> }}
        scroll={{ x: 1180 }}
        columns={[
          {
            title: '行业',
            dataIndex: 'industryName',
            width: 150,
            fixed: 'left',
            render: (value) => <Tag color="blue">{value || '--'}</Tag>
          },
          {
            title: '研报标题',
            dataIndex: 'title',
            width: 420,
            ellipsis: true,
            render: (value: string, item) => (
              <Button
                type="link"
                className="market-info-link"
                icon={<LinkOutlined />}
                onClick={() => openReport(item)}
              >
                {value || '--'}
              </Button>
            )
          },
          { title: '东方财富评级', dataIndex: 'emRatingName', width: 120, render: (value) => value || '--' },
          { title: '评级变动', dataIndex: 'ratingChange', width: 100, render: ratingChangeName },
          { title: '机构评级', dataIndex: 'sRatingName', width: 100, render: (value) => value || '--' },
          { title: '分析师', dataIndex: 'researcher', width: 180, ellipsis: true, render: (value) => value || '--' },
          { title: '机构', dataIndex: 'orgSName', width: 150, ellipsis: true, render: (value) => value || '--' },
          { title: '日期', dataIndex: 'publishDate', width: 110, render: displayDate }
        ]}
      />
    </Card>
  );
}
