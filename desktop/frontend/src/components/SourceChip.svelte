<script lang="ts">
  // A source toggle in the left rail: glyph + label in the source's signature
  // color, lit when selected, dimmed when excluded from the loaded set.
  import { SOURCE_META, cssVar } from '../lib/palette';

  let {
    id,
    selected,
    onToggle,
  }: { id: string; selected: boolean; onToggle: (id: string) => void } = $props();

  const meta = $derived(SOURCE_META[id]);
  const color = $derived(meta ? cssVar(meta.varName) : cssVar('--text-muted'));
</script>

<button
  class="chip"
  class:on={selected}
  style:--c={color}
  onclick={() => onToggle(id)}
  aria-pressed={selected}
  title={selected ? `Hide ${meta?.label}` : `Show ${meta?.label}`}
>
  <span class="glyph" aria-hidden="true">{meta?.glyph ?? '·'}</span>
  <span class="label">{meta?.label ?? id}</span>
  <span class="state" aria-hidden="true"></span>
</button>

<style>
  .chip {
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    padding: 7px 10px;
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    background: transparent;
    color: var(--text-muted);
    font-size: 13px;
    text-align: left;
    transition:
      border-color 0.12s,
      color 0.12s,
      background 0.12s;
  }
  .chip:hover {
    background: var(--surface-raised);
  }
  .chip.on {
    color: var(--text);
    border-color: color-mix(in srgb, var(--c) 45%, var(--hairline));
    background: color-mix(in srgb, var(--c) 8%, transparent);
  }
  .glyph {
    color: var(--c);
    font-size: 13px;
    width: 14px;
    text-align: center;
    opacity: 0.45;
  }
  .chip.on .glyph {
    opacity: 1;
  }
  .label {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .state {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--hairline-strong);
  }
  .chip.on .state {
    background: var(--c);
  }
</style>
