package usage

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var views = map[string]bool{"daily": true, "weekly": true, "monthly": true, "session": true, "summary": true, "blocks": true, "statusline": true}
var sources = map[string]bool{SourceClaude: true, SourceCodex: true, SourceOpenCode: true, SourceAmp: true, SourcePI: true}

func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(stdout)
			return nil
		}
		return err
	}
	if cfg.View == "statusline" {
		cfg.Sources = []string{SourceClaude}
		cfg.View = "blocks"
		cfg.Active = true
		cfg.Compact = true
	}
	if cfg.Progress && writerIsTerminal(stderr) {
		cfg.progress = newProgressIndicator(stderr)
		cfg.progress.Start()
		defer cfg.progress.Close()
	}
	events, warnings, err := loadEvents(cfg)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if cfg.Debug {
			fmt.Fprintln(stderr, "warning:", w)
		}
	}
	rows := aggregate(events, cfg)
	out, closeOut, err := outputWriter(stdout, cfg.OutputPath)
	if err != nil {
		return err
	}
	if closeOut != nil {
		defer closeOut()
	}
	switch cfg.Format {
	case "json":
		return writeJSON(out, cfg, rows, warnings)
	case "csv":
		return writeCSV(out, cfg, rows)
	case "html":
		return writeHTML(out, title(cfg), cfg, rows, warnings)
	}
	if cfg.View == "blocks" && cfg.Compact {
		writeStatusline(out, rows, cfg)
		return nil
	}
	if len(rows) == 0 {
		fmt.Fprintln(out, "No usage data found.")
		return nil
	}
	if cfg.Format == "pretty" {
		writePrettyTable(out, title(cfg), rows, cfg)
		return nil
	}
	writeTable(out, title(cfg), rows, cfg)
	return nil
}

func parseArgs(args []string) (Config, error) {
	cfg := Config{
		View:            "daily",
		Sources:         append([]string(nil), allSources...),
		Location:        time.Local,
		Order:           "desc",
		Mode:            "auto",
		SessionLength:   5 * time.Hour,
		RefreshInterval: 5 * time.Second,
		SourcePaths:     map[string][]string{},
		Speed:           "auto",
		StartOfWeek:     "monday",
		Format:          "table",
		Progress:        true,
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		return cfg, flag.ErrHelp
	}
	fs := flag.NewFlagSet("llmut", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var since, until, timezone, sourcePath, claudePath, codexPath, opencodePath, ampPath, piPath, tokenLimit, sessionLength, refresh string
	var ignoredCache bool
	var ignoredLocale string
	fs.BoolVar(&cfg.JSON, "json", false, "emit JSON")
	fs.BoolVar(&cfg.JSON, "j", false, "emit JSON")
	fs.StringVar(&cfg.Format, "format", "table", "output format: table, pretty, json, csv, html")
	fs.StringVar(&cfg.OutputPath, "output", "", "write report to file")
	fs.StringVar(&cfg.OutputPath, "o", "", "write report to file")
	fs.BoolVar(&cfg.Breakdown, "breakdown", false, "show per-model breakdown")
	fs.BoolVar(&cfg.Breakdown, "b", false, "show per-model breakdown")
	fs.BoolVar(&cfg.Instances, "instances", false, "group by project")
	fs.BoolVar(&cfg.Instances, "i", false, "group by project")
	fs.BoolVar(&cfg.Active, "active", false, "only active block")
	fs.BoolVar(&cfg.Active, "a", false, "only active block")
	fs.BoolVar(&cfg.Recent, "recent", false, "recent blocks")
	fs.BoolVar(&cfg.Recent, "r", false, "recent blocks")
	fs.BoolVar(&cfg.Compact, "compact", false, "compact output")
	fs.BoolVar(&cfg.Debug, "debug", false, "debug")
	fs.BoolVar(&cfg.Live, "live", false, "refresh output until interrupted")
	fs.BoolVar(&cfg.Progress, "progress", true, "show scan progress on stderr")
	fs.BoolFunc("no-progress", "disable scan progress", func(string) error {
		cfg.Progress = false
		return nil
	})
	fs.BoolVar(&ignoredCache, "cache", false, "accepted for statusline compatibility")
	fs.Bool("offline", false, "use builtin/offline pricing")
	fs.Bool("O", false, "use builtin/offline pricing")
	fs.StringVar(&since, "since", "", "start date")
	fs.StringVar(&until, "until", "", "end date")
	fs.StringVar(&timezone, "timezone", "", "timezone")
	fs.StringVar(&timezone, "z", "", "timezone")
	fs.StringVar(&cfg.Order, "order", "desc", "asc or desc")
	fs.StringVar(&cfg.StartOfWeek, "start-of-week", "monday", "monday or sunday")
	fs.StringVar(&cfg.Mode, "mode", "auto", "auto, calculate, display")
	fs.StringVar(&cfg.Project, "project", "", "project filter")
	fs.StringVar(&cfg.Project, "p", "", "project filter")
	fs.StringVar(&cfg.ID, "id", "", "session id filter")
	fs.IntVar(&cfg.Top, "top", 0, "limit rows for summary view; defaults to 10")
	fs.StringVar(&sourcePath, "path", "", "source path")
	fs.StringVar(&claudePath, "claude-path", "", "Claude projects path")
	fs.StringVar(&codexPath, "codex-path", "", "Codex home or sessions path")
	fs.StringVar(&opencodePath, "opencode-path", "", "OpenCode data path")
	fs.StringVar(&ampPath, "amp-path", "", "Amp data path")
	fs.StringVar(&piPath, "pi-path", "", "pi-agent sessions path")
	fs.StringVar(&tokenLimit, "token-limit", "", "token warning limit")
	fs.StringVar(&sessionLength, "session-length", "5", "block length hours")
	fs.StringVar(&refresh, "refresh-interval", "5", "refresh interval seconds")
	fs.StringVar(&cfg.Speed, "speed", "auto", "codex pricing speed: auto, standard, fast")
	fs.StringVar(&ignoredLocale, "locale", "", "accepted for ccusage compatibility")
	fs.Int("debug-samples", 0, "debug sample count")
	fs.String("config", "", "config file placeholder")
	// Allow flags and the positional [source] [view] arguments to appear in any
	// order. Go's flag package stops parsing at the first non-flag token, so
	// loop, collecting the leftover positionals between flag groups. This makes
	// `llmut --since 2026-05-01 daily` behave the same as `llmut daily --since 2026-05-01`.
	var positionals []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return cfg, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	if len(positionals) > 0 && sources[positionals[0]] {
		cfg.Sources = []string{positionals[0]}
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && views[positionals[0]] {
		cfg.View = positionals[0]
		positionals = positionals[1:]
	}
	if len(positionals) > 0 && sources[positionals[0]] {
		return cfg, fmt.Errorf("source must appear before view, for example: llmut %s daily", positionals[0])
	}
	if len(positionals) > 0 {
		return cfg, fmt.Errorf("unexpected argument: %s", positionals[0])
	}
	if cfg.JSON {
		cfg.Format = "json"
	}
	if cfg.Format != "table" && cfg.Format != "pretty" && cfg.Format != "json" && cfg.Format != "csv" && cfg.Format != "html" {
		return cfg, fmt.Errorf("--format must be table, pretty, json, csv, or html")
	}
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return cfg, err
		}
		cfg.Location = loc
	}
	var err error
	if cfg.Since, err = parseDateBound(since, cfg.Location, false); err != nil {
		return cfg, err
	}
	if cfg.Until, err = parseDateBound(until, cfg.Location, true); err != nil {
		return cfg, err
	}
	if cfg.Order != "asc" && cfg.Order != "desc" {
		return cfg, fmt.Errorf("--order must be asc or desc")
	}
	if cfg.StartOfWeek != "monday" && cfg.StartOfWeek != "sunday" {
		return cfg, fmt.Errorf("--start-of-week must be monday or sunday")
	}
	if cfg.Mode != "auto" && cfg.Mode != "calculate" && cfg.Mode != "display" {
		return cfg, fmt.Errorf("--mode must be auto, calculate, or display")
	}
	if cfg.Speed != "auto" && cfg.Speed != "standard" && cfg.Speed != "fast" {
		return cfg, fmt.Errorf("--speed must be auto, standard, or fast")
	}
	if cfg.Speed == "auto" {
		cfg.Speed = detectCodexSpeed()
	}
	if tokenLimit != "" && tokenLimit != "max" {
		cfg.TokenLimit, _ = strconv.ParseInt(tokenLimit, 10, 64)
	}
	if h, err := strconv.ParseFloat(sessionLength, 64); err == nil && h > 0 {
		cfg.SessionLength = time.Duration(h * float64(time.Hour))
	}
	if sec, err := strconv.ParseFloat(refresh, 64); err == nil && sec > 0 {
		cfg.RefreshInterval = time.Duration(sec * float64(time.Second))
	}
	addPath := func(source, value string) {
		if value != "" {
			cfg.SourcePaths[source] = expandList(value, "")
		}
	}
	if sourcePath != "" && len(cfg.Sources) == 1 {
		addPath(cfg.Sources[0], sourcePath)
	}
	addPath(SourceClaude, claudePath)
	addPath(SourceCodex, codexPath)
	addPath(SourceOpenCode, opencodePath)
	addPath(SourceAmp, ampPath)
	addPath(SourcePI, piPath)
	if cfg.View == "blocks" {
		cfg.Sources = []string{SourceClaude}
	}
	return cfg, nil
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

func detectCodexSpeed() string {
	for _, root := range defaultSourcePaths(SourceCodex) {
		config := strings.TrimSuffix(root, "/sessions") + "/config.toml"
		b, err := os.ReadFile(config)
		if err == nil {
			s := strings.ToLower(string(b))
			if strings.Contains(s, `service_tier = "priority"`) || strings.Contains(s, `service_tier = "fast"`) {
				return "fast"
			}
		}
	}
	return "standard"
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

func title(cfg Config) string {
	scope := "All Sources"
	if len(cfg.Sources) == 1 {
		scope = sourceLabel(cfg.Sources[0])
	}
	return fmt.Sprintf("Coding Agent Usage Report - %s - %s", strings.Title(cfg.View), scope)
}
