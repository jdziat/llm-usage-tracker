// api.ts — the binding contract to the Go core (window.go.main.UsageService),
// with a self-contained MOCK fallback so the whole UI renders in a plain
// browser (no Wails runtime) against believable, fabricated sample data.
//
// The JSON shapes here mirror pkg/core exactly: Tokens, Row (embeds Tokens),
// Result (data/totals/trend/sparklines/warnings). Warnings serialize to plain
// strings via core.Warning.MarshalJSON, hence `warnings?: string[]`.

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

type UsageService = {
  GetUsage(query: Query): Promise<Result>;
  ListSources(): Promise<string[]>;
};

declare global {
  interface Window {
    go?: {
      main?: {
        UsageService?: UsageService;
      };
    };
    runtime?: {
      EventsOn?(event: string, cb: () => void): void;
      EventsOff?(event: string): void;
    };
  }
}

/** True when running inside the real Wails runtime (binding present). */
export function isLive(): boolean {
  return Boolean(window.go?.main?.UsageService);
}

function service(): UsageService | null {
  return window.go?.main?.UsageService ?? null;
}

export async function listSources(): Promise<string[]> {
  const svc = service();
  if (svc) return svc.ListSources();
  // Mock mode — surface the five canonical agents.
  return Promise.resolve([...MOCK_SOURCES]);
}

export async function getUsage(query: Query): Promise<Result> {
  const svc = service();
  if (svc) return svc.GetUsage(query);
  // Mock mode — fabricate a rich, query-shaped Result. A tiny latency makes
  // the staged-loading skeletons visible during browser review.
  return new Promise((resolve) => {
    setTimeout(() => resolve(buildMockResult(query)), 220);
  });
}

export function onUsageChanged(cb: () => void): void {
  if (!isLive()) return;
  window.runtime?.EventsOn?.('usage:changed', cb);
}
