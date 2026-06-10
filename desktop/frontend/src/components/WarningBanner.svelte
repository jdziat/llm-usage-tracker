<script lang="ts">
  // Non-blocking amber warning banner for partial source failures. The CLI
  // swallows these into --debug; the desktop never hides them. Collapsed to a
  // one-liner, expands to the full Warning list. Degraded, never broken.
  let { warnings }: { warnings: string[] } = $props();
  let open = $state(false);
</script>

{#if warnings.length}
  <div class="banner" role="status">
    <div class="head">
      <span class="ic" aria-hidden="true">⚠</span>
      <span class="msg">
        {warnings.length} source {warnings.length === 1 ? 'notice' : 'notices'} — showing all readable data
      </span>
      <button class="toggle" onclick={() => (open = !open)} aria-expanded={open}>
        {open ? 'Hide' : 'Details'} {open ? '▴' : '▾'}
      </button>
    </div>
    {#if open}
      <ul class="list mono">
        {#each warnings as w}
          <li>{w}</li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .banner {
    border: 1px solid color-mix(in srgb, var(--caution) 50%, var(--hairline));
    background: color-mix(in srgb, var(--caution) 9%, var(--surface));
    border-radius: var(--r-card);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 14px;
  }
  .ic {
    color: var(--caution);
    font-size: 14px;
  }
  .msg {
    flex: 1;
    font-size: 13px;
    color: var(--text);
  }
  .toggle {
    background: transparent;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12px;
    padding: 3px 9px;
  }
  .toggle:hover {
    color: var(--text);
    border-color: var(--hairline-strong);
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0 14px 12px 38px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted);
  }
  .list li {
    line-height: 1.5;
  }
</style>
