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
	var since, until, timezone, sourcePath, claudePath, codexPath, opencodePath, ampPath, piPath, tokenLimit, sessionLength, refresh string
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
	fs.StringVar(&ignoredLocale, "locale", "", "no-op, accepted for ccusage compatibility")
	fs.Int("debug-samples", 0, "no-op, accepted for ccusage compatibility")
	fs.String("config", "", "no-op, accepted for ccusage compatibility")
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
	if len(positionals) > 0 && sources[positionals[0]] {
		q.Sources = []string{positionals[0]}
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && views[positionals[0]] {
		q.View = positionals[0]
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && sources[positionals[0]] {
		return q, opts, live, fmt.Errorf("source must appear before view, for example: llmut %s daily", positionals[0])
	}
	if len(positionals) > 0 {
		return q, opts, live, fmt.Errorf("unexpected argument: %s", positionals[0])
	}
	if jsonOut {
		opts.Format = "json"
	}
	if opts.Format != "table" && opts.Format != "pretty" && opts.Format != "json" && opts.Format != "csv" && opts.Format != "html" {
		return q, opts, live, fmt.Errorf("--format must be table, pretty, json, csv, or html")
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
  llmut pi session --pi-path ~/.pi/agent/sessions`)
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
