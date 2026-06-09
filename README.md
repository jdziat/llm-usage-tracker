# llmut

Track token usage and estimated cost across your local coding-agent logs — offline, from one CLI.

[![CI](https://github.com/jdziat/llm-usage-tracker/actions/workflows/ci.yml/badge.svg)](https://github.com/jdziat/llm-usage-tracker/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jdziat/llm-usage-tracker.svg)](https://pkg.go.dev/github.com/jdziat/llm-usage-tracker)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`llmut` reads the log files that coding agents already write to your machine, aggregates token counts, and estimates USD cost from a built-in pricing table. It supports five agents:

- **Claude Code** (`claude`)
- **OpenAI Codex CLI** (`codex`)
- **OpenCode** (`opencode`)
- **Amp** (`amp`)
- **pi-agent** (`pi`)

Everything happens locally: zero network calls, zero telemetry, zero runtime dependencies (pure Go standard library). Pricing is an offline table compiled into the binary — `llmut` never phones home.

The command surface is modeled after [ccusage](https://github.com/ryoppippi/ccusage), so existing `ccusage` muscle memory (and statusline hooks) carry over.

## Install

With Go:

```bash
go install github.com/jdziat/llm-usage-tracker/cmd/llmut@latest
```

Prebuilt binaries for Linux and macOS (amd64/arm64) and Windows (amd64) are on the [GitHub Releases](https://github.com/jdziat/llm-usage-tracker/releases) page.

Or build from source:

```bash
git clone https://github.com/jdziat/llm-usage-tracker
cd llm-usage-tracker
go build ./cmd/llmut
```

## Quick start

```bash
llmut                              # daily usage across all sources
llmut summary --since 2026-05-01   # top models by cost
llmut codex monthly --json         # one source, machine-readable
llmut claude blocks --active       # current Claude 5-hour billing window
```

Sample `llmut summary` output:

```
Coding Agent Usage Report - Summary - All Sources
-------------------------------------------------
Model                                     Input        Output     Cache Cr.        Cache Rd.            Total         Cost  Models
------------------------------  ---------------  ------------  ------------  ---------------  ---------------  -----------  ------------------------------
claude-opus-4-8                       2,841,332       412,907    18,322,118       96,114,209      117,690,566    $187.0997  claude-opus-4-8
claude-sonnet-4-6                     1,104,219       241,883     4,200,761       22,018,940       27,565,803     $29.2994  claude-sonnet-4-6
gpt-5.5                               5,210,448       933,026             0       41,002,815       47,146,289     $20.9687  gpt-5.5
gemini-2.5-flash                        802,114       151,209             0                0          953,323      $0.6187  gemini-2.5-flash
------------------------------  ---------------  ------------  ------------  ---------------  ---------------  -----------  ------------------------------
Total                                 9,958,113     1,739,025    22,522,879      159,135,964      193,355,981    $237.9864
```

The general shape is `llmut [source] [view] [options]` — omit the source to aggregate all of them. Run `llmut help` for the short reference, or see [docs/cli.md](docs/cli.md) for every flag.

## Views

| View | What it shows |
|---|---|
| `daily` | Usage per local calendar date (the default) |
| `weekly` | Usage per week (`--start-of-week monday\|sunday`) |
| `monthly` | Usage per calendar month |
| `session` | Usage per agent session ID |
| `summary` | Top models by cost over the timeframe (default top 10, `--top N`) |
| `blocks` | Claude-only 5-hour billing-window aggregation (`--active`, `--recent`) |
| `statusline` | Compact one-line active-block output for status hooks (Claude-only) |

## Sources

| Source | Default log paths | Env override |
|---|---|---|
| `claude` | `~/.config/claude/projects`, `~/.claude/projects` | `CLAUDE_CONFIG_DIR` |
| `codex` | `~/.codex/sessions` | `CODEX_HOME` |
| `opencode` | `~/.local/share/opencode` | `OPENCODE_DATA_DIR` |
| `amp` | `~/.local/share/amp` | `AMP_DATA_DIR` |
| `pi` | `~/.pi/agent/sessions` | `PI_AGENT_DIR` |

Each source also has a `--<source>-path` flag (e.g. `--codex-path`), and single-source commands accept `--path`. Log formats and discovery rules are documented in [docs/sources.md](docs/sources.md).

## Formats

- `table` — plain terminal table (default)
- `pretty` — boxed terminal table
- `json` — structured report (`--json`/`-j` is shorthand)
- `csv` — flat rows for spreadsheets
- `html` — self-contained HTML report

Use `--format <name>` to pick one and `--output`/`-o` to write to a file instead of stdout.

### Live mode

`--live` re-renders `table`/`pretty` output every `--refresh-interval` seconds (default 5), clearing the terminal between frames and re-reading the logs each cycle, so the report tracks agent activity as it happens. Exit cleanly with Ctrl-C. It works with all views, including `blocks` and `statusline`, but it is an error to combine `--live` with `--format json/csv/html` or `--output`.

```bash
llmut claude blocks --active --live
llmut daily --live --refresh-interval 2
```

## Cost estimation

Costs come from a pricing table built into the binary: for each known model family it stores USD per million tokens for input, output, cache-write, and cache-read tokens. Model IDs found in logs are normalized first — provider prefixes (`anthropic/`, `openai/`, `google/`, ...) are stripped, and dated or suffixed variants (e.g. `claude-fable-5[1m]`) map to their base family — so the same model is priced consistently regardless of which agent logged it. The table was last verified against provider pricing pages on 2026-06-09.

Estimates are exactly that: estimates. They do not account for subscription plans, batch discounts, or provider-side promotions. Unknown models still have their tokens counted, but report `$0.0000` rather than guessing. The full table, normalization rules, and update process live in [docs/pricing.md](docs/pricing.md).

Two flags tune the calculation:

- `--mode auto|calculate|display` — whether to trust cost figures recorded in the logs (`display`), always recompute from the pricing table (`calculate`), or prefer recorded values and fill gaps by computing (`auto`, default).
- `--speed auto|standard|fast` — selects the pricing speed tier. Fast-mode multipliers currently exist only for Claude fast-mode models (`claude-opus-4-8` 2x, `claude-opus-4-7`/`4-6` 6x); all other models, including GPT/Codex models, are unaffected. `auto` detects the Codex `service_tier` from `~/.codex/config.toml`.

## ccusage compatibility

`llmut` accepts several `ccusage` flags as documented no-ops so it can drop into existing scripts and statusline hooks unchanged: `--cache`, `--offline`/`-O` (pricing is always offline), `--locale`, `--config`, and `--debug-samples`.

## Documentation

- [docs/cli.md](docs/cli.md) — full command and flag reference
- [docs/sources.md](docs/sources.md) — per-agent log formats and discovery rules
- [docs/pricing.md](docs/pricing.md) — pricing table and cost model
- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup, CI, releases

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 Jordan Dziat.
