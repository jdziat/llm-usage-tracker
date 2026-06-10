<script lang="ts">
  import { onMount } from 'svelte';
  import { getUsage, listSources, type Result, type Row, type Tokens } from './api';

  let loading = $state(true);
  let error = $state('');
  let result = $state<Result | null>(null);

  const nf = new Intl.NumberFormat();
  const money = new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: 4,
  });

  onMount(async () => {
    try {
      const sources = await listSources();
      result = await getUsage({
        Sources: sources,
        View: 'summary',
        By: 'source',
        Order: 'desc',
        Mode: 'auto',
      });
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      loading = false;
    }
  });

  function totalTokens(tokens: Tokens): number {
    return (
      tokens.inputTokens +
      tokens.outputTokens +
      tokens.cacheCreationTokens +
      tokens.cacheReadTokens
    );
  }

  function models(row: Row): string {
    return row.modelsUsed?.length ? row.modelsUsed.join(', ') : 'unknown';
  }
</script>

<main>
  <header>
    <div>
      <p class="eyebrow">llmut desktop</p>
      <h1>Usage Overview</h1>
    </div>
  </header>

  {#if loading}
    <section class="state" aria-live="polite">Loading usage...</section>
  {:else if error}
    <section class="state error" role="alert">{error}</section>
  {:else if !result || result.data.length === 0}
    <section class="state">No usage data found for the configured sources.</section>
  {:else}
    <section class="totals" aria-label="Totals">
      <div>
        <span>Total Tokens</span>
        <strong>{nf.format(totalTokens(result.totals))}</strong>
      </div>
      <div>
        <span>Input</span>
        <strong>{nf.format(result.totals.inputTokens)}</strong>
      </div>
      <div>
        <span>Output</span>
        <strong>{nf.format(result.totals.outputTokens)}</strong>
      </div>
      <div>
        <span>Cost</span>
        <strong>{money.format(result.totals.costUSD)}</strong>
      </div>
    </section>

    {#if result.warnings?.length}
      <section class="warnings" aria-label="Warnings">
        {#each result.warnings as warning}
          <p>{warning}</p>
        {/each}
      </section>
    {/if}

    <section class="table-wrap" aria-label="Usage rows">
      <table>
        <thead>
          <tr>
            <th scope="col">Source</th>
            <th scope="col">Models</th>
            <th scope="col" class="num">Input</th>
            <th scope="col" class="num">Output</th>
            <th scope="col" class="num">Cache Create</th>
            <th scope="col" class="num">Cache Read</th>
            <th scope="col" class="num">Total</th>
            <th scope="col" class="num">Cost</th>
          </tr>
        </thead>
        <tbody>
          {#each result.data as row}
            <tr>
              <th scope="row">{row.key}</th>
              <td>{models(row)}</td>
              <td class="num">{nf.format(row.inputTokens)}</td>
              <td class="num">{nf.format(row.outputTokens)}</td>
              <td class="num">{nf.format(row.cacheCreationTokens)}</td>
              <td class="num">{nf.format(row.cacheReadTokens)}</td>
              <td class="num">{nf.format(totalTokens(row))}</td>
              <td class="num">{money.format(row.costUSD)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>
  {/if}
</main>
