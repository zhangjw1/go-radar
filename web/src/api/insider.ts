import axios from 'axios';

export interface InsiderWallet {
  id: number;
  address: string;
  label: string;
  created_at: string;
  updated_at: string;
}

export interface TokenHolding {
  mint_address: string;
  token_name: string;
  balance: number;
  decimals: number;
  usd_value: number;
  total_bought: number;
  total_sold: number;
  remaining: number;
  cost_basis: number;
  current_value: number;
  pnl: number;
  pnl_percent: number;
}

export interface InsiderTransaction {
  id: number;
  wallet_id: number;
  signature: string;
  mint_address: string;
  token_name: string;
  tx_type: string;
  amount: number;
  sol_amount: number;
  block_time: string;
}

export interface WalletAnalytics {
  wallet_id: number;
  win_rate: number;
  total_pnl: number;
  realized_pnl: number;
  unrealized_pnl: number;
  tx_count: number;
  buy_count: number;
  sell_count: number;
  total_cost: number;
  avg_cost_per_token: number;
  last_active_time: string | null;
  period: string;
}

export interface AlertRule {
  id?: number;
  wallet_id: number | null;
  rule_type: string;
  threshold: number;
  channel_ids: number[];
  enabled: boolean;
  wallet?: InsiderWallet;
  created_at?: string;
}

export interface NotificationChannel {
  id?: number;
  name: string;
  channel_type: string;
  recipient: string;
  webhook_url: string;
  bot_token: string;
  chat_id: string;
  enabled: boolean;
}

export interface AlertHistory {
  id: number;
  alert_type: string;
  level: string;
  message: string;
  created_at: string;
}

export interface SyncStatus {
  syncing: boolean;
  last_error: string;
  engine: string;
}

interface ItemResponse<T> {
  item: T;
}

interface ListResponse<T> {
  items: T[];
}

export function queryInsiderWallets() {
  return axios.get<ListResponse<InsiderWallet>>('/radar-api/insider/wallets');
}

export function createInsiderWallet(
  payload: Pick<InsiderWallet, 'address' | 'label'>
) {
  return axios.post<ItemResponse<InsiderWallet>>(
    '/radar-api/insider/wallets',
    payload
  );
}

export function updateInsiderWallet(id: number, label: string) {
  return axios.put<ItemResponse<InsiderWallet>>(
    `/radar-api/insider/wallets/${id}`,
    { label }
  );
}

export function deleteInsiderWallet(id: number) {
  return axios.delete(`/radar-api/insider/wallets/${id}`);
}

export function queryInsiderWallet(id: number) {
  return axios.get<ItemResponse<InsiderWallet>>(
    `/radar-api/insider/wallets/${id}`
  );
}

export function queryInsiderPortfolio(id: number) {
  return axios.get<ListResponse<TokenHolding>>(
    `/radar-api/insider/wallets/${id}/portfolio`
  );
}

export function queryInsiderTransactions(id: number, limit = 100) {
  return axios.get<ListResponse<InsiderTransaction>>(
    `/radar-api/insider/wallets/${id}/transactions`,
    {
      params: { limit },
    }
  );
}

export function queryInsiderAnalytics(id: number, period = 'all') {
  return axios.get<ItemResponse<WalletAnalytics>>(
    `/radar-api/insider/wallets/${id}/analytics`,
    {
      params: { period },
    }
  );
}

export function triggerInsiderSync() {
  return axios.post('/radar-api/insider/sync/trigger');
}

export function queryInsiderSyncStatus() {
  return axios.get<ItemResponse<SyncStatus>>('/radar-api/insider/sync/status');
}

export function updateInsiderEngine(engine: 'service' | 'legacy') {
  return axios.post('/radar-api/settings', {
    settings: {
      insider_monitor_engine: engine,
    },
  });
}

export function queryAlertRules() {
  return axios.get<ListResponse<AlertRule>>('/radar-api/insider/alerts/rules');
}

export function saveAlertRule(rule: AlertRule) {
  if (rule.id) {
    return axios.put<ItemResponse<AlertRule>>(
      `/radar-api/insider/alerts/rules/${rule.id}`,
      rule
    );
  }
  return axios.post<ItemResponse<AlertRule>>(
    '/radar-api/insider/alerts/rules',
    rule
  );
}

export function queryAlertHistory() {
  return axios.get<ListResponse<AlertHistory>>(
    '/radar-api/insider/alerts/history'
  );
}

export function queryNotificationChannels() {
  return axios.get<ListResponse<NotificationChannel>>(
    '/radar-api/insider/alerts/channels'
  );
}

export function saveNotificationChannel(channel: NotificationChannel) {
  if (channel.id) {
    return axios.put<ItemResponse<NotificationChannel>>(
      `/radar-api/insider/alerts/channels/${channel.id}`,
      channel
    );
  }
  return axios.post<ItemResponse<NotificationChannel>>(
    '/radar-api/insider/alerts/channels',
    channel
  );
}
