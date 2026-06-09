# Contributing to llmut

Thanks for contributing. llmut is intentionally small: a zero-dependency Go CLI
that reads local coding-agent logs and aggregates token usage and estimated
cost. It never makes network calls. Keep changes in that spirit.

## Dev setup

You need Go 1.25 or newer (see `go.mod`). No other tooling is required.

```sh
git clone https://github.com/jdziat/llm-usage-tracker
cd llm-usage-tracker
go build ./cmd/llmut
go test ./...
```

## Quality bar

CI (`.github/workflows/ci.yml`) runs on Linux, macOS, and Windows. Before
opening a PR, run locally what CI runs:

```sh
gofmt -l .        # must print nothing
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./...
```

Keep code portable across all three platforms (use `filepath.Join`, never
hardcode path separators).

### Zero-dependency policy

`go.mod` has no requirements, and we want to keep it that way. llmut uses the
standard library only. PRs that add a module dependency need strong
justification and will usually be asked to inline the small amount of stdlib
code that achieves the same thing.

## Testing conventions

- Tests build synthetic log fixtures in `t.TempDir()` and point the parsers at
  them. `internal/usage/parse_test.go` is the canonical example: it writes a
  fake Codex `rollout-*.jsonl` or Claude `session.jsonl` into a temp dir, then
  asserts on the parsed events.
- **Never commit real agent logs, real project paths, or personal usage
  data.** Invent token counts, session IDs, and paths like `/work/repo`.
- Pricing changes get exact-cost assertions over 1M-token inputs; see
  `internal/usage/pricing_test.go`.

## Common contributions

### Adding or updating a model price

1. Add the model to `builtinPrices` in `internal/usage/pricing.go` (USD per
   million tokens: input, output, cacheWrite, cacheRead). Update the
   "Last verified" comment date.
2. If dated or suffixed variants of the ID appear in logs (e.g.
   `claude-opus-4-8-20260514`, `claude-fable-5[1m]`), add a case to
   `normalizeModel` so they map to the base ID — substring fallback in
   `lookupPrice` can otherwise match the wrong (older) entry.
3. If the model has fast-mode pricing, add its multiplier to
   `fastMultipliers`.
4. Add a test in `internal/usage/pricing_test.go` covering the base ID and
   any variants.

### Adding a source

Sources are documented in `docs/sources.md`. A new source touches:

- `internal/usage/types.go`: a `Source*` constant and `allSources` entry.
- `internal/usage/paths.go`: `defaultSourcePaths` (default log dir + env
  override).
- `internal/usage/parse.go`: a reader (reuse `readGenericSource` if the format
  is close to OpenCode/Amp/pi), wired into `loadEvents`.
- `internal/usage/cli.go`: the `sources` map, a `--<source>-path` flag, and
  the help text.
- Tests in `parse_test.go` with a synthetic fixture, and a `docs/sources.md`
  section describing the log format and default path.

### Adding an output format

Formats live in `internal/usage/report.go` (`writeTable`, `writePrettyTable`,
`writeJSON`, `writeCSV`, `writeHTML`). Add a `write*` function there, wire it
into the format switch in `Run` and the `--format` validation in `parseArgs`
(`internal/usage/cli.go`), and update the help text. Note that `--live` only
works with `table` and `pretty`; non-interactive formats must remain valid
targets for `--output`.

## Commit style

Conventional commits, scope optional:

```
feat(llmut): add gemini-3-pro pricing
fix: handle empty jsonl lines in codex parser
docs: document AMP_DATA_DIR override
chore: bump staticcheck
```

CI runs on every push to `main` and on pull requests.

## Releases (maintainers)

Releases are tag-driven. Pushing a `v*` tag (e.g. `v0.3.0`) triggers
GoReleaser (`.goreleaser.yaml`), which builds CGO-free binaries for
linux/darwin/windows on amd64 and arm64 (no windows/arm64) and publishes
tar.gz archives (zip on Windows) with a checksums file. The changelog is
generated from commit messages; `docs:`, `test:`, and `chore:` commits are
excluded.
