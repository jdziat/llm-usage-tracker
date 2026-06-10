package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadCodexUsesLastTokenUsageAndModelContext(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "05", "18")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(sessionDir, "rollout-2026-05-18T09-24-00-abc.jsonl")
	data := `{"timestamp":"2026-05-18T16:24:18Z","type":"turn_context","payload":{"cwd":"/work/repo","model":"gpt-5.5"}}
{"timestamp":"2026-05-18T16:24:25Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":9,"reasoning_output_tokens":3,"total_tokens":109},"last_token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":9,"reasoning_output_tokens":3,"total_tokens":109}}}}
{"timestamp":"2026-05-18T16:25:25Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":250,"cached_input_tokens":100,"output_tokens":15,"reasoning_output_tokens":4,"total_tokens":265},"last_token_usage":{"input_tokens":150,"cached_input_tokens":60,"output_tokens":6,"reasoning_output_tokens":1,"total_tokens":156}}}}
`
	if err := os.WriteFile(logPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readCodex([]string{filepath.Join(root, "sessions")}, Query{Speed: "standard"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if got := events[0].Tokens.Input; got != 60 {
		t.Fatalf("first non-cached input = %d, want 60", got)
	}
	if got := events[1].Tokens.Input; got != 90 {
		t.Fatalf("second non-cached input = %d, want 90", got)
	}
	if events[1].Tokens.CacheRead != 60 || events[1].Tokens.Output != 6 || events[1].Tokens.Reasoning != 1 {
		t.Fatalf("unexpected second tokens: %+v", events[1].Tokens)
	}
	if events[0].Model != "gpt-5.5" || events[0].Project != "/work/repo" {
		t.Fatalf("metadata not propagated: %+v", events[0])
	}
}

func TestReadClaudeDeduplicatesRepeatedRequestUsage(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(projectDir, "session.jsonl")
	line := `{"requestId":"req_1","type":"assistant","timestamp":"2026-05-18T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readClaude([]string{root}, Query{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 deduped event, got %d", len(events))
	}
	if events[0].Tokens.Input != 10 || events[0].Tokens.Output != 4 || events[0].Tokens.CacheCreation != 2 || events[0].Tokens.CacheRead != 3 {
		t.Fatalf("unexpected tokens: %+v", events[0].Tokens)
	}
}

func TestReadClaudeSkipsZeroTokenSyntheticEvents(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(projectDir, "session.jsonl")
	data := `{"requestId":"req_synth","type":"assistant","timestamp":"2026-05-18T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"<synthetic>","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0}}}
{"requestId":"req_real","type":"assistant","timestamp":"2026-05-18T10:01:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":2}}}
`
	if err := os.WriteFile(logPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readClaude([]string{root}, Query{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected only real event, got %d", len(events))
	}
	if events[0].Model == "<synthetic>" {
		t.Fatal("synthetic event was not filtered")
	}
}

func TestReadClaudeTolerantSkipsScannerErrorFile(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(projectDir, "session.jsonl")
	valid := `{"requestId":"req_1","type":"assistant","timestamp":"2026-05-18T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}`
	tooLong := strings.Repeat("x", 16*1024*1024+1)
	if err := os.WriteFile(logPath, []byte(valid+"\n{"+tooLong+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readClaude([]string{root}, Query{Tolerant: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected tolerant mode to skip scanner-error file, got %d events", len(events))
	}
}

func TestAggregateDailyRespectsTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{{
		Source:    SourceCodex,
		SessionID: "s1",
		Model:     "gpt-5.5",
		Time:      time.Date(2026, 5, 18, 23, 30, 0, 0, time.UTC),
		Tokens:    Tokens{Input: 1},
	}}
	rows := aggregate(events, Query{View: "daily", Sources: []string{SourceCodex}, Location: loc, Order: "asc"})
	if len(rows) != 1 || rows[0].Key != "2026-05-19" {
		t.Fatalf("rows = %+v, want Tokyo next-day key", rows)
	}
}
