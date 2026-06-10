<script lang="ts">
  // uPlot trend chart — the scope-trace hero. Supports the single amber trace
  // used by Overview and named multi-source traces used by Trends.
  import uPlot from 'uplot';
  import { cssVar } from '../lib/palette';
  import { fmtCost, fmtTokens, fmtDayShort } from '../lib/format';

  type ChartLine = {
    id: string;
    label: string;
    values: number[];
    color: string;
  };

  let {
    series,
    start,
    metric = 'cost',
    height = 280,
    accent = 'amber',
    lines,
    previousLines = [],
    activeId,
  }: {
    series: number[];
    start?: string;
    metric?: 'cost' | 'tokens';
    height?: number;
    accent?: 'amber' | 'teal';
    lines?: ChartLine[];
    previousLines?: ChartLine[];
    activeId?: string | null;
  } = $props();

  let host = $state<HTMLDivElement>();
  let plot: uPlot | null = null;
  let hoverIdx = $state<number | null>(null);

  // x = day index 0..n-1 mapped to real dates for the axis labels.
  const currentLines = $derived.by<ChartLine[]>(() => {
    if (lines?.length) return lines;
    const accentColor = accent === 'teal' ? cssVar('--teal') : cssVar('--amber');
    return [{ id: 'total', label: 'Total', values: series, color: accentColor }];
  });

  const pointCount = $derived(Math.max(0, ...currentLines.map((line) => line.values.length)));

  const dates = $derived.by(() => {
    const base = start ? new Date(start) : new Date();
    return Array.from({ length: pointCount }, (_, i) => {
      const d = new Date(base);
      d.setDate(base.getDate() + i);
      return d;
    });
  });

  function fmtY(v: number): string {
    return metric === 'cost' ? fmtCost(v) : fmtTokens(v);
  }

  function gridStroke() {
    return cssVar('--grid');
  }

  function withAlpha(color: string, alpha: number): string {
    if (color.startsWith('#')) {
      const hex = color.slice(1);
      const full = hex.length === 3 ? hex.split('').map((c) => c + c).join('') : hex;
      const r = parseInt(full.slice(0, 2), 16);
      const g = parseInt(full.slice(2, 4), 16);
      const b = parseInt(full.slice(4, 6), 16);
      return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }
    const rgb = color.match(/\d+(\.\d+)?/g);
    if (rgb && rgb.length >= 3) return `rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, ${alpha})`;
    return color;
  }

  function lineAlpha(id: string, ghost = false): number {
    const base = ghost ? 0.4 : 1;
    return activeId && activeId !== id ? base * 0.28 : base;
  }

  function build() {
    if (!host) return;
    plot?.destroy();
    const xs = Array.from({ length: pointCount }, (_, i) => i);
    const accentColor = accent === 'teal' ? cssVar('--teal') : cssVar('--amber');
    const fillColor = accent === 'teal' ? cssVar('--teal-fill') : cssVar('--amber-fill');
    const axisColor = cssVar('--text-muted');
    const grid = gridStroke();
    const singleDefaultLine = !lines?.length && currentLines.length === 1;
    const plotSeries: uPlot.Series[] = [
      {},
      ...currentLines.map((line) => ({
        label: line.label,
        stroke: withAlpha(line.color, lineAlpha(line.id)),
        width: activeId === line.id || !activeId ? 1.7 : 1.1,
        fill: singleDefaultLine ? fillColor : undefined,
        points: { show: false },
        paths: uPlot.paths.linear!(),
      })),
      ...previousLines.map((line) => ({
        label: `${line.label} previous`,
        stroke: withAlpha(line.color, lineAlpha(line.id, true)),
        width: 1.1,
        dash: [6, 5],
        points: { show: false },
        paths: uPlot.paths.linear!(),
      })),
    ];
    const data = [
      xs,
      ...currentLines.map((line) => line.values),
      ...previousLines.map((line) => line.values),
    ] as uPlot.AlignedData;

    const opts: uPlot.Options = {
      width: host.clientWidth || 600,
      height,
      cursor: {
        x: true,
        y: false,
        points: { show: true, size: 7, stroke: accentColor, fill: cssVar('--canvas'), width: 2 },
      },
      legend: { show: false },
      scales: { x: { time: false }, y: { range: (_u, _min, max) => [0, max * 1.12] } },
      series: plotSeries,
      axes: [
        {
          stroke: axisColor,
          grid: { show: false },
          ticks: { show: false },
          font: '11px "IBM Plex Mono", monospace',
          gap: 6,
          values: (_u, splits) =>
            splits.map((i) => {
              const d = dates[Math.round(i)];
              return d ? fmtDayShort(d.toISOString()) : '';
            }),
        },
        {
          stroke: axisColor,
          grid: { stroke: grid, width: 1 },
          ticks: { show: false },
          font: '11px "IBM Plex Mono", monospace',
          size: 56,
          gap: 6,
          values: (_u, splits) => splits.map((v) => fmtY(v)),
        },
      ],
      hooks: {
        setCursor: [
          (u) => {
            hoverIdx = u.cursor.idx ?? null;
          },
        ],
      },
    };
    plot = new uPlot(opts, data, host);
  }

  $effect(() => {
    // re-track series + theme-changing intent
    void series;
    void lines;
    void previousLines;
    void activeId;
    build();
    const ro = new ResizeObserver(() => {
      if (plot && host) plot.setSize({ width: host.clientWidth, height });
    });
    if (host) ro.observe(host);
    return () => {
      ro.disconnect();
      plot?.destroy();
      plot = null;
    };
  });

  const hoverDate = $derived(hoverIdx != null ? dates[hoverIdx] : null);
  const hoverLine = $derived.by(() => {
    if (activeId) return currentLines.find((line) => line.id === activeId) ?? currentLines[0];
    return currentLines[0];
  });
  const hoverVal = $derived(hoverIdx != null ? hoverLine?.values[hoverIdx] : null);
</script>

<div class="chart">
  <div class="host" bind:this={host}></div>
  {#if hoverDate && hoverVal != null}
    <div class="readout mono">
      <span class="d">{fmtDayShort(hoverDate.toISOString())}</span>
      <span class="v" style:color={hoverLine?.color ?? cssVar('--amber')}>{fmtY(hoverVal)}</span>
    </div>
  {/if}
</div>

<style>
  .chart {
    position: relative;
    width: 100%;
  }
  .host {
    width: 100%;
  }
  .readout {
    position: absolute;
    top: 0;
    right: 0;
    display: flex;
    gap: 10px;
    align-items: baseline;
    padding: 4px 8px;
    background: var(--surface-raised);
    border: 1px solid var(--hairline);
    border-radius: var(--r-chip);
    font-size: 12px;
    pointer-events: none;
  }
  .readout .d {
    color: var(--text-muted);
  }
  .readout .v {
    color: var(--amber);
    font-weight: 600;
  }
</style>
