package usage

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

var views = map[string]bool{"daily": true, "weekly": true, "monthly": true, "session": true, "summary": true, "blocks": true, "statusline": true}
var sources = map[string]bool{core.SourceClaude: true, core.SourceCodex: true, core.SourceOpenCode: true, core.SourceAmp: true, core.SourcePI: true}

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "completion" {
		return printCompletion(args[1:], stdout)
	}
	q, opts, live, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(stdout)
			return nil
		}
		return err
	}
	if live {
		if err := validateLiveConfig(opts); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runLive(q, opts, stdout, stderr, ctx.Done())
	}
	res, err := load(q, opts, stderr)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		if opts.Debug {
			fmt.Fprintln(stderr, "warning:", warningString(w))
		}
	}
	out, closeOut, err := outputWriter(stdout, opts.OutputPath)
	if err != nil {
		return err
	}
	if closeOut != nil {
		defer closeOut()
	}
	return Render(out, res, q, opts)
}

func validateLiveConfig(opts RenderOptions) error {
	if opts.OutputPath != "" || (opts.Format != "table" && opts.Format != "pretty") {
		return fmt.Errorf("--live requires table or pretty output to a terminal")
	}
	return nil
}

func parseArgs(args []string) (core.Query, RenderOptions, bool, error) {
	q := core.Query{
		View:          "daily",
		Sources:       core.AllSources(),
		Location:      time.Local,
		Order:         "desc",
		Mode:          "auto",
		SessionLength: 5 * time.Hour,
		SourcePaths:   map[string][]string{},
		Speed:         "auto",
		StartOfWeek:   "monday",
	}
	opts := RenderOptions{
		Format:          "table",
		progress:        true,
		refreshInterval: 5 * time.Second,
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		return q, opts, false, flag.ErrHelp
	}
	fs := flag.NewFlagSet("llmut", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var since, until, timezone, sourcePath, claudePath, codexPath, opencodePath, ampPath, piPath, tokenLimit, sessionLength, refresh, fields, configPath string
	var ignoredCache bool
	var ignoredLocale string
	var jsonOut bool
	var live bool
	fs.BoolVar(&jsonOut, "json", false, "emit JSON")
	fs.BoolVar(&jsonOut, "j", false, "emit JSON")
	fs.StringVar(&opts.Format, "format", "table", "output format: table, pretty, json, csv, html")
	fs.StringVar(&opts.OutputPath, "output", "", "write report to file")
	fs.StringVar(&opts.OutputPath, "o", "", "write report to file")
	fs.BoolVar(&opts.Breakdown, "breakdown", false, "show per-model breakdown")
	fs.BoolVar(&opts.Breakdown, "b", false, "show per-model breakdown")
	fs.BoolVar(&q.Instances, "instances", false, "group by project")
	fs.BoolVar(&q.Instances, "i", false, "group by project")
	fs.BoolVar(&q.Active, "active", false, "only active block")
	fs.BoolVar(&q.Active, "a", false, "only active block")
	fs.BoolVar(&q.Recent, "recent", false, "recent blocks")
	fs.BoolVar(&q.Recent, "r", false, "recent blocks")
	fs.BoolVar(&opts.Compact, "compact", false, "compact output")
	fs.BoolVar(&opts.Debug, "debug", false, "debug")
	fs.BoolVar(&live, "live", false, "refresh output until interrupted")
	fs.BoolVar(&opts.progress, "progress", true, "show scan progress on stderr")
	fs.BoolFunc("no-progress", "disable scan progress", func(string) error {
		opts.progress = false
		return nil
	})
	fs.BoolVar(&ignoredCache, "cache", false, "no-op, accepted for ccusage statusline compatibility")
	fs.Bool("offline", false, "no-op, accepted for ccusage compatibility (pricing is always offline)")
	fs.Bool("O", false, "no-op, accepted for ccusage compatibility (pricing is always offline)")
	fs.StringVar(&since, "since", "", "start date")
	fs.StringVar(&until, "until", "", "end date")
	fs.StringVar(&timezone, "timezone", "", "timezone")
	fs.StringVar(&timezone, "z", "", "timezone")
	fs.StringVar(&q.Order, "order", "desc", "asc or desc")
	fs.StringVar(&q.StartOfWeek, "start-of-week", "monday", "monday or sunday")
	fs.StringVar(&q.Mode, "mode", "auto", "auto, calculate, display")
	fs.StringVar(&q.Project, "project", "", "project filter")
	fs.StringVar(&q.Project, "p", "", "project filter")
	fs.StringVar(&q.ID, "id", "", "session id filter")
	fs.IntVar(&q.Top, "top", 0, "limit rows for summary view; defaults to 10")
	fs.StringVar(&sourcePath, "path", "", "source path")
	fs.StringVar(&claudePath, "claude-path", "", "Claude projects path")
	fs.StringVar(&codexPath, "codex-path", "", "Codex home or sessions path")
	fs.StringVar(&opencodePath, "opencode-path", "", "OpenCode data path")
	fs.StringVar(&ampPath, "amp-path", "", "Amp data path")
	fs.StringVar(&piPath, "pi-path", "", "pi-agent sessions path")
	fs.StringVar(&tokenLimit, "token-limit", "", "token warning limit")
	fs.StringVar(&sessionLength, "session-length", "5", "block length hours")
	fs.StringVar(&refresh, "refresh-interval", "5", "refresh interval seconds")
	fs.StringVar(&q.Speed, "speed", "auto", "codex pricing speed: auto, standard, fast")
	fs.StringVar(&fields, "fields", "", "comma-separated columns for table, pretty, and csv output; JSON always emits full rows")
	fs.StringVar(&ignoredLocale, "locale", "", "no-op, accepted for ccusage compatibility")
	fs.Int("debug-samples", 0, "no-op, accepted for ccusage compatibility")
	fs.StringVar(&configPath, "config", "", "config file path")
	// Allow flags and the positional [source] [view] arguments to appear in any
	// order. Go's flag package stops parsing at the first non-flag token, so
	// loop, collecting the leftover positionals between flag groups. This makes
	// `llmut --since 2026-05-01 daily` behave the same as `llmut daily --since 2026-05-01`.
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return q, opts, live, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	explicit := explicitFlags(fs)
	if configPath == "" {
		configPath = defaultConfigPath()
	}
	if len(positionals) > 0 && sources[positionals[0]] {
		q.Sources = []string{positionals[0]}
		explicit["source"] = true
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && views[positionals[0]] {
		q.View = positionals[0]
		explicit["view"] = true
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && sources[positionals[0]] {
		return q, opts, live, fmt.Errorf("source must appear before view, for example: llmut %s daily", positionals[0])
	}
	if len(positionals) > 0 {
		return q, opts, live, fmt.Errorf("unexpected argument: %s", positionals[0])
	}
	if err := applyConfigDefaults(configPath, explicit, &q, &opts, &since, &until, &timezone, &sourcePath, &claudePath, &codexPath, &opencodePath, &ampPath, &piPath, &tokenLimit, &sessionLength, &refresh, &fields); err != nil {
		return q, opts, live, err
	}
	if jsonOut {
		opts.Format = "json"
	}
	if fields != "" {
		parsedFields, err := parseFields(fields)
		if err != nil {
			return q, opts, live, err
		}
		opts.Fields = parsedFields
	}
	if opts.Format != "table" && opts.Format != "pretty" && opts.Format != "json" && opts.Format != "csv" && opts.Format != "html" {
		return q, opts, live, fmt.Errorf("--format must be table, pretty, json, csv, or html")
	}
	if !views[q.View] {
		return q, opts, live, fmt.Errorf("view must be daily, weekly, monthly, session, summary, blocks, or statusline")
	}
	for _, source := range q.Sources {
		if !sources[source] {
			return q, opts, live, fmt.Errorf("source must be claude, codex, opencode, amp, or pi")
		}
	}
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return q, opts, live, err
		}
		q.Location = loc
	}
	var err error
	if q.Since, err = parseDateBound(since, q.Location, false); err != nil {
		return q, opts, live, err
	}
	if q.Until, err = parseDateBound(until, q.Location, true); err != nil {
		return q, opts, live, err
	}
	if q.Order != "asc" && q.Order != "desc" {
		return q, opts, live, fmt.Errorf("--order must be asc or desc")
	}
	if q.StartOfWeek != "monday" && q.StartOfWeek != "sunday" {
		return q, opts, live, fmt.Errorf("--start-of-week must be monday or sunday")
	}
	if q.Mode != "auto" && q.Mode != "calculate" && q.Mode != "display" {
		return q, opts, live, fmt.Errorf("--mode must be auto, calculate, or display")
	}
	if q.Speed != "auto" && q.Speed != "standard" && q.Speed != "fast" {
		return q, opts, live, fmt.Errorf("--speed must be auto, standard, or fast")
	}
	if q.Speed == "auto" {
		q.Speed = core.DetectCodexSpeed()
	}
	if tokenLimit != "" && tokenLimit != "max" {
		opts.TokenLimit, _ = strconv.ParseInt(tokenLimit, 10, 64)
	}
	if h, err := strconv.ParseFloat(sessionLength, 64); err == nil && h > 0 {
		q.SessionLength = time.Duration(h * float64(time.Hour))
	}
	if sec, err := strconv.ParseFloat(refresh, 64); err == nil && sec > 0 {
		opts.refreshInterval = time.Duration(sec * float64(time.Second))
	}
	addPath := func(source, value string) {
		if value != "" {
			q.SourcePaths[source] = core.ExpandList(value, "")
		}
	}
	if sourcePath != "" && len(q.Sources) == 1 {
		addPath(q.Sources[0], sourcePath)
	}
	addPath(core.SourceClaude, claudePath)
	addPath(core.SourceCodex, codexPath)
	addPath(core.SourceOpenCode, opencodePath)
	addPath(core.SourceAmp, ampPath)
	addPath(core.SourcePI, piPath)
	if q.View == "statusline" {
		q.Sources = []string{core.SourceClaude}
		q.View = "blocks"
		q.Active = true
		opts.Compact = true
	}
	if q.View == "blocks" {
		q.Sources = []string{core.SourceClaude}
	}
	q.Tolerant = opts.Debug
	return q, opts, live, nil
}

func explicitFlags(fs *flag.FlagSet) map[string]bool {
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})
	if explicit["j"] {
		explicit["json"] = true
	}
	if explicit["json"] {
		explicit["format"] = true
	}
	if explicit["o"] {
		explicit["output"] = true
	}
	if explicit["b"] {
		explicit["breakdown"] = true
	}
	if explicit["i"] {
		explicit["instances"] = true
	}
	if explicit["a"] {
		explicit["active"] = true
	}
	if explicit["r"] {
		explicit["recent"] = true
	}
	if explicit["z"] {
		explicit["timezone"] = true
	}
	if explicit["p"] {
		explicit["project"] = true
	}
	if explicit["no-progress"] {
		explicit["progress"] = true
	}
	return explicit
}

func applyConfigDefaults(path string, explicit map[string]bool, q *core.Query, opts *RenderOptions, since, until, timezone, sourcePath, claudePath, codexPath, opencodePath, ampPath, piPath, tokenLimit, sessionLength, refresh, fields *string) error {
	cfg, ok, err := loadConfigFile(path)
	if err != nil || !ok {
		return err
	}
	if !explicit["source"] {
		switch {
		case len(cfg.Sources) > 0:
			q.Sources = append([]string(nil), cfg.Sources...)
		case cfg.Source != "":
			q.Sources = []string{cfg.Source}
		}
	}
	if cfg.View != "" && !explicit["view"] {
		q.View = cfg.View
	}
	if cfg.Format != "" && !explicit["format"] {
		opts.Format = cfg.Format
	}
	if cfg.Output != "" && !explicit["output"] {
		opts.OutputPath = cfg.Output
	}
	if cfg.Breakdown != nil && !explicit["breakdown"] {
		opts.Breakdown = *cfg.Breakdown
	}
	if cfg.Instances != nil && !explicit["instances"] {
		q.Instances = *cfg.Instances
	}
	if cfg.Active != nil && !explicit["active"] {
		q.Active = *cfg.Active
	}
	if cfg.Recent != nil && !explicit["recent"] {
		q.Recent = *cfg.Recent
	}
	if cfg.Compact != nil && !explicit["compact"] {
		opts.Compact = *cfg.Compact
	}
	if cfg.Debug != nil && !explicit["debug"] {
		opts.Debug = *cfg.Debug
	}
	if cfg.Progress != nil && !explicit["progress"] {
		opts.progress = *cfg.Progress
	}
	if cfg.Since != "" && !explicit["since"] {
		*since = cfg.Since
	}
	if cfg.Until != "" && !explicit["until"] {
		*until = cfg.Until
	}
	if cfg.Timezone != "" && !explicit["timezone"] {
		*timezone = cfg.Timezone
	}
	if cfg.Order != "" && !explicit["order"] {
		q.Order = cfg.Order
	}
	if cfg.StartOfWeek != "" && !explicit["start-of-week"] {
		q.StartOfWeek = cfg.StartOfWeek
	}
	if cfg.Mode != "" && !explicit["mode"] {
		q.Mode = cfg.Mode
	}
	if cfg.Project != "" && !explicit["project"] {
		q.Project = cfg.Project
	}
	if cfg.ID != "" && !explicit["id"] {
		q.ID = cfg.ID
	}
	if cfg.Top != nil && !explicit["top"] {
		q.Top = *cfg.Top
	}
	if cfg.Path != "" && !explicit["path"] {
		*sourcePath = cfg.Path
	}
	if cfg.ClaudePath != "" && !explicit["claude-path"] {
		*claudePath = cfg.ClaudePath
	}
	if cfg.CodexPath != "" && !explicit["codex-path"] {
		*codexPath = cfg.CodexPath
	}
	if cfg.OpenCodePath != "" && !explicit["opencode-path"] {
		*opencodePath = cfg.OpenCodePath
	}
	if cfg.AmpPath != "" && !explicit["amp-path"] {
		*ampPath = cfg.AmpPath
	}
	if cfg.PIPath != "" && !explicit["pi-path"] {
		*piPath = cfg.PIPath
	}
	if cfg.TokenLimit != "" && !explicit["token-limit"] {
		*tokenLimit = string(cfg.TokenLimit)
	}
	if cfg.SessionLength != "" && !explicit["session-length"] {
		*sessionLength = string(cfg.SessionLength)
	}
	if cfg.RefreshInterval != "" && !explicit["refresh-interval"] {
		*refresh = string(cfg.RefreshInterval)
	}
	if cfg.Speed != "" && !explicit["speed"] {
		q.Speed = cfg.Speed
	}
	if cfg.Fields != "" && !explicit["fields"] {
		*fields = cfg.Fields
	}
	for source, value := range cfg.Paths {
		switch source {
		case core.SourceClaude:
			if !explicit["claude-path"] {
				*claudePath = value
			}
		case core.SourceCodex:
			if !explicit["codex-path"] {
				*codexPath = value
			}
		case core.SourceOpenCode:
			if !explicit["opencode-path"] {
				*opencodePath = value
			}
		case core.SourceAmp:
			if !explicit["amp-path"] {
				*ampPath = value
			}
		case core.SourcePI:
			if !explicit["pi-path"] {
				*piPath = value
			}
		}
	}
	return nil
}

func parseDateBound(s string, loc *time.Location, end bool) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	s = strings.TrimSpace(s)
	layout := "2006-01-02"
	if len(s) == 8 && !strings.Contains(s, "-") {
		layout = "20060102"
	}
	t, err := time.ParseInLocation(layout, s, loc)
	if err != nil {
		return time.Time{}, err
	}
	if end {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `llmut tracks local coding-agent LLM usage.

Usage:
  llmut [source] [view] [options]
  llmut completion bash|zsh|fish

Sources:
  claude, codex, opencode, amp, pi

Views:
  daily, weekly, monthly, session, summary, blocks, statusline

Examples:
  llmut daily --since 2026-05-01 --breakdown
  llmut summary --since 2026-05-01 --top 10
  llmut codex monthly --json --speed fast
  llmut daily --format html --output usage.html
  llmut daily --format csv --output usage.csv
  llmut daily --no-progress
  llmut claude blocks --active
  llmut claude blocks --active --live
  llmut daily --live --refresh-interval 2
  llmut pi session --pi-path ~/.pi/agent/sessions

Options:
  --fields <list> selects ordered table/pretty/csv columns. Valid fields:
    key, input, output, cache_cr, cache_rd, reasoning, total, cost, credits, models,
    source, session_id, project, start, last
    JSON output always emits the full structured row.
  --config <path> reads JSON defaults. If absent, llmut checks $LLMUT_CONFIG,
    then $XDG_CONFIG_HOME/llmut/config.json, then ~/.config/llmut/config.json.

Completions:
  llmut completion bash
  llmut completion zsh
  llmut completion fish`)
}

func printCompletion(args []string, w io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("completion shell required; supported shells: bash, zsh, fish")
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(w, bashCompletion())
	case "zsh":
		fmt.Fprint(w, zshCompletion())
	case "fish":
		fmt.Fprint(w, fishCompletion())
	default:
		return fmt.Errorf("unsupported completion shell %q; supported shells: bash, zsh, fish", args[0])
	}
	return nil
}

func bashCompletion() string {
	return `# bash completion for llmut
_llmut() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  case "$prev" in
    --format) COMPREPLY=( $(compgen -W "table pretty json csv html" -- "$cur") ); return 0 ;;
    --order) COMPREPLY=( $(compgen -W "asc desc" -- "$cur") ); return 0 ;;
    --start-of-week) COMPREPLY=( $(compgen -W "monday sunday" -- "$cur") ); return 0 ;;
    --mode) COMPREPLY=( $(compgen -W "auto calculate display" -- "$cur") ); return 0 ;;
    --speed) COMPREPLY=( $(compgen -W "auto standard fast" -- "$cur") ); return 0 ;;
  esac
  if [[ "$cur" == -* ]]; then
    COMPREPLY=( $(compgen -W "--json -j --format --output -o --breakdown -b --instances -i --active -a --recent -r --compact --debug --live --progress --no-progress --cache --offline -O --since --until --timezone -z --order --start-of-week --mode --project -p --id --top --path --claude-path --codex-path --opencode-path --amp-path --pi-path --token-limit --session-length --refresh-interval --speed --fields --config --locale --debug-samples --help -h" -- "$cur") )
    return 0
  fi
  COMPREPLY=( $(compgen -W "completion claude codex opencode amp pi daily weekly monthly session summary blocks statusline" -- "$cur") )
}
complete -F _llmut llmut
`
}

func zshCompletion() string {
	return `#compdef llmut
_llmut() {
  local -a sources views flags
  sources=(claude codex opencode amp pi)
  views=(daily weekly monthly session summary blocks statusline)
  flags=(
    '--json[emit JSON]' '-j[emit JSON]' '--format[output format]:format:(table pretty json csv html)'
    '--output[write report to file]:file:_files' '-o[write report to file]:file:_files'
    '--breakdown[show per-model breakdown]' '-b[show per-model breakdown]'
    '--instances[group by project]' '-i[group by project]' '--active[only active block]' '-a[only active block]'
    '--recent[recent blocks]' '-r[recent blocks]' '--compact[compact output]' '--debug[debug]' '--live[live refresh]'
    '--progress[show progress]' '--no-progress[disable progress]' '--cache[compatibility no-op]' '--offline[compatibility no-op]' '-O[compatibility no-op]'
    '--since[start date]:date:' '--until[end date]:date:' '--timezone[timezone]:timezone:' '-z[timezone]:timezone:'
    '--order[sort order]:order:(asc desc)' '--start-of-week[start of week]:(monday sunday)'
    '--mode[cost mode]:mode:(auto calculate display)' '--project[project filter]:project:' '-p[project filter]:project:'
    '--id[session id filter]:id:' '--top[row limit]:count:' '--path[source path]:path:_files'
    '--claude-path[Claude projects path]:path:_files' '--codex-path[Codex path]:path:_files'
    '--opencode-path[OpenCode path]:path:_files' '--amp-path[Amp path]:path:_files' '--pi-path[pi-agent path]:path:_files'
    '--token-limit[token warning limit]:tokens:' '--session-length[block length hours]:hours:'
    '--refresh-interval[refresh seconds]:seconds:' '--speed[codex speed]:speed:(auto standard fast)'
    '--fields[column list]:fields:' '--config[config file]:file:_files' '--locale[compatibility no-op]:locale:'
    '--debug-samples[compatibility no-op]:count:' '--help[help]' '-h[help]'
  )
  _arguments -C \
    '1:source, view, or subcommand:((completion\:completion claude\:Claude codex\:Codex opencode\:OpenCode amp\:Amp pi\:pi daily\:daily weekly\:weekly monthly\:monthly session\:session summary\:summary blocks\:blocks statusline\:statusline))' \
    '2:source, view, or shell:((bash\:bash zsh\:zsh fish\:fish daily\:daily weekly\:weekly monthly\:monthly session\:session summary\:summary blocks\:blocks statusline\:statusline))' \
    $flags
}
_llmut "$@"
`
}

func fishCompletion() string {
	return `# fish completion for llmut
complete -c llmut -f
complete -c llmut -n '__fish_use_subcommand' -a 'completion claude codex opencode amp pi daily weekly monthly session summary blocks statusline'
complete -c llmut -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c llmut -l json -s j -d 'emit JSON'
complete -c llmut -l format -a 'table pretty json csv html' -d 'output format'
complete -c llmut -l output -s o -r -d 'write report to file'
complete -c llmut -l breakdown -s b -d 'show per-model breakdown'
complete -c llmut -l instances -s i -d 'group by project'
complete -c llmut -l active -s a -d 'only active block'
complete -c llmut -l recent -s r -d 'recent blocks'
complete -c llmut -l compact -d 'compact output'
complete -c llmut -l debug -d 'debug'
complete -c llmut -l live -d 'refresh output'
complete -c llmut -l progress -d 'show progress'
complete -c llmut -l no-progress -d 'disable progress'
complete -c llmut -l since -r -d 'start date'
complete -c llmut -l until -r -d 'end date'
complete -c llmut -l timezone -s z -r -d 'timezone'
complete -c llmut -l order -a 'asc desc' -d 'sort order'
complete -c llmut -l start-of-week -a 'monday sunday' -d 'start of week'
complete -c llmut -l mode -a 'auto calculate display' -d 'cost mode'
complete -c llmut -l project -s p -r -d 'project filter'
complete -c llmut -l id -r -d 'session id filter'
complete -c llmut -l top -r -d 'limit rows'
complete -c llmut -l path -r -d 'source path'
complete -c llmut -l claude-path -r -d 'Claude projects path'
complete -c llmut -l codex-path -r -d 'Codex path'
complete -c llmut -l opencode-path -r -d 'OpenCode path'
complete -c llmut -l amp-path -r -d 'Amp path'
complete -c llmut -l pi-path -r -d 'pi-agent path'
complete -c llmut -l token-limit -r -d 'token warning limit'
complete -c llmut -l session-length -r -d 'block length hours'
complete -c llmut -l refresh-interval -r -d 'refresh seconds'
complete -c llmut -l speed -a 'auto standard fast' -d 'codex pricing speed'
complete -c llmut -l fields -r -d 'column list'
complete -c llmut -l config -r -d 'config file'
complete -c llmut -l cache -d 'compatibility no-op'
complete -c llmut -l offline -d 'compatibility no-op'
complete -c llmut -s O -d 'compatibility no-op'
complete -c llmut -l locale -r -d 'compatibility no-op'
complete -c llmut -l debug-samples -r -d 'compatibility no-op'
complete -c llmut -l help -s h -d 'help'
`
}

func outputWriter(stdout io.Writer, path string) (io.Writer, func() error, error) {
	if path == "" || path == "-" {
		return stdout, nil, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func title(q core.Query) string {
	scope := "All Sources"
	if len(q.Sources) == 1 {
		scope = core.SourceLabel(q.Sources[0])
	}
	view := q.View
	if view != "" {
		view = strings.ToUpper(view[:1]) + view[1:]
	}
	return fmt.Sprintf("Coding Agent Usage Report - %s - %s", view, scope)
}

func load(q core.Query, opts RenderOptions, stderr io.Writer) (core.Result, error) {
	var indicator *progressIndicator
	var onProgress core.Progress
	if opts.progress && writerIsTerminal(stderr) {
		indicator = newProgressIndicator(stderr)
		indicator.Start()
		defer indicator.Close()
		onProgress = func(stage string) {
			if strings.HasPrefix(stage, "Scanned ") || strings.HasPrefix(stage, "Calculated ") {
				indicator.Done(stage)
				return
			}
			indicator.Set(stage)
		}
	}
	return core.LoadAndAggregate(q, onProgress)
}

func warningString(w core.Warning) string {
	if w.Source == "" {
		return w.Message
	}
	return w.Source + ": " + w.Message
}
