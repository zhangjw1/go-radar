export const radarSources = [
  { key: 's1', label: 'S1 币安公告', focus: '公告催化 / 上所预期' },
  { key: 's2', label: 'S2 费率反转', focus: '费率转负 / OI 抬升' },
  { key: 's3', label: 'S3 热度确认', focus: '热度 / 费率 / OI 共振' },
  { key: 's5', label: 'S5 链上发现', focus: '链上新币 / 叙事 / 动量' },
  { key: 's7', label: 'S7 Vitalik Sell', focus: 'DEX / CEX / LP 转出' },
];

const chainLabels: Record<string, string> = {
  binance_perp: 'Binance 永续',
  binance_alpha: 'Binance Alpha',
  eth: 'Ethereum',
  ethereum: 'Ethereum',
  bsc: 'BSC',
  base: 'Base',
  sol: 'Solana',
  solana: 'Solana',
};

const signalTypeLabels: Record<string, string> = {
  vitalik_sell: 'Vitalik 卖出',
  heat: '热度异动',
  heat_plus_oi: '热度放大 + OI 增长',
  heat_plus_negative_funding: '热度放大 + 负费率',
  oi_anomaly: 'OI 异动',
  momentum: '连续动量',
  narrative_tagged: '命中叙事标签',
  flap_support: 'FLAP 支撑',
  resonance: '跨源共振',
  funding_flip_oi_rising: '费率翻空 + OI 抬升',
  alpha_discovery: '公告预期发现',
  listing: '正式上币',
  airdrop: 'HODLer 空投',
  alpha: 'Binance Alpha',
};

export function sourceLabel(source: string) {
  return radarSources.find((item) => item.key === source)?.label || source;
}

export function chainLabel(chain: string) {
  return chainLabels[chain] || chain;
}

export function signalTypeLabel(signalType: string) {
  return signalTypeLabels[signalType] || signalType;
}

export function priorityColor(priority: string) {
  if (priority === 'high') return 'red';
  if (priority === 'medium') return 'orange';
  return 'green';
}

export function priorityLabel(priority: string) {
  if (priority === 'high') return '高';
  if (priority === 'medium') return '中';
  if (priority === 'low') return '低';
  return priority || '-';
}

export function scoreLevel(score: number) {
  if (score >= 100) return 'critical';
  if (score >= 80) return 'hot';
  if (score >= 60) return 'warm';
  return 'normal';
}

export function scoreText(score: number) {
  if (score >= 100) return '立刻关注';
  if (score >= 80) return '高优先';
  if (score >= 60) return '重点观察';
  return '常规跟踪';
}

export function formatScore(score: number) {
  if (typeof score !== 'number') return '-';
  return Number.isInteger(score) ? `${score}` : score.toFixed(2);
}

export function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return `${date.getMonth() + 1}-${date.getDate()} ${String(
    date.getHours()
  ).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

export function summarizeReason(signalType: string, reason: string) {
  const raw = (reason || '').trim();
  if (!raw) return '-';

  const fundingMatch = raw.match(
    /Funding flipped from ([^ ]+) to ([^,]+), OI \+([0-9.]+)%/i
  );
  if (signalType === 'funding_flip_oi_rising' && fundingMatch) {
    return `费率由 ${fundingMatch[1]} 翻到 ${fundingMatch[2]}，OI 抬升 ${fundingMatch[3]}%`;
  }

  const momentumMatch = raw.match(
    /Market cap rose for (\d+) consecutive rounds, \+([0-9.]+)%/i
  );
  if (signalType === 'momentum' && momentumMatch) {
    return `连续 ${momentumMatch[1]} 轮市值上行，累计涨幅 ${momentumMatch[2]}%`;
  }

  return raw
    .replace('Funding flipped from', '费率由')
    .replace('to', '转为')
    .replace('Market cap rose for', '市值连续')
    .replace('consecutive rounds', '轮上行')
    .replace('OI', '持仓量')
    .replace('flipped', '翻转');
}
