import type { SignalEvent } from '@/api/radar';
import {
  CHAIN_LABELS,
  PRIORITY_LABELS,
  RADAR_SOURCES,
  SIGNAL_TYPE_LABELS,
} from '@/constants/radar';

export interface SafetySummary {
  honeypot: boolean;
  mintable: boolean;
  safe: boolean;
  buyTax: number;
  sellTax: number;
}

export function shortAddress(address?: string): string {
  if (!address) return '-';
  if (address.length <= 14) return address;
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

export function chainLabel(chain?: string): string {
  if (!chain) return '-';
  return CHAIN_LABELS[chain] || chain;
}

export function sourceLabel(source?: string): string {
  if (!source) return '-';
  return RADAR_SOURCES.find((item) => item.key === source)?.label || source;
}

export function priorityLabel(priority?: string): string {
  if (!priority) return '-';
  return PRIORITY_LABELS[priority] || priority;
}

export function formatTime(value?: string | null): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const mm = String(date.getMonth() + 1).padStart(2, '0');
  const dd = String(date.getDate()).padStart(2, '0');
  const hh = String(date.getHours()).padStart(2, '0');
  const min = String(date.getMinutes()).padStart(2, '0');
  return `${mm}-${dd} ${hh}:${min}`;
}

export function formatRelTime(value?: string | null): string {
  if (!value) return '-';
  const ms = new Date(value).getTime();
  if (Number.isNaN(ms)) return value;
  const diffSec = Math.max(0, Math.floor((Date.now() - ms) / 1000));
  if (diffSec < 60) return `${diffSec}s前`;
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h前`;
  return `${Math.floor(diffSec / 86400)}d前`;
}

export function formatPrice(value?: number | null): string {
  if (value === null || value === undefined) return '-';
  const num = Number(value);
  if (Number.isNaN(num)) return '-';
  if (num === 0) return '$0';
  return `$${num.toLocaleString('en-US', {
    maximumFractionDigits: num >= 1 ? 4 : 8,
  })}`;
}

export function formatCompactUnit(value?: number | null): string {
  if (value === null || value === undefined) return '-';
  const num = Number(value);
  if (Number.isNaN(num)) return '-';
  const abs = Math.abs(num);
  if (abs >= 100000000) return `$${(num / 100000000).toFixed(2)}亿`;
  if (abs >= 10000) return `$${(num / 10000).toFixed(2)}万`;
  return `$${num.toLocaleString('en-US', { maximumFractionDigits: 2 })}`;
}

export function formatUsd(value?: number | null, withSign = false): string {
  if (value === null || value === undefined) return '$0.00';
  const num = Number(value);
  if (Number.isNaN(num)) return '$0.00';
  let sign = '';
  if (num < 0) sign = '-';
  else if (withSign && num > 0) sign = '+';
  return `${sign}$${Math.abs(num).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

export function parseJsonObject(raw?: string | null): Record<string, unknown> {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

export function extractSafetyFromSignals(
  list: SignalEvent[]
): SafetySummary | null {
  const found = list.find((item) => {
    const raw = parseJsonObject(item.raw_json);
    return raw.safety && typeof raw.safety === 'object';
  });
  if (!found) return null;
  const safety = parseJsonObject(found.raw_json).safety as Record<
    string,
    unknown
  >;
  return {
    honeypot: Boolean(safety.honeypot),
    mintable: Boolean(safety.mintable),
    safe: safety.safe !== false && !safety.honeypot,
    buyTax: Number(safety.buy_tax || 0),
    sellTax: Number(safety.sell_tax || 0),
  };
}

export function signalTypeLabel(
  signalType: string,
  reason = '',
  rawJson = ''
): string {
  if (signalType === 'funding_flip_oi_rising') {
    const raw = parseJsonObject(rawJson);
    const dir = String(raw.position_direction || '');
    if (dir === '多转空' || dir === '空转多') return `费率${dir} + OI 抬升`;
    if (reason.includes('多转空') || reason.includes('正转负')) {
      return '费率多转空 + OI 抬升';
    }
    if (reason.includes('空转多') || reason.includes('负转正')) {
      return '费率空转多 + OI 抬升';
    }
    return '费率翻转 + OI 抬升';
  }
  return SIGNAL_TYPE_LABELS[signalType] || signalType;
}

export function summarizeReason(signalType: string, reason?: string): string {
  const raw = (reason || '').trim();
  if (!raw) return '-';
  const fundingMatch = raw.match(
    /Funding flipped from ([^ ]+) to ([^,]+), OI \+([0-9.]+)%/i
  );
  if (signalType === 'funding_flip_oi_rising' && fundingMatch) {
    return `费率由 ${fundingMatch[1]} 翻至 ${fundingMatch[2]}，OI 抬升 ${fundingMatch[3]}%`;
  }
  const momentumMatch = raw.match(
    /Market cap rose for (\d+) consecutive rounds, \+([0-9.]+)%/i
  );
  if (signalType === 'momentum' && momentumMatch) {
    return `连续 ${momentumMatch[1]} 轮市值上行，累计涨幅 ${momentumMatch[2]}%`;
  }
  return raw;
}

export function isValidSolanaBase58(address: string): boolean {
  return /^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(address.trim());
}

export function exportSignalsToCsv(
  items: SignalEvent[],
  filename = 'go-radar-signals.csv'
): void {
  const header = [
    '时间',
    '来源',
    '链/市场',
    '标的',
    '地址',
    '信号类型',
    '优先级',
    '评分',
    '说明',
    '推送时间',
  ];
  const rows = items.map((item) => [
    formatTime(item.created_at),
    sourceLabel(item.source),
    chainLabel(item.chain),
    item.symbol,
    item.address,
    signalTypeLabel(item.signal_type, item.reason, item.raw_json),
    priorityLabel(item.priority),
    item.score,
    summarizeReason(item.signal_type, item.reason),
    item.pushed_at ? formatTime(item.pushed_at) : '未推送',
  ]);
  const csv = [header, ...rows]
    .map((row) =>
      row.map((cell) => `"${String(cell ?? '').replace(/"/g, '""')}"`).join(',')
    )
    .join('\n');
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
