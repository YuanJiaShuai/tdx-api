import { useEffect, useRef } from 'react';

interface HQChartPanelProps {
  symbol: string;
  period?: string;
  count?: number;
  pageSize?: number;
  className?: string;
}

export function HQChartPanel({ symbol, period = 'day', count = 800, pageSize = 80, className }: HQChartPanelProps) {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const container = ref.current;
    if (!container) return;
    const api = window.TDXHQChart;
    if (!api?.isAvailable?.()) {
      container.innerHTML = '<div class="chart-empty">HQChart 未加载</div>';
      return;
    }
    const ok = api.renderKLine(container, {
      symbol,
      period,
      count,
      pageSize
    });
    if (!ok) {
      container.innerHTML = '<div class="chart-empty">图表加载失败</div>';
      return;
    }
    const onResize = () => api.resize?.(container);
    window.addEventListener('resize', onResize);
    onResize();
    return () => {
      window.removeEventListener('resize', onResize);
      api.destroy?.(container);
    };
  }, [count, pageSize, period, symbol]);

  return <div ref={ref} className={className ? `${className} hq-chart-surface` : 'hq-chart-surface'} />;
}
