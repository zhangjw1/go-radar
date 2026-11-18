// 雷达来源、链名、信号类型与系统设置元数据常量表

export interface RadarSourceMeta {
  key: string;
  label: string;
  shortLabel: string;
  focus: string;
}

export const RADAR_SOURCES: RadarSourceMeta[] = [
  {
    key: 's1',
    label: 'S1 币安公告',
    shortLabel: 'S1',
    focus: '公告催化 / 上所预期',
  },
  {
    key: 's2',
    label: 'S2 费率反转',
    shortLabel: 'S2',
    focus: '费率转负 / OI 抬升',
  },
  {
    key: 's3',
    label: 'S3 热度确认',
    shortLabel: 'S3',
    focus: '热度 / 费率 / OI 共振',
  },
  {
    key: 's5',
    label: 'S5 链上发现',
    shortLabel: 'S5',
    focus: '链上新币 / 叙事 / 动量',
  },
  {
    key: 's7',
    label: 'S7 Vitalik Sell',
    shortLabel: 'S7',
    focus: 'DEX / CEX / LP 转出',
  },
];

export const SCANNER_KEYS = ['s1', 's2', 's3', 's5', 's7', 'insider'] as const;

export const CHAIN_LABELS: Record<string, string> = {
  binance_perp: 'Binance 永续',
  binance_alpha: 'Binance Alpha',
  eth: 'Ethereum',
  ethereum: 'Ethereum',
  bsc: 'BSC',
  base: 'Base',
  sol: 'Solana',
  solana: 'Solana',
};

export const WATCHLIST_CHAIN_OPTIONS = [
  { value: 'solana', label: 'Solana' },
  { value: 'base', label: 'Base' },
  { value: 'bsc', label: 'BSC' },
  { value: 'ethereum', label: 'Ethereum' },
  { value: 'binance_perp', label: 'Binance 永续' },
  { value: 'binance_alpha', label: 'Binance Alpha' },
];

export const WATCHLIST_STATUS_OPTIONS: Array<{
  value: 'active' | 'watch' | 'paused';
  label: string;
}> = [
  { value: 'active', label: 'active (高优监控)' },
  { value: 'watch', label: 'watch (常规观察)' },
  { value: 'paused', label: 'paused (暂停)' },
];

export const SIGNAL_TYPE_LABELS: Record<string, string> = {
  vitalik_sell: 'Vitalik 卖出',
  heat: '热度异动',
  heat_plus_oi: '热度放大 + OI 增长',
  heat_plus_negative_funding: '热度放大 + 负费率',
  heat_plus_oi_negative_funding: '热度 + OI + 负费率',
  heat_report: '热度汇总报告',
  oi_anomaly: 'OI 异动',
  momentum: '连续动量',
  narrative_tagged: '命中叙事标签',
  flap_support: 'FLAP 支撑',
  resonance: '跨源共振',
  funding_flip_oi_rising: '费率翻空 + OI 抬升',
  alpha_discovery: '公告预期发现',
  alpha_followup: 'Alpha 跟进',
  alpha_launch: 'Alpha 上线',
  listing: '正式上币',
  airdrop: 'HODLer 空投',
  alpha: 'Binance Alpha',
};

export const PRIORITY_LABELS: Record<string, string> = {
  high: '高 (high)',
  medium: '中 (medium)',
  low: '低 (low)',
};

export const TIME_RANGE_LABELS: Record<string, string> = {
  '1h': '最近 1 小时',
  '6h': '最近 6 小时',
  '24h': '最近 24 小时',
  '7d': '最近 7 天',
};

export type SettingTabKey =
  | 'jobs'
  | 'scanner'
  | 'strategy'
  | 'push'
  | 'network'
  | 'alerts';

export const SETTING_TABS: Array<{ key: SettingTabKey; label: string }> = [
  { key: 'jobs', label: '任务记录' },
  { key: 'scanner', label: '扫描器' },
  { key: 'strategy', label: '策略参数' },
  { key: 'push', label: '推送与 Telegram' },
  { key: 'network', label: '网络与数据源' },
  { key: 'alerts', label: '钱包告警' },
];

export interface SettingMetaItem {
  key: string;
  name: string;
  description: string;
  category: 'scanner' | 'strategy' | 'push' | 'network';
  inputType?: 'boolean' | 'number' | 'text' | 'password';
  envOnly?: boolean;
}

// 后端设置项元信息，标注 envOnly 的参数目前仅由环境变量生效
export const SETTING_META: Record<string, SettingMetaItem> = {
  enable_scanner_s1: {
    key: 'enable_scanner_s1',
    name: '启用 S1 币安公告',
    description: '监听币安公告与 Alpha 事件催化',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_scanner_s2: {
    key: 'enable_scanner_s2',
    name: '启用 S2 费率反转',
    description: '监控永续合约资金费率翻转与 OI 抬升',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_scanner_s3: {
    key: 'enable_scanner_s3',
    name: '启用 S3 热度确认',
    description: '监控成交额放量、负费率与 OI 增长',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_scanner_s5: {
    key: 'enable_scanner_s5',
    name: '启用 S5 链上发现',
    description: '监控链上新币、叙事标签与连续动量',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_scanner_s7: {
    key: 'enable_scanner_s7',
    name: '启用 S7 Vitalik Sell',
    description: '监控 Vitalik 关联地址 ERC-20 转出路径',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_scanner_insider: {
    key: 'enable_scanner_insider',
    name: '启用 Insider 钱包同步',
    description: '定时拉取 Solana 监控钱包持仓与交易',
    category: 'scanner',
    inputType: 'boolean',
  },
  enable_binance_square: {
    key: 'enable_binance_square',
    name: '启用币安广场辅助源',
    description: '是否抓取币安广场热度内容作为辅助信号',
    category: 'scanner',
    inputType: 'boolean',
  },
  scan_interval_s1: {
    key: 'scan_interval_s1',
    name: 'S1 扫描间隔 (秒)',
    description: 'S1 扫描器调度周期',
    category: 'scanner',
    inputType: 'number',
  },
  scan_interval_s2: {
    key: 'scan_interval_s2',
    name: 'S2 扫描间隔 (秒)',
    description: 'S2 扫描器调度周期',
    category: 'scanner',
    inputType: 'number',
  },
  scan_interval_s3: {
    key: 'scan_interval_s3',
    name: 'S3 扫描间隔 (秒)',
    description: 'S3 扫描器调度周期',
    category: 'scanner',
    inputType: 'number',
  },
  scan_interval_s5: {
    key: 'scan_interval_s5',
    name: 'S5 扫描间隔 (秒)',
    description: 'S5 扫描器调度周期',
    category: 'scanner',
    inputType: 'number',
  },
  scan_interval_s7: {
    key: 'scan_interval_s7',
    name: 'S7 扫描间隔 (秒)',
    description: 'S7 扫描器调度周期',
    category: 'scanner',
    inputType: 'number',
  },
  scan_interval_insider: {
    key: 'scan_interval_insider',
    name: 'Insider 同步间隔 (秒)',
    description: 'Solana 钱包监控轮询周期',
    category: 'scanner',
    inputType: 'number',
  },
  signal_time_bucket_minutes: {
    key: 'signal_time_bucket_minutes',
    name: '信号去重时间桶 (分钟)',
    description: '同类信号去重时间窗口',
    category: 'strategy',
    inputType: 'number',
  },
  resonance_lookback_minutes: {
    key: 'resonance_lookback_minutes',
    name: '跨源共振回看窗口 (分钟)',
    description: '检查多源同标的共振的回看窗口',
    category: 'strategy',
    inputType: 'number',
  },
  s5_min_gain_pct: {
    key: 's5_min_gain_pct',
    name: 'S5 最小涨幅 (%)',
    description: '触发 S5 连续动量的最小累计涨幅',
    category: 'strategy',
    inputType: 'number',
  },
  s5_momentum_consecutive_up: {
    key: 's5_momentum_consecutive_up',
    name: 'S5 连续上涨轮数',
    description: 'S5 动量确认所需的连续采样上涨次数',
    category: 'strategy',
    inputType: 'number',
  },
  s5_momentum_medium_quota: {
    key: 's5_momentum_medium_quota',
    name: 'S5 中优先级配额',
    description: '单轮 S5 中优先级动量信号推送额度',
    category: 'strategy',
    inputType: 'number',
  },
  s3_vol_surge_mult: {
    key: 's3_vol_surge_mult',
    name: 'S3 放量倍数',
    description: '判断成交额突增的倍数阈值',
    category: 'strategy',
    inputType: 'number',
    envOnly: true,
  },
  s3_min_vol_usd: {
    key: 's3_min_vol_usd',
    name: 'S3 最小成交额 (USD)',
    description: 'S3 信号触发所需最小 24h 成交额',
    category: 'strategy',
    inputType: 'number',
    envOnly: true,
  },
  s3_min_oi_delta_pct: {
    key: 's3_min_oi_delta_pct',
    name: 'S3 最小 OI 增幅 (%)',
    description: 'S3 信号触发所需 6h 持仓量增幅',
    category: 'strategy',
    inputType: 'number',
    envOnly: true,
  },
  s3_min_oi_usd: {
    key: 's3_min_oi_usd',
    name: 'S3 最小持仓规模 (USD)',
    description: 'S3 信号触发所需最小 OI 规模',
    category: 'strategy',
    inputType: 'number',
    envOnly: true,
  },
  s7_min_notify_usd: {
    key: 's7_min_notify_usd',
    name: 'S7 最小通知金额 (USD)',
    description: 'S7 触发推送的最小转出美元价值',
    category: 'strategy',
    inputType: 'number',
    envOnly: true,
  },
  token_push_cooldown_minutes: {
    key: 'token_push_cooldown_minutes',
    name: '普通代币推送冷却 (分钟)',
    description: '普通标的同级信号再次推送的最小间隔',
    category: 'push',
    inputType: 'number',
  },
  watchlist_cooldown_minutes: {
    key: 'watchlist_cooldown_minutes',
    name: '观察名单冷却 (分钟)',
    description: '观察名单标的推送冷却间隔',
    category: 'push',
    inputType: 'number',
  },
  s3_digest_cooldown_minutes: {
    key: 's3_digest_cooldown_minutes',
    name: 'S3 摘要合并冷却 (分钟)',
    description: 'S3 摘要批次推送最小间隔',
    category: 'push',
    inputType: 'number',
  },
  tg_chat_id: {
    key: 'tg_chat_id',
    name: 'Telegram Chat ID',
    description: '接收信号推送的 Telegram 群组或频道 ID',
    category: 'push',
    inputType: 'text',
  },
  tg_proxy_url: {
    key: 'tg_proxy_url',
    name: 'Telegram 专用代理',
    description: '请求 api.telegram.org 使用的 HTTP 代理',
    category: 'push',
    inputType: 'text',
  },
  tg_bot_token: {
    key: 'tg_bot_token',
    name: 'Telegram Bot Token',
    description: 'Telegram 机器人密钥',
    category: 'network',
    inputType: 'password',
  },
  gmgn_cookie: {
    key: 'gmgn_cookie',
    name: 'GMGN Cookie',
    description: 'GMGN 鉴权 Cookie',
    category: 'network',
    inputType: 'password',
  },
  gmgn_timeout_seconds: {
    key: 'gmgn_timeout_seconds',
    name: 'GMGN 超时 (秒)',
    description: 'GMGN 接口请求超时时间',
    category: 'network',
    inputType: 'number',
  },
  gmgn_retries: {
    key: 'gmgn_retries',
    name: 'GMGN 重试次数',
    description: 'GMGN 接口请求失败后的重试次数',
    category: 'network',
    inputType: 'number',
  },
  s7_eth_rpc_url: {
    key: 's7_eth_rpc_url',
    name: 'S7 ETH RPC',
    description: 'S7 扫描器使用的以太坊 RPC 节点地址',
    category: 'network',
    inputType: 'text',
  },
  selective_proxy_url: {
    key: 'selective_proxy_url',
    name: '选择性代理地址',
    description: '仅对指定域名启用的代理地址',
    category: 'network',
    inputType: 'text',
  },
  selective_proxy_domains: {
    key: 'selective_proxy_domains',
    name: '选择性代理域名',
    description: '逗号分隔的代理目标域名列表',
    category: 'network',
    inputType: 'text',
  },
  http_trust_env: {
    key: 'http_trust_env',
    name: '读取系统代理环境变量',
    description: '是否信任 HTTP_PROXY 等系统环境变量',
    category: 'network',
    inputType: 'boolean',
  },
};

export const PUSH_RULES_DOC = [
  {
    source: '跨源共振 / force_push',
    immediate: '总是立即推送（绕过冷却）',
    digest: '—',
  },
  {
    source: 'S1 币安公告',
    immediate: 'high 且为上币公告；或观察名单 + high',
    digest: '—',
  },
  {
    source: 'S2 费率反转',
    immediate: 'high；或观察名单 + medium',
    digest: '—',
  },
  {
    source: 'S3 热度确认',
    immediate: 'high',
    digest: 'medium 进入定时摘要合并',
  },
  {
    source: 'S5 链上发现',
    immediate: 'momentum high；medium（受配额限制）；观察名单 + high',
    digest: '—',
  },
  {
    source: 'S7 Vitalik Sell',
    immediate: 'high；或观察名单 + 金额达标',
    digest: '—',
  },
];
