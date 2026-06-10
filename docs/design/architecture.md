# Technical Architecture — llmut Desktop (Wails) + Core API Extraction

**Status:** Proposal · **Audience:** skeptical CTO · **Grounded in:** repo @ commit `7540ae6`, all of `internal/usage/*.go`, the 7 test files, `go.mod`, `.goreleaser.yaml`, `.github/workflows/*`, `CONTRIBUTING.md`.

This document makes seven decisions and defends each against cost, risk, and maintainability. The throughline: **the zero-dependency CLI is the product's stated selling point and its own CONTRIBUTING contract — every decision here is constrained to keep `go install github.com/jdziat/llm-usage-tracker/cmd/llmut@latest` resolving to stdlib only.**

---

## Decision 1 — Wails v3 (not v2)

**Pick: Wails v3 (alpha).**

| Factor | v2 (stable) | v3 (alpha) | Verdict |
|---|---|---|---|
| Stability | Battle-tested | Alpha, API "reasonably stable" June 2026 | v2 wins, but see risk math |
| Multi-window | Single-window centric | Native multi-window | v3 — we want a detached "live statusline" mini-window + main dashboard |
| Build system | Hidden/magic `wails build` | Taskfile-orchestrated, transparent | v3 — auditable, fits a project that prizes build hygiene |
| App API | Global, implicit | Procedural (`application.New`) | v3 — cleaner to wrap one Go service struct |
| Binding model | Reflection over bound structs | Same model, improved codegen | Parity |

**Justification.** The alpha risk is real but **bounded by our architecture, not by Wails**: per Decision 2 the entire Wails surface lives in a *separate module* that no CLI user ever compiles. A v3 breaking change touches `desktop/` only — it cannot regress the CLI, cannot touch `go install` of the binary, and cannot break a single existing test. The blast radius of "alpha" is one directory. Against that capped downside, v3 buys the two features this product actually needs: a **detached always-on-top statusline window** (the GUI analogue of `llmut statusline`, the app's most-used surface) and a **transparent Taskfile build** we can read and pin. Choosing v2 to dodge alpha risk would forfeit multi-window for a risk we've already isolated. If v3 alpha proves unworkable mid-build, the fallback is mechanical: the backend is plain `core.LoadAndAggregate` calls behind a thin `app.go`, so dropping to v2 is a rewrite of `main.go` + binding glue only, ~1 day, with zero change to `pkg/core` or the CLI.

**Pin policy:** `desktop/go.mod` pins an exact Wails v3 commit/tag (not `@latest`); upgrades are deliberate PRs gated by `desktop-ci.yml`.

---

## Decision 2 — Repo restructure: multi-module + `go.work` (dev-only)

**Pick: two modules in one repo, joined by a gitignored `go.work`. Reject build tags. Reject same-module second `cmd/`.**

**Why the alternatives fail (decisively):** Build tags (`//go:build wails`) gate *compilation, not the dependency graph*. The instant Wails appears in any `require`, it lands in `go.mod`/`go.sum` for **every** consumer, and `go install .../cmd/llmut@latest` downloads webview2/webkit bindings even though the tag excludes the files from the build. Same fatal flaw for a second `cmd/llmut-gui/` in the existing module: one `go.mod` ⇒ Wails is a require of the whole module ⇒ `go install` of *any* `cmd/` pulls the full graph. Both break the README install promise and the CONTRIBUTING zero-dep policy. Only a **separate module** quarantines the dependency graph.

**Hard blocker this forces:** the core today lives in `internal/usage`, and Go's `internal/` rule forbids a second module from importing it. **The data core must be promoted out of `internal/`.**

### Directory tree

```
llm-usage-tracker/                         module 1 — STAYS ZERO-DEP
├── go.mod                                 # github.com/jdziat/llm-usage-tracker — no require block
├── go.sum                                 # MUST stay empty/absent (CI-asserted)
├── cmd/llmut/main.go                      # CLI entry, stdlib only
├── pkg/core/                              # NEW exported data API (stdlib only)
│   ├── query.go                           # Query, Result, Warning, Progress, LoadAndAggregate
│   ├── types.go                           # Tokens, Event, Row, ModelBreakdown (moved verbatim)
│   ├── parse.go  paths.go  pricing.go     # moved verbatim (cfg→Query rename only)
│   ├── aggregate.go  jsonutil.go          # moved verbatim
│   └── *_test.go                          # moved with the code they test
├── internal/usage/                        # CLI-PRIVATE render + arg-parse (stdlib only)
│   ├── cli.go                             # Run, parseArgs (→ Query+RenderOptions), renderReport
│   ├── report.go  live.go  progress.go    # presentation; progress becomes a callback sink
│   └── *_test.go
├── desktop/                               module 2 — OWNS ALL WAILS DEPS
│   ├── go.mod                             # github.com/jdziat/llm-usage-tracker/desktop
│   ├── go.sum                             # webview2/webkit + build tooling quarantined HERE
│   ├── main.go                            # application.New(...) (Wails v3)
│   ├── app.go                             # UsageService wrapping core.LoadAndAggregate
│   ├── Taskfile.yml  wails.json
│   └── frontend/                          # Vite + Svelte (Decision 4)
├── go.work                                # GITIGNORED — use ./ ./desktop, dev convenience
├── go.work.sum                            # GITIGNORED
└── .goreleaser.yaml  .github/workflows/
```

### go.mod layout

```
// ./go.mod                                  // UNCHANGED — the contract
module github.com/jdziat/llm-usage-tracker
go 1.25
// (no require block, ever)

// ./desktop/go.mod
module github.com/jdziat/llm-usage-tracker/desktop
go 1.25
require (
    github.com/jdziat/llm-usage-tracker v0.x   // resolves to pkg/core
    github.com/wailsapp/wails/v3 vX.Y.Z         // pinned exact
)
```

**Why this wins:** `pkg/core` is exported, so `desktop/` legally imports it. Wails + native deps live only in `desktop/go.sum`. Root `go.mod` stays empty of requires; `go install .../cmd/llmut@latest` resolves module 1 alone. The README claim and CONTRIBUTING policy survive **verbatim**.

**The `go.work` trap — handled explicitly:** `go.work` is local-dev only and **gitignored**. Committing it would make root `go build ./...` pull desktop deps in some toolchain configs, re-contaminating the graph. CI for module 1 runs with **`GOWORK=off`** plus a hard assertion (Decision 5) that fails the build if a require ever appears. `.gitignore` gains: `/go.work`, `/go.work.sum`, `desktop/build/bin/`, `desktop/frontend/node_modules/`, `desktop/frontend/dist/`.

**Accepted trade-off:** the desktop module references a *published* core version for clean from-source builds, or the workspace/`replace` locally. Fine — end users build `desktop/` from source via its Taskfile; they never `go install` it.

---

## Decision 3 — Core API extraction (CLI behavior + all tests stay green)

**Pick:** split `Config` into a **data** struct (`Query`) and a **presentation** struct (`RenderOptions`); extract one pure function `LoadAndAggregate`; make `Run` a thin orchestrator; collapse `runLive` onto the new core.

### Concrete signatures (in `pkg/core`)

```go
package core

type Query struct {
    Sources       []string
    SourcePaths   map[string][]string
    Since, Until  time.Time
    Location      *time.Location
    View          string         // daily|weekly|monthly|session|summary|blocks
    Project, ID   string
    Order         string         // asc|desc
    Top           int
    StartOfWeek   string          // monday|sunday
    Mode          string          // auto|calculate|display
    Speed         string          // auto|standard|fast
    Instances     bool
    Active, Recent bool
    SessionLength time.Duration
}

type Warning struct{ Source, Message string }
type Progress func(stage string)            // nil in headless/Wails/test contexts

type Result struct {
    View     string
    Rows     []Row
    Totals   Tokens                          // computed once; kills the dup sum loops
    Warnings []Warning
}

// The single reusable entry point. Zero io.Writer, zero formatting.
func LoadAndAggregate(q Query, onProgress Progress) (Result, error)
```

`LoadAndAggregate` body = today's `loadEvents` loop + the cost-recompute pass (parse.go:47–51, conditions carried **intact**) + `aggregate`, with every `cfg.progress.Set/Done` replaced by `if onProgress != nil { onProgress(stage) }`. It sums `Totals` once — deleting the ad-hoc loops in `writeJSON` (report.go:22) and `writeHTML` (report.go:137).

### Presentation stays CLI-local (`internal/usage`)

```go
type RenderOptions struct {
    Format     string  // table|pretty|json|csv|html
    OutputPath string
    Breakdown  bool
    Compact    bool
    TokenLimit int64
    Debug      bool
}
func Render(w io.Writer, res core.Result, q core.Query, opts RenderOptions) error
```

`Render` is the existing `renderReport`/`writeTable`/`writePrettyTable`/`writeJSON`/`writeCSV`/`writeHTML`/`writeStatusline` dispatch, re-typed to take `Result` instead of `([]Row, []string)`. Render reads `q.View`/`q.StartOfWeek`/`q.SessionLength` for `keyHeader`/`title`/statusline labels; `writeStatusline` reads `q.SessionLength` (data) + `opts.TokenLimit` (presentation) — both available, no coupling.

`Run` collapses to:
```go
func Run(args []string, stdout, stderr io.Writer) error {
    q, opts, live, err := parseArgs(args)           // now returns the two structs + live flag
    // help, statusline-rewrite, blocks→claude resolution all happen INSIDE parseArgs
    if live { return runLive(q, opts, stdout, stderr, stop) }
    var onProgress core.Progress
    if opts.progressEnabled && writerIsTerminal(stderr) { /* wire spinner */ }
    res, err := core.LoadAndAggregate(q, onProgress)
    for _, w := range res.Warnings { if opts.Debug { fmt.Fprintln(stderr, "warning:", w.Message) } }
    return Render(out, res, q, opts)
}
```
`runLive` becomes a loop over `LoadAndAggregate` + `Render`, **deleting the duplicated load/aggregate/render pipeline** that currently lives in both `Run` and `live.go:23-34`.

### Migration that keeps tests green — the load-bearing detail

The 7 test files pin exact call shapes I must preserve or update in lockstep:
- `aggregate_test.go` / `parse_test.go` call **`aggregate(events, Config{View:..., Sources:..., Location:..., Order:..., Top:...})`** with positional `Config` literals.
- `cli_test.go` calls **`parseArgs(args)`** and reads **`cfg.Since`**.
- `report_test.go` calls **`writeCSV(&buf, Config{}, rows)`** and **`writeHTML(..., Config{View:"daily"}, rows, []string{...})`**.
- `live_test.go` constructs a full **`Config{...}`** and calls **`runLive(cfg, ...)`**.

**Strategy — three-step, each independently green (`go test ./...` passes after every step):**

1. **Move data files to `pkg/core` with the field set unchanged.** Define `type Query = ...` and keep aggregators/readers taking the data struct. Because all data fields the tests set (`View, Sources, Location, Order, Top, Mode, SessionLength, Speed, StartOfWeek`) are exactly the DATA subset, the test `Config{...}` literals are mechanically rewritten to `core.Query{...}` **with identical field names** — a `gofmt`-clean rename, no semantic change. The aggregate/parse tests move alongside the code into `pkg/core` and assert on the same `Row` values.
2. **Introduce `LoadAndAggregate` and `Render`; rewrite `Run`/`runLive` to call them.** `report_test.go` and `live_test.go` stay in `internal/usage`; their `Config{}` literals become `core.Query{}` + `RenderOptions{}` pairs. The CSV/HTML byte output is **unchanged** because `csvRow`/`writeHTMLRow`/the column set are untouched — only the function's parameter types change.
3. **Delete the old `Config` god-struct and the unexported `progress` field.** Confirm `staticcheck`/`go vet` clean, confirm `gofmt -l` empty.

### Two behavior-preservation watch-items (called out so they don't silently regress)

- **`--format json` warnings field changes type** `[]string` → `[]core.Warning`. To keep `--format json` byte-stable for existing scripts, **marshal `Warning` as its `Message` string** via a custom `MarshalJSON` (emits the same `"source: message"` form `loadEvents` builds today at parse.go:36). `Result` carries the same JSON tags as the old `jsonReport` (`view`/`data`/`totals`/`warnings`), so the JSON schema is preserved exactly.
- **The `Debug`-gated `continue`** in `readClaude`/`readCodex` (parse.go:114,180) that tolerates malformed files must carry into `pkg/core`. It moves behind a `q`-adjacent flag (e.g. `Query.Tolerant bool`, set from `--debug` by `parseArgs`) so the GUI can opt in too. Malformed-file behavior unchanged.

**Net:** ~100% of `aggregate.go`, `pricing.go`, `paths.go`, `jsonutil.go`, and the readers reuse verbatim (only the param *type* renames `Config`→`Query`). The `Row`/`Tokens`/`Event` types already carry JSON tags and **become the Wails-marshaled DTOs as-is**.

---

## Decision 4 — Frontend: Svelte 5 + Vite + uPlot, over Wails v3 bindings + events

**Pick: Svelte 5 (runes) + Vite + TypeScript, charts via uPlot, tables hand-rolled (virtualized).** Reject React (heavier runtime, more ceremony for a single-author data tool), reject SolidJS (smaller ecosystem, marginal gain over Svelte 5 runes), reject a chart mega-lib (Chart.js/ECharts) for the primary time-series.

**Rationale.**
- **Svelte 5** compiles to minimal JS with no virtual-DOM runtime — the dashboard ships small and starts fast inside the webview, which matters because Wails embeds the frontend in the binary. Runes give fine-grained reactivity ideal for a live-updating numbers grid without a state-management library.
- **uPlot** for the cost/token time-series: it renders tens of thousands of points at 60fps in a few KB, which is exactly the "data-dense" requirement and far lighter than ECharts/Chart.js. It pairs with the planned CLI **sparkline/trend** feature — same bucketed series, two renderers.
- **Tables hand-rolled + virtualized.** The dashboard's core is the same `[]Row` the CLI renders; a custom Svelte table with column selection (the GUI twin of the planned `--fields` flag) and row virtualization avoids a grid dependency and keeps full control over the 8+ columns including the currently-hidden `Reasoning`/`Credits`/`Start`/`LastActivity`.
- **Vite** is the Wails v3 default scaffold; HMR during `wails3 dev`.

**How it talks to Go.** `app.go` exposes a bound service whose methods take/return the **already-JSON-tagged DTOs**:
```go
type UsageService struct{}
func (s *UsageService) GetUsage(q core.Query) (core.Result, error) {
    return core.LoadAndAggregate(q, func(stage string) {
        application.Get().EmitEvent("scan:progress", stage)  // Wails v3 event
    })
}
func (s *UsageService) ListSources() []string { return core.AllSources() }
```
Wails v3 codegen produces TS bindings, so the frontend calls `GetUsage(query)` and gets a typed `Result` — `Row`/`Tokens` marshal through their existing JSON tags with **zero DTO duplication**. **Live updates** use Wails **events, not polling**: the Go side runs a watch loop (stdlib `os.Stat` mtime poll on the source dirs, matching the CLI's `--live` approach — no `fsnotify` dependency) and `EmitEvent("usage:changed")`; the frontend re-invokes `GetUsage` and diffs into the table. The progress callback streams `scan:progress` to a frontend progress bar — the GUI wiring of the same `core.Progress` seam the CLI wires to its spinner.

**Screen layout (MVP):** left rail = source toggles + view switcher (daily/weekly/monthly/session/summary/blocks) + date range; main = uPlot trend chart over the period (cost or total, toggle) above the virtualized `Result.Rows` table with column picker and per-model breakdown expansion; top strip = `Result.Totals` metric cards (Input/Output/Cache/Total/Cost) reusing the HTML writer's metric concept; optional detached always-on-top **statusline window** mirroring `llmut statusline`.

---

## Decision 5 — Packaging, signing, CI: fork the release; keep the CLI track untouched

**The release pipeline MUST fork.** Today `release.yml` runs on `ubuntu-latest` only and GoReleaser cross-compiles 5 CGO-free CLI archives. Wails is the **opposite**: it needs **CGO + native webview bindings ⇒ no cross-compilation ⇒ each OS bundle builds on its own runner.** GoReleaser does not produce `.app`/`.dmg`/AppImage/NSIS. So:

- **Track 1 (unchanged):** existing GoReleaser job, Linux runner, 5 archives + `checksums.txt`. Run with **`GOWORK=off`**. Left exactly as-is so CLI shipping is unaffected.
- **Track 2 (new):** a matrixed `desktop-release.yml` (`ubuntu`/`macos`/`windows-latest`), each running `task build` / `wails3 build`, uploading its bundle to the **same** GitHub Release under a distinct name scheme `llmut-desktop-<os>-<arch>` (no collision with CLI archives). Both triggered by the `v*` tag.
  - **Linux:** WebKitGTK (`apt-get install libwebkit2gtk-4.1-dev libgtk-3-dev`) → AppImage + `.deb`.
  - **macOS:** system WebKit → `.app` (zipped/`.dmg`), built on `macos-latest`.
  - **Windows:** WebView2 (runtime on Win10+/11) → NSIS `.exe`.

**Signing / notarization — phased, decided:**
- **macOS:** ship the first desktop release **unsigned + un-notarized** with a documented Gatekeeper-bypass note. Signing (Developer ID `codesign` + `notarytool` + staple) is a **follow-up phase** gated on acquiring an Apple Developer account + CI secrets (`APPLE_DEVELOPER_ID`, app-specific password/API key).
- **Windows:** ship **unsigned** initially (SmartScreen warning documented); Authenticode (EV/OV cert) deferred — cost + secret.
- **Linux:** no OS signing required; optionally GPG-sign the `.deb`.

**CI — two workflows, path-filtered so the 95% of PRs that don't touch the GUI stay fast:**
- **`ci.yml` (existing) hardened:** keep the 3-OS stdlib matrix, run with `GOWORK=off`, and **add a zero-dep regression assertion** that fails the build if a dependency sneaks into the root module:
  ```sh
  test ! -s go.sum                          # empty/absent go.sum
  ! grep -q '^require' go.mod                # no require block
  ```
  This makes the zero-dependency selling point a *tested invariant*, not a hope.
- **`desktop-ci.yml` (new), path-filtered to `desktop/**` + `pkg/core/**`:** matrix `ubuntu`/`macos`/`windows`, adds `actions/setup-node@v4` (Vite/TS) + Linux WebKitGTK deps, installs pinned `wails3`, runs `npm ci` in `frontend/` then `task build`, plus `cd desktop && go vet ./... && go test ./...`. Cache npm + Go build cache (Wails builds are slow). Triggered on `pkg/core/**` too, since the desktop app depends on it.

---

## Decision 6 — Phased delivery (each phase ships independently)

| Phase | Deliverable | Ships as | New deps in root? | Gate |
|---|---|---|---|---|
| **P0 — Core refactor** | Promote data core to `pkg/core`; `LoadAndAggregate`/`Query`/`Result`/`Warning`/`Progress`; `Run`/`runLive` thinned; CI zero-dep assertion added | Normal CLI release `v0.2.0`, fully backward-compatible | **No** | All 7 test files green after each of the 3 migration steps; `--format json` byte-stable |
| **P1 — CLI expansion** | Stdlib-only, reuse pure aggregators: `--fields` (unlocks hidden Reasoning/Credits/Start/Last), real `--config` loader (kills the dead no-op), `completion bash\|zsh\|fish`, `--budget`+exit-code, `--by project\|source`, period-`--compare`, `--sparkline` | CLI releases `v0.3.x` | **No** | Each feature independently flagged + tested |
| **P2 — Desktop skeleton** | `desktop/go.mod`, gitignored `go.work`, Wails v3 scaffold, `UsageService` over `core.LoadAndAggregate`, Svelte+Vite frontend; `wails3 dev` runs locally | Dev-only; no release | Quarantined in `desktop/` | `wails3 dev` renders real `Result` from workspace core |
| **P3 — Desktop MVP** | Trend chart (uPlot) + virtualized table w/ column picker + totals cards + source/view/date controls + live mtime-watch events; `desktop-ci.yml` builds bundle on 3 OSes | Release Track 2 assets, **unsigned**, w/ bypass docs | Quarantined | 3-OS desktop CI green; CLI Track 1 untouched |
| **P4 — Polish** | Detached statusline window, budget notifications, `serve` HTTP dashboard on the *same* `LoadAndAggregate`, macOS notarization + Windows Authenticode once certs land | Signed desktop release | Quarantined | Gatekeeper/SmartScreen clean |

**What ships independently:** P0 and P1 are pure CLI releases that never touch Wails and preserve the zero-dep promise — they have standalone value (the `--fields`/`--config`/budget features) even if the desktop app slips. P2–P4 are additive in a quarantined module; a desktop delay cannot block CLI releases.

---

## Decision 7 — Top 5 risks and mitigations

1. **Dependency contamination of the root module** (the existential risk — it would break `go install` and violate the project's own CONTRIBUTING contract). **Mitigation:** separate module + gitignored `go.work` + `GOWORK=off` in CI + the hard `test ! -s go.sum && ! grep -q '^require' go.mod` assertion. Contamination becomes a *red CI build*, not a silent regression.

2. **Wails v3 alpha breaking changes mid-build.** **Mitigation:** pin an exact v3 version (never `@latest`); blast radius is `desktop/` only (CLI/core/tests untouched); documented ~1-day fallback to v2 because the backend is plain `core` calls behind a thin `app.go`. `desktop-ci.yml` catches breakage on upgrade PRs before merge.

3. **Migration silently changes CLI output and breaks downstream scripts** (`--format json` warnings type change; the `Debug`-gated malformed-file `continue`; the cost-recompute conditions). **Mitigation:** custom `Warning.MarshalJSON` keeps JSON byte-stable; `Result` reuses the old `jsonReport` JSON tags; the tolerant-read flag and cost-pass conditions move **intact**; the existing `report_test`/`parse_test` assertions are the regression net, and the 3-step migration keeps `go test ./...` green at every step.

4. **CI cost/time blowup** (3 OS runners + Node + WebKitGTK + slow Wails compile on every PR). **Mitigation:** path-filter `desktop-ci.yml` to `desktop/**` + `pkg/core/**` so CLI/core-only PRs never pay the desktop cost; cache npm + Go build; leave the fast stdlib `ci.yml` as the default feedback loop.

5. **Unsigned desktop bundles get blocked by Gatekeeper/SmartScreen, tanking first-run trust.** **Mitigation:** ship unsigned **with explicit, documented bypass instructions** in P3; sequence signing/notarization into P4 behind cert/secret acquisition so it doesn't block the MVP; Linux AppImage/`.deb` need no signing and can be the recommended first-class download until macOS/Windows certs land.

---

## Files this plan touches (all absolute)

**Move/rename (data core → `pkg/core`):** `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/{types,parse,paths,pricing,aggregate,jsonutil}.go` and their `_test.go` siblings.
**Rewrite (CLI shell, stays in `internal/usage`):** `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/{cli,report,live,progress}.go` — `parseArgs` emits `(Query, RenderOptions, live)`; `Run`/`runLive` call `core.LoadAndAggregate`+`Render`; `progress.go` becomes a callback sink.
**New (desktop module):** `/home/jdziat/Code/jdziat/llm-usage-tracker/desktop/{go.mod,go.sum,main.go,app.go,Taskfile.yml,wails.json,frontend/}`.
**Edit (build/CI/policy):** `/home/jdziat/Code/jdziat/llm-usage-tracker/.gitignore` (+`go.work`, `go.work.sum`, desktop build/node artifacts), `/home/jdziat/Code/jdziat/llm-usage-tracker/.github/workflows/ci.yml` (`GOWORK=off` + zero-dep assertion), new `desktop-ci.yml`, new `desktop-release.yml`; `.goreleaser.yaml` and `release.yml` **untouched**.
**Unchanged (the contract):** `/home/jdziat/Code/jdziat/llm-usage-tracker/go.mod`, `/home/jdziat/Code/jdziat/llm-usage-tracker/cmd/llmut/main.go`.
