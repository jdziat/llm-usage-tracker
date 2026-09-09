# Sources and log formats

llmut reads usage data exclusively from log files that coding agents already write to your local disk. It never talks to the network. This document describes, per source, where llmut looks for logs, what it parses out of them, and how to add a new source.

Ground truth lives in two files:

- `internal/usage/paths.go` — discovery (default directories, env overrides, file walking)
- `internal/usage/parse.go` — parsing (per-source readers and the generic reader)

## Overview

| Source | Default location | Env override | Files scanned | Reader |
|---|---|---|---|---|
| `claude` | `~/.config/claude/projects`, `~/.claude/projects` | `CLAUDE_CONFIG_DIR` | `*.jsonl` | `readClaude` |
| `codex` | `~/.codex/sessions` | `CODEX_HOME` | `*.jsonl` | `readCodex` |
| `opencode` | `~/.local/share/opencode` | `OPENCODE_DATA_DIR` | `*.jsonl`, `*.json` | `readGenericSource` |
| `amp` | `~/.local/share/amp` | `AMP_DATA_DIR` | `*.jsonl`, `*.json` | `readGenericSource` |
| `pi` | `~/.pi/agent/sessions` | `PI_AGENT_DIR` | `*.jsonl`, `*.json` | `readGenericSource` |

With no source argument, llmut scans all five sources and merges the events. A single positional source (`llmut codex daily`) restricts the scan to that source.

## Discovery

All sources share the same discovery machinery (`defaultSourcePaths`, `expandList`, `discoverFiles` in `paths.go`):

- **Built-in defaults** are only used if the directory actually exists (`existing()` stats each candidate). Missing defaults are silently skipped — an agent you do not use contributes nothing.
- **Env overrides** accept a comma-separated list of directories. `~` and `~/...` are expanded to your home directory. For `CLAUDE_CONFIG_DIR` and `CODEX_HOME` each entry is treated as the agent's config root: if `<entry>/projects` (Claude) or `<entry>/sessions` (Codex) exists, that subdirectory is scanned; otherwise the entry itself is. The other env vars are used as-is.
- **CLI flags** beat env vars, which beat defaults. `--claude-path`, `--codex-path`, `--opencode-path`, `--amp-path`, and `--pi-path` set the roots for one source each; `--path` does the same for the source named on the command line — it only applies when exactly one source positional is given, and is ignored otherwise (including for the `blocks`/`statusline` views, which force the `claude` source without a positional). A per-source flag overrides `--path` for that source.
- **File walking** is recursive (`filepath.WalkDir`). Directories named `.git`, `node_modules`, or `memory` are skipped entirely. Unreadable entries are ignored rather than failing the run.
- **Extensions**: the Claude and Codex readers only consider `.jsonl` files; the generic reader considers both `.jsonl` and `.json`.

JSONL files are scanned line by line (`scanJSONL`) with a 16 MiB maximum line size. Blank lines, lines that do not start with `{`, and lines that fail to parse as JSON are skipped — a corrupt line never aborts the file. Numbers are decoded with `json.Number`, and integer fields also tolerate string values (`"42"`).

## Claude Code (`claude`)

**Discovery.** `CLAUDE_CONFIG_DIR` if set; otherwise both `~/.config/claude/projects` and `~/.claude/projects` are scanned when present (events from both are merged). Only `.jsonl` files are read. Note: the `blocks` and `statusline` views always force the source to `claude`, regardless of the source argument.

**Parsing.** Each line is a transcript record. llmut keeps only records where `message.role == "assistant"` and `message.usage` is present. A fabricated example of an accepted line:

```json
{"type":"assistant","timestamp":"2026-06-08T14:21:07.512Z","sessionId":"b3f1c2d4-9e0a-4b5c-8d6e-7f8091a2b3c4","requestId":"req_011CKxample","cwd":"/home/alex/projects/demo-app","message":{"id":"msg_01Example","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":12,"output_tokens":418,"cache_creation_input_tokens":2048,"cache_read_input_tokens":35840}}}
```

Details:

- **Deduplication.** Claude Code can write the same API response more than once (streaming partials, resumed sessions). Each event gets a dedup key — `requestId`, falling back to `message.id`, then top-level `uuid` — scoped per file. Repeats within the same file are dropped. If none of the three keys is present the record is kept unconditionally.
- **Synthetic events.** Records with `message.model == "<synthetic>"` and zero tokens and zero cost are skipped (Claude Code writes these for internal bookkeeping).
- **Tokens.** Read from `message.usage`: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.
- **Cost.** If `message.usage.costUSD` is present and positive it is honored and the event is marked as having a known raw cost; otherwise cost is computed from the built-in pricing table. `--mode calculate` forces recomputation even when a logged cost exists; `--mode display` trusts whatever is logged.
- **Session and project.** Session ID is the `sessionId` field, falling back to the file basename. Project is the `cwd` field, falling back to the name of the file's parent directory (Claude stores transcripts under one directory per project).
- **Timestamp.** Top-level `timestamp`, RFC 3339.

## Codex CLI (`codex`)

**Discovery.** `CODEX_HOME` if set (with `/sessions` appended per entry when it exists); otherwise `~/.codex/sessions`. Only `.jsonl` files. Codex names files like `rollout-2026-06-08T15-02-11-<uuid>.jsonl`; llmut strips the `rollout-` prefix and date components to recover the session UUID when the log itself does not name the session.

**Parsing.** A Codex rollout file is a typed event stream, so the reader is stateful per file:

- `session_meta` — `payload.id` becomes the session ID, `payload.cwd` the project, `payload.model` the model.
- `turn_context` — updates `cwd` and `model` mid-file when non-empty (the model can change between turns).
- `event_msg` with `payload.type == "token_count"` — the actual usage record.

A fabricated two-line example:

```json
{"timestamp":"2026-06-08T15:02:11.040Z","type":"session_meta","payload":{"id":"0196f3c2-7a1b-4d2e-9c3f-5a6b7c8d9e0f","cwd":"/home/alex/projects/demo-app","model":"gpt-5.5"}}
{"timestamp":"2026-06-08T15:02:39.881Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":5210,"cached_input_tokens":4096,"output_tokens":640,"reasoning_output_tokens":128}}}}
```

Details:

- **`last_token_usage` preferred.** When `payload.info.last_token_usage` is present it is the per-turn usage and is used directly. When only `total_token_usage` (a running total) is present, llmut computes the delta from the previous total seen in the same file. Either way, events whose token total is zero are dropped (note: the total counts input + output + cache; a reasoning-only delta is dropped too).
- **Cached input is subtracted.** Codex reports `input_tokens` inclusive of cached tokens. llmut records `cached_input_tokens` as cache reads and subtracts them from input (clamped at zero) so the two are not double-billed.
- **Reasoning tokens.** `reasoning_output_tokens` are tracked separately as reasoning tokens.
- **Model fallback.** If no `session_meta`/`turn_context` named a model, the event falls back to `gpt-5`. The event is flagged internally (`IsFallback`), though no current report format surfaces the flag.
- **Cost.** Codex logs carry no cost, so it is always computed from the pricing table. `--speed` selects the pricing tier; fast-tier multipliers exist for the Claude Opus fast-mode models and for the OpenAI models with published fast rates, including `gpt-6-astra`, the GPT-5.6 family, and `gpt-5.3-codex` (see [docs/pricing.md](pricing.md)). `--speed auto` (the default) detects `service_tier = "priority"` or `"fast"` in the `config.toml` next to the default sessions directory.

## OpenCode, Amp, pi-agent (`opencode`, `amp`, `pi`) — the generic reader

These three sources share `readGenericSource`, a schema-tolerant reader for JSON/JSONL logs that contain a usage object somewhere in each record. `.jsonl` files yield one candidate event per line; `.json` files are parsed as a single object and yield at most one event.

A fabricated record the generic reader accepts:

```json
{"sessionID":"ses_8f2a41","time":"2026-06-08T16:45:02Z","model":"claude-fable-5","project":"demo-app","usage":{"inputTokens":2048,"outputTokens":512,"cacheReadInputTokens":18432,"cost":0.0871}}
```

**Finding the usage map.** `findUsageMap` first checks the keys `usage`, `tokenUsage`, `token_usage`, `total_token_usage`, and `last_token_usage` on the record, then recurses depth-first into every nested object. A candidate counts as usage when it has a positive token count under one of `input_tokens`, `inputTokens`, `prompt_tokens`, `output_tokens`, `outputTokens`, `completion_tokens`, or a positive cost under `cost`, `costUSD`, or `totalCost`. (Caveat for contributors: a usage map keyed only with `promptTokens`/`completionTokens` camelCase, or containing only cache tokens with no cost, will not be detected — extend `looksLikeUsage` if a new agent needs it.)

**Field aliases.** Once a usage map is found, the first non-zero match wins for each field:

| Field | Accepted keys |
|---|---|
| input | `input_tokens`, `inputTokens`, `prompt_tokens`, `promptTokens` |
| output | `output_tokens`, `outputTokens`, `completion_tokens`, `completionTokens` |
| cache write | `cache_creation_input_tokens`, `cacheCreationInputTokens`, `cache_creation_tokens`, `cacheCreationTokens` |
| cache read | `cache_read_input_tokens`, `cacheReadInputTokens`, `cached_input_tokens`, `cachedInputTokens` |
| reasoning | `reasoning_output_tokens`, `reasoningTokens` |
| cost (USD) | `cost`, `costUSD`, `totalCost`, `total_cost` |
| credits | `credits`, `credit` |

Records where tokens, cost, and credits are all zero are dropped — and as with Codex, the token total counts input + output + cache only, so a record carrying nothing but reasoning tokens is dropped too. A positive cost marks the event as having a known raw cost (see `--mode` above); credits (Amp) are accumulated and reported separately.

**Other fields.**

- **Timestamp.** First match of `timestamp`, `time`, `createdAt`, `updatedAt`, `lastActivity` — checked on the top-level record first, then on the usage map. Accepted layouts: RFC 3339 (with or without fractional seconds), `2006-01-02 15:04:05`, `2006-01-02`.
- **Model.** First non-empty of top-level `model`, `modelID`, `modelId`, then `model` inside the usage map, then `message.model`; otherwise `"unknown"`. Provider prefixes such as `anthropic/` or `openai/` are stripped later by `normalizeModel` in `pricing.go`.
- **Session.** First non-empty of `sessionID`, `sessionId`, `id`; otherwise the file basename.
- **Project.** First non-empty of `cwd`, `project`; otherwise the file's parent directory name.

## Adding a new source

Suppose you want to add an agent called `foo`. The full wiring, all inside `internal/usage/`:

1. **Constant** — add `SourceFoo = "foo"` to the `const` block in `types.go` and append it to `allSources` (this is what "scan everything" iterates).
2. **Default paths** — add a case to `defaultSourcePaths` in `paths.go`, following the existing pattern: an env override fed through `expandList` (comma lists, `~` expansion, optional well-known subdirectory) and built-in defaults wrapped in `existing()` so missing directories are skipped.
3. **Reader** — if the agent writes JSON/JSONL with a discoverable usage object, you may not need any parsing code at all: the `default` branch of the `switch` in `loadEvents` (`parse.go`) already routes unknown sources to `readGenericSource`. Check whether the agent's token keys are covered by the alias tables above and extend `genericEvent`/`looksLikeUsage` if not. If the format is bespoke (stateful streams like Codex), write a `readFoo` function in `parse.go` and add an explicit `case SourceFoo:` to `loadEvents`.
4. **CLI** — in `cli.go`: add the source to the `sources` map, declare a `--foo-path` flag and call `addPath(SourceFoo, fooPath)` next to the existing ones, list the source in `printHelp`, and add a `sourceLabel` mapping if the display name differs from the constant (as `pi` → `pi-agent` does).
5. **Pricing** — make sure the model IDs the agent logs normalize to entries in `builtinPrices` (`pricing.go`). Unknown models still count tokens but report `$0.0000`, which silently understates cost.
6. **Tests** — add cases to `parse_test.go`. The convention is synthetic fixtures written into `t.TempDir()` with `os.WriteFile`, then asserting on the events returned by the reader (see `TestReadCodexUsesLastTokenUsageAndModelContext` for the shape). Fabricate every fixture: never commit real logs — they contain prompts, file paths, and project names.
7. **Docs** — add the source to the table and a section in this file, and mention it in the README.
