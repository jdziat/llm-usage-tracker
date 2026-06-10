<script lang="ts">
  // Dense, virtualized usage table — the [] Row data lens. Windowed rendering
  // (only visible rows are in the DOM) so it stays smooth at tens of thousands
  // of rows. Source color-coding, breakdown-row expansion, tabular mono
  // numerals, column selection, and the "unknown model = muted cost + tooltip"
  // treatment. Fixed row height keeps the virtualization math exact.
  import type { Row } from '../api';
  import { fmtCost, fmtTokens, fmtDateTime, totalTokens, durationMs, fmtDuration } from '../lib/format';
  import { sourceColor, sourceGlyph, isPriced, sourceForModel } from '../lib/palette';

  let { rows, dense = false }: { rows: Row[]; dense?: boolean } = $props();

  type ColId =
    | 'key'
    | 'source'
    | 'models'
    | 'input'
    | 'output'
    | 'cacheCreate'
    | 'cacheRead'
    | 'reasoning'
    | 'total'
    | 'duration'
    | 'cost';

  const ALL_COLS: { id: ColId; label: string; num: boolean }[] = [
    { id: 'key', label: 'Key', num: false },
    { id: 'source', label: 'Source', num: false },
    { id: 'models', label: 'Models', num: false },
    { id: 'input', label: 'Input', num: true },
    { id: 'output', label: 'Output', num: true },
    { id: 'cacheCreate', label: 'Cache +', num: true },
    { id: 'cacheRead', label: 'Cache →', num: true },
    { id: 'reasoning', label: 'Reason', num: true },
    { id: 'total', label: 'Total', num: true },
    { id: 'duration', label: 'Duration', num: true },
    { id: 'cost', label: 'Cost', num: true },
  ];

  let visibleCols = $state<Set<ColId>>(
    new Set<ColId>(['key', 'source', 'models', 'input', 'output', 'cacheRead', 'total', 'cost']),
  );
  let colMenuOpen = $state(false);
  let expanded = $state<Set<number>>(new Set());

  const cols = $derived(ALL_COLS.filter((c) => visibleCols.has(c.id)));

  // A single grid-template shared by the header + every row, with a leading
  // fixed track for the expander column so all cells stay aligned.
  const gridTemplate = $derived(
    '14px ' +
      cols.map((c) => (c.num ? '1fr' : c.id === 'models' ? '2.2fr' : c.id === 'key' ? '1.5fr' : '0.9fr')).join(' '),
  );

  function toggleCol(id: ColId) {
    const next = new Set(visibleCols);
    if (next.has(id)) {
      if (next.size <= 2) return;
      next.delete(id);
    } else next.add(id);
    visibleCols = next;
  }

  function toggleRow(index: number) {
    const next = new Set(expanded);
    if (next.has(index)) next.delete(index);
    else next.add(index);
    expanded = next;
  }

  function hasUnknown(r: Row): boolean {
    return (r.modelsUsed ?? []).some((m) => !isPriced(m));
  }

  const unpricedRows = $derived(rows.filter((r) => hasUnknown(r) && r.costUSD === 0));

  // ── virtualization ─────────────────────────────────────────────────────
  let scroller = $state<HTMLDivElement>();
  let scrollTop = $state(0);
  let viewportH = $state(560);
  const rowH = $derived(dense ? 26 : 34);
  const OVERSCAN = 8;

  // Flatten rows + their expanded breakdown rows into a single virtual list so
  // the windowing math covers detail rows too. Each entry carries a `uid` that
  // is unique even when row keys collide (e.g. many sessions share a project
  // name) — required for a keyed {#each} and for stable expansion state.
  type FlatRow =
    | { kind: 'row'; uid: string; row: Row; index: number }
    | { kind: 'breakdown'; uid: string; index: number; model: string; r: Row };

  const flat = $derived.by<FlatRow[]>(() => {
    const out: FlatRow[] = [];
    rows.forEach((row, index) => {
      out.push({ kind: 'row', uid: 'r' + index, row, index });
      if (expanded.has(index) && row.modelBreakdowns) {
        row.modelBreakdowns.forEach((b, bi) => {
          out.push({
            kind: 'breakdown',
            uid: 'r' + index + '-b' + bi,
            index,
            model: b.model,
            r: { ...b, key: b.model, modelsUsed: [b.model] },
          });
        });
      }
    });
    return out;
  });

  const totalH = $derived(flat.length * rowH);
  const startIdx = $derived(Math.max(0, Math.floor(scrollTop / rowH) - OVERSCAN));
  const endIdx = $derived(Math.min(flat.length, Math.ceil((scrollTop + viewportH) / rowH) + OVERSCAN));
  const window = $derived(flat.slice(startIdx, endIdx));
  const offsetY = $derived(startIdx * rowH);

  function onScroll(e: Event) {
    scrollTop = (e.target as HTMLDivElement).scrollTop;
  }

  $effect(() => {
    if (!scroller) return;
    const ro = new ResizeObserver(() => {
      if (scroller) viewportH = scroller.clientHeight;
    });
    ro.observe(scroller);
    viewportH = scroller.clientHeight;
    return () => ro.disconnect();
  });

  // When a row's models share a provider prefix (claude-opus-4-8 / -sonnet-4-6 /
  // -haiku-4-5), strip it from the chip labels so the differentiating suffix is
  // what shows — otherwise the chips truncate exactly on the part that matters.
  // The full names stay in the cell's title tooltip and the breakdown rows.
  function sharedModelPrefix(models: string[]): string {
    if (models.length < 2) return '';
    let p = models[0];
    for (const m of models.slice(1)) {
      while (p && !m.startsWith(p)) p = p.slice(0, -1);
      if (!p) return '';
    }
    const i = p.lastIndexOf('-');
    return i > 0 ? p.slice(0, i + 1) : '';
  }
  function chipLabel(m: string, prefix: string): string {
    return prefix && m.startsWith(prefix) && m.length > prefix.length ? m.slice(prefix.length) : m;
  }

  function cell(r: Row, id: ColId): { text: string; muted?: boolean; cls?: string } {
    switch (id) {
      case 'input':
        return { text: fmtTokens(r.inputTokens) };
      case 'output':
        return { text: fmtTokens(r.outputTokens) };
      case 'cacheCreate':
        return { text: fmtTokens(r.cacheCreationTokens) };
      case 'cacheRead':
        return { text: fmtTokens(r.cacheReadTokens), muted: true };
      case 'reasoning':
        return { text: r.reasoningTokens ? fmtTokens(r.reasoningTokens) : '—', muted: !r.reasoningTokens };
      case 'total':
        return { text: fmtTokens(totalTokens(r)), cls: 'strong' };
      case 'duration': {
        const ms = durationMs(r.start, r.lastActivity);
        return { text: ms ? fmtDuration(ms) : '—', muted: !ms };
      }
      case 'cost':
        return { text: fmtCost(r.costUSD) };
      default:
        return { text: '' };
    }
  }
</script>

<div class="table">
  <div class="toolbar">
    <span class="count mono">{rows.length} rows</span>
    <div class="col-picker">
      <button class="cp-btn" onclick={() => (colMenuOpen = !colMenuOpen)} aria-expanded={colMenuOpen}>
        Columns ▾
      </button>
      {#if colMenuOpen}
        <div class="cp-menu" role="menu">
          {#each ALL_COLS as c (c.id)}
            <label class="cp-item">
              <input type="checkbox" checked={visibleCols.has(c.id)} onchange={() => toggleCol(c.id)} />
              {c.label}
            </label>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  <div class="grid-head" style:grid-template-columns={gridTemplate}>
    <span class="expander-h"></span>
    {#each cols as c (c.id)}
      <span class="th" class:num={c.num}>{c.label}</span>
    {/each}
  </div>

  <div class="scroller" bind:this={scroller} onscroll={onScroll}>
    <div class="spacer" style:height="{totalH}px">
      <div class="window" style:transform="translateY({offsetY}px)">
        {#each window as item (item.uid)}
          {#if item.kind === 'row'}
            {@const r = item.row}
            {@const unknown = hasUnknown(r)}
            <div
              class="tr"
              class:expandable={r.modelBreakdowns && r.modelBreakdowns.length > 1}
              style:height="{rowH}px"
              style:grid-template-columns={gridTemplate}
              style:--src={sourceColor(r.source ?? sourceForModel(r.modelsUsed?.[0] ?? ''))}
            >
              <button
                class="expander"
                class:has={r.modelBreakdowns && r.modelBreakdowns.length > 1}
                onclick={() => toggleRow(item.index)}
                aria-label="Toggle breakdown"
              >
                {#if r.modelBreakdowns && r.modelBreakdowns.length > 1}{expanded.has(item.index) ? '▾' : '▸'}{/if}
              </button>
              {#each cols as c (c.id)}
                {#if c.id === 'key'}
                  <span class="td key"><span class="src-tick"></span>{r.key}</span>
                {:else if c.id === 'source'}
                  <span class="td src">
                    <span class="src-glyph">{sourceGlyph(r.source ?? sourceForModel(r.modelsUsed?.[0] ?? ''))}</span>
                    {r.source ?? sourceForModel(r.modelsUsed?.[0] ?? '') ?? '—'}
                  </span>
                {:else if c.id === 'models'}
                  {@const shared = sharedModelPrefix(r.modelsUsed ?? [])}
                  <span class="td models" title={r.modelsUsed?.join(', ')}>
                    {#each (r.modelsUsed ?? []).slice(0, 3) as m (m)}
                      <span class="model-chip" class:unpriced={!isPriced(m)}>{chipLabel(m, shared)}</span>
                    {/each}
                    {#if (r.modelsUsed?.length ?? 0) > 3}<span class="more">+{(r.modelsUsed?.length ?? 0) - 3}</span>{/if}
                  </span>
                {:else if c.id === 'cost'}
                  <span class="td num cost" class:unpriced={unknown && r.costUSD === 0}>
                    {fmtCost(r.costUSD)}
                    {#if unknown && r.costUSD === 0}<span class="info" title="Unknown model — tokens counted, cost unpriced">ⓘ</span>{/if}
                  </span>
                {:else}
                  {@const cv = cell(r, c.id)}
                  <span class="td num" class:muted={cv.muted} class:strong={cv.cls === 'strong'}>{cv.text}</span>
                {/if}
              {/each}
            </div>
          {:else}
            {@const r = item.r}
            <div
              class="tr breakdown"
              style:height="{rowH}px"
              style:grid-template-columns={gridTemplate}
            >
              <span class="expander"></span>
              {#each cols as c (c.id)}
                {#if c.id === 'key'}
                  <span class="td key sub">↳ {item.model}</span>
                {:else if c.id === 'source'}
                  <span class="td src"></span>
                {:else if c.id === 'models'}
                  <span class="td models"></span>
                {:else if c.id === 'cost'}
                  <span class="td num cost" class:unpriced={!isPriced(item.model)}>{fmtCost(r.costUSD)}</span>
                {:else}
                  {@const cv = cell(r, c.id)}
                  <span class="td num" class:muted={cv.muted}>{cv.text}</span>
                {/if}
              {/each}
            </div>
          {/if}
        {/each}
      </div>
    </div>
  </div>

  <div class="footer mono">
    {#if unpricedRows.length}
      <span class="unpriced-tally">
        ⓘ {unpricedRows.length} {unpricedRows.length === 1 ? 'row' : 'rows'} unpriced (unknown model — tokens counted, cost $0)
      </span>
    {/if}
  </div>
</div>

<style>
  .table {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    background: var(--surface);
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid var(--hairline);
  }
  .count {
    font-size: 12px;
    color: var(--text-muted);
  }
  .col-picker {
    position: relative;
  }
  .cp-btn {
    background: transparent;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12px;
    padding: 4px 10px;
  }
  .cp-btn:hover {
    color: var(--text);
    border-color: var(--hairline-strong);
  }
  .cp-menu {
    position: absolute;
    right: 0;
    top: calc(100% + 4px);
    z-index: 20;
    background: var(--surface-raised);
    border: 1px solid var(--hairline-strong);
    border-radius: var(--r-card);
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 150px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  }
  .cp-item {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
    padding: 4px 8px;
    border-radius: var(--r-chip);
    color: var(--text);
    cursor: pointer;
  }
  .cp-item:hover {
    background: var(--surface);
  }
  .cp-item input {
    accent-color: var(--amber);
  }

  .grid-head,
  .tr {
    display: grid;
    align-items: center;
    gap: 12px;
    padding: 0 12px;
  }
  .grid-head {
    height: 32px;
    border-bottom: 1px solid var(--hairline);
    background: var(--surface);
    position: sticky;
    top: 0;
    z-index: 2;
  }
  .expander-h,
  .expander {
    width: 14px;
    flex-shrink: 0;
  }
  .th {
    font-size: 10.5px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--text-muted);
    white-space: nowrap;
  }
  .th.num {
    text-align: right;
  }

  .scroller {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
  }
  .spacer {
    position: relative;
  }
  .window {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    will-change: transform;
  }
  .tr {
    border-bottom: 1px solid var(--hairline);
    font-size: var(--num-size);
  }
  .tr:hover {
    background: var(--surface-raised);
  }
  .tr.breakdown {
    background: color-mix(in srgb, var(--canvas) 40%, var(--surface));
  }
  .expander {
    background: transparent;
    border: 0;
    color: var(--text-faint);
    font-size: 10px;
    padding: 0;
    text-align: center;
  }
  .expander.has {
    cursor: pointer;
    color: var(--text-muted);
  }
  .expander.has:hover {
    color: var(--amber);
  }
  .td {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .td.num {
    font-family: 'IBM Plex Mono', monospace;
    font-variant-numeric: tabular-nums;
    text-align: right;
    color: var(--text);
  }
  .td.num.muted {
    color: var(--text-faint);
  }
  .td.num.strong {
    color: var(--text);
    font-weight: 600;
  }
  .td.key {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text);
    font-weight: 500;
  }
  .td.key.sub {
    color: var(--text-muted);
    font-family: 'IBM Plex Mono', monospace;
    font-size: 12px;
    font-weight: 400;
  }
  .src-tick {
    width: 3px;
    height: 14px;
    border-radius: 1px;
    background: var(--src, var(--hairline-strong));
    flex-shrink: 0;
  }
  .td.src {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--text-muted);
  }
  .src-glyph {
    color: var(--src, var(--text-muted));
  }
  .td.models {
    display: flex;
    gap: 5px;
    align-items: center;
    overflow: hidden;
  }
  .model-chip {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 11px;
    padding: 1px 6px;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
    flex-shrink: 0;
  }
  .model-chip.unpriced {
    border-style: dashed;
    color: var(--text-faint);
  }
  .more {
    font-size: 11px;
    color: var(--text-faint);
  }
  .cost {
    font-weight: 600;
  }
  .cost.unpriced {
    color: var(--text-faint);
    font-weight: 400;
  }
  .info {
    color: var(--caution);
    margin-left: 4px;
    cursor: help;
  }
  .footer {
    padding: 8px 12px;
    border-top: 1px solid var(--hairline);
    font-size: 11.5px;
    color: var(--text-faint);
    min-height: 14px;
  }
  .unpriced-tally {
    color: var(--caution);
  }
</style>
