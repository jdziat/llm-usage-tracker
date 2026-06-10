# llmut Desktop App — Design Vision

> **The one-sentence promise:** *llmut is the flight recorder for your AI coding spend — open it and know, in five seconds, exactly where every token and dollar went across every agent you run, live.*

A ruthless framing first: the CLI already answers *"what did I spend?"* The desktop app must answer the question the CLI structurally **cannot** — *"where is this going, is it normal, and what is driving it right now?"* That is a spatial, temporal, comparative question. Terminals render one table at a time; a window renders a **room of meters that update together.** That is the entire reason this app should exist, and the design must earn it on every screen.

---

## 1. Product positioning

**Category:** A local-first **observability cockpit for coding-agent cost**, not a "usage viewer." Think `htop`/`Activity Monitor` for AI spend, not a spreadsheet with a chart bolted on.

**Who it's for, in two distinct postures the IA must serve simultaneously:**
- **The individual developer** running Claude Code + Codex + opencode side by side, who wants to feel the cost of a session *as it happens* and not get surprised by a cache-heavy refactor.
- **The engineering lead** who pays for several seats/projects and needs the *attribution* answer: which project, which model, which week is eating the budget — and whether this week is an anomaly.

**Positioning pillars (each is a design constraint, not a slogan):**
1. **Local-first and silent.** Zero network, ever. The app reads the same log files the CLI reads (`~/.claude/projects`, `~/.codex/sessions`, `~/.local/share/opencode`, `~/.local/share/amp`, `~/.pi/agent/sessions`) and the discovery-honoring env vars (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `OPENCODE_DATA_DIR`, `AMP_DATA_DIR`, `PI_AGENT_DIR`). The UI must *visibly* signal "nothing leaves this machine" — it is a trust differentiator, surfaced as a small persistent "Offline · Local files only" pill in the title bar, not buried in an About box.
2. **Multi-agent native.** Every competitor (ccusage) is single-tool. llmut already unifies five sources behind one `Source` field. The desktop's hero capability is the **side-by-side, same-axis comparison of all five agents** — a thing no CLI table conveys well.
3. **Live by default.** The `blocks` view already models Claude's 5-hour billing windows with an `--active` notion of "the window you're in right now." The desktop makes that window a **living gauge**, not a row you re-run a command to refresh.
4. **Same engine, no drift.** The backend calls the exact `LoadAndAggregate(Query)` core the CLI uses, so a number in the GUI is byte-identical to the same number in `llmut`. We never reimplement aggregation in JavaScript. (Architecturally: the frontend is a *view* over `Result.Rows`; all math stays in Go.)

**What we explicitly refuse to be:** a generic SaaS dashboard, a telemetry uploader, a "beautiful" chart toy. The aesthetic target is *instrument panel*, not *marketing site*.

---

## 2. The primary user job

> *"I run AI coding agents all day. Tell me where the money goes — by time, by model, by project, by session — and warn me before a window gets expensive. Make the answer trustworthy enough that I'd quote it to my finance team."*

Decomposed into the four sub-jobs the IA is built around:
- **Orient** — "Is today/this week normal?" → Overview.
- **Attribute** — "What is the cost *made of*?" (model mix, cache vs. fresh tokens, reasoning tokens) → Breakdown.
- **Investigate over time** — "When did it spike, and is the trend up?" → Trends.
- **Drill to the cause** — "Which project, which session, which 5-hour window?" → Drill-down + Live.

Everything else (settings, sources, exports) is in service of these four.

---

## 3. Information architecture & navigation

**Shell:** A persistent **left rail** (icon + label, ~200px, collapsible to 56px icons) — this is a desktop app, so we use the chrome a window affords rather than cramming tabs. The rail is the primary nav; it never scrolls away.

```
┌────────────────────────────────────────────────────────────────┐
│  llmut          [ Offline · Local ]      ◷ Last scan 0:02 ago   │  ← title bar
├──────────┬─────────────────────────────────────────────────────┤
│ ◉ Overview│                                                     │
│ ◫ Models  │            ACTIVE VIEW CANVAS                       │
│ ∿ Trends  │                                                     │
│ ⊞ Projects│   ┌─ Global filter bar (sticky, top of canvas) ─┐  │
│ ▸ Sessions│   │ Range ▾  Sources ▾  Project ▾  Model ▾  ⟳   │  │
│ ◐ Live    │   └──────────────────────────────────────────────┘  │
│ ──────────│                                                     │
│ ⚙ Sources │                                                     │
│ ⚙ Settings│                                                     │
└──────────┴─────────────────────────────────────────────────────┘
```

**Two global, always-present elements** (the spine of the product):

1. **The Filter Bar** (sticky under the title bar, on every data screen). One set of controls drives *every* view, mapping 1:1 onto the `Query` struct so the GUI and CLI share semantics:
   - **Range** — segmented control `Today · 7d · 30d · This Month · Custom…`, mapping to `Since`/`Until`. Custom opens a dual-month date picker. (CLI parity: `--since/--until`.)
   - **Sources** — a multi-select chip group with the five agents, each with its own glyph + signature color (see §6). De-selecting = excluding that source from the loaded set (`Query.Sources`). All five lit by default.
   - **Project** — typeahead over discovered project names (`Query.Project` substring filter), with a "× All projects" reset.
   - **Model** — multi-select of normalized model families seen in range (drives a client-side row filter over `ModelBreakdowns`).
   - **Group-by toggle** — `Combined ⇄ Per-project` (`Query.Instances`).
   - **⟳ Refresh / Live toggle** — manual rescan, plus an auto-refresh switch (the data-side `Progress` callback emits scan stages so the spinner is real, not faked).
   - Filters are **URL-state-like and persistent** across views: switch from Overview to Trends and your range/sources/project carry over. This is the single biggest ergonomic win over re-typing flags.

2. **The Time-scale selector** lives *inside* the filter bar context for time-bucketed screens: `Day · Week · Month` segmented control = `Query.View` (`daily`/`weekly`/`monthly`). It only appears where it's meaningful (Overview, Trends).

**Why a rail + global filter bar and not tabs-everywhere:** the user's mental model is "one dataset, many lenses." The filter bar *is* the dataset selector; the rail *is* the lens selector. They're orthogonal, so they get orthogonal controls. This is the structural advantage the CLI can't offer — in the CLI, changing the lens means re-running with different positionals and re-typing all the flags.

---

## 4. The screens

### 4.1 Overview — *"Is today normal?"*

The landing screen. Goal: **five-second orientation.** Layout, top to bottom:

- **Hero stat strip (4 "instrument" cards), full-width row.** Each card is a number with a sparkline-sized context behind it:
  1. **Spend (range)** — big mono number, e.g. `$42.18`, with a faint Δ vs. the previous equal-length period (`▲ 12% vs prior 7d`) in the accent-amber/teal of up/down.
  2. **Total tokens** — `38.2M`, with the **cache-read ratio** as a thin stacked bar beneath it (cache reads are cheap; a high ratio is a *good news* signal worth surfacing — currently invisible in the CLI's human views).
  3. **Active models** — count + the top model's name (`6 models · opus-4-8 leads`).
  4. **Live window** — if a Claude 5-hour `blocks` window is currently open, this card becomes a **mini live gauge**: a radial ring filling toward `--session-length`, time remaining (`3h 12m left`), spend-so-far. If no window is open, it reads "No active window." This is the only card that animates on its own.

- **The "Spend by day" hero chart** (≈55% of remaining height). A **bar-per-bucket column chart** (day/week/month per the time-scale selector), where each bar is **stacked by source color** so you read both the *total height* (how much) and the *composition* (which agents) at a glance. Hovering a bar pops an inspector with the exact `Tokens` and `CostUSD` for that bucket; clicking a bar deep-links into Trends focused on that bucket. A dashed horizontal **budget line** overlays if a budget is set (see §5 settings parity).

- **Two compact panels side-by-side below it:**
  - **Top models (right)** — the `summary` view, top 5 by cost, as horizontal bars with cost labels. "See all →" jumps to Models.
  - **Top projects (left)** — same, keyed on project. (This realizes the `--by project` rollup the CLI roadmap wants, but visual.)

**What makes it feel alive:** the hero strip and the live ring update on the same scan tick; a successful rescan flashes a 1-line "Updated · 0.3s · 1,204 events" toast that fades, so you trust freshness without it being noisy.

### 4.2 Models — *"What is the cost made of?"*

The attribution screen, keyed on model. This is where the **hidden data the CLI computes but hides** finally gets a home: `Reasoning` tokens, `Credits`, cache-creation vs. cache-read split.

- **Left 40%: model leaderboard.** Each model is a card-row: normalized family name (`claude-opus-4-8`), its source glyph, total cost (mono), and a **token-composition micro-bar** — a single horizontal bar segmented into Input / Output / Cache-Create / Cache-Read / Reasoning, each its own hue (see §6 data-viz palette). Sorted by cost desc (`sortedSummaryRows`). Clicking selects it.

- **Right 60%: the selected model's anatomy.**
  - A **donut of token categories** (the five token kinds) with absolute counts and the dollar contribution of each — making vivid that, e.g., cache-read is 60% of tokens but 4% of cost. This is the single most *educational* view in the product and it's pure presentation over data already in `Tokens`.
  - A **"fast vs. standard" callout** for models that carry a fast multiplier (opus-4-8 = 2×, 4-7/4-6 = 6×). If the user's effective `Speed` is `fast`, show "You're on fast tier — this model costs 2× standard" with the standard-tier counterfactual cost. This turns the invisible `fastMultiplier` logic into an actionable insight. No competitor surfaces this.
  - A **price provenance line:** "$5.00 / $25.00 per 1M in/out · offline table, verified 2026-06-09" — reinforcing the trust pillar and explaining `$0` rows honestly (`Unknown model — counted at $0`).

### 4.3 Trends — *"When did it spike, is it climbing?"*

The temporal screen, and the marquee feature the CLI fundamentally can't do well: **comparison.**

- **Primary: a multi-series area/line chart** over the range, one line per source (toggle to per-model or per-project via a small selector). Y-axis selectable: **Cost · Total tokens · Reasoning tokens**. Smooth but honest (no fake curve interpolation that misrepresents discrete buckets — stepped or straight segments).
- **Compare mode** — a toggle that overlays the **previous equal-length period as a ghost line** (dashed, 40% opacity), with a delta legend. This is the `--compare previous` roadmap feature made spatial: you *see* the climb, you don't compute it.
- **Brushing** — drag across the chart to set the range (writes back into the global `Since/Until`). Selecting a span updates every card and reveals a "Selected: May 3–9 · $18.40 · ▲22%" summary bar.
- **Anomaly ticks** — buckets more than ~2σ above the trailing mean get a small amber caret; hovering explains "3.1× your daily median." Cheap to compute over `Result.Rows`, high perceived intelligence.

### 4.4 Projects — *"Which project is eating the budget?"*

A **treemap** as the hero: each rectangle is a project, area ∝ cost, color = the project's dominant source. Big rectangles are your expensive repos at a glance — a treemap answers "where's the money" faster than any sorted list. Click a rectangle to drill into that project's session list (4.5) pre-filtered. Below the treemap, a sortable table fallback (cost, tokens, sessions, last active) for the keyboard-driven user who wants precise numbers and column sort.

### 4.5 Sessions (drill-down) — *"Which session, exactly?"*

The deepest lens, finally surfacing `Row.Start`, `Row.LastActivity`, `SessionID`, `Project` — all computed today, all invisible in the CLI's table/pretty output.

- **A virtualized list/table of sessions** (handles tens of thousands of rows), columns: project · source glyph · **duration** (`LastActivity − Start`, rendered as a thin horizontal "duration bar" positioned on a shared timeline so you literally *see* when sessions overlapped) · models-used chips · tokens · cost.
- **Click a row → a detail drawer** slides from the right: the session's full model breakdown donut, its token composition, its time span, its source path on disk (with a "Reveal in Finder/Files" button — a genuinely desktop-only affordance), and the raw event count. This is the "open the black box" moment.
- Sort/filter inherits the global bar; a per-screen search box filters by `SessionID`/project substring (CLI parity: `--id`, `--project`).

### 4.6 Live — *"What's happening right now?"*

The screen that justifies a *window* over a *command*. It is the `blocks --active --live` experience, redesigned as a cockpit:

- **The big gauge:** a large radial progress ring for the current Claude 5-hour window — elapsed/`SessionLength`, time remaining counting down in real time, spend accumulated in the window, and a projected end-of-window spend (linear extrapolation, clearly labeled "projected"). If `--token-limit` is set, a second arc shows percent-of-limit, turning amber at 80%, red at 100% (parity with the statusline `%`).
- **A live event ticker** down the right side: the most recent events streaming in as the file watcher detects appended lines — `14:32 · opus-4-8 · +12.4k tok · +$0.06`. This is the heartbeat that makes the app feel *alive* in a way no periodic full-rescan ever could.
- **Per-source live tiles** across the bottom — one small tile per active agent showing its last-minute token rate, so a user running Claude + Codex concurrently sees both pulses at once.
- Implementation honesty: live updates ride file-watch (append detection) for the ticker, with the full `LoadAndAggregate` rescan on a slower cadence for totals — the `Progress` callback drives a subtle scan-pulse on the refresh control so the user knows it's working, never a frozen screen.

### 4.7 Sources — *"Where is my data coming from?"*

A configuration *and* diagnostics screen, because in a local-first tool the #1 support question is "why don't I see my Codex usage?"

- **Five source rows**, each showing: the agent glyph + name, a **green/amber/grey status dot** (found & readable / found but empty / not found), the **resolved path(s)** on disk, event count discovered, and date span of data. This makes the otherwise-silent path discovery (`defaultSourcePaths`, the env-var overrides) *visible and debuggable*.
- Each row has an **"Override path…"** picker (native folder dialog — desktop-only) writing into `Query.SourcePaths`, and a "Use env var (`CODEX_HOME`)" hint showing what would be auto-detected.
- A **Codex speed detector** indicator: shows whether `config.toml` resolved to fast/standard tier (the `detectCodexSpeed` logic), with a manual override — surfacing a previously buried decision that changes every Codex cost number.

### 4.8 Settings — preferences & parity

Maps to the data/presentation split cleanly. Sections:
- **Defaults** — default range, default view, timezone (`Query.Location`), start-of-week (monday/sunday), cost mode (auto/calculate/display), Codex speed. These are the data-concern config fields; they're exactly the fields a future `~/.config/llmut/config.toml` would hold, so the GUI becomes the *editor* for that file (and writes it, so CLI and GUI stay in sync — a quietly killer feature).
- **Budgets & alerts** — set a USD budget per period (day/week/month) and an optional token limit; these power the budget line in Overview and the Live gauge thresholds, and (desktop-only) fire a **native OS notification** when a live window crosses a threshold.
- **Appearance** — theme (System / Dark / Light / "Phosphor" high-contrast), density (Comfortable / Compact), number format.
- **Export** — one-click export of the *current filtered view* to CSV / JSON / HTML / a print-ready PDF, reusing the existing writers' field set (including the Reasoning/Credits columns the CSV already emits). A "Copy as CLI command" button regenerates the exact `llmut …` invocation that reproduces the current screen — a delightful bridge for power users and a teaching tool.

---

## 5. Signature interactions — what makes it feel alive

1. **Linked filtering, everywhere.** One filter bar drives all screens; changing range/source/project animates a coordinated update across every visible chart (cross-filtering, like a real BI cockpit). The mental model: *I am steering one dataset through different lenses,* never *I am re-running a query.*
2. **Hover-to-inspect, click-to-drill.** Every bar, ring segment, treemap tile, and table row has a hover inspector (exact tokens + cost) and a click that **deep-links into the next screen pre-filtered** — Overview bar → Trends bucket → Projects tile → Sessions list → session drawer. The whole app is one continuous drill path. This *is* the GUI's reason to exist; a CLI cannot link views.
3. **The live pulse.** A single shared scan tick updates the hero strip, the live ring, and the ticker together, with a sub-second "Updated" flash. The Live screen's event ticker and countdown make idle-watching genuinely useful — the app earns a permanent spot on a second monitor.
4. **Compare-as-a-gesture.** Brushing a span on Trends, or flipping Compare-to-previous, turns the hardest analytical question ("is this normal?") into a drag and a glance.
5. **Keyboard-first power layer.** `⌘K` command palette to jump views / set range / pick a project; number keys 1–6 switch lenses; `/` focuses the filter search. Respects the developer audience — never forces the mouse.
6. **Honest motion.** Animation is functional only: number tweens on data change (200ms), the live ring's continuous sweep, a gentle pulse on refresh. **No decorative parallax, no gradient shimmer.** Motion communicates state change; nothing moves for delight alone.

---

## 6. Visual direction — *instrument panel, not marketing site*

The current HTML export literally ships `font-family: Inter` on a `#f7f8fa` slate-and-blue card grid. That is precisely the generic look to **reject.** The desktop app's identity is **"oscilloscope meets terminal"** — a calm, dark, data-dense surface where the *data is the only thing that glows.*

**Default theme: Dark ("Graphite"), with a distinctive amber/teal signal language.**

| Role | Hex | Use |
|---|---|---|
| Canvas base | `#0E1116` | app background (near-black, slightly warm graphite — not pure black, not blue-black) |
| Surface | `#171B22` | cards, rails, panels |
| Surface raised | `#1F242D` | hovered rows, drawers, popovers |
| Hairline | `#2A3039` | 1px borders, gridlines, separators |
| Text primary | `#E6E1D6` | a warm off-white (paper, not clinical white) — sets the "phosphor on graphite" tone |
| Text muted | `#8B94A3` | labels, axes, secondary |
| **Signal Amber** | `#F2A33C` | the brand accent — spend, primary KPIs, the live ring, "up/expensive" |
| **Signal Teal** | `#3FB6A8` | the counter-accent — savings, "down/good", cache-read efficiency |
| Alert Red | `#E5484D` | over-budget, errors, anomaly carets |
| Caution | `#E3B341` | approaching-limit, warnings |

**Source identity colors** (each agent owns a hue so stacked bars and tiles are instantly legible — this *is* the multi-agent brand):

| Source | Hex | Glyph |
|---|---|---|
| claude | `#D97757` (terracotta) | ◆ |
| codex | `#10A37F` (codex green) | ▲ |
| opencode | `#6E9FEC` (steel blue) | ● |
| amp | `#C77DFF` (violet) | ✦ |
| pi | `#E8C547` (gold) | ▸ |

**Token-category data-viz palette** (the five token kinds in composition bars/donuts — sequential, distinguishable on dark, colorblind-considered):

| Category | Hex |
|---|---|
| Input | `#5AA9E6` |
| Output | `#3FB6A8` |
| Cache-create | `#9B8AFB` |
| Cache-read | `#4C5A6E` (deliberately dim — it's cheap; visually "background") |
| Reasoning | `#F2A33C` |

**Typography (no Inter, no system-default-as-personality):**
- **Display / numerals:** **Berkeley Mono** (or **IBM Plex Mono** as the open fallback) for *every number* — costs, token counts, dates. Monospaced figures align in columns automatically and read as "instrument readout." This single choice is most of the app's character.
- **UI text / labels:** **Söhne** or, as an open-source-safe default, **Geist** *(not* Inter*)* — a grotesque with a slightly mechanical bone structure that pairs with the mono numerals without fighting them.
- **Headings:** the same UI face, tightened tracking, sentence case. No marketing-bold.
- Tabular figures and `font-variant-numeric: tabular-nums` enforced on all data.

**Data-viz house style:**
- **Flat, sharp, gridded.** Thin 1px hairline gridlines (`#2A3039`), no drop shadows on chart elements, no rounded bar caps, no glossy fills. Area fills at ~12–18% opacity over a 1px stroke. The aesthetic reference is a **scope trace / spec sheet**, not a Dribbble dashboard.
- **One accent does the talking.** A chart is mostly muted greys; the *current* metric glows amber. Composition uses the source or token palettes. We never rainbow-vomit.
- **Density: Comfortable default, Compact available.** Compact targets the power user who wants ccusage-table information density with GUI legibility — tighter row heights, smaller numerals, more rows per screen.
- **Light theme ("Paper"):** a warm `#F7F4EC` canvas with `#1A1D23` ink, same amber/teal signals — for screenshots and bright rooms. Dark is the hero; light is a first-class equal, not an afterthought.
- **Rounding:** 6px radii on cards, 4px on chips/buttons — soft enough to feel modern, hard enough to feel like tooling. No pill-everything.

The net feeling: opening llmut should feel like flipping on a **bench instrument** — dark, precise, the numbers in crisp mono, one warm amber glow tracking what matters, every agent color-coded. Distinctly *not* another Inter-and-purple SaaS panel.

---

## 7. Empty, loading & error states

These are where local-first tools usually fail the user; we treat them as primary screens.

- **First-run / no sources found:** Not a sad shrug. A **"Sources" diagnostic checklist** front and center: the five agents, each with its expected default path and env-var, and a live ✓/✗ as the app probes disk. "We looked in `~/.codex/sessions` — not found. Point us at it →" with a folder picker. The empty state *teaches the path model* and gets the user to data in one click. Tone: a calm pre-flight check, never an error.
- **Sources found but range empty:** "No usage in this range." with the nearest range that *does* have data offered as a one-tap chip ("Jump to last 30 days →"). Never a blank canvas.
- **Scanning / loading:** the `Progress` callback drives a **real, staged** indicator ("Scanning Claude · 412 files…"), not a fake spinner. Skeleton shells for cards/charts (hairline-outlined placeholders that match final layout, so nothing jumps). First paint of cached/last-known totals immediately, refined as the scan completes — never a full-screen blocker.
- **Partial failure (a source errors):** the CLI swallows per-source read errors into warnings (surfaced only under `--debug`). The desktop **never hides them** — a non-blocking amber banner: "Couldn't read 3 Codex files — showing the rest. Details →" expands to the `Warning{Source, Message}` list. The other four sources render fully. Degraded, never broken.
- **Unknown model ($0 cost):** rows with models absent from the offline price table render the cost cell in muted text with an `ⓘ Unknown model — tokens counted, cost unpriced` tooltip, and a one-line footer tally "2 models unpriced (1.2M tokens)." Honest about the limit instead of silently showing $0.
- **Stale data / watcher lost:** if file-watching drops, the freshness clock in the title bar turns amber ("Last scan 6m ago — paused") with a one-tap resume. The user always knows whether what they're looking at is live.

---

## 8. How the desktop exceeds the CLI

The bar: every item here is *impossible or miserable* in a terminal, and most run on data the engine **already computes today**.

1. **Coordinated, multi-panel state.** The CLI shows one table per invocation. The desktop shows spend + composition + trend + live window **at once, cross-filtered.** Orientation drops from "run four commands and hold them in your head" to "one glance."
2. **Drill-through navigation.** Click a day → see its sessions → open a session's anatomy. The CLI has no concept of "click this number to see what's inside it." This is the core GUI dividend.
3. **The hidden data, finally visible.** `Reasoning` tokens, `Credits`, cache-create vs. cache-read split, `Start`/`LastActivity` durations, the fast-tier multiplier — all computed by the engine, all absent from the CLI's human-readable views. The desktop's Models and Sessions screens are built specifically to surface them. Zero new aggregation; pure presentation upside.
4. **Real liveness.** A continuously sweeping 5-hour-window gauge, a streaming event ticker, and OS-native budget notifications. `--live` clears and re-prints a table on a timer; the desktop *pulses.*
5. **Spatial comparison.** Treemaps for project attribution, ghost-line period-over-period overlays, anomaly ticks — "is this normal?" answered by shape, not arithmetic.
6. **Native OS integration.** Folder pickers for source paths, "Reveal in Finder/Files" on a session's raw log, native notifications on budget breach, optional menu-bar/tray live readout. None of these exist for a CLI.
7. **Config as a first-class surface.** The Settings screen *edits the shared config file*, so adjusting defaults in the GUI changes the CLI's behavior too. The GUI becomes the friendly front door to the same engine, not a parallel universe.
8. **Trust artifacts for the engineering lead.** Print-ready PDF/HTML export of the current filtered view, with price-provenance footers and the "verified on" date — something you can drop into a budget review. The CLI exports raw CSV; the desktop exports a *defensible report.*

The throughline: **same numbers, same engine, same offline-and-silent guarantee — but rearranged from a sequence of tables into a room of live, linked instruments.** That rearrangement is the product.

---

### Phased delivery (design-led sequence)
1. **Phase 1 — Read & orient:** Shell + filter bar + **Overview** + **Models**, dark "Graphite" theme, static `LoadAndAggregate` over the range. Ships the core orientation + attribution jobs and surfaces the hidden token/reasoning data immediately.
2. **Phase 2 — Time & drill:** **Trends** (with compare-to-previous + brushing) and **Projects → Sessions** drill-through with the session detail drawer. Delivers the comparison and investigation jobs the CLI can't.
3. **Phase 3 — Live:** **Live** cockpit (radial window gauge + event ticker + per-source tiles) on file-watch, plus **Sources** diagnostics and native budget notifications. Delivers the "feels alive" promise and the local-first trust surface.
4. **Phase 4 — Polish & parity:** **Settings** as the shared-config editor, light "Paper" theme, Compact density, exports (PDF/HTML/CSV/JSON), command palette + "Copy as CLI command." Closes the loop between GUI and CLI.

**Relevant grounding files:** `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/types.go` (the `Tokens`/`Row`/`Event` DTOs the frontend renders, incl. the hidden `Reasoning`/`Credits`/`Start`/`LastActivity` fields), `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/aggregate.go` (the seven view aggregators the screens map onto), `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/pricing.go` (offline price table + fast multipliers the Models screen visualizes), `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/paths.go` (source discovery the Sources screen exposes), `/home/jdziat/Code/jdziat/llm-usage-tracker/internal/usage/report.go` (existing writers/field set the exports reuse — and the `Inter`+slate HTML the dark theme deliberately rejects).
