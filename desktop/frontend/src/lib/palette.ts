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

// Offline price table (mirror of builtinPrices in pkg/core/pricing.go), USD
// per million tokens as [input, output]. The Go core is authoritative for the
// numbers a row actually shows; this drives the price provenance line and the
// unpriced/estimated treatments, including in mock mode where there is no core.
// Last verified against provider pricing pages on 2026-09-08.
export const MODEL_PRICES: Record<string, [number, number]> = {
  'claude-fable-5-1': [10, 50],
  'claude-mythos-5-1': [10, 50],
  'claude-fable-5': [10, 50],
  'claude-mythos-5': [10, 50],
  'claude-opus-5': [5, 25],
  'claude-opus-4-8': [5, 25],
  'claude-opus-4-7': [5, 25],
  'claude-opus-4-6': [5, 25],
  'claude-opus-4-5': [5, 25],
  'claude-opus-4-1': [15, 75],
  'claude-opus-4': [15, 75],
  'claude-sonnet-5': [2, 10],
  'claude-sonnet-4-6': [3, 15],
  'claude-sonnet-4-5': [3, 15],
  'claude-sonnet-4': [3, 15],
  'claude-haiku-4-5': [1, 5],
  'claude-3-7-sonnet': [3, 15],
  'claude-3-5-sonnet': [3, 15],
  'claude-3-5-haiku': [0.8, 4],
  'gpt-6-astra': [10, 50],
  'gpt-5.6-sol': [4, 20],
  'gpt-5.6-terra': [2, 12],
  'gpt-5.6-luna': [0.2, 1.2],
  'gpt-5.5': [5, 30],
  'gpt-5.4': [2.5, 15],
  'gpt-5.3-codex': [1.75, 14],
  'gpt-5.3-codex-spark': [0.25, 2],
  'gpt-5.2': [1.75, 14],
  'gpt-5': [1.25, 10],
  'gpt-4.1': [2, 8],
  'gpt-4.1-mini': [0.4, 1.6],
  'gemini-3.1-pro-preview': [2, 12],
  'gemini-3-pro-preview': [2, 12],
  'gemini-3.8-flash': [0.75, 3.75],
  'gemini-3.7-flash': [0.75, 3.75],
  'gemini-3.6-flash': [0.75, 3.75],
  'gemini-3-flash-preview': [0.5, 3],
  'gemini-2.5-pro': [1.25, 10],
  'gemini-2.5-flash': [0.3, 2.5],
};

const PRICED_MODELS = new Set(Object.keys(MODEL_PRICES));

/** Fast-tier multipliers (the invisible cost driver the Models screen surfaces). */
export const FAST_MULTIPLIER: Record<string, number> = {
  'claude-opus-5': 2,
  'claude-opus-4-8': 2,
  'claude-opus-4-7': 6,
  'claude-opus-4-6': 6,
  'gpt-6-astra': 2,
  'gpt-5.6-sol': 2,
  'gpt-5.6-terra': 2,
  'gpt-5.6-luna': 2,
  'gpt-5.5': 2.5,
  'gpt-5.4': 2,
  'gpt-5.3-codex': 2,
  'gpt-5.2': 2,
  'gpt-5': 2,
  'gpt-4.1': 1.75,
  'gpt-4.1-mini': 1.75,
};

/**
 * Family defaults, mirroring familyDefaults in pkg/core/pricing.go: a model
 * whose exact ID is not in the table yet is still priced from its family's
 * flagship rather than reported as unpriced. Every token must be present;
 * order is most specific first.
 */
const FAMILY_DEFAULTS: { tokens: string[]; key: string }[] = [
  { tokens: ['fable'], key: 'claude-fable-5-1' },
  { tokens: ['mythos'], key: 'claude-mythos-5-1' },
  { tokens: ['opus'], key: 'claude-opus-5' },
  { tokens: ['sonnet'], key: 'claude-sonnet-5' },
  { tokens: ['haiku'], key: 'claude-haiku-4-5' },
  { tokens: ['codex'], key: 'gpt-5.3-codex' },
  { tokens: ['gpt-4'], key: 'gpt-4.1' },
  { tokens: ['gpt'], key: 'gpt-6-astra' },
  { tokens: ['gemini', 'flash'], key: 'gemini-3-flash-preview' },
  { tokens: ['gemini'], key: 'gemini-3.1-pro-preview' },
];

/**
 * Alias rewrites, mirroring the normalizeModel switch in pkg/core/pricing.go.
 * Order is load-bearing: a pattern must precede any pattern that is a
 * substring of it. Without this rung a bare `gpt-6` or a `gemini-3-pro-exp`
 * would fall through to a family match here while the Go core prices it from
 * a quoted rate, and the Models screen would mark a real rate as estimated.
 */
const ALIASES: [string, string][] = [
  ['gemini-3.1-pro', 'gemini-3.1-pro-preview'],
  ['gemini-3-pro', 'gemini-3-pro-preview'],
  ['gemini-3.8-flash', 'gemini-3.8-flash'],
  ['gemini-3.7-flash', 'gemini-3.7-flash'],
  ['gemini-3.6-flash', 'gemini-3.6-flash'],
  ['gemini-3-flash', 'gemini-3-flash-preview'],
  ['gpt-6-astra', 'gpt-6-astra'],
  ['gpt-6', 'gpt-6-astra'],
  ['gpt-5.6-terra', 'gpt-5.6-terra'],
  ['gpt-5.6-luna', 'gpt-5.6-luna'],
  ['gpt-5.6', 'gpt-5.6-sol'],
  ['gpt-5.3-codex-spark', 'gpt-5.3-codex-spark'],
  ['gpt-5.3-codex', 'gpt-5.3-codex'],
  ['fable-5-1', 'claude-fable-5-1'],
  ['mythos-5-1', 'claude-mythos-5-1'],
  ['fable-5', 'claude-fable-5'],
  ['mythos-5', 'claude-mythos-5'],
  ['opus-5', 'claude-opus-5'],
  ['opus-4-8', 'claude-opus-4-8'],
  ['opus-4-7', 'claude-opus-4-7'],
  ['opus-4-6', 'claude-opus-4-6'],
  ['opus-4-5', 'claude-opus-4-5'],
  ['opus-4-1', 'claude-opus-4-1'],
  ['sonnet-5', 'claude-sonnet-5'],
  ['sonnet-4-6', 'claude-sonnet-4-6'],
  ['sonnet-4-5', 'claude-sonnet-4-5'],
  ['haiku-4-5', 'claude-haiku-4-5'],
];

export type PriceMatch = 'exact' | 'variant' | 'family' | 'none';

/**
 * How a model ID resolves against the price table, mirroring PriceMatch in
 * pkg/core/pricing.go. Rows carry the raw logged ID, so a dated or suffixed
 * variant has to resolve by containment rather than by an exact hit, and the
 * longest key wins so a two-key ID resolves the same way every time.
 */
export function resolvePrice(model: string): { key?: string; match: PriceMatch } {
  const raw = model.toLowerCase().trim().replace(/^(anthropic|openai|google|google-gla|vertex)\//, '');
  const alias = ALIASES.find(([pattern]) => raw.includes(pattern));
  const m = alias ? alias[1] : raw;
  if (PRICED_MODELS.has(m)) return { key: m, match: 'exact' };
  let best = '';
  for (const k of PRICED_MODELS) {
    if (!m.includes(k)) continue;
    if (k.length > best.length || (k.length === best.length && k < best)) best = k;
  }
  if (best) return { key: best, match: 'variant' };
  for (const f of FAMILY_DEFAULTS) {
    if (f.tokens.every((t) => m.includes(t))) return { key: f.key, match: 'family' };
  }
  return { match: 'none' };
}

/** How a model ID resolves against the price table. */
export function priceMatch(model: string): PriceMatch {
  return resolvePrice(model).match;
}

/** True when the model carries a price the Go core can compute, exact or inferred. */
export function isPriced(model: string): boolean {
  return priceMatch(model) !== 'none';
}

/** True when the price shown is inferred from the family, not a quoted rate. */
export function isApproxPriced(model: string): boolean {
  return priceMatch(model) === 'family';
}
