<script lang="ts">
  // Overview — the hero. Totals strip + the cost trend (amber trace on a
  // hairline grid) + the usage table below. Five-second orientation.
  import { app } from '../lib/store.svelte';
  import type { Row } from '../api';
  import TrendChart from '../components/TrendChart.svelte';
  import UsageTable from '../components/UsageTable.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import { fmtCost, fmtTokens } from '../lib/format';
  import { sourceColor, sourceGlyph, sourceLabel, sourceForModel } from '../lib/palette';

  let modelRows = $state<Row[]>([]);
  let projectRows = $state<Row[]>([]);

  // Top-models and top-projects panels read sibling summary lenses.
  $effect(() => {
    void app.range;
    void app.selectedSources;
    void app.lastScan;
    void (async () => {
      const [m, p] = await Promise.all([
        app.query({ View: 'summary', By: 'model', Top: 5 }),
        app.query({ View: 'summary', By: 'project', Top: 5 }),
      ]);
      modelRows = m.data;
      projectRows = p.data;
    })();
  });

  const top = $derived.by(() => {
    const rows = app.result?.data ?? [];
    const lead = [...rows].sort((a, b) => b.costUSD - a.costUSD)[0];
    return lead;
  });

  function maxCost(rows: Row[]): number {
    return Math.max(1, ...rows.map((r) => r.costUSD));
  }
</script>

<div class="overview">
  <section class="hero-chart card">
    <header class="ch-head">
      <div>
        <h2>Spend over time</h2>
        <p class="sub">{app.trendMetric === 'cost' ? 'Cost per day' : 'Tokens per day'} across the selected range</p>
      </div>
      <div class="metric-toggle">
        <button class:on={app.trendMetric === 'cost'} onclick={() => app.setTrendMetric('cost')}>Cost</button>
        <button class:on={app.trendMetric === 'tokens'} onclick={() => app.setTrendMetric('tokens')}>Tokens</button>
      </div>
    </header>
    {#if app.loading || !app.trend}
      <Skeleton variant="chart" height={260} />
    {:else}
      <TrendChart
        series={app.trend.series}
        start={app.trend.start}
        metric={app.trendMetric}
        height={260}
      />
      <div class="ch-readouts mono">
        <span><em>min</em> {app.trendMetric === 'cost' ? fmtCost(app.trend.min) : fmtTokens(app.trend.min)}</span>
        <span><em>max</em> <b>{app.trendMetric === 'cost' ? fmtCost(app.trend.max) : fmtTokens(app.trend.max)}</b></span>
        <span><em>total</em> {app.trendMetric === 'cost' ? fmtCost(app.trend.total) : fmtTokens(app.trend.total)}</span>
      </div>
    {/if}
  </section>

  <div class="panels">
    <section class="panel card">
      <header class="p-head">
        <h3>Top models</h3>
        <button class="see" onclick={() => app.setScreen('models')}>See all →</button>
      </header>
      {#if app.loading}
        <Skeleton variant="rows" />
      {:else}
        <ul class="bars">
          {#each modelRows as r (r.key)}
            {@const src = r.source ?? sourceForModel(r.key)}
            <li>
              <span class="bl" style:--c={sourceColor(src)}>
                <span class="g">{sourceGlyph(src)}</span>{r.key}
              </span>
              <span class="track"><span class="fill" style:background={sourceColor(src)} style:width="{(r.costUSD / maxCost(modelRows)) * 100}%"></span></span>
              <span class="bv mono">{fmtCost(r.costUSD)}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section class="panel card">
      <header class="p-head">
        <h3>Top projects</h3>
        <button class="see" onclick={() => app.setScreen('table')}>See rows →</button>
      </header>
      {#if app.loading}
        <Skeleton variant="rows" />
      {:else}
        <ul class="bars">
          {#each projectRows as r (r.key)}
            <li>
              <span class="bl" style:--c={sourceColor(r.source)}>
                <span class="g">{sourceGlyph(r.source)}</span>{r.key}
              </span>
              <span class="track"><span class="fill" style:background={sourceColor(r.source)} style:width="{(r.costUSD / maxCost(projectRows)) * 100}%"></span></span>
              <span class="bv mono">{fmtCost(r.costUSD)}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>

  <section class="table-section">
    <header class="ts-head">
      <h3>Usage by source {#if top}· <span class="lead mono" style:color={sourceColor(top.source)}>{sourceLabel(top.source)} leads at {fmtCost(top.costUSD)}</span>{/if}</h3>
    </header>
    {#if app.loading}
      <Skeleton variant="rows" />
    {:else}
      <UsageTable rows={app.result?.data ?? []} dense={app.density === 'compact'} />
    {/if}
  </section>
</div>

<style>
  .overview {
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
  .ch-head,
  .p-head,
  .ts-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .ch-head {
    margin-bottom: 14px;
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
  .ch-readouts {
    display: flex;
    gap: 22px;
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--hairline);
    font-size: 12px;
    color: var(--text);
  }
  .ch-readouts em {
    color: var(--text-muted);
    font-style: normal;
    margin-right: 6px;
    text-transform: uppercase;
    font-size: 10px;
    letter-spacing: 0.05em;
  }
  .ch-readouts b {
    color: var(--amber);
  }

  .panels {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--gap);
  }
  .p-head {
    margin-bottom: 12px;
  }
  .see {
    background: transparent;
    border: 0;
    color: var(--text-muted);
    font-size: 12px;
  }
  .see:hover {
    color: var(--amber);
  }
  .bars {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 9px;
  }
  .bars li {
    display: grid;
    grid-template-columns: 140px 1fr 72px;
    align-items: center;
    gap: 12px;
  }
  .bl {
    font-size: 12.5px;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .bl .g {
    color: var(--c, var(--text-muted));
  }
  .track {
    height: 8px;
    background: var(--hairline);
    border-radius: 2px;
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
  }
  .bv {
    text-align: right;
    font-size: 12.5px;
    color: var(--text);
  }
  .ts-head {
    margin-bottom: 12px;
  }
  .lead {
    font-weight: 600;
  }
  .table-section :global(.table) {
    max-height: 420px;
  }

  @media (max-width: 920px) {
    .panels {
      grid-template-columns: 1fr;
    }
  }
</style>
