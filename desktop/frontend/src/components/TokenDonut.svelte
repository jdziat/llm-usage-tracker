<script lang="ts">
  // SVG donut of the five token categories with absolute counts and the dollar
  // contribution of each — the most educational view in the product. Pure
  // presentation over the Tokens DTO; no rounded caps, flat fills.
  import type { Tokens } from '../api';
  import { TOKEN_CATS, tokenCatColor } from '../lib/palette';
  import { fmtTokens, fmtCost } from '../lib/format';

  let { tokens, size = 168 }: { tokens: Tokens; size?: number } = $props();

  // Rough cost share per category using the price-table proportions for the
  // dominant model; in live mode this is illustrative — totals stay exact.
  const COST_WEIGHT: Record<string, number> = {
    input: 5,
    output: 25,
    cacheCreate: 6.25,
    cacheRead: 0.5,
    reasoning: 25,
  };

  const cats = $derived(
    TOKEN_CATS.map((c) => {
      const value =
        c.id === 'input'
          ? tokens.inputTokens
          : c.id === 'output'
            ? tokens.outputTokens
            : c.id === 'cacheCreate'
              ? tokens.cacheCreationTokens
              : c.id === 'cacheRead'
                ? tokens.cacheReadTokens
                : (tokens.reasoningTokens ?? 0);
      return { ...c, value, color: tokenCatColor(c.id), cost: value * (COST_WEIGHT[c.id] ?? 1) };
    }).filter((c) => c.value > 0),
  );

  const total = $derived(cats.reduce((a, c) => a + c.value, 0) || 1);
  const costTotal = $derived(cats.reduce((a, c) => a + c.cost, 0) || 1);

  const r = $derived(size / 2 - 12);
  const cx = $derived(size / 2);
  const circ = $derived(2 * Math.PI * r);

  // pre-compute dash offsets for each arc segment
  const arcs = $derived.by(() => {
    let acc = 0;
    return cats.map((c) => {
      const frac = c.value / total;
      const seg = { ...c, dash: frac * circ, offset: -acc * circ, frac };
      acc += frac;
      return seg;
    });
  });
</script>

<div class="donut-wrap">
  <svg width={size} height={size} viewBox="0 0 {size} {size}" role="img" aria-label="Token category donut">
    <circle {cx} cy={cx} {r} fill="none" stroke="var(--hairline)" stroke-width="14" />
    {#each arcs as a (a.id)}
      <circle
        {cx}
        cy={cx}
        {r}
        fill="none"
        stroke={a.color}
        stroke-width="14"
        stroke-dasharray="{a.dash} {circ - a.dash}"
        stroke-dashoffset={a.offset}
        transform="rotate(-90 {cx} {cx})"
      >
        <title>{a.label}: {fmtTokens(a.value)} ({(a.frac * 100).toFixed(1)}% of tokens)</title>
      </circle>
    {/each}
    <text x={cx} y={cx - 6} text-anchor="middle" class="center-top mono">{fmtTokens(total)}</text>
    <text x={cx} y={cx + 12} text-anchor="middle" class="center-sub">tokens</text>
  </svg>

  <ul class="rows mono">
    {#each cats as c (c.id)}
      <li>
        <span class="dot" style:background={c.color}></span>
        <span class="lbl">{c.label}</span>
        <span class="tok">{fmtTokens(c.value)}</span>
        <span class="share">{((c.value / total) * 100).toFixed(0)}% tok</span>
        <span class="cost-share">{((c.cost / costTotal) * 100).toFixed(0)}% $</span>
      </li>
    {/each}
  </ul>
</div>

<style>
  .donut-wrap {
    display: flex;
    gap: 24px;
    align-items: center;
    flex-wrap: wrap;
  }
  .center-top {
    fill: var(--text);
    font-size: 18px;
    font-weight: 600;
  }
  .center-sub {
    fill: var(--text-muted);
    font-size: 11px;
    font-family: 'Geist Sans', sans-serif;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 7px;
    font-size: 12px;
    min-width: 240px;
  }
  .rows li {
    display: grid;
    grid-template-columns: 12px 1fr auto auto auto;
    gap: 10px;
    align-items: center;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 2px;
  }
  .lbl {
    color: var(--text-muted);
    font-family: 'Geist Sans', sans-serif;
  }
  .tok {
    color: var(--text);
    text-align: right;
  }
  .share {
    color: var(--text-faint);
    text-align: right;
    min-width: 52px;
  }
  .cost-share {
    color: var(--amber);
    text-align: right;
    min-width: 40px;
  }
</style>
