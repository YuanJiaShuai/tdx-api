import type { StockPool } from '../types';

export function normalizeSymbol(value: unknown): string {
  return String(value || '')
    .trim()
    .toUpperCase()
    .replace(/^(SH|SZ|BJ)/, '');
}

export function priceFromMilli(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number / 1000 : 0;
}

export function formatPrice(value: unknown): string {
  const price = Number(value);
  return Number.isFinite(price) && price > 0 ? price.toFixed(2) : '--';
}

export function formatSigned(value: unknown, suffix = ''): string {
  const number = Number(value);
  if (!Number.isFinite(number)) return '--';
  const normalized = Math.abs(number) < 0.005 ? 0 : number;
  return `${normalized > 0 ? '+' : ''}${normalized.toFixed(2)}${suffix}`;
}

export function formatAmount(value: unknown): string {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) return '--';
  if (number >= 100000000) return `${(number / 100000000).toFixed(2)}亿`;
  if (number >= 10000) return `${(number / 10000).toFixed(2)}万`;
  return number.toFixed(0);
}

export function formatPercent(value: unknown): string {
  const number = Number(value);
  if (!Number.isFinite(number)) return '--';
  return `${number.toFixed(2)}%`;
}

export function localTime(value?: string): string {
  const date = value ? new Date(value) : new Date();
  if (Number.isNaN(date.getTime())) return new Date().toLocaleTimeString('zh-CN', { hour12: false });
  return date.toLocaleTimeString('zh-CN', { hour12: false });
}

interface UniverseTermLike {
  pool?: string;
  pool_id?: string;
  symbols?: string[];
}

function poolDisplayName(pools: StockPool[], poolID: string): string {
  const pool = pools.find((item) => item.id === poolID);
  return pool?.name || poolID || '未知池';
}

function termSummary(term: UniverseTermLike, pools: StockPool[]): string {
  const symbols = Array.isArray(term.symbols) ? term.symbols : [];
  if (symbols.length) return `手动代码${symbols.length}只`;
  const poolID = term.pool || term.pool_id || '';
  return poolDisplayName(pools, poolID);
}

function joinTerms(terms: UniverseTermLike[] | undefined, pools: StockPool[], fallback: string): string {
  if (!Array.isArray(terms) || terms.length === 0) return fallback;
  return terms.map((term) => termSummary(term, pools)).join(' + ');
}

// summarizeStrategyUniverse renders a strategy config's candidate-range description,
// e.g. "沪市主板 + 创业板 − 排除池" for expression configs or "全市场A股" for legacy ones.
export function summarizeStrategyUniverse(config: Record<string, unknown>, pools: StockPool[]): string {
  const universe = config.universe;
  const poolID = typeof config.pool_id === 'string' ? config.pool_id : '';
  const symbols = Array.isArray(config.symbols) ? config.symbols : [];
  if (typeof universe === 'string') {
    switch (universe.toLowerCase()) {
      case 'all_a':
      case 'all':
        return '全市场A股';
      case 'market':
        return `市场 · ${poolDisplayName(pools, poolID || 'market-all-a')}`;
      case 'symbols':
        return symbols.length ? `手动代码 ${symbols.length}只` : '手动代码(空)';
      case 'pool':
      case '':
        return `股票池 · ${poolDisplayName(pools, poolID || 'watchlist')}`;
      default:
        return `范围 ${universe}`;
    }
  }
  if (universe && typeof universe === 'object') {
    const expr = universe as { include?: UniverseTermLike[]; intersect?: UniverseTermLike[]; exclude?: UniverseTermLike[] };
    const include = joinTerms(expr.include, pools, '未设置起点');
    const intersect = joinTerms(expr.intersect, pools, '');
    const exclude = joinTerms(expr.exclude, pools, '');
    return `${include}${intersect ? ` ∩ ${intersect}` : ''}${exclude ? ` − ${exclude}` : ''}`;
  }
  return `默认观察池 · ${poolDisplayName(pools, poolID || 'watchlist')}`;
}

export function parseConfigJson(value?: string | null): Record<string, unknown> {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}
