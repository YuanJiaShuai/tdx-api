import type { Formula, FormulaArg } from '../types';

export function normalizeArgValue(value: unknown): string | number | boolean {
  const text = String(value ?? '').trim();
  if (text === 'true') return true;
  if (text === 'false') return false;
  if (text !== '' && Number.isFinite(Number(text))) return Number(text);
  return text;
}

export function normalizeFormulaArgs(args: unknown): FormulaArg[] {
  if (!Array.isArray(args)) return [];
  return args
    .map((item) => {
      const arg = item && typeof item === 'object' ? (item as Record<string, unknown>) : {};
      const name = arg.Name ?? arg.name;
      const value = arg.Value ?? arg.value;
      return {
        Name: String(name || '').trim(),
        Value: normalizeArgValue(value)
      };
    })
    .filter((item) => item.Name && item.Value !== '');
}

export function parseFormulaArgs(formula?: Pick<Formula, 'args_json'> | { args?: unknown }): FormulaArg[] {
  const source = formula || {};
  const raw = 'args_json' in source ? source.args_json : (source as { args?: unknown }).args;
  if (Array.isArray(raw)) return normalizeFormulaArgs(raw);
  if (!String(raw || '').trim()) return [];
  try {
    return normalizeFormulaArgs(JSON.parse(String(raw)));
  } catch {
    return [];
  }
}

export function formatFormulaArgs(formula?: Formula): string {
  const args = parseFormulaArgs(formula);
  return args.length ? args.map((item) => `${item.Name}=${item.Value}`).join('，') : '无';
}

export function compactFormulaScript(script: string): string {
  return String(script || '').replace(/\s+/g, ' ').trim() || '暂无脚本';
}

export function safeParseFormulaArgsJSON(value?: string): FormulaArg[] {
  if (!String(value || '').trim()) return [];
  try {
    return normalizeFormulaArgs(JSON.parse(String(value)));
  } catch {
    return [];
  }
}

export function formulaTypeLabel(type: string): string {
  if (type === 'indicator') return '图表指标';
  if (type === 'selection') return '选股公式';
  return type || '--';
}

export function periodLabel(period: string): string {
  const labels: Record<string, string> = {
    day: '日K',
    week: '周K',
    month: '月K',
    minute5: '5分钟',
    minute30: '30分钟'
  };
  return labels[period] || period || '--';
}
