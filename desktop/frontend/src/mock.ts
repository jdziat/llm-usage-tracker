// mock.ts — fabricated, deterministic sample data for browser review.
//
// Renders the FULL UI with no Wails runtime: ~5 sources, ~8 models, a 30-point
// trend series with a visible spike, realistic token/cost figures, a couple of
// warnings, and at least one unknown/$0 model. Never real usage — pure fixture.
//
// The shape of the returned Result depends on the query's View (and By), so the
// app's screens (summary/daily/trend/session) each get appropriately-keyed data,
// exactly as core.LoadAndAggregate would produce.

import type { ModelBreakdown, Query, Result, Row, Tokens, Trend } from './api';

export const MOCK_SOURCES = ['claude', 'codex', 'opencode', 'amp', 'pi'] as const;

// Deterministic PRNG so every reload paints identical data (stable screenshots).
function rng(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 0xffffffff;
  };
}

const zeroTokens = (): Tokens => ({
  inputTokens: 0,
  outputTokens: 0,
  cacheCreationTokens: 0,
  cacheReadTokens: 0,
  reasoningTokens: 0,
  costUSD: 0,
  credits: 0,
});

function addInto(a: Tokens, b: Tokens): void {
  a.inputTokens += b.inputTokens;
  a.outputTokens += b.outputTokens;
  a.cacheCreationTokens += b.cacheCreationTokens;
  a.cacheReadTokens += b.cacheReadTokens;
  a.reasoningTokens = (a.reasoningTokens ?? 0) + (b.reasoningTokens ?? 0);
  a.costUSD += b.costUSD;
  a.credits = (a.credits ?? 0) + (b.credits ?? 0);
}

// Per-1M-token prices mirroring pkg/core/pricing.go. `null` => unknown model.
type Price = { input: number; output: number; cacheWrite: number; cacheRead: number } | null;

type ModelSpec = {
  model: string;
  source: string;
  price: Price;
  weight: number; // relative share of total spend
  reasoning: boolean;
};

// ~8 models across the five agents, incl. one unknown/$0 model (claude-fable-5
// is priced, so we add an explicitly unpriced experimental model too).
const MODELS: ModelSpec[] = [
  { model: 'claude-opus-4-8', source: 'claude', price: { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5 }, weight: 30, reasoning: true },
  { model: 'claude-sonnet-4-6', source: 'claude', price: { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 }, weight: 16, reasoning: false },
  { model: 'claude-haiku-4-5', source: 'claude', price: { input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.1 }, weight: 5, reasoning: false },
  { model: 'gpt-5.5', source: 'codex', price: { input: 1.25, output: 10, cacheWrite: 1.25, cacheRead: 0.125 }, weight: 14, reasoning: true },
  { model: 'gpt-5.3-codex', source: 'codex', price: { input: 1.25, output: 10, cacheWrite: 1.25, cacheRead: 0.125 }, weight: 12, reasoning: true },
  { model: 'gemini-2.5-pro', source: 'opencode', price: { input: 1.25, output: 10, cacheWrite: 1.25, cacheRead: 0.31 }, weight: 10, reasoning: false },
  { model: 'claude-fable-5', source: 'amp', price: { input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1 }, weight: 8, reasoning: false },
  { model: 'glm-4.6-experimental', source: 'pi', price: null, weight: 6, reasoning: false }, // unknown / $0
];

const PROJECTS = ['llm-usage-tracker', 'web-platform', 'infra-pipeline', 'mobile-app', 'data-warehouse', 'docs-site'];

// Build a deterministic per-day, per-model intensity field across N days with a
// visible spike on a chosen day. Returns daily Tokens per model.
const DAYS = 30;
const SPIKE_DAY = 21; // index into the 0..29 window — a clear amber spike

function dayDate(i: number): Date {
  const end = new Date(2026, 5, 9); // 2026-06-09 (the "today" in context)
  const d = new Date(end);
  d.setDate(end.getDate() - (DAYS - 1 - i));
  d.setHours(0, 0, 0, 0);
  return d;
}

function costFor(price: Price, t: Tokens): number {
  if (!price) return 0;
  return (
    (t.inputTokens * price.input +
      t.outputTokens * price.output +
      t.cacheCreationTokens * price.cacheWrite +
      t.cacheReadTokens * price.cacheRead) /
    1_000_000
  );
}

type Cell = { model: ModelSpec; tokens: Tokens };

// The canonical fabricated dataset: a [day][model] grid of token buckets.
function buildGrid(): Cell[][] {
  const rand = rng(0xc0ffee);
  const grid: Cell[][] = [];
  for (let d = 0; d < DAYS; d++) {
    const row: Cell[] = [];
    // weekend dip + the spike day surge shape the daily envelope.
    const dow = dayDate(d).getDay();
    const weekend = dow === 0 || dow === 6 ? 0.4 : 1;
    const ramp = 0.6 + (d / DAYS) * 0.7; // a gentle upward trend
    const spike = d === SPIKE_DAY ? 3.1 : d === SPIKE_DAY - 1 || d === SPIKE_DAY + 1 ? 1.4 : 1;
    const envelope = weekend * ramp * spike;
    for (const m of MODELS) {
      const base = m.weight * envelope * (0.7 + rand() * 0.6);
      const t = zeroTokens();
      t.inputTokens = Math.round(base * 1800 * (0.8 + rand() * 0.5));
      t.outputTokens = Math.round(base * 520 * (0.8 + rand() * 0.5));
      t.cacheCreationTokens = Math.round(base * 900 * (0.6 + rand() * 0.8));
      // cache-read dominates token counts but is cheap — the educational point.
      t.cacheReadTokens = Math.round(base * 14000 * (0.7 + rand() * 0.7));
      t.reasoningTokens = m.reasoning ? Math.round(base * 380 * (0.5 + rand() * 0.9)) : 0;
      t.costUSD = costFor(m.price, t);
      t.credits = m.source === 'amp' ? Math.round(base * 4) : 0;
      row.push({ model: m, tokens: t });
    }
    grid.push(row);
  }
  return grid;
}

const GRID = buildGrid();

// Apply the query's source filter (Sources omitted => all).
function activeSources(q: Query): Set<string> {
  const list = q.Sources && q.Sources.length ? q.Sources : [...MOCK_SOURCES];
  return new Set(list);
}

function inRange(d: Date, q: Query): boolean {
  if (q.Since) {
    const s = new Date(q.Since);
    if (!Number.isNaN(s.getTime()) && d < new Date(s.getFullYear(), s.getMonth(), s.getDate())) return false;
  }
  if (q.Until) {
    const u = new Date(q.Until);
    if (!Number.isNaN(u.getTime()) && d > u) return false;
  }
  return true;
}

function modelsUsed(breakdowns: ModelBreakdown[]): string[] {
  return [...breakdowns]
    .sort((a, b) => b.costUSD - a.costUSD || b.outputTokens - a.outputTokens)
    .map((m) => m.model);
}

function tokensToBreakdown(model: string, t: Tokens): ModelBreakdown {
  return { model, ...t };
}

// ── Summary view (By: source | model | project) ────────────────────────────
function summaryRows(q: Query): Row[] {
  const srcs = activeSources(q);
  const by = q.By ?? 'model';
  const buckets = new Map<string, { row: Row; bd: Map<string, ModelBreakdown> }>();

  for (let d = 0; d < DAYS; d++) {
    const date = dayDate(d);
    if (!inRange(date, q)) continue;
    for (const cell of GRID[d]) {
      if (!srcs.has(cell.model.source)) continue;
      // project assignment is deterministic per (model, day) to spread spend.
      const proj = PROJECTS[(d + cell.model.model.length) % PROJECTS.length];
      let key: string;
      if (by === 'source') key = cell.model.source;
      else if (by === 'project') key = proj;
      else key = cell.model.model;

      let bucket = buckets.get(key);
      if (!bucket) {
        const row: Row = {
          key,
          ...zeroTokens(),
          source: by === 'source' ? key : cell.model.source,
          modelsUsed: [],
        };
        bucket = { row, bd: new Map() };
        buckets.set(key, bucket);
      }
      addInto(bucket.row, cell.tokens);
      const existing = bucket.bd.get(cell.model.model);
      if (existing) addInto(existing, cell.tokens);
      else bucket.bd.set(cell.model.model, tokensToBreakdown(cell.model.model, { ...cell.tokens }));
    }
  }

  const rows = [...buckets.values()].map(({ row, bd }) => {
    row.modelBreakdowns = [...bd.values()].sort((a, b) => b.costUSD - a.costUSD);
    row.modelsUsed = modelsUsed(row.modelBreakdowns);
    return row;
  });
  rows.sort((a, b) => b.costUSD - a.costUSD || b.outputTokens - a.outputTokens);
  if (q.Top && q.Top > 0) return rows.slice(0, q.Top);
  return rows;
}

// ── Time-bucketed view (daily) ──────────────────────────────────────────────
function dailyRows(q: Query): Row[] {
  const srcs = activeSources(q);
  const rows: Row[] = [];
  for (let d = 0; d < DAYS; d++) {
    const date = dayDate(d);
    if (!inRange(date, q)) continue;
    const bd = new Map<string, ModelBreakdown>();
    const agg: Row = {
      key: date.toISOString().slice(0, 10),
      ...zeroTokens(),
      start: date.toISOString(),
      lastActivity: new Date(date.getTime() + 18 * 3600_000).toISOString(),
      modelsUsed: [],
    };
    let any = false;
    for (const cell of GRID[d]) {
      if (!srcs.has(cell.model.source)) continue;
      any = true;
      addInto(agg, cell.tokens);
      const ex = bd.get(cell.model.model);
      if (ex) addInto(ex, cell.tokens);
      else bd.set(cell.model.model, tokensToBreakdown(cell.model.model, { ...cell.tokens }));
    }
    if (!any) continue;
    agg.modelBreakdowns = [...bd.values()].sort((a, b) => b.costUSD - a.costUSD);
    agg.modelsUsed = modelsUsed(agg.modelBreakdowns);
    rows.push(agg);
  }
  rows.reverse(); // most-recent first, like sortedRows desc
  return rows;
}

// ── Session view (drill-down rows) ──────────────────────────────────────────
function sessionRows(q: Query): Row[] {
  const srcs = activeSources(q);
  const rand = rng(0x5e5510);
  const rows: Row[] = [];
  let id = 0;
  for (let d = DAYS - 1; d >= 0 && rows.length < 60; d--) {
    const date = dayDate(d);
    if (!inRange(date, q)) continue;
    for (const cell of GRID[d]) {
      if (!srcs.has(cell.model.source)) continue;
      if (rand() > 0.45) continue; // not every model spawns a session each day
      const proj = PROJECTS[(d + id) % PROJECTS.length];
      const startMs = date.getTime() + Math.floor(rand() * 16) * 3600_000;
      const durMs = (20 + Math.floor(rand() * 180)) * 60_000;
      const t = { ...cell.tokens };
      // scale a single session to a slice of the day's volume
      const f = 0.15 + rand() * 0.4;
      const scaled: Tokens = {
        inputTokens: Math.round(t.inputTokens * f),
        outputTokens: Math.round(t.outputTokens * f),
        cacheCreationTokens: Math.round(t.cacheCreationTokens * f),
        cacheReadTokens: Math.round(t.cacheReadTokens * f),
        reasoningTokens: Math.round((t.reasoningTokens ?? 0) * f),
        costUSD: t.costUSD * f,
        credits: Math.round((t.credits ?? 0) * f),
      };
      const sid = `${cell.model.source.slice(0, 2)}-${(0x9000 + id * 37).toString(16)}`;
      rows.push({
        key: proj,
        ...scaled,
        source: cell.model.source,
        sessionId: sid,
        project: `~/code/${proj}`,
        start: new Date(startMs).toISOString(),
        lastActivity: new Date(startMs + durMs).toISOString(),
        modelsUsed: [cell.model.model],
        modelBreakdowns: [tokensToBreakdown(cell.model.model, scaled)],
      });
      id++;
    }
  }
  rows.sort((a, b) => (b.lastActivity ?? '').localeCompare(a.lastActivity ?? ''));
  return rows;
}

// ── Trend series (30-pt, cost or tokens, with the spike preserved) ──────────
function buildTrendSeries(q: Query): Trend {
  const srcs = activeSources(q);
  const metric = q.Sparkline === 'tokens' ? 'tokens' : 'cost';
  const series: number[] = [];
  let start: Date | undefined;
  let end: Date | undefined;
  for (let d = 0; d < DAYS; d++) {
    const date = dayDate(d);
    if (!inRange(date, q)) continue;
    if (!start) start = date;
    end = date;
    let v = 0;
    for (const cell of GRID[d]) {
      if (!srcs.has(cell.model.source)) continue;
      v += metric === 'tokens'
        ? cell.tokens.inputTokens + cell.tokens.outputTokens + cell.tokens.cacheCreationTokens + cell.tokens.cacheReadTokens
        : cell.tokens.costUSD;
    }
    series.push(metric === 'tokens' ? Math.round(v) : Number(v.toFixed(4)));
  }
  const min = series.length ? Math.min(...series) : 0;
  const max = series.length ? Math.max(...series) : 0;
  const total = series.reduce((a, b) => a + b, 0);
  return {
    metric,
    start: start?.toISOString(),
    end: end?.toISOString(),
    series,
    min,
    max,
    total: metric === 'tokens' ? Math.round(total) : Number(total.toFixed(2)),
  };
}

function totalsOf(rows: Row[]): Tokens {
  const t = zeroTokens();
  for (const r of rows) addInto(t, r);
  return t;
}

function mockWarnings(q: Query): string[] {
  const srcs = activeSources(q);
  const w: string[] = [];
  // a couple of believable partial-failure warnings (only when source is lit)
  if (srcs.has('codex')) w.push('codex: couldn’t read 3 of 412 files (truncated JSONL) — showing the rest');
  if (srcs.has('pi')) w.push('pi: glm-4.6-experimental not in offline price table — tokens counted, cost unpriced');
  return w;
}

export function buildMockResult(q: Query): Result {
  const view = q.View ?? 'summary';
  let data: Row[];
  let trend: Trend | undefined;

  switch (view) {
    case 'session':
      data = sessionRows(q);
      break;
    case 'trend':
      data = dailyRows(q); // trend view also returns daily rows in core
      trend = buildTrendSeries(q);
      break;
    case 'daily':
    case 'weekly':
    case 'monthly':
      data = dailyRows(q);
      break;
    case 'summary':
    default:
      data = summaryRows(q);
      break;
  }

  return {
    view,
    data,
    totals: totalsOf(data.length ? (view === 'session' ? data : data) : []),
    trend,
    warnings: mockWarnings(q),
  };
}

/** A standalone daily series for the Overview hero chart, source-filtered. */
export function mockDaily(q: Query): Row[] {
  return dailyRows(q);
}

/** A standalone trend for the Trends screen, independent of the active view. */
export function mockTrend(q: Query, metric: 'cost' | 'tokens'): Trend {
  return buildTrendSeries({ ...q, Sparkline: metric === 'tokens' ? 'tokens' : 'cost' });
}

/** Prior equal-length period for compare mode. The fixture only covers the
 * current 30-day window, so synthesize a stable, slightly lower ghost series. */
export function mockPreviousTrend(q: Query, metric: 'cost' | 'tokens'): Trend {
  const current = mockTrend(q, metric);
  const series = current.series.map((v, i) => {
    const wave = 0.78 + ((i % 5) - 2) * 0.025;
    const eased = i < current.series.length * 0.7 ? wave : wave + 0.08;
    return metric === 'tokens' ? Math.round(v * eased) : Number((v * eased).toFixed(4));
  });
  const start = current.start ? new Date(current.start) : undefined;
  if (start) start.setDate(start.getDate() - series.length);
  const end = current.end ? new Date(current.end) : undefined;
  if (end) end.setDate(end.getDate() - series.length);
  const total = series.reduce((a, b) => a + b, 0);
  return {
    metric,
    start: start?.toISOString(),
    end: end?.toISOString(),
    series,
    min: series.length ? Math.min(...series) : 0,
    max: series.length ? Math.max(...series) : 0,
    total: metric === 'tokens' ? Math.round(total) : Number(total.toFixed(2)),
  };
}
