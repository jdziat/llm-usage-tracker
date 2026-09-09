# Cost estimation

llmut estimates USD cost from token counts using a built-in, offline price table
(`pkg/core/pricing.go`). No network calls are made; prices are compiled into
the binary and were last verified against provider pricing pages on **2026-09-08**.

Everything on this page is an *estimate*. See [Accuracy caveats](#accuracy-caveats)
before treating the numbers as a bill.

## Cost formula

Each usage event carries four token counts: input, output, cache write
(cache creation), and cache read. Cost is the sum of each count times its
per-million-token (MTok) price:

```
cost = (input      × price.input
      + output     × price.output
      + cacheWrite × price.cacheWrite
      + cacheRead  × price.cacheRead) / 1,000,000  × fastMultiplier
```

If a model's `cacheWrite` or `cacheRead` price is unset (zero) in the table, it
falls back to the **input** price. In the current table this applies to the
cache-write price of every Gemini entry and of the OpenAI entries outside the
GPT-5.6 and GPT-6 families, which only define an explicit cache-read rate.

`fastMultiplier` is 1 unless fast mode applies — see [Fast mode](#fast-mode---speed).

Worked example (`claude-opus-4-8`, standard speed):

| Category | Tokens | Price/MTok | Cost |
|---|---:|---:|---:|
| Input | 1,200,000 | $5.00 | $6.00 |
| Output | 350,000 | $25.00 | $8.75 |
| Cache write | 2,400,000 | $6.25 | $15.00 |
| Cache read | 18,000,000 | $0.50 | $9.00 |
| **Total** | | | **$38.75** |

## Built-in price table

USD per million tokens. Source of truth: `builtinPrices` in
`pkg/core/pricing.go`.

| Model | Input | Output | Cache write | Cache read |
|---|---:|---:|---:|---:|
| claude-fable-5-1 | 10.00 | 50.00 | 12.50 | 0.25 |
| claude-mythos-5-1 | 10.00 | 50.00 | 12.50 | 0.25 |
| claude-fable-5 | 10.00 | 50.00 | 12.50 | 1.00 |
| claude-mythos-5 | 10.00 | 50.00 | 12.50 | 1.00 |
| claude-opus-5 | 5.00 | 25.00 | 6.25 | 0.50 |
| claude-opus-4-8 | 5.00 | 25.00 | 6.25 | 0.50 |
| claude-opus-4-7 | 5.00 | 25.00 | 6.25 | 0.50 |
| claude-opus-4-6 | 5.00 | 25.00 | 6.25 | 0.50 |
| claude-opus-4-5 | 5.00 | 25.00 | 6.25 | 0.50 |
| claude-opus-4-1 | 15.00 | 75.00 | 18.75 | 1.50 |
| claude-opus-4 | 15.00 | 75.00 | 18.75 | 1.50 |
| claude-sonnet-5 | 2.00 | 10.00 | 2.50 | 0.20 |
| claude-sonnet-4-6 | 3.00 | 15.00 | 3.75 | 0.30 |
| claude-sonnet-4-5 | 3.00 | 15.00 | 3.75 | 0.30 |
| claude-sonnet-4 | 3.00 | 15.00 | 3.75 | 0.30 |
| claude-haiku-4-5 | 1.00 | 5.00 | 1.25 | 0.10 |
| claude-3-7-sonnet | 3.00 | 15.00 | 3.75 | 0.30 |
| claude-3-5-sonnet | 3.00 | 15.00 | 3.75 | 0.30 |
| claude-3-5-haiku | 0.80 | 4.00 | 1.00 | 0.08 |
| gpt-6-astra | 10.00 | 50.00 | 12.50 | 1.00 |
| gpt-5.6-sol | 4.00 | 20.00 | 5.00 | 0.40 |
| gpt-5.6-terra | 2.00 | 12.00 | 2.50 | 0.20 |
| gpt-5.6-luna | 0.20 | 1.20 | 0.25 | 0.02 |
| gpt-5.5 | 5.00 | 30.00 | (= input) | 0.50 |
| gpt-5.4 | 2.50 | 15.00 | (= input) | 0.25 |
| gpt-5.3-codex | 1.75 | 14.00 | (= input) | 0.175 |
| gpt-5.3-codex-spark | 0.25 | 2.00 | (= input) | 0.025 |
| gpt-5.2 | 1.75 | 14.00 | (= input) | 0.175 |
| gpt-5 | 1.25 | 10.00 | (= input) | 0.125 |
| gpt-4.1 | 2.00 | 8.00 | (= input) | 0.50 |
| gpt-4.1-mini | 0.40 | 1.60 | (= input) | 0.10 |
| gemini-3.1-pro-preview | 2.00 | 12.00 | (= input) | 0.20 |
| gemini-3-pro-preview | 2.00 | 12.00 | (= input) | 0.20 |
| gemini-3.8-flash | 0.75 | 3.75 | (= input) | 0.075 |
| gemini-3.7-flash | 0.75 | 3.75 | (= input) | 0.075 |
| gemini-3.6-flash | 0.75 | 3.75 | (= input) | 0.075 |
| gemini-3-flash-preview | 0.50 | 3.00 | (= input) | 0.05 |
| gemini-2.5-pro | 1.25 | 10.00 | (= input) | 0.125 |
| gemini-2.5-flash | 0.30 | 2.50 | (= input) | 0.03 |

Prices are short-context, standard-tier list rates. GPT-5.6 Sol is on
promotional pricing that OpenAI commits to hold at least through
2026-11-21. Cache reads on Claude Fable 5.1 and Mythos 5.1 are 0.025x base
input rather than the usual 0.1x.

`(= input)` means the entry does not set a cache-write price, so the input
price is used per the fallback rule above.

## Model normalization

Logs record model IDs in many shapes: provider-prefixed, dated, or suffixed.
`normalizeModel` and `lookupPrice` resolve them in four steps, most specific first:

1. **Lowercase, trim, strip provider prefixes** — `anthropic/`, `openai/`,
   `google/`, `google-gla/`, `vertex/`.
2. **Contains-based variant mapping** — if the result contains a known family
   substring it maps to the canonical key. Examples:
   - `anthropic/claude-opus-4-8` → `claude-opus-4-8`
   - `claude-fable-5[1m]` → `claude-fable-5`
   - `claude-sonnet-4-5-20250929` → `claude-sonnet-4-5`
   - anything containing `gemini-3-pro` → `gemini-3-pro-preview`
   - a bare `gpt-6` → `gpt-6-astra`, a bare `gpt-5.6` → `gpt-5.6-sol`

   Case order is load-bearing: a case must precede any case whose pattern is a
   substring of it, or the shorter pattern claims the longer ID (`fable-5-1`
   before `fable-5`, `gpt-5.3-codex-spark` before `gpt-5.3-codex`).
3. **Longest-key containment at lookup time** — if the normalized ID is not an
   exact key in `builtinPrices`, `lookupPrice` scans the table for keys the
   normalized ID *contains* and takes the **longest** one (so a dated
   `gpt-4.1-2025-04-14` matches `gpt-4.1`, and a `claude-fable-5-1-preview`
   matches `claude-fable-5-1`, not `claude-fable-5`). Longest wins because Go
   map iteration is randomized: taking the first hit would price a two-key ID
   differently from run to run.
4. **Family default** — if still nothing matches, `familyDefaults` prices the
   model from its family's current flagship (`opus` → `claude-opus-5`,
   `gemini` + `flash` → `gemini-3-flash-preview`, and so on). This exists
   because a tier that ships before the table is updated otherwise reports
   $0 and silently understates a whole month: `claude-opus-5` did exactly that,
   halving reported spend until it was added.

`PriceMatch(model)` reports which rung answered: `exact`, `variant`,
`family`, or `none`. A `family` result is an estimate of the right order of
magnitude, not a rate the provider published for that ID; the desktop Models
screen marks those with `≈`.

If nothing matches at all — a model from a provider the table does not cover —
the model is unknown: its **tokens are still counted and reported, but its cost
is $0.0000**. If you see a model with significant tokens and zero cost, it is
missing from the table and from `familyDefaults` — see
[Updating prices](#updating-prices).

## Fast mode (`--speed`)

Some models offer a fast tier that raises the base input/output rate: Anthropic
fast mode on the Opus family, and OpenAI fast mode (the tier renamed from
priority processing on 2026-07-30, which Codex selects with `service_tier`).
Cache multipliers apply on top of that fast base, so llmut models fast mode as
a uniform multiplier across all four token categories (`fastMultipliers` in
`pkg/core/pricing.go`):

| Model | Fast multiplier | Rationale |
|---|---:|---|
| claude-opus-5 | 2x | $10/$50 fast vs $5/$25 standard |
| claude-opus-4-8 | 2x | $10/$50 fast vs $5/$25 standard |
| claude-opus-4-7 | 6x | $30/$150 fast vs $5/$25 standard |
| claude-opus-4-6 | 6x | $30/$150 fast vs $5/$25 standard |
| gpt-6-astra | 2x | $20/$100 fast vs $10/$50 standard |
| gpt-5.6-sol | 2x | $8/$40 fast vs $4/$20 standard |
| gpt-5.6-terra | 2x | $4/$24 fast vs $2/$12 standard |
| gpt-5.6-luna | 2x | $0.40/$2.40 fast vs $0.20/$1.20 standard |
| gpt-5.5 | 2.5x | $12.50/$75 fast vs $5/$30 standard |
| gpt-5.4 | 2x | $5/$30 fast vs $2.50/$15 standard |
| gpt-5.3-codex | 2x | $3.50/$28 fast vs $1.75/$14 standard |
| gpt-5.2 | 2x | $3.50/$28 fast vs $1.75/$14 standard |
| gpt-5 | 2x | $2.50/$20 fast vs $1.25/$10 standard |
| gpt-4.1 | 1.75x | $3.50/$14 fast vs $2/$8 standard |
| gpt-4.1-mini | 1.75x | $0.70/$2.80 fast vs $0.40/$1.60 standard |

Models not in this table always get 1x, even with `--speed fast`. GPT-5.3 Codex
Spark has no published fast rate, so it is one of them.

The `--speed` flag controls whether the multiplier applies:

- `--speed standard` — never apply fast multipliers.
- `--speed fast` — apply fast multipliers to the models above.
- `--speed auto` (default) — detect from the Codex CLI config: llmut reads
  `config.toml` next to each *default* Codex sessions root (`CODEX_HOME` if
  set, otherwise `~/.codex` — and only when its `sessions` directory actually
  exists) and resolves to `fast` when it finds `service_tier = "priority"` or
  `service_tier = "fast"`, otherwise `standard`. A `--codex-path` override is
  not consulted for detection.

Any other value is rejected: `--speed must be auto, standard, or fast`.

## `--mode`: trust recorded costs or recompute

Some logs (notably Claude Code's) record a cost alongside the token counts.
`--mode` controls whether llmut trusts that recorded value or recomputes from
the price table. The exact rule, from `loadEvents` in
`pkg/core/parse.go`: an event's cost is recomputed when

```
recordedCost == 0  ||  mode == "calculate"  ||  (mode == "auto" && the raw log record carried no nonzero cost)
```

Which gives this behavior:

| Mode | Log has a nonzero recorded cost | Cost missing or zero in log |
|---|---|---|
| `auto` (default) | recorded cost is kept | recomputed from the table |
| `calculate` | always recomputed | recomputed |
| `display` | recorded cost is kept | recomputed |

Notes:

- Even in `display` mode, events with no recorded cost are filled in by
  calculation — a source that never logs costs (e.g. Codex) never shows $0 just
  because the field was absent.
- `auto` and `display` currently behave the same, because "the raw record
  carried a nonzero cost" is exactly how the recorded-cost flag is set at parse
  time. `auto` keys off the raw record, `display` off the stored value; today
  those coincide.
- Invalid values are rejected; only `auto`, `calculate`, and `display` are
  accepted.

Use `--mode calculate` to get a consistent apples-to-apples estimate across all
sources from the built-in table; use `--mode display` when you want whatever
the agent itself logged.

## Updating prices

Prices change. To update or add a model:

1. Edit `builtinPrices` in `pkg/core/pricing.go`. Prices are USD per
   million tokens; omit `cacheWrite`/`cacheRead` only if falling back to the
   input price is correct for that provider.
2. If dated or suffixed variants of the model ID exist in the wild
   (e.g. `claude-sonnet-4-5-20250929`, `claude-fable-5[1m]`), add a matching
   `normalizeModel` case so they collapse to your new key. The comment above
   `builtinPrices` reminds you of this. Case order is load-bearing: put the
   longer pattern first.
3. If the model opens a new family, add a `familyDefaults` entry so the next
   tier in that family is estimated rather than dropped to $0. If it renames an
   existing family's flagship, repoint that entry.
4. Mirror the change in `desktop/frontend/src/lib/palette.ts`, which carries
   `MODEL_PRICES`, `FAST_MULTIPLIER`, `ALIASES` and `FAMILY_DEFAULTS` for the
   desktop provenance line and its unpriced/estimated treatments. The
   `TestDesktopMirrors*` tests in `pkg/core/pricing_mirror_test.go` fail when
   the two tables drift, so a server-only edit does not pass tests.
5. Update the "Last verified" date in the `builtinPrices` comment, at the top
   of this page, and in the `MODEL_PRICES` comment. The convention is the date
   you checked the provider pages, not the date of the edit.

Authoritative sources:

- Anthropic: <https://platform.claude.com/docs/en/about-claude/pricing>
- OpenAI: <https://developers.openai.com/api/docs/pricing>
- Google Gemini: <https://ai.google.dev/gemini-api/docs/pricing>

## Accuracy caveats

- **These are estimates**, computed offline from a static table. They will
  drift whenever providers change prices between releases of llmut.
- **Subscription plans do not bill per token.** If you use Claude Code on a
  Claude Pro/Max plan, or Codex through a ChatGPT plan, the dollar figures show
  what the same usage *would* cost at API list rates — useful for comparing
  days, models, or agents, not for predicting an invoice.
- **Batch discounts and long-context premiums are not modeled.** Providers
  charge less for batch workloads and more for very large prompts (e.g.
  above-200K-token tiers); llmut applies the single base rate per model.
- **Unknown models cost $0.0000**, which silently understates totals until the
  model is added to the table. A model whose family is recognized is estimated
  from that family's flagship instead, which is an order-of-magnitude guess:
  `PriceMatch` returns `family` for those.
- Fast mode is approximated as a flat multiplier; `--speed auto` only inspects
  the Codex CLI config, so Claude fast-mode usage needs an explicit
  `--speed fast` to be priced as fast.
