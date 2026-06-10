<script lang="ts">
  // Models — the attribution screen, keyed on model. Left: a leaderboard of
  // model card-rows with a token-composition micro-bar each. Right: the
  // selected model's anatomy — a token-category donut, a fast-tier callout, and
  // a price-provenance line that explains $0 rows honestly.
  import { app } from '../lib/store.svelte';
  import type { Row } from '../api';
  import CompositionBar from '../components/CompositionBar.svelte';
  import TokenDonut from '../components/TokenDonut.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import { fmtCost, fmtTokens, totalTokens } from '../lib/format';
  import { sourceColor, sourceGlyph, sourceForModel, isPriced, FAST_MULTIPLIER } from '../lib/palette';

  let rows = $state<Row[]>([]);
  let selectedKey = $state<string | null>(null);
  let loading = $state(true);

  $effect(() => {
    void app.range;
    void app.selectedSources;
    void app.lastScan;
    loading = true;
    void (async () => {
      const res = await app.query({ View: 'summary', By: 'model' });
      rows = res.data;
      loading = false;
    })();
  });

  $effect(() => {
    if (!rows.some((r) => r.key === selectedKey)) {
      selectedKey = rows[0]?.key ?? null;
    }
  });

  const selected = $derived(rows.find((r) => r.key === selectedKey) ?? rows[0] ?? null);
  const maxCost = $derived(Math.max(1, ...rows.map((r) => r.costUSD)));
  const selectedTokens = $derived(selected ? totalTokens(selected) : 0);

  // illustrative offline price table (mirror of pkg/core/pricing.go in/out).
  const PRICES: Record<string, [number, number]> = {
    'claude-opus-4-8': [5, 25],
    'claude-sonnet-4-6': [3, 15],
    'claude-haiku-4-5': [1, 5],
    'gpt-5.5': [1.25, 10],
    'gpt-5.3-codex': [1.25, 10],
    'gemini-2.5-pro': [1.25, 10],
    'claude-fable-5': [10, 50],
  };

  const fastMult = $derived(selected ? (FAST_MULTIPLIER[selected.key] ?? 0) : 0);
  const price = $derived(selected ? PRICES[selected.key] : undefined);
</script>

<div class="models">
  <section class="leaderboard card">
    <header><h2>Models by cost</h2></header>
    {#if loading}
      <Skeleton variant="rows" />
    {:else}
      <ul class="rows">
        {#each rows as r (r.key)}
          {@const src = r.source ?? sourceForModel(r.key)}
          <li>
            <button class="row" class:active={r.key === selectedKey} onclick={() => (selectedKey = r.key)}>
              <div class="r-top">
                <span class="glyph" style:color={sourceColor(src)}>{sourceGlyph(src)}</span>
                <span class="name" class:unpriced={!isPriced(r.key)}>{r.key}</span>
                <span class="cost mono" class:unpriced={!isPriced(r.key) && r.costUSD === 0}>
                  {fmtCost(r.costUSD)}
                  {#if !isPriced(r.key) && r.costUSD === 0}<span class="info" title="Unknown model — tokens counted, cost unpriced">ⓘ</span>{/if}
                </span>
              </div>
              <CompositionBar tokens={r} height={6} />
              <div class="r-bot mono">
                <span>{fmtTokens(totalTokens(r))} tok</span>
                <span class="track-mini"><span class="fill" style:width="{(r.costUSD / maxCost) * 100}%"></span></span>
              </div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="anatomy card">
    {#if loading || !selected}
      <Skeleton variant="chart" height={200} />
    {:else}
      {@const src = selected.source ?? sourceForModel(selected.key)}
      <header class="a-head">
        <div>
          <span class="a-glyph" style:color={sourceColor(src)}>{sourceGlyph(src)}</span>
          <h2 class:unpriced={!isPriced(selected.key)}>{selected.key}</h2>
        </div>
        <span class="a-cost mono">{fmtCost(selected.costUSD)}</span>
      </header>

      <div class="donut-block">
        <TokenDonut tokens={selected} />
      </div>

      <div class="stat-grid">
        <div>
          <span class="sg-l">Input</span>
          <span class="sg-v mono">{fmtTokens(selected.inputTokens)}</span>
        </div>
        <div>
          <span class="sg-l">Output</span>
          <span class="sg-v mono">{fmtTokens(selected.outputTokens)}</span>
        </div>
        <div>
          <span class="sg-l">Cache read</span>
          <span class="sg-v mono">{fmtTokens(selected.cacheReadTokens)}</span>
        </div>
        <div>
          <span class="sg-l">Total</span>
          <span class="sg-v mono">{fmtTokens(selectedTokens)}</span>
        </div>
      </div>

      {#if fastMult}
        <div class="callout">
          <span class="co-tag">FAST TIER</span>
          <p>
            This model carries a <b class="mono">{fastMult}×</b> fast multiplier. On the standard tier the same tokens
            would cost <b class="mono">{fmtCost(selected.costUSD / fastMult)}</b> — you're paying
            <b class="mono">{fmtCost(selected.costUSD - selected.costUSD / fastMult)}</b> extra for speed.
          </p>
        </div>
      {/if}

      <div class="provenance mono">
        {#if price}
          <span class="prov-price">${price[0].toFixed(2)} / ${price[1].toFixed(2)} per 1M in/out</span>
          <span class="prov-sep">·</span>
          <span class="prov-src">offline price table, verified 2026-06-09</span>
        {:else}
          <span class="prov-unknown">⚠ Unknown model — tokens counted, cost unpriced ($0). Not in the offline table.</span>
        {/if}
      </div>
    {/if}
  </section>
</div>

<style>
  .models {
    display: grid;
    grid-template-columns: minmax(320px, 40%) 1fr;
    gap: var(--gap);
    align-items: start;
  }
  .card {
    background: var(--surface);
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    padding: var(--pad-card);
  }
  h2 {
    font-size: 15px;
    font-weight: 600;
  }
  h2.unpriced {
    color: var(--text-muted);
  }
  .leaderboard header {
    margin-bottom: 14px;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .row {
    width: 100%;
    text-align: left;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--r-card);
    padding: 11px 12px;
    display: flex;
    flex-direction: column;
    gap: 9px;
    transition: border-color 0.12s, background 0.12s;
  }
  .row:hover {
    background: var(--surface-raised);
  }
  .row.active {
    border-color: color-mix(in srgb, var(--amber) 40%, var(--hairline));
    background: color-mix(in srgb, var(--amber) 7%, transparent);
  }
  .r-top {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .glyph {
    width: 14px;
    text-align: center;
  }
  .name {
    flex: 1;
    font-size: 13px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .name.unpriced {
    color: var(--text-muted);
  }
  .cost {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }
  .cost.unpriced {
    color: var(--text-faint);
    font-weight: 400;
  }
  .info {
    color: var(--caution);
    cursor: help;
  }
  .r-bot {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 11px;
    color: var(--text-muted);
  }
  .track-mini {
    flex: 1;
    height: 3px;
    background: var(--hairline);
    border-radius: 2px;
    overflow: hidden;
  }
  .track-mini .fill {
    display: block;
    height: 100%;
    background: var(--amber-dim);
  }

  .anatomy {
    min-height: 320px;
  }
  .a-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  .a-head > div {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .a-glyph {
    font-size: 16px;
  }
  .a-cost {
    font-size: 22px;
    font-weight: 600;
    color: var(--amber);
  }
  .donut-block {
    padding: 8px 0 20px;
  }
  .stat-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1px;
    margin: 0 0 16px;
    background: var(--hairline);
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    overflow: hidden;
  }
  .stat-grid > div {
    background: var(--surface);
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .sg-l {
    font-size: 10px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .sg-v {
    color: var(--text);
    font-weight: 600;
  }
  .callout {
    border: 1px solid color-mix(in srgb, var(--amber) 40%, var(--hairline));
    background: color-mix(in srgb, var(--amber) 8%, transparent);
    border-radius: var(--r-card);
    padding: 12px 14px;
    display: flex;
    gap: 12px;
    align-items: flex-start;
    margin-bottom: 16px;
  }
  .co-tag {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 10px;
    letter-spacing: 0.08em;
    color: var(--amber);
    border: 1px solid color-mix(in srgb, var(--amber) 45%, var(--hairline));
    border-radius: var(--r-chip);
    padding: 3px 7px;
    flex-shrink: 0;
    margin-top: 1px;
  }
  .callout p {
    font-size: 12.5px;
    line-height: 1.55;
    color: var(--text);
  }
  .callout b {
    color: var(--amber);
  }
  .provenance {
    font-size: 11.5px;
    color: var(--text-muted);
    padding-top: 14px;
    border-top: 1px solid var(--hairline);
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .prov-sep {
    color: var(--text-faint);
  }
  .prov-src {
    color: var(--text-faint);
  }
  .prov-unknown {
    color: var(--caution);
  }

  @media (max-width: 980px) {
    .models {
      grid-template-columns: 1fr;
    }
  }
</style>
