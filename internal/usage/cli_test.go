package usage

import (
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
			cfg, err := parseArgs(tc.args)
			if err != nil {
				t.Fatalf("parseArgs(%v) returned error: %v", tc.args, err)
			}
			if !cfg.Since.Equal(want) {
				t.Fatalf("parseArgs(%v) Since = %v, want %v", tc.args, cfg.Since, want)
			}
		})
	}
}

func TestParseArgsRejectsSourceAfterView(t *testing.T) {
	if _, err := parseArgs([]string{"daily", "claude"}); err == nil {
		t.Fatal("expected error for source after view, got nil")
	}
}

func TestParseArgsRejectsUnknownPositional(t *testing.T) {
	if _, err := parseArgs([]string{"daily", "bogus"}); err == nil {
		t.Fatal("expected error for unknown positional, got nil")
	}
}
