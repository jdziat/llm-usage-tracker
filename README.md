# llmut

`llmut` is a Go CLI for local coding-agent usage tracking, modeled after the current `ccusage` command surface.

It reads local logs for Claude Code, Codex, OpenCode, Amp, and pi-agent, aggregates token and estimated cost data, and emits terminal tables or JSON.
It can also generate CSV and self-contained HTML reports.

## Build

```bash
go build ./cmd/llmut
```

## Examples

```bash
llmut daily --since 2026-05-01 --breakdown
llmut summary --since 2026-05-01 --top 10
llmut codex monthly --json --speed fast
llmut claude session --project my-repo
llmut daily --format pretty
llmut daily --format html --output usage.html
llmut daily --format csv --output usage.csv
llmut daily --no-progress
llmut claude blocks --active
llmut pi daily --pi-path ~/.pi/agent/sessions
```

## Supported Views

- `daily`, `weekly`, `monthly`: aggregate by local date, ISO week, or month.
- `session`: aggregate by source session ID.
- `summary`: top projects over the selected timeframe, sorted by cost and limited to 10 rows by default.
- `blocks`: Claude-only 5-hour billing-window style aggregation.
- `statusline`: compact Claude active-block output for status hooks.

## Supported Sources

- Claude Code: `~/.config/claude/projects`, `~/.claude/projects`, or `CLAUDE_CONFIG_DIR`.
- Codex: `~/.codex/sessions` or `CODEX_HOME`.
- OpenCode: `~/.local/share/opencode` or `OPENCODE_DATA_DIR`.
- Amp: `~/.local/share/amp` or `AMP_DATA_DIR`.
- pi-agent: `~/.pi/agent/sessions` or `PI_AGENT_DIR`.

Source focused commands use the same shape as `ccusage`, for example `llmut codex daily` or `llmut claude monthly`.

## Report Formats

- `--format table`: default terminal table.
- `--format pretty`: boxed terminal table.
- `--format json` or `--json`: structured report for scripts.
- `--format csv`: spreadsheet-friendly rows.
- `--format html`: self-contained HTML report.

Use `--output <path>` or `-o <path>` to write any format to a file.

For `summary`, use `--top N` to change the project count.

## Progress

`llmut` shows an interactive spinner on stderr while scanning sources when stderr is a terminal. Progress is suppressed automatically for redirected output and can be disabled explicitly with `--no-progress`.

## Notes

Pricing is calculated offline from a built-in table for common Claude, OpenAI, and Gemini coding models. Unknown models are still counted but report `$0.0000` until a price entry is added.
