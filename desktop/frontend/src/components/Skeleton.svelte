<script lang="ts">
  // Hairline-outlined placeholder shells that match the final layout so nothing
  // jumps when data lands. Variant chooses card-strip vs chart vs table shape.
  let { variant = 'card', height }: { variant?: 'card' | 'chart' | 'rows'; height?: number } = $props();
</script>

{#if variant === 'card'}
  <div class="shell card">
    <span class="bar w40"></span>
    <span class="bar big w60"></span>
  </div>
{:else if variant === 'chart'}
  <div class="shell chart" style:height="{height ?? 280}px">
    <span class="grid"></span>
  </div>
{:else}
  <div class="shell rows">
    {#each Array(8) as _, i (i)}
      <div class="row">
        <span class="bar w20"></span>
        <span class="bar w30"></span>
        <span class="bar w15"></span>
        <span class="bar w15"></span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .shell {
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    background: var(--surface);
    padding: var(--pad-card);
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .chart {
    position: relative;
    overflow: hidden;
  }
  .chart .grid {
    position: absolute;
    inset: var(--pad-card);
    background-image: repeating-linear-gradient(
      to bottom,
      transparent,
      transparent 39px,
      var(--hairline) 39px,
      var(--hairline) 40px
    );
    opacity: 0.5;
  }
  .rows {
    display: flex;
    flex-direction: column;
    gap: 0;
    padding: 0;
  }
  .row {
    display: flex;
    gap: 16px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--hairline);
  }
  .bar {
    height: 12px;
    border-radius: 3px;
    background: linear-gradient(90deg, var(--hairline), var(--surface-raised), var(--hairline));
    background-size: 200% 100%;
    animation: shimmer 1.3s linear infinite;
    display: inline-block;
  }
  .bar.big {
    height: 24px;
  }
  .w15 {
    width: 15%;
  }
  .w20 {
    width: 20%;
  }
  .w30 {
    width: 30%;
  }
  .w40 {
    width: 40%;
  }
  .w60 {
    width: 60%;
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
