import { Empty, Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMemo } from 'react';

export interface DynamicTableProps<T extends object = Record<string, unknown>> {
  rows: T[];
  rowKey?: string | ((record: T, index?: number) => string);
  size?: 'small' | 'middle' | 'large';
  scrollY?: number;
}

function cellText(value: unknown): string {
  if (value === null || value === undefined || value === '') return '--';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '--';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function stableRowKey(record: object): string {
  const value = record as Record<string, unknown>;
  const id = value.id ?? value.ID ?? value.code ?? value.Code ?? value.symbol ?? value.Symbol ?? value.name ?? value.Name;
  if (id !== undefined && id !== null && id !== '') return String(id);
  return JSON.stringify(value);
}

export function DynamicTable<T extends object = Record<string, unknown>>({ rows, rowKey, size = 'small', scrollY }: DynamicTableProps<T>) {
  const columns = useMemo<ColumnsType<T>>(() => {
    const first = rows[0] || {};
    const keys = Object.keys(first as Record<string, unknown>);
    return keys.map((key) => ({
      title: key,
      dataIndex: key,
      key,
      ellipsis: true,
      render: (value: unknown) => <span>{cellText(value)}</span>
    }));
  }, [rows]);

  if (!rows.length) return <Empty className="work-empty" description="暂无表格数据" />;
  return (
    <Table
      size={size}
      rowKey={rowKey || stableRowKey}
      columns={columns}
      dataSource={rows}
      pagination={false}
      scroll={scrollY ? { x: true, y: scrollY } : { x: true }}
    />
  );
}
