import { Button, Drawer, Form, Input, List, Popconfirm, Select, Space, Tag, Typography, message } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';
import { useEffect } from 'react';
import { apiFetch } from '../lib/api';
import {
  compactFormulaScript,
  formatFormulaArgs,
  formulaTypeLabel,
  normalizeFormulaArgs,
  periodLabel,
  safeParseFormulaArgsJSON
} from '../lib/formula';
import type { Formula, FormulaArg } from '../types';

const { Text } = Typography;

interface FormulaManagerProps {
  open: boolean;
  formulas: Formula[];
  loading: boolean;
  editingFormula?: Formula;
  onClose: () => void;
  onReload: () => Promise<void>;
  onEdit: (formula?: Formula) => void;
}

interface FormulaFormValues {
  name: string;
  type: string;
  period: string;
  right: number;
  script: string;
  args: FormulaArg[];
}

export function FormulaManager({
  open,
  formulas,
  loading,
  editingFormula,
  onClose,
  onReload,
  onEdit
}: FormulaManagerProps) {
  const [form] = Form.useForm<FormulaFormValues>();

  useEffect(() => {
    if (!open) return;
    if (editingFormula) {
      form.setFieldsValue({
        name: editingFormula.name,
        type: editingFormula.type || 'indicator',
        period: editingFormula.period || 'day',
        right: editingFormula.right ?? 1,
        script: editingFormula.script,
        args: safeParseFormulaArgsJSON(editingFormula.args_json)
      });
    } else {
      form.setFieldsValue({
        name: '',
        type: 'indicator',
        period: 'day',
        right: 1,
        script: '',
        args: []
      });
    }
  }, [editingFormula, form, open]);

  async function saveFormula(values: FormulaFormValues) {
    const payload = {
      id: editingFormula?.id || '',
      name: values.name,
      type: values.type,
      period: values.period,
      right: Number(values.right ?? 1),
      script: values.script,
      args_json: JSON.stringify(normalizeFormulaArgs(values.args || [])),
      enabled: true
    };
    await apiFetch(editingFormula?.id ? `/api/formulas/${editingFormula.id}` : '/api/formulas', {
      method: editingFormula?.id ? 'PUT' : 'POST',
      body: JSON.stringify(payload)
    });
    message.success('公式已保存');
    onEdit(undefined);
    await onReload();
  }

  async function deleteFormula(id: string) {
    await apiFetch(`/api/formulas/${id}`, { method: 'DELETE' });
    message.success('公式已删除');
    if (editingFormula?.id === id) onEdit(undefined);
    await onReload();
  }

  return (
    <Drawer
      title="公式管理"
      width={920}
      open={open}
      onClose={onClose}
      className="formula-drawer"
      destroyOnHidden
    >
      <div className="formula-manager-grid">
        <section>
          <div className="section-head">
            <h3>{editingFormula ? '编辑公式' : '新建公式'}</h3>
            <Button icon={<PlusOutlined />} onClick={() => onEdit(undefined)}>
              新建
            </Button>
          </div>
          <Form form={form} layout="vertical" onFinish={saveFormula}>
            <Form.Item name="name" label="公式名称" rules={[{ required: true, message: '请输入公式名称' }]}>
              <Input placeholder="例如：主力拉升" />
            </Form.Item>
            <div className="form-grid">
              <Form.Item name="type" label="类型" rules={[{ required: true }]}>
                <Select
                  options={[
                    { value: 'indicator', label: '图表指标' },
                    { value: 'selection', label: '选股公式' }
                  ]}
                />
              </Form.Item>
              <Form.Item name="period" label="周期" rules={[{ required: true }]}>
                <Select
                  options={[
                    { value: 'day', label: '日K' },
                    { value: 'week', label: '周K' },
                    { value: 'month', label: '月K' },
                    { value: 'minute5', label: '5分钟' },
                    { value: 'minute30', label: '30分钟' }
                  ]}
                />
              </Form.Item>
              <Form.Item name="right" label="复权">
                <Select
                  options={[
                    { value: 1, label: '前复权' },
                    { value: 0, label: '不复权' },
                    { value: 2, label: '后复权' }
                  ]}
                />
              </Form.Item>
            </div>
            <Form.Item name="script" label="脚本" rules={[{ required: true, message: '请输入公式脚本' }]}>
              <Input.TextArea rows={10} className="formula-script-input" placeholder="CROSS(MA(C,5),MA(C,20));" />
            </Form.Item>
            <Form.List name="args">
              {(fields, { add, remove }) => (
                <div className="formula-args-block">
                  <div className="section-head compact-head">
                    <h4>公式参数</h4>
                    <Button size="small" onClick={() => add({ Name: '', Value: '' })}>
                      新增参数
                    </Button>
                  </div>
                  {fields.map((field) => (
                    <Space key={field.key} align="baseline" className="arg-row">
                      <Form.Item {...field} name={[field.name, 'Name']} rules={[{ required: true, message: '参数名' }]}>
                        <Input placeholder="参数名" />
                      </Form.Item>
                      <Form.Item {...field} name={[field.name, 'Value']} rules={[{ required: true, message: '参数值' }]}>
                        <Input placeholder="参数值" />
                      </Form.Item>
                      <Button danger onClick={() => remove(field.name)}>
                        删除
                      </Button>
                    </Space>
                  ))}
                  {!fields.length ? <Text type="secondary">暂无参数，可新增。</Text> : null}
                </div>
              )}
            </Form.List>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} className="save-formula-btn">
              保存公式
            </Button>
          </Form>
        </section>

        <section>
          <h3>公式列表</h3>
          <List
            loading={loading}
            dataSource={formulas}
            locale={{ emptyText: '暂无公式' }}
            renderItem={(formula) => (
              <List.Item
                actions={[
                  <Button key="edit" icon={<EditOutlined />} onClick={() => onEdit(formula)}>
                    编辑
                  </Button>,
                  <Popconfirm
                    key="delete"
                    title="确认删除这个公式？"
                    okText="删除"
                    cancelText="取消"
                    onConfirm={() => deleteFormula(formula.id)}
                  >
                    <Button danger icon={<DeleteOutlined />}>
                      删除
                    </Button>
                  </Popconfirm>
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <span>{formula.name}</span>
                      <Tag color={formula.enabled ? 'green' : 'default'}>{formula.enabled ? '启用' : '停用'}</Tag>
                    </Space>
                  }
                  description={
                    <div className="formula-list-description">
                      <Text type="secondary">
                        {formulaTypeLabel(formula.type)} · {periodLabel(formula.period)} · 参数：{formatFormulaArgs(formula)}
                      </Text>
                      <code title={formula.script}>{compactFormulaScript(formula.script)}</code>
                    </div>
                  }
                />
              </List.Item>
            )}
          />
        </section>
      </div>
    </Drawer>
  );
}
