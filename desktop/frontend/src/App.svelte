<script lang="ts">
  // App shell — the instrument-panel chrome. Title bar (offline + freshness),
  // a persistent left rail (lens switcher + source toggles + range + theme/
  // density), a top strip of totals "instrument" cards, a non-blocking warning
  // banner, and the active screen canvas. One filter drives every lens.
  import { onMount } from 'svelte';
  import { app, RANGE_LABELS, type RangeId, type ScreenId } from './lib/store.svelte';
  import { totalTokens, fmtTokens, fmtCost } from './lib/format';
  import { SOURCE_ORDER } from './lib/palette';
  import MetricCard from './components/MetricCard.svelte';
  import SourceChip from './components/SourceChip.svelte';
  import WarningBanner from './components/WarningBanner.svelte';
  import EmptyState from './components/EmptyState.svelte';
  import Overview from './screens/Overview.svelte';
  import Models from './screens/Models.svelte';
  import Trends from './screens/Trends.svelte';
  import TableScreen from './screens/TableScreen.svelte';

  const NAV: { id: ScreenId; label: string; glyph: string }[] = [
    { id: 'overview', label: 'Overview', glyph: '◉' },
    { id: 'models', label: 'Models', glyph: '◫' },
    { id: 'trends', label: 'Trends', glyph: '∿' },
    { id: 'table', label: 'Row data', glyph: '▤' },
  ];

  const RANGES: RangeId[] = ['today', '7d', '30d', 'month'];

  let now = $state(new Date());

  onMount(() => {
    void app.init();
    const tick = setInterval(() => (now = new Date()), 1000);
    return () => clearInterval(tick);
  });

  const totals = $derived(app.result?.totals ?? null);

  // Δ vs prior period — fabricated-but-believable in mock mode; the spark gives
  // the card context. (Live mode would use core's --compare previous.)
  const spend = $derived(totals?.costUSD ?? 0);
  const trendSeries = $derived(app.trend?.series ?? []);
  const priorDelta = $derived.by(() => {
    const s = trendSeries;
    if (s.length < 4) return 0;
    const half = Math.floor(s.length / 2);
    const recent = s.slice(half).reduce((a, b) => a + b, 0);
    const prior = s.slice(0, half).reduce((a, b) => a + b, 0) || 1;
    return Math.round(((recent - prior) / prior) * 100);
  });

  const cacheRatio = $derived.by(() => {
    if (!totals) return 0;
    const tot = totalTokens(totals);
    return tot > 0 ? totals.cacheReadTokens / tot : 0;
  });

  const activeModels = $derived(
    new Set((app.result?.data ?? []).flatMap((r) => r.modelsUsed ?? [])).size,
  );
  const inputOutputTotal = $derived((totals?.inputTokens ?? 0) + (totals?.outputTokens ?? 0));
  const inputShare = $derived(inputOutputTotal > 0 ? (totals?.inputTokens ?? 0) / inputOutputTotal : 0);
  const outputShare = $derived(inputOutputTotal > 0 ? (totals?.outputTokens ?? 0) / inputOutputTotal : 0);

  function freshnessLabel(): string {
    if (!app.lastScan) return 'scanning…';
    const secs = Math.floor((now.getTime() - app.lastScan.getTime()) / 1000);
    if (secs < 5) return 'just now';
    if (secs < 60) return `${secs}s ago`;
    const m = Math.floor(secs / 60);
    return `${m}m ago`;
  }
  const stale = $derived.by(() => {
    if (!app.lastScan) return false;
    return now.getTime() - app.lastScan.getTime() > 6 * 60 * 1000;
  });

  const isEmpty = $derived(!app.loading && (app.result?.data?.length ?? 0) === 0);
  const noSources = $derived(app.selectedSources.size === 0);
</script>

<div class="shell" class:rail-collapsed={app.railCollapsed}>
  <!-- title bar -->
  <header class="titlebar">
    <div class="brand">
      <span class="logo">llmut</span>
      <span class="offline" title="Zero network — reads local log files only">● Offline · Local files only</span>
      {#if !app.live}<span class="mock-tag mono">MOCK DATA</span>{/if}
    </div>
    <div class="freshness mono" class:stale aria-live="polite">
      <span class="clock">◷</span>
      Last scan {freshnessLabel()}
      {#if app.scanMs > 0}<span class="scan-ms">· {app.scanMs}ms</span>{/if}
      {#if stale}<button class="resume">resume</button>{/if}
    </div>
  </header>

  <div class="body">
    <!-- left rail -->
    <nav class="rail">
      <div class="rail-section nav-list">
        {#each NAV as n (n.id)}
          <button
            class="nav-item"
            class:active={app.screen === n.id}
            onclick={() => app.setScreen(n.id)}
            title={n.label}
          >
            <span class="nav-glyph">{n.glyph}</span>
            <span class="nav-label">{n.label}</span>
          </button>
        {/each}
      </div>

      <div class="rail-divider"></div>

      <div class="rail-section">
        <span class="rail-head">Range</span>
        <div class="range-seg">
          {#each RANGES as r (r)}
            <button class:on={app.range === r} onclick={() => app.setRange(r)}>{RANGE_LABELS[r]}</button>
          {/each}
        </div>
      </div>

      <div class="rail-section">
        <span class="rail-head">Sources</span>
        <div class="src-list">
          {#each SOURCE_ORDER as s (s)}
            {#if app.allSources.includes(s)}
              <SourceChip id={s} selected={app.selectedSources.has(s)} onToggle={(id) => app.toggleSource(id)} />
            {/if}
          {/each}
        </div>
      </div>

      <div class="rail-spacer"></div>

      <div class="rail-section chrome-toggles">
        <button class="chrome-btn" onclick={() => app.toggleTheme()} title="Toggle theme">
          <span class="ct-glyph">{app.theme === 'dark' ? '☾' : '☀'}</span>
          <span class="ct-label">{app.theme === 'dark' ? 'Graphite' : 'Paper'}</span>
        </button>
        <button class="chrome-btn" onclick={() => app.toggleDensity()} title="Toggle density">
          <span class="ct-glyph">{app.density === 'comfortable' ? '≣' : '≡'}</span>
          <span class="ct-label">{app.density === 'comfortable' ? 'Comfortable' : 'Compact'}</span>
        </button>
        <button
          class="chrome-btn collapse"
          onclick={() => (app.railCollapsed = !app.railCollapsed)}
          title="Collapse rail"
        >
          <span class="ct-glyph">{app.railCollapsed ? '»' : '«'}</span>
          <span class="ct-label">Collapse</span>
        </button>
      </div>
    </nav>

    <!-- canvas -->
    <main class="canvas">
      <!-- top totals strip -->
      <section class="metric-strip" aria-label="Totals">
        <MetricCard
          label="Spend (range)"
          value={fmtCost(spend)}
          delta={priorDelta}
          primary
          spark={trendSeries}
          skeleton={app.loading}
        />
        <MetricCard
          label="Total tokens"
          value={totals ? fmtTokens(totalTokens(totals)) : '—'}
          skeleton={app.loading}
        >
          {#snippet sub()}
            <div class="cache-ratio">
              <span class="cr-track"><span class="cr-fill" style:width="{cacheRatio * 100}%"></span></span>
              <span class="cr-label mono">{(cacheRatio * 100).toFixed(0)}% cache-read</span>
            </div>
          {/snippet}
        </MetricCard>
        <MetricCard label="Input" value={totals ? fmtTokens(totals.inputTokens) : '—'} skeleton={app.loading}>
          {#snippet sub()}
            <div class="io-context">
              <span class="io-track">
                <span class="io-input" style:width="{inputShare * 100}%"></span>
                <span class="io-output" style:width="{outputShare * 100}%"></span>
              </span>
              <span class="io-label mono">{(inputShare * 100).toFixed(0)}% of in/out</span>
            </div>
          {/snippet}
        </MetricCard>
        <MetricCard label="Output" value={totals ? fmtTokens(totals.outputTokens) : '—'} skeleton={app.loading}>
          {#snippet sub()}
            <div class="io-context">
              <span class="io-track">
                <span class="io-input" style:width="{inputShare * 100}%"></span>
                <span class="io-output" style:width="{outputShare * 100}%"></span>
              </span>
              <span class="io-label mono">{(outputShare * 100).toFixed(0)}% of in/out</span>
            </div>
          {/snippet}
        </MetricCard>
        <MetricCard
          label="Active models"
          value={app.loading ? '—' : String(activeModels)}
          skeleton={app.loading}
        />
      </section>

      {#if app.result?.warnings?.length}
        <div class="warn-slot"><WarningBanner warnings={app.result.warnings} /></div>
      {/if}

      <!-- active screen -->
      <div class="screen">
        {#if app.error}
          <div class="error-state" role="alert">
            <h2>Couldn't load usage</h2>
            <p class="mono">{app.error}</p>
          </div>
        {:else if noSources}
          <EmptyState mode="no-sources" />
        {:else if isEmpty}
          <EmptyState mode="no-range" onJump={() => app.setRange('30d')} />
        {:else if app.screen === 'overview'}
          <Overview />
        {:else if app.screen === 'models'}
          <Models />
        {:else if app.screen === 'trends'}
          <Trends />
        {:else}
          <TableScreen />
        {/if}
      </div>
    </main>
  </div>
</div>

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: var(--canvas);
  }

  /* title bar */
  .titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 42px;
    padding: 0 16px;
    border-bottom: 1px solid var(--hairline);
    background: var(--surface);
    flex-shrink: 0;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .logo {
    font-family: 'IBM Plex Mono', monospace;
    font-weight: 600;
    font-size: 14px;
    letter-spacing: 0.02em;
    color: var(--text);
  }
  .offline {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--teal);
    border: 1px solid color-mix(in srgb, var(--teal) 35%, var(--hairline));
    border-radius: 999px;
    padding: 2px 10px;
  }
  .mock-tag {
    font-size: 10px;
    letter-spacing: 0.08em;
    color: var(--caution);
    border: 1px dashed color-mix(in srgb, var(--caution) 45%, var(--hairline));
    border-radius: var(--r-chip);
    padding: 2px 7px;
  }
  .freshness {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11.5px;
    color: var(--text-muted);
  }
  .freshness.stale {
    color: var(--caution);
  }
  .clock {
    color: var(--text-muted);
  }
  .freshness.stale .clock {
    color: var(--caution);
  }
  .scan-ms {
    color: var(--text-faint);
  }
  .resume {
    margin-left: 6px;
    background: transparent;
    border: 1px solid var(--caution);
    border-radius: var(--r-chip);
    color: var(--caution);
    font-size: 11px;
    padding: 1px 8px;
  }

  .body {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  /* rail */
  .rail {
    width: 208px;
    flex-shrink: 0;
    border-right: 1px solid var(--hairline);
    background: var(--surface);
    display: flex;
    flex-direction: column;
    padding: 14px 12px;
    gap: 16px;
    overflow-y: auto;
    transition: width 0.16s ease;
  }
  .rail-collapsed .rail {
    width: 60px;
    padding: 14px 8px;
  }
  .rail-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .nav-list {
    gap: 3px;
  }
  .nav-item {
    display: flex;
    align-items: center;
    gap: 11px;
    padding: 8px 10px;
    border: 0;
    border-radius: var(--r-chip);
    background: transparent;
    color: var(--text-muted);
    font-size: 13.5px;
    text-align: left;
    transition: background 0.12s, color 0.12s;
  }
  .nav-item:hover {
    background: var(--surface-raised);
    color: var(--text);
  }
  .nav-item.active {
    background: color-mix(in srgb, var(--amber) 12%, transparent);
    color: var(--amber);
  }
  .nav-glyph {
    width: 18px;
    text-align: center;
    font-size: 14px;
    flex-shrink: 0;
  }
  .rail-collapsed .nav-label,
  .rail-collapsed .rail-head,
  .rail-collapsed .range-seg,
  .rail-collapsed .src-list,
  .rail-collapsed .ct-label {
    display: none;
  }
  .rail-divider {
    height: 1px;
    background: var(--hairline);
    margin: 2px 0;
  }
  .rail-head {
    font-size: 10px;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .range-seg {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px;
  }
  .range-seg button {
    background: transparent;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12px;
    padding: 5px 0;
  }
  .range-seg button:hover {
    border-color: var(--hairline-strong);
    color: var(--text);
  }
  .range-seg button.on {
    background: color-mix(in srgb, var(--amber) 14%, transparent);
    border-color: color-mix(in srgb, var(--amber) 40%, var(--hairline));
    color: var(--amber);
  }
  .src-list {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .rail-spacer {
    flex: 1;
  }
  .chrome-toggles {
    gap: 4px;
  }
  .chrome-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 10px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12.5px;
    text-align: left;
  }
  .chrome-btn:hover {
    background: var(--surface-raised);
    color: var(--text);
  }
  .ct-glyph {
    width: 18px;
    text-align: center;
    flex-shrink: 0;
  }

  /* canvas */
  .canvas {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    padding: 16px;
    gap: var(--gap);
    overflow-y: auto;
  }
  .metric-strip {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: var(--gap);
    flex-shrink: 0;
  }
  .cache-ratio {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .cr-track {
    flex: 1;
    height: 4px;
    background: var(--hairline);
    border-radius: 2px;
    overflow: hidden;
  }
  .cr-fill {
    display: block;
    height: 100%;
    background: var(--tok-cache-read);
  }
  .cr-label {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
  }
  .io-context {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .io-track {
    flex: 1;
    height: 4px;
    background: var(--hairline);
    border-radius: 2px;
    overflow: hidden;
    display: flex;
  }
  .io-input {
    height: 100%;
    background: var(--tok-input);
  }
  .io-output {
    height: 100%;
    background: var(--tok-output);
  }
  .io-label {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
  }
  .warn-slot {
    flex-shrink: 0;
  }
  .screen {
    flex: 1;
    min-height: 0;
  }
  .error-state {
    border: 1px solid var(--red);
    border-radius: var(--r-card);
    background: color-mix(in srgb, var(--red) 8%, var(--surface));
    padding: 28px;
  }
  .error-state h2 {
    font-size: 16px;
    color: var(--red);
    margin-bottom: 8px;
  }
  .error-state p {
    color: var(--text-muted);
    font-size: 12px;
  }

  @media (max-width: 1080px) {
    .metric-strip {
      grid-template-columns: repeat(3, 1fr);
    }
  }
  @media (max-width: 720px) {
    .metric-strip {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
