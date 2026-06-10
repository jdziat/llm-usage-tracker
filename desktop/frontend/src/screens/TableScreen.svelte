<script lang="ts">
  // The full-height data-table lens — the dense [] Row view as a primary screen.
  import { app } from '../lib/store.svelte';
  import type { Row } from '../api';
  import UsageTable from '../components/UsageTable.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  type ViewId = 'summary-source' | 'summary-model' | 'summary-project' | 'daily' | 'session';
  let view = $state<ViewId>('daily');
  let rows = $state<Row[]>([]);
  let loading = $state(true);

  const VIEWS: { id: ViewId; label: string }[] = [
    { id: 'daily', label: 'Daily' },
    { id: 'summary-source', label: 'By source' },
    { id: 'summary-model', label: 'By model' },
    { id: 'summary-project', label: 'By project' },
    { id: 'session', label: 'Sessions' },
  ];

  function queryFor(v: ViewId) {
    switch (v) {
      case 'summary-source':
        return { View: 'summary', By: 'source' };
      case 'summary-model':
        return { View: 'summary', By: 'model' };
      case 'summary-project':
        return { View: 'summary', By: 'project' };
      case 'session':
        return { View: 'session' };
      default:
        return { View: 'daily' };
    }
  }

  $effect(() => {
    void app.range;
    void app.selectedSources;
    void app.lastScan;
    const v = view;
    loading = true;
    void (async () => {
      const res = await app.query(queryFor(v));
      rows = res.data;
      loading = false;
    })();
  });
</script>

<div class="table-screen">
  <header class="head">
    <h2>Row data</h2>
    <div class="view-switch">
      {#each VIEWS as v (v.id)}
        <button class:on={view === v.id} onclick={() => (view = v.id)}>{v.label}</button>
      {/each}
    </div>
  </header>
  {#if loading}
    <Skeleton variant="rows" />
  {:else}
    <UsageTable {rows} dense={app.density === 'compact'} />
  {/if}
</div>

<style>
  .table-screen {
    display: flex;
    flex-direction: column;
    gap: var(--gap);
    height: 100%;
    min-height: 0;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  h2 {
    font-size: 15px;
    font-weight: 600;
  }
  .view-switch {
    display: flex;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    overflow: hidden;
  }
  .view-switch button {
    background: transparent;
    border: 0;
    border-right: 1px solid var(--hairline);
    color: var(--text-muted);
    font-size: 12px;
    padding: 6px 13px;
  }
  .view-switch button:last-child {
    border-right: 0;
  }
  .view-switch button.on {
    background: color-mix(in srgb, var(--amber) 14%, transparent);
    color: var(--amber);
  }
  .table-screen :global(.table) {
    flex: 1;
    min-height: 0;
  }
</style>
