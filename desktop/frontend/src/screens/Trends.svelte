<script lang="ts">
  // Trends — the temporal screen. The trend series rendered as a larger uPlot
  // chart with min/max/total readouts, an anomaly note for the spike, and a
  // per-source contribution breakdown for the range.
  import { app } from '../lib/store.svelte';
  import type { Row } from '../api';
  import TrendChart from '../components/TrendChart.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import { fmtCost, fmtTokens } from '../lib/format';
  import { sourceColor, sourceGlyph, sourceLabel } from '../lib/palette';

  type ChartLine = {
    id: string;
    label: string;
    values: number[];
    color: string;
  };

  let sourceRows = $state<Row[]>([]);
  let activeSource = $state<string | null>(null);

  $effect(() => {
    void app.range;
    void app.selectedSources;
    void app.lastScan;
    void (async () => {
      const res = await app.query({ View: 'summary', By: 'source' });
      sourceRows = res.data;
    })();
  });

  const t = $derived(app.trend);
  const isCost = $derived(app.trendMetric === 'cost');
  const fmtY = (v: number) => (isCost ? fmtCost(v) : fmtTokens(v));

  // mean + the index of the peak, to annotate the anomaly.
  const stats = $derived.by(() => {
    if (!t || !t.series.length) return null;
    const n = t.series.length;
    const mean = t.total / n;
    let peakIdx = 0;
    t.series.forEach((v, i) => {
      if (v > t.series[peakIdx]) peakIdx = i;
    });
    const ratio = mean > 0 ? t.series[peakIdx] / mean : 0;
    return { mean, peakIdx, ratio, n };
  });

  function peakDate(): string {
    if (!t || !stats || !t.start) return '—';
    const d = new Date(t.start);
    d.setDate(d.getDate() + stats.peakIdx);
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  }

  const grandTotal = $derived(sourceRows.reduce((a, r) => a + r.costUSD, 0) || 1);

  const sourceLines = $derived.by<ChartLine[]>(() =>
    sourceRows
      .map((r) => {
        const id = r.source ?? r.key;
        const trend = app.sourceTrends[id];
        return trend
          ? {
              id,
              label: sourceLabel(id),
              values: trend.series,
              color: sourceColor(id),
            }
          : null;
      })
      .filter((line): line is ChartLine => line != null),
  );

  const previousLines = $derived.by<ChartLine[]>(() =>
    sourceLines
      .map((line) => {
        const trend = app.previousSourceTrends[line.id];
        return trend ? { ...line, values: trend.series } : null;
      })
      .filter((line): line is ChartLine => line != null),
  );
</script>

<div class="trends">
  <section class="big-chart card">
    <header class="t-head">
      <div>
        <h2>{isCost ? 'Cost' : 'Token'} trend</h2>
        <p class="sub">Per-day {isCost ? 'spend' : 'token volume'} over the selected range</p>
      </div>
      <div class="metric-toggle">
        <button class:on={isCost} onclick={() => app.setTrendMetric('cost')}>Cost</button>
        <button class:on={!isCost} onclick={() => app.setTrendMetric('tokens')}>Tokens</button>
        <button class:on={app.comparePrevious} onclick={() => app.setComparePrevious(!app.comparePrevious)}>Compare</button>
      </div>
    </header>

    {#if app.loading || !t || !sourceLines.length}
      <Skeleton variant="chart" height={360} />
    {:else}
      <TrendChart
        series={t.series}
        start={t.start}
        metric={app.trendMetric}
        height={360}
        lines={sourceLines}
        previousLines={app.comparePrevious ? previousLines : []}
        activeId={activeSource}
      />

      <div class="legend" role="group" onmouseleave={() => (activeSource = null)}>
        {#each sourceLines as line (line.id)}
          <button
            class:active={activeSource === line.id}
            onmouseenter={() => (activeSource = line.id)}
            onfocus={() => (activeSource = line.id)}
            onclick={() => (activeSource = activeSource === line.id ? null : line.id)}
            style:--c={line.color}
          >
            <span class="dot"></span>{line.label}
          </button>
        {/each}
        {#if app.comparePrevious}<span class="ghost-key mono">dashed = previous period</span>{/if}
      </div>

      <div class="readouts">
        <div class="ro">
          <span class="ro-l">Minimum</span>
          <span class="ro-v mono">{fmtY(t.min)}</span>
        </div>
        <div class="ro">
          <span class="ro-l">Maximum</span>
          <span class="ro-v mono amber">{fmtY(t.max)}</span>
        </div>
        <div class="ro">
          <span class="ro-l">Daily mean</span>
          <span class="ro-v mono">{stats ? fmtY(stats.mean) : '—'}</span>
        </div>
        <div class="ro">
          <span class="ro-l">Range total</span>
          <span class="ro-v mono">{fmtY(t.total)}</span>
        </div>
      </div>

      {#if stats && stats.ratio >= 1.8}
        <div class="anomaly">
          <span class="caret">⌃</span>
          Anomaly: <b>{peakDate()}</b> ran <b class="mono">{stats.ratio.toFixed(1)}×</b> your daily mean
          (<span class="mono">{fmtY(t.max)}</span> vs <span class="mono">{fmtY(stats.mean)}</span>).
        </div>
      {/if}
    {/if}
  </section>

  <section class="contrib card">
    <header><h3>Contribution by source</h3></header>
    {#if app.loading}
      <Skeleton variant="rows" />
    {:else}
      <ul class="bars">
        {#each sourceRows as r (r.key)}
          <li
            onmouseenter={() => (activeSource = r.source ?? r.key)}
            onmouseleave={() => (activeSource = null)}
            class:muted={activeSource != null && activeSource !== (r.source ?? r.key)}
          >
            <span class="bl" style:--c={sourceColor(r.source ?? r.key)}>
              <span class="g">{sourceGlyph(r.source ?? r.key)}</span>{sourceLabel(r.source ?? r.key)}
            </span>
            <span class="track">
              <span class="fill" style:background={sourceColor(r.source ?? r.key)} style:width="{(r.costUSD / grandTotal) * 100}%"></span>
            </span>
            <span class="pct mono">{((r.costUSD / grandTotal) * 100).toFixed(0)}%</span>
            <span class="bv mono">{fmtCost(r.costUSD)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .trends {
    display: flex;
    flex-direction: column;
    gap: var(--gap);
  }
  .card {
    background: var(--surface);
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    padding: var(--pad-card);
  }
  .t-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  h2 {
    font-size: 15px;
    font-weight: 600;
  }
  h3 {
    font-size: 13px;
    font-weight: 600;
  }
  .sub {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 2px;
  }
  .metric-toggle {
    display: flex;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    overflow: hidden;
  }
  .metric-toggle button {
    background: transparent;
    border: 0;
    color: var(--text-muted);
    font-size: 12px;
    padding: 5px 12px;
  }
  .metric-toggle button.on {
    background: color-mix(in srgb, var(--amber) 14%, transparent);
    color: var(--amber);
  }
  .legend {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    margin-top: 12px;
  }
  .legend button {
    display: flex;
    align-items: center;
    gap: 6px;
    background: transparent;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12px;
    padding: 4px 8px;
  }
  .legend button:hover,
  .legend button.active {
    border-color: color-mix(in srgb, var(--c) 55%, var(--hairline));
    color: var(--text);
    background: color-mix(in srgb, var(--c) 12%, transparent);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--c);
  }
  .ghost-key {
    color: var(--text-faint);
    font-size: 11px;
    margin-left: 4px;
  }
  .readouts {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1px;
    margin-top: 18px;
    background: var(--hairline);
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    overflow: hidden;
  }
  .ro {
    background: var(--surface);
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .ro-l {
    font-size: 10.5px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .ro-v {
    font-size: 17px;
    font-weight: 600;
    color: var(--text);
  }
  .ro-v.amber {
    color: var(--amber);
  }
  .anomaly {
    margin-top: 14px;
    padding: 10px 14px;
    border: 1px solid color-mix(in srgb, var(--amber) 40%, var(--hairline));
    background: color-mix(in srgb, var(--amber) 8%, transparent);
    border-radius: var(--r-card);
    font-size: 12.5px;
    color: var(--text);
  }
  .anomaly .caret {
    color: var(--amber);
    font-weight: 700;
    margin-right: 4px;
  }
  .anomaly b {
    color: var(--amber);
  }

  .contrib header {
    margin-bottom: 14px;
  }
  .bars {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .bars li {
    display: grid;
    grid-template-columns: 120px 1fr 44px 72px;
    align-items: center;
    gap: 12px;
    transition: opacity 0.12s;
  }
  .bars li.muted {
    opacity: 0.38;
  }
  .bl {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 13px;
    color: var(--text);
  }
  .bl .g {
    color: var(--c, var(--text-muted));
  }
  .track {
    height: 9px;
    background: var(--hairline);
    border-radius: 2px;
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
  }
  .pct {
    text-align: right;
    font-size: 12px;
    color: var(--text-muted);
  }
  .bv {
    text-align: right;
    font-size: 12.5px;
    color: var(--text);
  }
</style>
