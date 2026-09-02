export interface ServiceStatus {
  id: string;
  name: string;
  status: string;
  healthy: boolean;
  latency_ms?: number;
}

export interface ServiceStatusResult {
  services: ServiceStatus[];
  ready: boolean;
  checked_at: string;
}

export interface QuoteK {
  Last?: number;
  Close?: number;
  Open?: number;
  High?: number;
  Low?: number;
}

export interface PriceLevel {
  Price?: number;
  Number?: number;
}

export interface Quote {
  Code?: string;
  Name?: string;
  K?: QuoteK;
  TotalHand?: number;
  Amount?: number;
  Rate?: number;
  BuyLevel?: PriceLevel[];
  SellLevel?: PriceLevel[];
}

export interface LongTigerRank {
  ACCUM_AMOUNT?: number;
  BILLBOARD_BUY_AMT?: number;
  BILLBOARD_DEAL_AMT?: number;
  BILLBOARD_NET_AMT?: number;
  BILLBOARD_SELL_AMT?: number;
  CHANGE_RATE?: number;
  CLOSE_PRICE?: number;
  DEAL_AMOUNT_RATIO?: number;
  DEAL_NET_RATIO?: number;
  D1_CLOSE_ADJCHRATE?: number;
  D2_CLOSE_ADJCHRATE?: number;
  D5_CLOSE_ADJCHRATE?: number;
  D10_CLOSE_ADJCHRATE?: number;
  EXPLAIN?: string;
  EXPLANATION?: string;
  FREE_MARKET_CAP?: number;
  SECUCODE?: string;
  SECURITY_CODE?: string;
  SECURITY_NAME_ABBR?: string;
  SECURITY_TYPE_CODE?: string;
  TRADE_DATE?: string;
  TURNOVERRATE?: number;
}

export interface LongTigerResponse {
  requested_date: string;
  trade_date: string;
  items: LongTigerRank[];
  source: string;
  cached_at?: string;
}

export interface MarketResearchReport {
  title?: string;
  stockName?: string;
  stockCode?: string;
  orgSName?: string;
  publishDate?: string;
  infoCode?: string;
  indvInduName?: string;
  emRatingName?: string;
  ratingChange?: number;
  sRatingName?: string;
  researcher?: string;
  market?: string;
}

export interface IndustryResearchReport {
  title?: string;
  stockName?: string;
  stockCode?: string;
  orgCode?: string;
  orgName?: string;
  orgSName?: string;
  publishDate?: string;
  infoCode?: string;
  industryCode?: string;
  industryName?: string;
  emIndustryCode?: string;
  emRatingName?: string;
  ratingChange?: number;
  sRatingName?: string;
  researcher?: string;
  author?: string[];
  reportType?: number;
  encodeUrl?: string;
}

export interface IndustryDictItem {
  bkCode?: string;
  fubkCode?: string;
  bkName?: string;
  publishCode?: string;
  firstLetter?: string;
}

export interface IndustryResearchResponse {
  items: IndustryResearchReport[];
  industry_code: string;
  days: number;
  limit: number;
  source: string;
  fetched_at: string;
}

export interface IndustryDictResponse {
  items: IndustryDictItem[];
  source: string;
  fetched_at: string;
}

export interface MarketNotice {
  art_code?: string;
  stock_code?: string;
  stock_name?: string;
  title?: string;
  column_name?: string;
  notice_date?: string;
  display_time?: string;
}

export interface HotMoneyTrade {
  TRADE_DATE?: string;
  EXPLANATION?: string;
  OPERATEDEPT_NAME?: string;
  BUY_AMT_REAL?: number | null;
  BUY_RATIO?: number | null;
  SELL_AMT_REAL?: number | null;
  SELL_RATIO?: number | null;
  SECURITY_CODE?: string;
  SECURITY_NAME_ABBR?: string;
  SECUCODE?: string;
}

export interface IndustryRankItem {
  bd_name?: string;
  bd_code?: string;
  bd_zxj?: string;
  bd_zd?: string;
  bd_zdf?: string;
  bd_zdf5?: string;
  bd_zdf20?: string;
  nzg_code?: string;
  nzg_name?: string;
  nzg_zxj?: string;
  nzg_zd?: string;
  nzg_zdf?: string;
}

export interface IndustryRankResponse {
  items: IndustryRankItem[];
  sort: string;
  limit: number;
  source: string;
  fetched_at: string;
}

export interface IndustryMoneyRankItem {
  cate_type?: string;
  category?: string;
  name?: string;
  avg_price?: string;
  avg_changeratio?: string;
  turnover?: string;
  inamount?: string;
  outamount?: string;
  netamount?: string;
  ratioamount?: string;
  ts_symbol?: string;
  ts_name?: string;
  ts_trade?: string;
  ts_changeratio?: string;
  ts_ratioamount?: string;
}

export interface IndustryMoneyRankResponse {
  items: IndustryMoneyRankItem[];
  category: string;
  sort: string;
  source: string;
  fetched_at: string;
}

export interface StockMoneyRankItem {
  symbol?: string;
  name?: string | boolean;
  trade?: string;
  changeratio?: string;
  turnover?: string;
  amount?: string;
  inamount?: string;
  outamount?: string;
  netamount?: string;
  ratioamount?: string;
  r0_in?: string;
  r0_out?: string;
  r0_net?: string;
  r3_in?: string;
  r3_out?: string;
  r3_net?: string;
  r0_ratio?: string;
  r3_ratio?: string;
  r0x_ratio?: string;
}

export interface StockMoneyRankResponse {
  items: StockMoneyRankItem[];
  sort: string;
  limit: number;
  source: string;
  fetched_at: string;
}

export interface RzrqRankItem {
  stockCode?: string;
  stockName?: string;
  date?: number;
  lrye?: string;
  lryeRate?: string;
  rzye?: string;
  rzyeRate?: string;
  rqye?: string;
  rqyeRate?: string;
  jmr?: string;
  jmrRate?: string;
  rzmre?: string;
  rzche?: string;
  rzjmce?: string;
  yezf?: string;
  close_price?: string;
  close_profit?: string;
  marketId?: string;
}

export interface RzrqRankResponse {
  type: string;
  list: RzrqRankItem[];
  requested_date?: string;
  data_date?: string;
  source: string;
  fetched_at: string;
}

export interface RzrqTrendItem {
  date?: string;
  rzye?: string;
  rzjlr?: string;
  spj?: string;
  spzf?: string;
}

export interface RzrqTrendResponse {
  type: string;
  code: string;
  items: RzrqTrendItem[];
  rzye_unit?: string;
  rzjlr_unit?: string;
  spj_unit?: string;
  spzf_unit?: string;
  update_time?: string;
  source: string;
  fetched_at: string;
}

export interface MarketTradingStatus {
  date: string;
  is_trading: boolean;
  checked_at: string;
  timezone: string;
}

export interface WatchlistRow {
  key: string;
  code: string;
  name: string;
  price?: string;
  change?: string;
  changePercent?: string;
  volume?: string;
  amount?: string;
  speed?: string;
  mainNetVolume?: string;
  mainNetInflow?: string;
  industry?: string;
  turnover?: string;
  category?: string;
  entrust?: string;
  volumeRatio?: string;
}

export interface FormulaArg {
  Name: string;
  Value: string | number | boolean;
}

export interface Formula {
  id: string;
  name: string;
  type: 'selection' | 'indicator' | string;
  script: string;
  args_json?: string;
  period: string;
  right: number;
  enabled: boolean;
  description?: string;
  created_at?: string;
  updated_at?: string;
}

export interface FormulaRunResponse {
  engine?: string;
  tick_ms?: number;
  data?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface StockPool {
  id: string;
  name: string;
  symbols: string[];
  description?: string;
  category?: string;
  system?: boolean;
  readonly?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface Strategy {
  id: string;
  name: string;
  description?: string;
  config_json: string;
  enabled: boolean;
  readonly?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface AutomationTask {
  id: string;
  name: string;
  type: string;
  cron: string;
  enabled: boolean;
  payload_json: string;
  webhook_ids: string;
  last_run_at?: string;
  next_run_at?: string;
  last_status?: string;
  last_message?: string;
  readonly?: boolean;
  system?: boolean;
}

export interface AutomationRun {
  id: string;
  task_id: string;
  task_name: string;
  task_type: string;
  status: string;
  started_at: string;
  finished_at?: string;
  log?: string;
  result_json?: string;
  matched_count?: number;
}

export interface SelectionResult {
  id: string;
  run_id: string;
  task_id: string;
  task_name: string;
  formula_id: string;
  formula_name: string;
  symbol: string;
  latest: number;
  detail_json: string;
  created_at: string;
}

export interface DecisionNote {
  symbol: string;
  status?: string;
  added_price?: number;
  add_reason?: string;
  plan_buy?: number;
  stop_loss?: number;
  review_note?: string;
  exclude_category?: string;
  exclude_reason?: string;
  created_at?: string;
  updated_at?: string;
}

export interface DailyReviewItem {
  result: SelectionResult;
  score: { total?: number; trend?: number; volume?: number; place?: number; risk?: number };
  track: {
    available?: boolean;
    date?: number;
    open_change?: number;
    max_gain?: number;
    drawdown?: number;
    close_change?: number;
    summary?: string;
  };
  note: DecisionNote;
  status?: string;
  watch?: boolean;
  excluded?: boolean;
}

export interface DailyReviewResponse {
  date?: string;
  summary?: {
    hits?: number;
    watch_count?: number;
    exclude_count?: number;
    avg_score?: number;
    handled_count?: number;
    tracked_count?: number;
    positive_count?: number;
    win_rate?: number;
    avg_close_change?: number;
  };
  items?: DailyReviewItem[];
  watch?: string[];
  exclude?: string[];
  notes?: DecisionNote[];
}

export interface Webhook {
  id: string;
  name: string;
  url: string;
  method: string;
  headers_json: string;
  events: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface TradingTrade {
  id: string;
  stockName: string;
  stockCode: string;
  direction?: 'buy' | 'sell';
  status: string;
  entryDate: string;
  entryPrice: number;
  exitDate?: string;
  exitPrice?: number;
  exitShares?: number;
  currentPrice: number;
  previousClosePrice?: number;
  shares: number;
  invalidPrice: number;
  positionLabel: string;
  targetOne: string;
  targetTwo: string;
  tradeMode: string;
  buyReason: string;
  exitRules: string;
  review: string;
}

export interface TradingSystemState {
  account: {
    principal: number;
    totalAssets: number;
    marketValue: number;
    dailyProfit: number;
    maxTradeRisk: number;
    maxPositionWeight: number;
  };
  discipline: {
    reason: boolean;
    invalid: boolean;
    risk: boolean;
    noImpulse: boolean;
  };
  fees: {
    buyCommissionRate: number;
    sellCommissionRate: number;
    stampTaxRate: number;
    transferFeeRate: number;
    minCommission: number;
  };
  filter: string;
  trades: TradingTrade[];
}

export interface AIProvider {
  id: string;
  name?: string;
  default_base_url?: string;
  default_models?: Array<{ id: string; name?: string }>;
}

export interface AICredential {
  id: string;
  name?: string;
  provider: string;
  base_url?: string;
  model?: string;
  enabled?: boolean;
  has_api_key?: boolean;
  api_key_masked?: string;
  source?: string;
  extra_json?: string;
}

export interface TDXHQChartAPI {
  isAvailable: () => boolean;
  renderKLine: (container: HTMLElement, options: Record<string, unknown>) => boolean;
  renderMinute?: (container: HTMLElement, options: Record<string, unknown>) => boolean;
  resize?: (container?: HTMLElement | null) => void;
  destroy?: (container?: HTMLElement | null) => void;
  getChart?: (container: HTMLElement) => unknown;
}

declare global {
  interface Window {
    TDXHQChart?: TDXHQChartAPI;
  }
}
