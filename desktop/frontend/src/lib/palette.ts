// palette.ts — the source identity + token-category color/glyph lookups, and
// model-family pricing knowledge for the "unknown model" treatment. Colors are
// read from CSS custom properties so they track the active theme.

export type SourceId = 'claude' | 'codex' | 'opencode' | 'amp' | 'pi';

export const SOURCE_META: Record<string, { label: string; glyph: string; varName: string }> = {
  claude: { label: 'Claude', glyph: '◆', varName: '--src-claude' },
  codex: { label: 'Codex', glyph: '▲', varName: '--src-codex' },
  opencode: { label: 'opencode', glyph: '●', varName: '--src-opencode' },
  amp: { label: 'amp', glyph: '✦', varName: '--src-amp' },
  pi: { label: 'pi', glyph: '▸', varName: '--src-pi' },
};

export const SOURCE_ORDER: SourceId[] = ['claude', 'codex', 'opencode', 'amp', 'pi'];

/** Resolve a CSS variable to its current computed value (theme-aware). */
export function cssVar(name: string): string {
  if (typeof window === 'undefined') return '#888';
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || '#888';
}

export function sourceColor(source?: string): string {
  const meta = source ? SOURCE_META[source] : undefined;
  return meta ? cssVar(meta.varName) : cssVar('--text-muted');
}

export function sourceGlyph(source?: string): string {
  return source && SOURCE_META[source] ? SOURCE_META[source].glyph : '·';
}

export function sourceLabel(source?: string): string {
  return source && SOURCE_META[source] ? SOURCE_META[source].label : (source ?? '—');
}

/** Infer the owning source from a model family name (for summary-by-model). */
export function sourceForModel(model: string): SourceId | undefined {
  const m = model.toLowerCase();
  if (m.startsWith('claude') || m.includes('opus') || m.includes('sonnet') || m.includes('haiku') || m.includes('fable') || m.includes('mythos')) return 'claude';
  if (m.startsWith('gpt') || m.includes('codex')) return 'codex';
  if (m.startsWith('gemini')) return 'opencode';
  return undefined;
}

// Token-category viz palette (composition bars / donuts).
export type TokenCat = 'input' | 'output' | 'cacheCreate' | 'cacheRead' | 'reasoning';

export const TOKEN_CATS: { id: TokenCat; label: string; varName: string }[] = [
  { id: 'input', label: 'Input', varName: '--tok-input' },
  { id: 'output', label: 'Output', varName: '--tok-output' },
  { id: 'cacheCreate', label: 'Cache create', varName: '--tok-cache-create' },
  { id: 'cacheRead', label: 'Cache read', varName: '--tok-cache-read' },
  { id: 'reasoning', label: 'Reasoning', varName: '--tok-reasoning' },
];

export function tokenCatColor(cat: TokenCat): string {
  const meta = TOKEN_CATS.find((c) => c.id === cat);
  return meta ? cssVar(meta.varName) : cssVar('--text-muted');
}

// Offline price table (mirror of pkg/core/pricing.go) — used only to decide
// whether a model is "unknown" (unpriced) for the muted-cost treatment. The
// authoritative numbers always come from the Go core in live mode.
const PRICED_MODELS = new Set([
  'claude-fable-5', 'claude-mythos-5', 'claude-opus-4-8', 'claude-opus-4-7', 'claude-opus-4-6',
  'claude-opus-4-5', 'claude-opus-4-1', 'claude-opus-4', 'claude-sonnet-4-6', 'claude-sonnet-4-5',
  'claude-sonnet-4', 'claude-haiku-4-5', 'claude-3-7-sonnet', 'claude-3-5-sonnet', 'claude-3-5-haiku',
  'gpt-5.5', 'gpt-5.4', 'gpt-5.3-codex', 'gpt-5.3-codex-spark', 'gpt-5.2', 'gpt-5', 'gpt-4.1',
  'gpt-4.1-mini', 'gemini-3-pro-preview', 'gemini-2.5-pro', 'gemini-2.5-flash',
]);

/** Fast-tier multipliers (the invisible cost driver the Models screen surfaces). */
export const FAST_MULTIPLIER: Record<string, number> = {
  'claude-opus-4-8': 2,
  'claude-opus-4-7': 6,
  'claude-opus-4-6': 6,
};

export function isPriced(model: string): boolean {
  return PRICED_MODELS.has(model);
}
