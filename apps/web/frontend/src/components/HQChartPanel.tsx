import { useEffect, useMemo, useRef } from 'react';

interface HQChartPanelProps {
  symbol: string;
  period?: string;
  count?: number;
  pageSize?: number;
  windows?: Array<Record<string, unknown>>;
  dataWidth?: number;
  className?: string;
}

export function HQChartPanel({
  symbol,
  period = 'day',
  count = 800,
  pageSize = 80,
  windows,
  dataWidth,
  className
}: HQChartPanelProps) {
  const ref = useRef<HTMLDivElement | null>(null);
  const isMinuteChart = useMemo(() => String(period).toLowerCase() === 'minute', [period]);

  useEffect(() => {
    const container = ref.current;
    if (!container) return;
    const api = window.TDXHQChart;
    if (!api?.isAvailable?.()) {
      container.innerHTML = '<div class="chart-empty">HQChart 未加载</div>';
      return;
    }
    const render = isMinuteChart && api.renderMinute ? api.renderMinute : api.renderKLine;
    const ok = render(container, {
      symbol,
      period,
      count,
      pageSize,
      windows,
      dataWidth
    });
    if (!ok) {
      container.innerHTML = '<div class="chart-empty">图表加载失败</div>';
      return;
    }
    const onResize = () => api.resize?.(container);
    window.addEventListener('resize', onResize);
    onResize();
    const resizeTimer = window.setTimeout(onResize, 180);
    return () => {
      window.clearTimeout(resizeTimer);
      window.removeEventListener('resize', onResize);
      api.destroy?.(container);
    };
  }, [count, dataWidth, isMinuteChart, pageSize, period, symbol, windows]);

  return <div ref={ref} className={className ? `${className} hq-chart-surface` : 'hq-chart-surface'} />;
}
