import axios from 'axios';

export interface SignalEvent {
  id?: number;
  source: string;
  chain: string;
  address: string;
  symbol: string;
  signal_type: string;
  priority: string;
  score: number;
  reason: string;
  raw_json?: string;
  created_at: string;
  pushed_at?: string;
}

export interface ScannerRun {
  id?: number;
  scanner: string;
  status: string;
  signal_count: number;
  snapshot_count: number;
  error?: string;
  started_at: string;
  finished_at?: string;
}

export interface WatchlistItem {
  chain: string;
  address: string;
  symbol?: string;
  name?: string;
  status: string;
  note?: string;
  updated_at: string;
}

export interface TokenProfile {
  id: number;
  chain: string;
  address: string;
  symbol: string;
  name: string;
  narrative_theme: string;
  narrative_tags_json: string;
  social_links_json: string;
  description: string;
  first_seen_at: string;
  last_seen_at: string;
}

export interface TokenSnapshot {
  id: number;
  token_id: number;
  source: string;
  price?: number | null;
  mc?: number | null;
  liq?: number | null;
  volume?: number | null;
  holders?: number | null;
  smart_money?: number | null;
  funding_pct?: number | null;
  oi_usd?: number | null;
  oi_d6h?: number | null;
  buys_1h?: number | null;
  sells_1h?: number | null;
  age_h?: number | null;
  raw_json: string;
  created_at: string;
}

export interface FilterOptions {
  sources: string[];
  chains: string[];
  signalTypes: string[];
  priorities: string[];
  timeRanges: string[];
}

export interface SignalsResponse {
  items: SignalEvent[];
  total?: number;
  current?: number;
  pageSize?: number;
  filters?: FilterOptions;
}

export interface TokenDetailResponse {
  token: TokenProfile;
  snapshots: TokenSnapshot[];
  signals: SignalEvent[];
  watch_item: WatchlistItem | null;
}

export interface SettingItem {
  key: string;
  value: string;
  default?: string;
  override?: string;
}

interface ListResponse<T> {
  items: T[];
  total?: number;
  current?: number;
  pageSize?: number;
}

interface SettingsResponse {
  settings: SettingItem[] | Record<string, string | number | boolean>;
  overrides: Record<string, string>;
}

export function querySignals(params?: Record<string, unknown>) {
  return axios.get<SignalsResponse>('/radar-api/signals', { params });
}

export function queryPushes(params?: Record<string, unknown>) {
  return axios.get<ListResponse<SignalEvent>>('/radar-api/pushes', { params });
}

export function queryJobs() {
  return axios.get<ListResponse<ScannerRun>>('/radar-api/jobs');
}

export function queryWatchlist() {
  return axios.get<ListResponse<WatchlistItem>>('/radar-api/watchlist');
}

export function querySettings() {
  return axios.get<SettingsResponse>('/radar-api/settings');
}

export function queryToken(
  chain: string,
  address: string,
  params?: Record<string, unknown>
) {
  return axios.get<TokenDetailResponse>(
    `/radar-api/tokens/${chain}/${address}`,
    { params }
  );
}
