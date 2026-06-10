// format.ts — number / token / date formatters shared across the app. Every
// figure renders mono + tabular; these helpers keep the instrument readout
// consistent (compact token counts, precise costs).

import type { Tokens } from '../api';

export function totalTokens(t: Tokens): number {
  return t.inputTokens + t.outputTokens + t.cacheCreationTokens + t.cacheReadTokens;
}

const nf = new Intl.NumberFormat('en-US');

export function fmtInt(n: number): string {
  return nf.format(Math.round(n));
}

/** Compact token count: 12.4M, 938K, 412. */
export function fmtTokens(n: number): string {
  const abs = Math.abs(n);
  if (abs >= 1_000_000_000) return (n / 1_000_000_000).toFixed(2) + 'B';
  if (abs >= 1_000_000) return (n / 1_000_000).toFixed(abs >= 10_000_000 ? 1 : 2) + 'M';
  if (abs >= 1_000) return (n / 1_000).toFixed(abs >= 100_000 ? 0 : 1) + 'K';
  return String(Math.round(n));
}

/** Cost: $0.0042 for tiny, $42.18 normal, $1,204 big. */
export function fmtCost(n: number): string {
  if (n === 0) return '$0.00';
  const abs = Math.abs(n);
  if (abs >= 1000) return '$' + nf.format(Math.round(n));
  if (abs >= 1) return '$' + n.toFixed(2);
  if (abs >= 0.01) return '$' + n.toFixed(3);
  return '$' + n.toFixed(4);
}

export function fmtPct(n: number): string {
  return (n >= 0 ? '+' : '') + n.toFixed(0) + '%';
}

/** ISO date -> "Jun 9". */
export function fmtDayShort(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

/** ISO date -> "Jun 9, 14:32". */
export function fmtDateTime(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return (
    d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ', ' +
    d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
  );
}

/** A duration in ms -> "1h 24m" / "42m" / "18s". */
export function fmtDuration(ms: number): string {
  if (ms <= 0) return '—';
  const s = Math.round(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return `${s}s`;
}

export function durationMs(start?: string, end?: string): number {
  if (!start || !end) return 0;
  const a = new Date(start).getTime();
  const b = new Date(end).getTime();
  if (Number.isNaN(a) || Number.isNaN(b)) return 0;
  return Math.max(0, b - a);
}
