// api.ts — the binding contract to the Go core, with a self-contained MOCK
// fallback so the whole UI renders in a plain browser (no Wails runtime)
// against believable, fabricated sample data.
//
// The JSON shapes here mirror pkg/core exactly: Tokens, Row (embeds Tokens),
// Result (data/totals/trend/sparklines/warnings). Warnings serialize to plain
// strings via core.Warning.MarshalJSON, hence `warnings?: string[]`.

import { Call, Events } from '@wailsio/runtime';
import { buildMockResult, MOCK_SOURCES } from './mock';

export type Tokens = {
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  reasoningTokens?: number;
  costUSD: number;
  credits?: number;
};

export type ModelBreakdown = Tokens & { model: string };

export type Row = Tokens & {
  key: string;
  source?: string;
  sessionId?: string;
  project?: string;
  start?: string;
  lastActivity?: string;
  modelsUsed: string[];
  modelBreakdowns?: ModelBreakdown[];
};

export type Trend = {
  metric: string;
  start?: string;
  end?: string;
  series: number[];
  min: number;
  max: number;
  total: number;
};

export type Query = {
  Sources?: string[];
  SourcePaths?: Record<string, string[]>;
  Since?: string;
  Until?: string;
  View?: string;
  Sparkline?: string;
  Compare?: string;
  By?: string;
  Project?: string;
  ID?: string;
  Order?: string;
  Top?: number;
  StartOfWeek?: string;
  Mode?: string;
  Speed?: string;
  Instances?: boolean;
  Active?: boolean;
  Recent?: boolean;
  Tolerant?: boolean;
};

export type Result = {
  view: string;
  data: Row[];
  totals: Tokens;
  trend?: Trend;
  sparklines?: Record<string, number[]>;
  warnings?: string[];
};

export type DiffResult = {
  current: Result;
  baseline: Result;
  deltas: Tokens;
};

declare global {
  interface Window {
    _wails?: {
      environment?: {
        OS?: string;
        Arch?: string;
        Debug?: boolean;
      };
    };
  }
}

/** True when running inside the real Wails runtime. */
export function isLive(): boolean {
  return Boolean(window._wails?.environment);
}

/**
 * Resolve once the Wails v3 runtime has injected `window._wails.environment`,
 * or false at the timeout. v3 injects the environment from a post-page-load
 * hook (WindowLoadFinished / navigationCompleted), which fires *after* module
 * eval and the first `onMount` — so `isLive()` reads false at construction even
 * in the real app. Callers must await this before sampling `isLive()` for the
 * first time, or the desktop app falls back to mock data and never subscribes
 * to live updates. A plain browser never injects the runtime, so we resolve
 * false at the deadline and the mock path takes over (design-review build).
 */
export function waitForRuntime(timeoutMs = 3000): Promise<boolean> {
  if (isLive()) return Promise.resolve(true);
  return new Promise((resolve) => {
    const start = performance.now();
    const id = setInterval(() => {
      if (isLive()) {
        clearInterval(id);
        resolve(true);
      } else if (performance.now() - start > timeoutMs) {
        clearInterval(id);
        resolve(false);
      }
    }, 30);
  });
}

export async function listSources(): Promise<string[]> {
  if (isLive()) return Call.ByName('main.UsageService.ListSources') as Promise<string[]>;
  // Mock mode — surface the five canonical agents.
  return Promise.resolve([...MOCK_SOURCES]);
}

export async function getUsage(query: Query): Promise<Result> {
  if (isLive()) return Call.ByName('main.UsageService.GetUsage', query) as Promise<Result>;
  // Mock mode — fabricate a rich, query-shaped Result. A tiny latency makes
  // the staged-loading skeletons visible during browser review.
  return new Promise((resolve) => {
    setTimeout(() => resolve(buildMockResult(query)), 220);
  });
}

export async function compareUsage(query: Query, baseline: Query): Promise<DiffResult> {
  if (isLive()) return Call.ByName('main.UsageService.CompareUsage', query, baseline) as Promise<DiffResult>;
  const [current, previous] = await Promise.all([getUsage(query), getUsage(baseline)]);
  return {
    current,
    baseline: previous,
    deltas: {
      inputTokens: current.totals.inputTokens - previous.totals.inputTokens,
      outputTokens: current.totals.outputTokens - previous.totals.outputTokens,
      cacheCreationTokens: current.totals.cacheCreationTokens - previous.totals.cacheCreationTokens,
      cacheReadTokens: current.totals.cacheReadTokens - previous.totals.cacheReadTokens,
      reasoningTokens: (current.totals.reasoningTokens ?? 0) - (previous.totals.reasoningTokens ?? 0),
      costUSD: current.totals.costUSD - previous.totals.costUSD,
      credits: (current.totals.credits ?? 0) - (previous.totals.credits ?? 0),
    },
  };
}

export function onUsageChanged(cb: () => void): void {
  if (!isLive()) return;
  Events.On('usage:changed', cb);
}
