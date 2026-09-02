import { Button, Modal, Space, message } from 'antd';
import { CloseOutlined, ReloadOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../lib/api';
import { formatAmount, formatPrice, formatSigned, normalizeSymbol, priceFromMilli } from '../lib/format';
import type { Quote } from '../types';
import { HQChartPanel } from './HQChartPanel';

export interface StockQuoteTarget {
  code: string;
  name?: string;
}

interface StockQuoteModalProps {
  target: StockQuoteTarget | null;
  onClose: () => void;
}

const periodOptions = [
  ['minute', '分时'],
  ['day', '日K'],
  ['week', '周K'],
  ['month', '月K'],
  ['minute30', '30分'],
  ['minute5', '5分']
] as const;

export function StockQuoteModal({ target, onClose }: StockQuoteModalProps) {
  const [quote, setQuote] = useState<Quote | null>(null);
  const [loading, setLoading] = useState(false);
  const [period, setPeriod] = useState('day');

  const loadQuote = useCallback(async () => {
    if (!target?.code) return;
    setLoading(true);
    try {
      const quotes = await apiFetch<Quote[]>(`/api/quote?code=${encodeURIComponent(normalizeSymbol(target.code))}`);
      setQuote(Array.isArray(quotes) ? quotes[0] || null : null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '行情加载失败');
    } finally {
      setLoading(false);
    }
  }, [target?.code]);

  useEffect(() => {
    setPeriod('day');
    setQuote(null);
    if (target?.code) void loadQuote();
  }, [loadQuote, target?.code]);

  const price = quote?.K ? priceFromMilli(quote.K.Close) : 0;
  const previousClose = quote?.K ? priceFromMilli(quote.K.Last) : 0;
  const change = price - previousClose;
  const changePercent = previousClose > 0 ? (change / previousClose) * 100 : 0;
  const symbol = normalizeSymbol(target?.code || quote?.Code || '');

  return (
    <Modal
      open={Boolean(target)}
      onCancel={onClose}
      footer={null}
      width={1240}
      centered
      destroyOnHidden
      closeIcon={<CloseOutlined />}
      className="watchlist-quote-modal stock-quote-modal"
      title={
        <div className="quote-dialog-title">
          <div>
            <strong>{target?.name || quote?.Name || '--'}</strong>
            <span>{symbol}</span>
          </div>
          <small>实时行情 · 五档盘口</small>
        </div>
      }
    >
      <div className="quote-dialog-toolbar">
        <Space wrap>
          {periodOptions.map(([value, label]) => (
            <Button key={value} size="small" type={period === value ? 'primary' : 'default'} onClick={() => setPeriod(value)}>
              {label}
            </Button>
          ))}
        </Space>
        <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={() => void loadQuote()}>
          刷新
        </Button>
      </div>
      <div className="quote-dialog-grid">
        <div className="quote-dialog-chart">
          {target ? <HQChartPanel key={`${symbol}-${period}`} symbol={symbol} period={period} count={800} pageSize={80} /> : null}
        </div>
        <aside className="quote-dialog-side">
          <section className="quote-dialog-summary">
            <span>现价</span>
            <strong className={change >= 0 ? 'market-up' : 'market-down'}>{price > 0 ? formatPrice(price) : '--'}</strong>
            <div className={change >= 0 ? 'market-up' : 'market-down'}>
              {previousClose > 0 ? `${formatSigned(change)} · ${formatSigned(changePercent, '%')}` : '--'}
            </div>
          </section>
          <section className="quote-dialog-metrics">
            {[
              ['开盘', quote?.K?.Open ? formatPrice(priceFromMilli(quote.K.Open)) : '--'],
              ['最高', quote?.K?.High ? formatPrice(priceFromMilli(quote.K.High)) : '--'],
              ['最低', quote?.K?.Low ? formatPrice(priceFromMilli(quote.K.Low)) : '--'],
              ['成交量', quote?.TotalHand ? formatAmount(quote.TotalHand * 100) : '--'],
              ['成交额', quote?.Amount ? formatAmount(quote.Amount) : '--'],
              ['涨速', quote?.Rate ? formatSigned(quote.Rate, '%') : '--']
            ].map(([label, value]) => (
              <div key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </section>
          <section className="quote-dialog-levels">
            <h4>五档盘口</h4>
            <div className="quote-dialog-level-columns">
              <div>
                <small>卖盘</small>
                {(quote?.SellLevel || []).slice().reverse().map((level, index) => (
                  <div className="quote-dialog-level sell" key={`sell-${index}`}>
                    <span>卖{5 - index}</span>
                    <b>{level.Price ? formatPrice(priceFromMilli(level.Price)) : '--'}</b>
                    <em>{level.Number ? Math.round(level.Number / 100) : '--'}</em>
                  </div>
                ))}
              </div>
              <div>
                <small>买盘</small>
                {(quote?.BuyLevel || []).map((level, index) => (
                  <div className="quote-dialog-level buy" key={`buy-${index}`}>
                    <span>买{index + 1}</span>
                    <b>{level.Price ? formatPrice(priceFromMilli(level.Price)) : '--'}</b>
                    <em>{level.Number ? Math.round(level.Number / 100) : '--'}</em>
                  </div>
                ))}
              </div>
            </div>
          </section>
        </aside>
      </div>
    </Modal>
  );
}
