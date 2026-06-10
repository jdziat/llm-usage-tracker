package usage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseArgsAllowsFlagsBeforePositionals(t *testing.T) {
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		args []string
	}{
		{"flag after view", []string{"daily", "--since", "2026-05-01"}},
		{"flag before view", []string{"--since", "2026-05-01", "daily"}},
		{"equals form before view", []string{"--since=2026-05-01", "daily"}},
		{"flag between source and view", []string{"claude", "--since", "2026-05-01", "daily"}},
		{"flag first with later flag", []string{"--since", "2026-05-01", "summary", "--top", "5"}},
		{"flag before source and view", []string{"--since", "2026-05-01", "claude", "daily"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _, _, err := parseArgs(tc.args)
			if err != nil {
				t.Fatalf("parseArgs(%v) returned error: %v", tc.args, err)
			}
			if !q.Since.Equal(want) {
				t.Fatalf("parseArgs(%v) Since = %v, want %v", tc.args, q.Since, want)
			}
		})
	}
}

func TestParseArgsRejectsSourceAfterView(t *testing.T) {
	if _, _, _, err := parseArgs([]string{"daily", "claude"}); err == nil {
		t.Fatal("expected error for source after view, got nil")
	}
}

func TestParseArgsRejectsUnknownPositional(t *testing.T) {
	if _, _, _, err := parseArgs([]string{"daily", "bogus"}); err == nil {
		t.Fatal("expected error for unknown positional, got nil")
	}
}

func TestParseArgsRejectsUnknownField(t *testing.T) {
	_, _, _, err := parseArgs([]string{"--fields", "bogus,cost"})
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), "valid fields") {
		t.Fatalf("error = %q, want valid fields message", err.Error())
	}
}

func TestParseArgsByFlag(t *testing.T) {
	q, _, _, err := parseArgs([]string{"summary", "--by", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if q.By != "project" {
		t.Fatalf("By = %q, want project", q.By)
	}
	if _, _, _, err := parseArgs([]string{"summary", "--by", "bogus"}); err == nil {
		t.Fatal("expected invalid --by error, got nil")
	}
}

func TestConfigPrecedenceAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{
		"source":"claude",
		"view":"summary",
		"format":"pretty",
		"timezone":"UTC",
		"order":"asc",
		"start_of_week":"sunday",
		"mode":"calculate",
		"by":"source",
		"speed":"standard",
		"top":5,
		"session_length":5,
		"fields":"models,cost,total",
		"budget":{"budget":25,"period":"week","token_budget":5000,"exit":true}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	q, opts, _, err := parseArgs([]string{"--config", configPath, "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Sources; len(got) != 1 || got[0] != "claude" {
		t.Fatalf("sources = %v, want config source claude", got)
	}
	if q.View != "summary" {
		t.Fatalf("view = %q, want config view summary", q.View)
	}
	if opts.Format != "json" {
		t.Fatalf("format = %q, want explicit flag json", opts.Format)
	}
	if q.Order != "asc" || q.StartOfWeek != "sunday" || q.Mode != "calculate" || q.Speed != "standard" || q.Top != 5 {
		t.Fatalf("config defaults not applied: q=%+v", q)
	}
	if q.By != "source" {
		t.Fatalf("by = %q, want source", q.By)
	}
	if q.Location.String() != "UTC" {
		t.Fatalf("timezone = %q, want UTC", q.Location)
	}
	if q.SessionLength != 5*time.Hour {
		t.Fatalf("session length = %v, want 5h", q.SessionLength)
	}
	if got := strings.Join(opts.Fields, ","); got != "models,cost,total" {
		t.Fatalf("fields = %q, want config fields", got)
	}
	if opts.Budget != 25 || opts.BudgetPeriod != "week" || opts.TokenBudget != 5000 || !opts.BudgetExit {
		t.Fatalf("budget config not applied: opts=%+v", opts)
	}

	q, opts, _, err = parseArgs([]string{"--config", filepath.Join(dir, "missing.json")})
	if err != nil {
		t.Fatal(err)
	}
	if q.View != "daily" || opts.Format != "table" {
		t.Fatalf("missing config changed defaults: view=%q format=%q", q.View, opts.Format)
	}
}

func TestCompletion(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run([]string{"completion", shell}, &out, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(out.String()) == "" {
				t.Fatalf("%s completion was empty", shell)
			}
		})
	}
	var out bytes.Buffer
	err := Run([]string{"completion", "powershell"}, &out, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected invalid shell error, got nil")
	}
	if !strings.Contains(err.Error(), "supported shells") {
		t.Fatalf("error = %q, want supported shells", err.Error())
	}
}
