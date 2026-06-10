# llmut CLI Expansion — Specification

This section specifies a materially more capable `llmut` CLI that preserves the project's three load-bearing commitments: **zero runtime dependencies** (Go stdlib only — verified: `go.mod` has no `require` block, no `go.sum`), **local-only** (no network; pricing is offline via `builtinPrices` in `pricing.go`), and **ccusage-compatible** (positional `[source] [view]`, `--json`, and the accepted no-op flags at `cli.go:144-170`).

Every feature below is grounded in the actual code. Crucially, the data layer **already computes** several fields that the human-readable renderers throw away — these are free features, not new aggregation:

- `Tokens.Reasoning` and `Tokens.Credits` (`types.go:20,22`) are parsed, summed (`Tokens.Add`, `types.go:30,32`), and emitted **only in CSV** (`report.go:111,273,275`). The table, pretty, and HTML renderers have a hard-coded 8-column header (`report.go:33,72,168`) that omits both.
- `Row.Start`, `Row.LastActivity`, `Row.SessionID`, `Row.Project` (`types.go:61-62,59-60`) are populated on every row by the aggregators (e.g. `aggregateSessions` at `aggregate.go:120`) but only CSV emits them (`csvRow`, `report.go:260`).
- `--config` is accepted and **silently discarded** (`cli.go:170`: `fs.String("config", "", "no-op...")`).

---

## Sequencing overview (value vs effort)

| # | Feature | Core-data work | Effort | Value |
|---|---------|----------------|--------|-------|
| 1 | `--fields` column selection | none (pure presentation) | S | High |
| 2 | Config file (`llmut.toml`) | small (Config plumbing) | S–M | High |
| 3 | Shell completions | none (pure presentation) | S | Med |
| 4 | Budget / limit alerting | small (totals comparison pass) | M | High |
| 5 | `--by` rollups (projects/sources) | small (generalize aggregator) | S | High |
| 6 | Period diff (`--compare`) | moderate (two-pass join) | M | High |
| 7 | Time-series / `--sparkline` | moderate (bucketing helper) | M | Med |
| 8 | Export presets (`--preset`) | small (presentation bundles) | S | Med |
| 9 | Interactive TUI (`llmut tui`) | presentation + dep decision | L | Med |
| 10 | `llmut serve` web dashboard | depends on `LoadAndAggregate` | M–L | High |

Items 1–8 are **stdlib-only** and reuse the existing pure aggregators (`aggregateByKey`, `aggregateSummary`, `filterEvents`), preserving the zero-dependency selling point. Item 9 requires a deliberate dependency decision (resolved below in favor of a stdlib raw-ANSI seed). Item 10 is the bridge to the Wails GUI and must be built on the `LoadAndAggregate` extraction from Discover Task A.

---

## 1. `--fields` — column selection and reordering (P0, pure presentation)

### Problem
All four human-readable renderers (`writeTable`, `writePrettyTable`, `writeHTML`, and the metrics summary) share one frozen 8-column layout: `Date | Input | Output | Cache Cr. | Cache Rd. | Total | Cost | Models` (`report.go:33`). The already-computed `Reasoning`, `Credits`, `Start`, `LastActivity`, and `Project` columns are invisible in every format except CSV.

### Specification
Add a single flag honored by `table`, `pretty`, and `html`:

```
--fields <list>     comma-separated, ordered column set
```

Field tokens (each maps to an existing `Row`/`Tokens` accessor — no new computation):

```
key  input  output  cache_creation  cache_read  cache  total  cost
reasoning  credits  models  start  last  duration  project  source
```

- `cache` is sugar for `cache_creation+cache_read` summed for display.
- `duration` is `Row.LastActivity.Sub(Row.Start)` rendered as `2h13m` (both fields already populated).
- Default when `--fields` is absent stays exactly the current 8 columns, so existing output is byte-stable.

```
llmut session --fields key,project,duration,reasoning,total,cost
llmut daily --fields key,input,output,cache,total,cost,credits --format pretty
```

Sample output (`session` view with the new columns surfacing hidden data):

```
Coding Agent Usage Report - Session - All Sources
-------------------------------------------------
Session              Project          Duration  Reasoning      Total       Cost
my-service           my-service          2h13m     412,500  3,201,994   $14.0312
docs-site            docs-site             47m      88,210    902,140    $3.1190
-----------------------------------------------------------------------------
Total                                   3h00m     500,710  4,104,134   $17.1502
```

### Implementation footprint
Touches only `report.go`. Replace the literal header slice and the per-row `[]string{...}` construction (`report.go:33-49`, `72-88`, `168`) with a column-descriptor table: a `map[string]column` where each `column` carries a header label, a width, an alignment, and a `func(Row) string` cell renderer. `reportWidths` (`report.go:337`) becomes a function of the selected column set. Zero aggregator changes; zero new parsing.

---

## 2. Config file — `llmut.toml` (P0, small core plumbing)

### Problem
`--config` is a dead no-op (`cli.go:170`). There is no way to set a default source subset, default view/format, default timezone, or persistent path overrides — every invocation re-types them. Path discovery today only honors env vars (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, etc., `paths.go:13-36`).

### Specification
A minimal TOML config (a hand-rolled stdlib parser — see "dependency note" — keeping zero deps). Only **data-concern** fields are loadable; presentation defaults (format/order/timezone/view) are also allowed because they are deterministic and safe to default.

**Discovery order (first match wins):**
1. `--config <path>` (now wired, not discarded)
2. `$LLMUT_CONFIG`
3. `$XDG_CONFIG_HOME/llmut/config.toml`
4. `~/.config/llmut/config.toml`

**Precedence (highest wins):** explicit flag > environment variable > config file > built-in default. This matches the existing default-then-flag flow in `parseArgs` (`cli.go:101-114`): the config file is loaded into the `cfg` struct **after** defaults are set but **before** `fs.Parse`, then flags override.

**Example `~/.config/llmut/config.toml`:**

```toml
# data concerns
sources       = ["claude", "codex"]
view          = "daily"
timezone      = "America/New_York"
start_of_week = "monday"
mode          = "auto"
speed         = "auto"
token_limit   = 4000000

[paths]
claude = ["~/work/.claude/projects"]
codex  = ["~/work/.codex/sessions"]

# presentation defaults
format        = "pretty"
order         = "desc"
fields        = "key,total,cost,models"

[budget]
monthly_usd = 150
weekly_usd  = 40
```

```
llmut                      # uses claude+codex, daily, pretty, NY timezone from file
llmut summary --format json  # file's format overridden by the flag
```

### Implementation footprint
New `config.go` with `type fileConfig struct {...}` and `loadConfigFile(path) (fileConfig, error)`. Wire one call into `parseArgs` between the default block (`cli.go:114`) and `fs.Parse` (`cli.go:178`), then a merge step that only writes a field if the flag was not explicitly set (detected via `fs.Visit`). The `[budget]` block feeds feature 4; `[paths]` reuses the existing `expandList` (`paths.go:52`) so `~` expansion and comma-lists work identically. This pairs directly with the Config data/presentation split from Discover Task A — the loader populates the data half.

### Dependency note
TOML is intentionally a flat, comment-tolerant subset (string, int, float, bool, and `["a","b"]` arrays, plus `[section]` headers). A ~120-line stdlib `bufio.Scanner` parser covers the schema above and honors the CONTRIBUTING zero-dependency policy. If a fuller TOML surface is ever wanted, JSON config (`config.json`) is a stdlib-native alternative selectable by file extension — `encoding/json` is already imported (`report.go:5`, `parse.go:6`).

---

## 3. Shell completions (P0, pure presentation)

### Problem
No completion code exists. The candidate sets are already enumerated as the `views` and `sources` maps (`cli.go:17-18`), and the flag set is introspectable via `fs.VisitAll`.

### Specification
A subcommand that prints a completion script to stdout:

```
llmut completion bash
llmut completion zsh
llmut completion fish
```

Generated scripts complete: the positional source (`claude codex opencode amp pi`), the positional view (`daily weekly monthly session summary blocks statusline`), all `--`/`-` flags, and enumerated flag values (`--format` -> `table pretty json csv html`; `--order` -> `asc desc`; `--mode` -> `auto calculate display`; `--speed` -> `auto standard fast`; `--by` -> `model project source`).

Install pattern documented in README:

```
llmut completion zsh > "${fpath[1]}/_llmut"
llmut completion bash | sudo tee /etc/bash_completion.d/llmut
```

### Implementation footprint
New `completion.go` with three template strings emitted via `text/template` (stdlib) or plain `fmt.Fprintf`. Source the candidate lists directly from the existing `sources`/`views` maps and a small enum-value table. The `completion` subcommand intercepts in `Run` before `parseArgs` (alongside the `help` check at `cli.go:115`). Pure stdlib.

---

## 4. Budget / limit alerting (P1, small core)

### Problem
`--token-limit` exists but only paints a percentage in the statusline (`writeStatusline`, `report.go:204-208`). There is no cost budget, no period scoping, and no actionable exit code.

### Specification

```
--budget <usd>                  alert threshold in USD over the period
--budget-period day|week|month  window the budget applies to (default: month)
--token-budget <n>              token-count threshold (parallels --budget)
--budget-exit                   exit non-zero (code 2) when over budget
```

Budgets are also loadable from the `[budget]` config block (feature 2). The check runs **after aggregation** as a comparison pass over the rows the selected view already produces — for `--budget-period month` it sums cost across the current calendar month from the same filtered events, reusing `filterEvents` (`aggregate.go:66`) with computed `Since`/`Until` bounds.

A banner prints to stderr (so it never corrupts `--format json`/`csv` on stdout); the table/pretty footer gains a budget line:

```
Coding Agent Usage Report - Daily - Claude
------------------------------------------
Date           Total        Cost
2026-06-09  1,204,330   $5.9120
2026-06-08    902,140   $3.1190
------------------------------------------
Total       2,106,470   $9.0310

Budget: $9.03 / $40.00 month  (22.6%)  ON TRACK
```

Over-budget:

```
warning: budget exceeded: $152.40 / $150.00 month (101.6%)
```

With `--budget-exit`, the process returns exit code 2 — making `llmut --budget 150 --budget-exit` a cron/CI guardrail. This is the natural hook the future GUI will surface as a desktop notification.

### Implementation footprint
New `budget.go` with `evaluateBudget(rows []Row, cfg Config) budgetStatus`. The banner writers live in `report.go`. The only `Run`-level change is mapping `budgetStatus.exceeded` to a return error/exit code. No new parsing; reuses `Tokens.CostUSD` totals already summed in `writeJSON` (`report.go:22-24`).

---

## 5. `--by` rollups — top projects/sources (P1, small core)

### Problem
`summary` rolls up **only by model** (`aggregateSummary`, `aggregate.go:34-64`). The identical map-accumulate pattern keyed on `ev.Project` or `ev.Source` would give project- and source-level rollups for free, paired with the existing `--top`.

### Specification

```
--by model|project|source     rollup dimension for the summary view (default: model)
```

`llmut summary --by project --top 5` and `llmut summary --by source` reuse `--top`, `--order`, and `--since`/`--until` unchanged.

```
llmut summary --by project --top 5 --since 2026-06-01
```

```
Coding Agent Usage Report - Summary - All Sources
-------------------------------------------------
Project            Total         Cost   Models
my-service     8,402,118    $38.7740   opus-4-8, sonnet-4-6
infra-tooling  3,109,442    $14.2010   gpt-5.3-codex
docs-site        902,140     $3.1190   opus-4-8
-------------------------------------------------
```

### Implementation footprint
Generalize `aggregateSummary` (`aggregate.go:34`) to take a key-extractor `func(Event) string` selecting `ev.Model`, `compactProject(ev.Project)`, or `sourceLabel(ev.Source)`. The sort (`sortedSummaryRows`, `aggregate.go:204`) and `--top` slice (`aggregate.go:56-62`) are untouched. `keyHeader` (`report.go:297`) gains `project`/`source` labels. Trivial generalization of an existing aggregator.

---

## 6. Period-over-period diff — `--compare` (P1, moderate core)

### Problem
There is no way to see whether spend went up or down versus a prior window. The aggregators are pure functions of `(events, cfg)`, so two passes over two windows joined on `Row.Key` produce delta columns with no new parsing.

### Specification

```
--compare previous                run the prior period of equal length
--compare 2026-05-01..2026-05-31  explicit baseline window
```

`previous` derives a baseline of equal length immediately preceding the current `--since`/`--until` (or, absent those, the prior calendar period implied by the view). Output adds delta + percent-change columns; a new `diff` render mode joins current and baseline rows on `Row.Key`:

```
llmut monthly --compare previous --since 2026-06-01
```

```
Coding Agent Usage Report - Monthly (vs previous) - All Sources
---------------------------------------------------------------
Month        Cost     Prev      Δ Cost    Δ %
2026-06   $142.30  $118.40    +$23.90  +20.2%
---------------------------------------------------------------
Total     $142.30  $118.40    +$23.90  +20.2%
```

JSON gains a `comparison` object alongside `data` so a GUI can chart it.

### Implementation footprint
New `compare.go`: `aggregateWithBaseline(events []Event, cfg Config) (current []Row, baseline []Row)` calling the existing `aggregate` twice with two `Since`/`Until` pairs, then `joinByKey(current, baseline) []DiffRow`. A `DiffRow` type and a `writeDiffTable`/`writeDiffJSON` renderer in `report.go`. Reuses 100% of `filterEvents` + `aggregateByKey`. This is a marquee feature competitors lack.

---

## 7. Time-series / `--sparkline` (P1, moderate core)

### Problem
No trend visualization exists. Buckets already exist conceptually in `weekKey`/daily key formatting (`aggregate.go:18-30`).

### Specification

```
--sparkline [cost|total]    append a unicode trend column per group (default: cost)
trend                        a new view bucketing one metric across the period
```

`llmut weekly --sparkline cost` adds a `▁▂▃▅▇`-style column showing each week's daily cost shape. `llmut trend --since 2026-06-01` renders one row of daily cost across the window:

```
llmut trend --since 2026-06-01 --sparkline cost
```

```
Coding Agent Usage Report - Trend (cost) - All Sources
------------------------------------------------------
2026-06-01 .. 2026-06-09
  ▁▂▃▂▅▇▆▄▇   min $1.12   max $9.03   total $48.71
```

Glyph mapping is a pure-stdlib min/max scale over a `[]float64` bucket series. ASCII fallback (`. : - = #`) when `--no-unicode` or a non-UTF-8 locale is detected.

### Implementation footprint
New `sparkline.go`: `bucketSeries(events []Event, cfg Config, metric string) []float64` feeding off `filterEvents`, plus `sparkline(series []float64) string`. The `trend` view dispatches in `aggregate` (`aggregate.go:12`) to a thin wrapper over `aggregateByKey` returning the daily series. Pure-stdlib glyph mapping; big perceived-polish win.

---

## 8. Export presets — `--preset` (P2, small core)

### Problem
Common export shapes (a wide CSV for spreadsheets, a self-contained HTML report) require remembering several flags. Presets bundle `--format` + `--fields` + `--breakdown` into one named token.

### Specification

```
--preset spreadsheet    csv + all token/cost/credits/reasoning columns + breakdown
--preset report         html + summary metrics + breakdown
--preset ledger         csv keyed by session with start/last/duration/project
```

```
llmut session --preset ledger -o june.csv
llmut summary --preset report -o models.html
```

Presets are pure presentation bundles; an explicit flag still overrides a preset's component (e.g. `--preset report --format pretty` forces pretty).

### Implementation footprint
A `presets map[string]presetSpec` table in `report.go` (or `config.go`), applied in `parseArgs` after flag parsing, before validation. No aggregator or parser changes.

---

## 9. Interactive TUI — `llmut tui` (P2, presentation; dependency decision required)

### Problem
The existing `--live` loop (`live.go`) re-scans **all** files every interval (`loadEvents` at `live.go:23`) and clears the whole screen (`terminalClear`, `live.go:9,33`) with no interactivity — no scrolling, no view switching, no filtering. A real TUI library (e.g. bubbletea) would break the zero-dependency CLI guarantee outright (it lands in the root `go.mod`, contaminating `go install .../cmd/llmut@latest`).

### Decision
**Hand-roll a minimal raw-ANSI TUI in stdlib**, seeded by the existing `--live` machinery. The `terminalClear` constant and the `loadEvents`→`aggregate`→`renderReport` loop already in `live.go:16-43` are the foundation; add raw-mode keystroke reading via `golang.org/x/term`'s stdlib-adjacent pattern **replaced by** a direct `syscall`-based raw-mode toggle (already importing `syscall` at `cli.go:13`) so no dependency is added.

```
llmut tui
```

Single-screen dashboard:
- Scrollable row table (the current `writeTable` output, paged).
- Hotkeys: `d`/`w`/`m`/`s` switch daily/weekly/monthly/session view; `b` blocks; `/` filters by project substring (feeds `cfg.Project`); `r` forces re-scan; `q` quits.
- A persistent footer showing budget status (feature 4) and live block remaining-time (`writeStatusline` logic, `report.go:199`).
- Incremental refresh: poll source-dir mtimes (stdlib `os.Stat`) and re-scan only on change, fixing the full-rescan-every-tick cost in `live.go:23`.

### Implementation footprint
New `tui.go` reusing `loadEvents`, `aggregate`, and the column renderers from feature 1. Raw-mode terminal handling via `syscall` termios ioctls (stdlib). If a richer TUI is ever desired, it is gated behind the **same separate-module isolation strategy** chosen for Wails (Discover Task C, Option C) so the CLI module stays dependency-free. Recommend deferring until the dependency-isolation decision lands — the Wails GUI likely supersedes much of this value.

---

## 10. `llmut serve` — local web dashboard (P2, requires `LoadAndAggregate`)

### Problem
HTML output today is a one-shot static file (`writeHTML`, `report.go:135`). There is no live, refreshable, queryable local view. This is the natural stepping-stone to the Wails GUI.

### Specification

```
llmut serve                       start a local dashboard on 127.0.0.1:8787
llmut serve --addr 127.0.0.1:9000
llmut serve --open                 launch the default browser at the URL
```

Endpoints (all served by stdlib `net/http`):
- `GET /` — the existing `writeHTML` report with an auto-refresh `<meta http-equiv="refresh">` tag, plus client-side query controls (view, source, since/until) that re-fetch the JSON endpoint.
- `GET /api/rows.json?view=daily&source=claude&since=2026-06-01` — returns the `Result` JSON (the same shape `writeJSON` emits, `report.go:13-18`), driving the page and any external tooling.
- `GET /api/health` — `{"ok":true}` for readiness.

Bound to loopback by default (local-only commitment); `--addr` may widen it explicitly. Zero new dependencies — stdlib `net/http` plus the existing HTML/JSON writers.

```
llmut serve --open
# listening on http://127.0.0.1:8787  (Ctrl-C to stop)
```

### Relationship to the Wails app — shared core, not shared frontend
**`serve` and the Wails backend must consume the same data API**, not `Run`. This depends on the `LoadAndAggregate(q Query, onProgress Progress) (Result, error)` extraction from Discover Task A and the promotion of that core out of `internal/` to an exported `pkg/core` (Discover Task C, Option C — `internal/` cannot be imported by the separate desktop module).

- **Shared backend:** both `serve`'s `/api/rows.json` handler and the Wails-bound app struct call `core.LoadAndAggregate(q, nil)` and marshal `Result`. The HTTP handler is a thin adapter that maps query params to a `core.Query`.
- **Frontend reuse is optional, not required.** `serve`'s server-rendered HTML (stdlib, zero JS deps) and the Wails frontend (Vite + vanilla TS in `desktop/frontend/`, per Discover Task C) can share the **`/api/rows.json` contract and the `Result` JSON schema** even if they do not share a single SPA bundle. Recommended path: define the `Result` JSON shape once (already exported via `Result` in the core), and let both `serve`'s page and the Wails frontend fetch/consume it identically. If a single SPA is later desired, the Wails `frontend/dist/` bundle can be embedded into the CLI via `embed.FS` and served by `serve` — but that would pull the built frontend into the CLI module's repo tree, so keep it behind a build tag or a separate `serve`-only asset path to protect the zero-dep `go install` claim.
- **Boundary that protects zero-dep:** `serve` stays in the CLI module using **only stdlib `net/http`** and the exported core; all webview/Vite/Wails machinery stays quarantined in `desktop/go.mod`. `go install .../cmd/llmut@latest` remains stdlib-only.

### Implementation footprint
New `serve.go` in the CLI module (stdlib `net/http`, `embed` optional). A `serve` subcommand intercepted in `Run` alongside `completion`/`help`. The handlers call the extracted `core.LoadAndAggregate`; this is why `serve` is sequenced **after** the API extraction, sharing exactly the data path the Wails GUI will use.

---

## Recommended delivery sequence

1. **`--fields`** — unlocks hidden Reasoning/Credits/Start/Project data; pure `report.go`.
2. **Config file** — kills the dead `--config` no-op; pairs with the data/presentation Config split.
3. **Completions** — cheap, stdlib, introspects existing `views`/`sources` maps.
4. **Budget alerting** + **`--by` rollups** — small core, high utility; budget reads the new config block.
5. **Period diff** + **sparklines** — moderate core, marquee features; reuse pure aggregators.
6. **Export presets** — trivial bundling once `--fields` exists.
7. **`serve`** — only after `LoadAndAggregate` extraction; shares the `Result` API and `/api/rows.json` contract with the Wails GUI.
8. **TUI** — defer until the dependency-isolation decision is settled; build on the stdlib `--live` seed if pursued.

Items 1–8 are achievable stdlib-only and reuse the existing pure aggregators (`aggregateByKey`, `aggregateSummary`, `filterEvents`) and the offline `builtinPrices` table, preserving the zero-dependency, local-only, ccusage-compatible character throughout. Item 7 is the deliberate bridge built on the same core API the desktop app consumes.

---

## Grounding file references
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/cli.go` — `Run`, `parseArgs`, dead `--config` (line 170), positional parsing (175-201), `views`/`sources` maps (17-18).
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/types.go` — `Tokens.Reasoning`/`Credits` (20,22), `Row.Start`/`LastActivity`/`Project` (59-62), `Config` (68).
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/report.go` — frozen 8-column header (33,72,168), CSV-only hidden fields (111,260-278), `writeStatusline` token-limit % (204-208), `keyHeader` (297), `reportWidths` (337).
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/aggregate.go` — `aggregateSummary` model-only rollup (34), `filterEvents` (66), `aggregateByKey` (91), `compactProject` (283).
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/paths.go` — env-var-only discovery (13-36), `expandList` `~` handling (52).
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/live.go` — `--live` full-rescan loop + `terminalClear` (9,23,33) — the TUI seed.
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/pricing.go` — offline `builtinPrices` (13) backing local-only cost.
- `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/parse.go` — `loadEvents` + cost pass (14-55) feeding `serve`'s shared core path.
