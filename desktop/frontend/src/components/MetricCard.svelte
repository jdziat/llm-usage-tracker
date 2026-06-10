<script lang="ts">
  // An "instrument" KPI card: a big mono number with a label, an optional Δ vs.
  // prior period, an optional sub-bar (e.g. cache-read ratio), and optional
  // sparkline context. The primary card glows amber; the rest stay muted.
  import type { Snippet } from 'svelte';
  import Sparkline from './Sparkline.svelte';

  let {
    label,
    value,
    delta,
    primary = false,
    spark,
    sub,
    skeleton = false,
  }: {
    label: string;
    value: string;
    delta?: number;
    primary?: boolean;
    spark?: number[];
    sub?: Snippet;
    skeleton?: boolean;
  } = $props();
</script>

<div class="card" class:primary aria-label={label}>
  <span class="label">{label}</span>
  {#if skeleton}
    <span class="skel num"></span>
  {:else}
    <div class="value-row">
      <strong class="num value">{value}</strong>
      {#if spark}
        <div class="spark"><Sparkline values={spark} accent={primary} /></div>
      {/if}
    </div>
    {#if delta != null}
      <span class="delta num" class:up={delta > 0} class:down={delta < 0}>
        {delta > 0 ? '▲' : delta < 0 ? '▼' : '·'} {Math.abs(delta).toFixed(0)}% vs prior
      </span>
    {/if}
    {#if sub}
      <div class="sub">{@render sub()}</div>
    {/if}
  {/if}
</div>

<style>
  .card {
    background: var(--surface);
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    padding: var(--pad-card);
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
  }
  .card.primary {
    border-color: color-mix(in srgb, var(--amber) 38%, var(--hairline));
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--amber) 12%, transparent);
  }
  .label {
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .value-row {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 12px;
  }
  .value {
    font-size: clamp(20px, 2.2vw, 28px);
    line-height: 1;
    font-weight: 600;
    color: var(--text);
  }
  .card.primary .value {
    color: var(--amber);
  }
  .spark {
    flex-shrink: 0;
    opacity: 0.9;
  }
  .delta {
    font-size: 11.5px;
    color: var(--text-faint);
  }
  .delta.up {
    color: var(--amber);
  }
  .delta.down {
    color: var(--teal);
  }
  .sub {
    margin-top: 2px;
  }
  .skel {
    height: 26px;
    width: 70%;
    border-radius: 3px;
    background: linear-gradient(90deg, var(--hairline), var(--surface-raised), var(--hairline));
    background-size: 200% 100%;
    animation: shimmer 1.3s linear infinite;
  }
  @keyframes shimmer {
    0% {
      background-position: 200% 0;
    }
    100% {
      background-position: -200% 0;
    }
  }
</style>
