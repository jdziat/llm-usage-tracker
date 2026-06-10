<script lang="ts">
  // A tiny inline SVG sparkline — context behind a KPI. Stroke only, no fill on
  // the muted variant; the accent variant gets a faint area. Scope-trace, flat.
  let {
    values,
    width = 120,
    height = 28,
    accent = false,
  }: { values: number[]; width?: number; height?: number; accent?: boolean } = $props();

  const path = $derived.by(() => {
    if (!values.length) return '';
    const max = Math.max(...values, 1);
    const min = Math.min(...values, 0);
    const span = max - min || 1;
    const step = values.length > 1 ? width / (values.length - 1) : width;
    return values
      .map((v, i) => {
        const x = i * step;
        const y = height - ((v - min) / span) * (height - 2) - 1;
        return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  });

  const area = $derived(path ? `${path} L${width},${height} L0,${height} Z` : '');
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" preserveAspectRatio="none" aria-hidden="true">
  {#if accent && area}
    <path d={area} fill="var(--amber-fill)" />
  {/if}
  <path
    d={path}
    fill="none"
    stroke={accent ? 'var(--amber)' : 'var(--text-faint)'}
    stroke-width="1.25"
    vector-effect="non-scaling-stroke"
  />
</svg>

<style>
  svg {
    display: block;
  }
</style>
