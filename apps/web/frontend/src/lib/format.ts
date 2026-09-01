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
