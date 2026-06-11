// store.svelte.ts — central reactive app state (Svelte 5 runes in a class).
// Holds the global filter (which maps onto core.Query), the active screen,
// theme/density chrome, and the loaded Result + freshness/loading flags. One
// store, shared by reference, so every screen reacts to the same filter — the
// "one dataset, many lenses" model from the design doc.

import { getUsage, isLive, listSources, onUsageChanged, waitForRuntime, type Query, type Result } from '../api';
import { mockPreviousTrend, mockTrend } from '../mock';
import type { Trend } from '../api';
import { SOURCE_ORDER, type SourceId } from './palette';

export type ScreenId = 'overview' | 'models' | 'trends' | 'table';
export type RangeId = 'today' | '7d' | '30d' | 'month';
export type Theme = 'dark' | 'light';
export type Density = 'comfortable' | 'compact';

export const RANGE_LABELS: Record<RangeId, string> = {
  today: 'Today',
  '7d': '7d',
  '30d': '30d',
  month: 'This month',
};

function isoDay(d: Date): string {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).toISOString();
}

// "Today" is pinned to the mock dataset's end date so the browser-review build
// always lands on populated data. In live mode the Go core uses real wall-clock.
const TODAY = new Date(2026, 5, 9);

function rangeSince(range: RangeId): string | undefined {
  const d = new Date(TODAY);
  switch (range) {
    case 'today':
      return isoDay(d);
    case '7d':
      d.setDate(d.getDate() - 6);
      return isoDay(d);
    case '30d':
      d.setDate(d.getDate() - 29);
      return isoDay(d);
    case 'month':
      return isoDay(new Date(d.getFullYear(), d.getMonth(), 1));
  }
}

class AppStore {
  // chrome
  theme = $state<Theme>('dark');
  density = $state<Density>('comfortable');
  screen = $state<ScreenId>('overview');
  railCollapsed = $state(false);

  // filter (maps onto Query)
  allSources = $state<SourceId[]>([...SOURCE_ORDER]);
  selectedSources = $state<Set<string>>(new Set(SOURCE_ORDER));
  range = $state<RangeId>('30d');

  // data
  loading = $state(true);
  result = $state<Result | null>(null);
  trend = $state<Trend | null>(null);
  sourceTrends = $state<Record<string, Trend>>({});
  previousSourceTrends = $state<Record<string, Trend>>({});
  trendMetric = $state<'cost' | 'tokens'>('cost');
  comparePrevious = $state(false);
  error = $state('');
  lastScan = $state<Date | null>(null);
  scanMs = $state(0);
  live = $state(false);
  private usageChangedSubscribed = false;
  // Resolved in init() *after* the runtime is detected — reading it at
  // construction would race the v3 environment injection (see init()).
  private devState: string | null = null;

  get sourcesArray(): string[] {
    return [...this.selectedSources];
  }

  baseQuery(extra: Partial<Query> = {}): Query {
    return {
      Sources: this.sourcesArray,
      Since: rangeSince(this.range),
      Until: isoDay(TODAY),
      Order: 'desc',
      Mode: 'auto',
      ...extra,
    };
  }

  toggleSource(id: string): void {
    const next = new Set(this.selectedSources);
    if (next.has(id)) {
      if (next.size === 1) return; // keep at least one lit
      next.delete(id);
    } else {
      next.add(id);
    }
    this.selectedSources = next;
    void this.reload();
  }

  setRange(range: RangeId): void {
    if (this.range === range) return;
    this.range = range;
    void this.reload();
  }

  setScreen(s: ScreenId): void {
    this.screen = s;
  }

  setTrendMetric(m: 'cost' | 'tokens'): void {
    if (this.trendMetric === m) return;
    this.trendMetric = m;
    void this.reload();
  }

  setComparePrevious(enabled: boolean): void {
    if (this.comparePrevious === enabled) return;
    this.comparePrevious = enabled;
    void this.reload();
  }

  toggleTheme(): void {
    this.theme = this.theme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', this.theme);
  }

  toggleDensity(): void {
    this.density = this.density === 'comfortable' ? 'compact' : 'comfortable';
    document.documentElement.setAttribute('data-density', this.density);
  }

  /** Load the primary screen result + the trend series in one tick. */
  async reload(): Promise<void> {
    this.loading = true;
    this.error = '';
    const t0 = performance.now();
    try {
      // The summary-by-source result is the spine the shell + table read from.
      const res = await getUsage(this.baseQuery({ View: 'summary', By: 'source' }));
      this.result = res;
      const [trend, sourceTrends, previousSourceTrends] = await Promise.all([
        this.loadTrend(),
        this.loadSourceTrends(),
        this.comparePrevious ? this.loadPreviousSourceTrends() : Promise.resolve({}),
      ]);
      this.trend = trend;
      this.sourceTrends = sourceTrends;
      this.previousSourceTrends = previousSourceTrends;
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      this.result = null;
    } finally {
      this.scanMs = Math.round(performance.now() - t0);
      this.lastScan = new Date();
      this.loading = false;
    }
  }

  /** Trend series for the active metric (cost/tokens), source-filtered. */
  private async loadTrend(): Promise<Trend | null> {
    if (isLive()) {
      const r = await getUsage(
        this.baseQuery({ View: 'trend', Sparkline: this.trendMetric === 'tokens' ? 'tokens' : 'cost' }),
      );
      return r.trend ?? null;
    }
    return mockTrend(this.baseQuery(), this.trendMetric);
  }

  private async loadSourceTrends(): Promise<Record<string, Trend>> {
    const sources = this.sourcesArray;
    const entries = await Promise.all(
      sources.map(async (source) => {
        if (isLive()) {
          const r = await getUsage(
            this.baseQuery({
              Sources: [source],
              View: 'trend',
              By: 'source',
              Sparkline: this.trendMetric === 'tokens' ? 'tokens' : 'cost',
            }),
          );
          return [source, r.trend ?? null] as const;
        }
        return [source, mockTrend(this.baseQuery({ Sources: [source] }), this.trendMetric)] as const;
      }),
    );
    return Object.fromEntries(entries.filter(([, trend]) => trend != null)) as Record<string, Trend>;
  }

  private previousBaseQuery(source: string): Query {
    const current = this.baseQuery({ Sources: [source] });
    const since = current.Since ? new Date(current.Since) : new Date(TODAY);
    const until = current.Until ? new Date(current.Until) : new Date(TODAY);
    const start = new Date(since.getFullYear(), since.getMonth(), since.getDate());
    const end = new Date(until.getFullYear(), until.getMonth(), until.getDate());
    const days = Math.max(1, Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1);
    const prevUntil = new Date(start);
    prevUntil.setDate(start.getDate() - 1);
    const prevSince = new Date(prevUntil);
    prevSince.setDate(prevUntil.getDate() - days + 1);
    return {
      ...current,
      Since: isoDay(prevSince),
      Until: isoDay(prevUntil),
    };
  }

  private async loadPreviousSourceTrends(): Promise<Record<string, Trend>> {
    const entries = await Promise.all(
      this.sourcesArray.map(async (source) => {
        const q = this.previousBaseQuery(source);
        if (isLive()) {
          const r = await getUsage({
            ...q,
            View: 'trend',
            By: 'source',
            Sparkline: this.trendMetric === 'tokens' ? 'tokens' : 'cost',
          });
          return [source, r.trend ?? null] as const;
        }
        return [source, mockPreviousTrend(this.baseQuery({ Sources: [source] }), this.trendMetric)] as const;
      }),
    );
    return Object.fromEntries(entries.filter(([, trend]) => trend != null)) as Record<string, Trend>;
  }

  /** A fresh result for a specific lens (Models / table column variants). */
  async query(extra: Partial<Query>): Promise<Result> {
    return getUsage(this.baseQuery(extra));
  }

  async init(): Promise<void> {
    document.documentElement.setAttribute('data-theme', this.theme);
    document.documentElement.setAttribute('data-density', this.density);
    // The v3 runtime injects window._wails.environment from a post-page-load
    // hook, which lands after this onMount fires — wait for it so live mode is
    // detected instead of falling back to mock data and never subscribing to
    // updates. In a plain browser this times out and we stay in mock mode.
    this.live = await waitForRuntime();
    this.devState = !this.live ? new URLSearchParams(window.location.search).get('state') : null;
    if (this.live && !this.usageChangedSubscribed) {
      onUsageChanged(() => void this.reload());
      this.usageChangedSubscribed = true;
    }
    try {
      const srcs = await listSources();
      this.allSources = srcs.filter((s): s is SourceId => SOURCE_ORDER.includes(s as SourceId));
      this.selectedSources = new Set(this.allSources);
    } catch {
      /* keep defaults */
    }
    if (this.devState === 'empty') {
      this.allSources = [];
      this.selectedSources = new Set();
      this.result = { view: 'summary', data: [], totals: zeroTotals() };
      this.lastScan = new Date();
      this.loading = false;
      return;
    }
    if (this.allSources.length === 0) {
      this.selectedSources = new Set();
      this.result = { view: 'summary', data: [], totals: zeroTotals() };
      this.lastScan = new Date();
      this.loading = false;
      return;
    }
    if (this.devState === 'loading') {
      this.loading = true;
      return;
    }
    await this.reload();
  }
}

function zeroTotals() {
  return {
    inputTokens: 0,
    outputTokens: 0,
    cacheCreationTokens: 0,
    cacheReadTokens: 0,
    reasoningTokens: 0,
    costUSD: 0,
    credits: 0,
  };
}

export const app = new AppStore();
