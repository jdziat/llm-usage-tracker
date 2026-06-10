<script lang="ts">
  // A single horizontal bar segmented into the five token categories — the
  // attribution micro-viz from the Models screen. Flat, sharp, no rounded caps.
  import type { Tokens } from '../api';
  import { TOKEN_CATS, tokenCatColor } from '../lib/palette';
  import { fmtTokens } from '../lib/format';

  let { tokens, height = 8, showLegend = false }: { tokens: Tokens; height?: number; showLegend?: boolean } =
    $props();

  const segs = $derived(
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
      return { ...c, value, color: tokenCatColor(c.id) };
    }).filter((s) => s.value > 0),
  );
  const total = $derived(segs.reduce((a, s) => a + s.value, 0) || 1);
</script>

<div class="comp" style:height="{height}px" role="img" aria-label="Token composition">
  {#each segs as s (s.id)}
    <span
      class="seg"
      style:width="{(s.value / total) * 100}%"
      style:background={s.color}
      title="{s.label}: {fmtTokens(s.value)} ({((s.value / total) * 100).toFixed(1)}%)"
    ></span>
  {/each}
</div>

{#if showLegend}
  <div class="legend mono">
    {#each segs as s (s.id)}
      <span class="lg">
        <span class="dot" style:background={s.color}></span>
        {s.label}
        <span class="lv">{fmtTokens(s.value)}</span>
        <span class="lp">{((s.value / total) * 100).toFixed(0)}%</span>
      </span>
    {/each}
  </div>
{/if}

<style>
  .comp {
    display: flex;
    width: 100%;
    border-radius: 2px;
    overflow: hidden;
    background: var(--hairline);
  }
  .seg {
    height: 100%;
    display: block;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 16px;
    margin-top: 12px;
    font-size: 12px;
  }
  .lg {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-muted);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    display: inline-block;
  }
  .lv {
    color: var(--text);
  }
  .lp {
    color: var(--text-faint);
  }
</style>
