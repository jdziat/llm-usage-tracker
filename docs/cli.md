# CLI reference

```
llmut [source] [view] [options]
```

`llmut` reads local coding-agent log files, aggregates token usage, and prints a
report with estimated USD cost. It never makes network calls; pricing comes from
a built-in offline table.

Positional arguments and flags may appear in any order — `llmut --since
2026-05-01 daily` and `llmut daily --since 2026-05-01` are equivalent. The one
ordering rule: when both positionals are given, the **source must precede the
view** (`llmut codex daily`, not `llmut daily codex`).

`llmut help`, `llmut --help`, and `llmut -h` print built-in usage help.

## Sources

If no source is given, all sources are scanned and rows are labeled per source
(except in the `summary` view, where rows are keyed by bare model name — the
same model used through two different agents produces two unlabeled rows).

| Source | Agent |
|---|---|
| `claude` | Claude Code |
| `codex` | OpenAI Codex CLI |
| `opencode` | OpenCode |
| `amp` | Amp |
| `pi` | pi-agent |

## Views

The default view is `daily`.

| View | Description |
|---|---|
| `daily` | Usage grouped by calendar day. |
| `weekly` | Usage grouped by week (see `--start-of-week`). |
| `monthly` | Usage grouped by calendar month. |
| `session` | Usage grouped by agent session. The row label is the project name when known, otherwise the session ID; `json`/`csv` output includes both fields. |
| `summary` | Top models ranked by cost (default top 10; see `--top`). |
| `blocks` | Claude-only 5-hour billing windows (see `--session-length`). |
| `statusline` | Compact one-line view of the active Claude block; shorthand for `claude blocks --active --compact`. Intended for status-bar hooks. |

Notes:

- `blocks` always scans the `claude` source, regardless of which source was
  selected.
- `statusline` prints `Claude: no active usage` when there is no active block,
  and appends a percent-of-limit figure when `--token-limit` is set.

## Flags

### Filtering and grouping

| Flag | Type | Default | Description |
|---|---|---|---|
| `--since` | date | — | Only include events on or after this date (`YYYY-MM-DD` or `YYYYMMDD`). |
| `--until` | date | — | Only include events up to this date, **end-of-day inclusive**. |
| `--timezone`, `-z` | string | system local | IANA timezone (e.g. `UTC`, `America/New_York`) used for date filtering and day/week/month bucketing. |
| `--start-of-week` | string | `monday` | `monday` or `sunday`. `monday` buckets by ISO week (keys like `2026-W23`); `sunday` buckets by the week's start date. |
| `--project`, `-p` | string | — | Only include events whose project path contains this substring (case-insensitive). |
| `--id` | string | — | Only include events whose session ID contains this substring. |
| `--order` | string | `desc` | Row sort order: `asc` or `desc`. |
| `--top` | int | `0` | Limit rows in the `summary` view; `0` means the default of 10. |
| `--breakdown`, `-b` | bool | `false` | Show a per-model breakdown under each row. |
| `--instances`, `-i` | bool | `false` | Additionally group daily/weekly/monthly rows by project. |

### Output and formatting

| Flag | Type | Default | Description |
|---|---|---|---|
| `--format` | string | `table` | Output format: `table`, `pretty` (boxed), `json`, `csv`, or `html` (self-contained page). |
| `--json`, `-j` | bool | `false` | Shorthand for `--format json`. |
| `--output`, `-o` | string | stdout | Write the report to a file (`-` means stdout). |
| `--compact` | bool | `false` | Compact output; with `blocks` it prints the one-line statusline form. |
| `--progress` | bool | `true` | Show a scan progress indicator on stderr (only when stderr is a terminal). |
| `--no-progress` | bool | — | Disable the scan progress indicator. |
| `--debug` | bool | `false` | Print per-source scan warnings to stderr. Corrupt JSONL lines are always skipped silently and never reported. |

### Live mode

| Flag | Type | Default | Description |
|---|---|---|---|
| `--live` | bool | `false` | Re-render the report every refresh interval, clearing the terminal between frames and re-reading the logs each cycle. Exits cleanly on Ctrl-C. Works with all views, including `blocks` and `statusline`. Only valid with `--format table` or `--format pretty`; combining it with `--format json/csv/html` or `--output` is an error. |
| `--refresh-interval` | seconds | `5` | Seconds between `--live` refreshes (fractional values accepted). |

### Sources and paths

Each path flag accepts a comma-separated list of directories; `~` is expanded
to your home directory. These override the defaults and environment variables
listed under [Environment variables](#environment-variables).

| Flag | Type | Default | Description |
|---|---|---|---|
| `--path` | string | — | Log path override for the selected source. Only applies when exactly one source positional is given (e.g. `llmut codex daily --path ~/backups/codex`). |
| `--claude-path` | string | — | Claude Code projects directory. |
| `--codex-path` | string | — | Codex home or sessions directory. Unlike `CODEX_HOME`, path flags are used as-is (no `sessions` subdirectory is auto-appended); pointing at the home directory still works because discovery scans recursively. |
| `--opencode-path` | string | — | OpenCode data directory. |
| `--amp-path` | string | — | Amp data directory. |
| `--pi-path` | string | — | pi-agent sessions directory. |

### Blocks and statusline

| Flag | Type | Default | Description |
|---|---|---|---|
| `--active`, `-a` | bool | `false` | Only show the currently active block. |
| `--recent`, `-r` | bool | `false` | Only show blocks from the last 72 hours. |
| `--session-length` | hours | `5` | Billing-block length in hours (fractional values accepted). |
| `--token-limit` | string | — | Token warning limit. With `statusline`, usage is shown as a percentage of this number. The value `max` is accepted (ccusage compatibility) and treated as no limit. |

### Pricing

| Flag | Type | Default | Description |
|---|---|---|---|
| `--mode` | string | `auto` | Cost source: `auto` trusts costs recorded in the logs when present and computes the rest from the built-in price table; `calculate` always recomputes from tokens; `display` only uses logged costs (events without a logged cost still fall back to computed cost). |
| `--speed` | string | `auto` | Pricing speed tier: `auto`, `standard`, or `fast`. `fast` applies fast-mode price multipliers, which currently exist only for Claude fast-mode models (`claude-opus-4-8` 2x, `claude-opus-4-7`/`4-6` 6x — see [docs/pricing.md](pricing.md)); all other models, including every GPT/Codex model, are priced at 1x regardless. `auto` reads `service_tier` from the `config.toml` next to the default Codex sessions root (`~/.codex/config.toml`, honoring `CODEX_HOME`) and picks `fast` when it is `"priority"` or `"fast"`, otherwise `standard`. |

### Compatibility no-ops

These flags are accepted so `llmut` can be dropped into existing ccusage
invocations and statusline hooks. They do nothing.

| Flag | Type | Description |
|---|---|---|
| `--cache` | bool | No-op (ccusage statusline compatibility). |
| `--offline`, `-O` | bool | No-op; pricing is always offline. |
| `--locale` | string | No-op. |
| `--config` | string | No-op. |
| `--debug-samples` | int | No-op. |

## Date formats

`--since` and `--until` accept two layouts:

- `YYYY-MM-DD` (e.g. `2026-05-01`)
- `YYYYMMDD` (e.g. `20260501`)

Dates are interpreted in the timezone selected by `--timezone` (default: the
system's local timezone). `--until` is inclusive through the end of the given
day, so `--since 2026-05-01 --until 2026-05-31` covers all of May.

## Environment variables

Default log locations can be overridden per source. Like the path flags, each
variable accepts a comma-separated list of directories with `~` expansion.
Path flags take precedence over environment variables.

| Variable | Source | Default when unset | Notes |
|---|---|---|---|
| `CLAUDE_CONFIG_DIR` | claude | `~/.config/claude/projects`, `~/.claude/projects` | A `projects` subdirectory is appended automatically if present. |
| `CODEX_HOME` | codex | `~/.codex/sessions` | A `sessions` subdirectory is appended automatically if present. |
| `OPENCODE_DATA_DIR` | opencode | `~/.local/share/opencode` | |
| `AMP_DATA_DIR` | amp | `~/.local/share/amp` | |
| `PI_AGENT_DIR` | pi | `~/.pi/agent/sessions` | |

## Exit behavior

On success, `llmut` exits `0`. On any failure (bad flag value, unreadable
output file, invalid timezone, etc.) it prints `error: <message>` to stderr
and exits with a nonzero status. Source-level scan failures are warnings, not
errors: they are printed to stderr only when `--debug` is set, and are embedded
in `json`/`html` output whether or not `--debug` is set. Corrupt JSONL lines
and unreadable files within a source are skipped silently.

## Examples

```sh
# Daily usage across all agents for the last month, with per-model breakdowns
llmut daily --since 2026-05-01 --breakdown

# Top 5 models by cost across all sources
llmut summary --since 2026-05-01 --top 5

# Codex-only monthly usage as JSON, priced at the fast tier
llmut codex monthly --json --speed fast

# Self-contained HTML report you can open in a browser
llmut daily --since 2026-04-01 --until 2026-06-01 --format html --output usage.html

# Weekly usage bucketed by Sunday-start weeks in a fixed timezone
llmut weekly --start-of-week sunday --timezone UTC

# Sessions for a single project, oldest first
llmut claude session --project side-quest --order asc

# Recent Claude 5-hour billing blocks
llmut claude blocks --recent

# Statusline hook line with a token budget (prints something like:
# "Claude 1,234,567 tokens $4.2100 2h10m0s left 62%")
llmut statusline --token-limit 2000000

# Live-updating pretty dashboard, refreshed every 2 seconds (Ctrl-C to exit)
llmut daily --live --format pretty --refresh-interval 2

# Read pi-agent logs from a non-default location
llmut pi session --pi-path ~/backups/pi-sessions,~/.pi/agent/sessions
```
