import { Typography } from 'antd';

const { Paragraph } = Typography;

const emptyText: Record<string, string> = {
  尚未查询: '等待查询结果。选择条件后执行查询。',
  暂无运行: '等待运行结果。执行任务后会显示返回数据。',
  暂无回测: '等待策略运行或回测。',
  尚未测试: '等待连接测试结果。',
  暂无测试: '等待测试结果。'
};

export function JsonPane({ value }: { value: unknown }) {
  let text = '';
  try {
    text = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  } catch {
    text = String(value);
  }
  const display = emptyText[text] || text || '暂无数据';
  return <Paragraph className="json-pane">{display}</Paragraph>;
}
