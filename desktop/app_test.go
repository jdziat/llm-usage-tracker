package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

// TestGetUsageDefaultsNilLocation reproduces the frontend's call shape: a Query
// with a nil Location (the JSON bridge can't carry *time.Location) over a source
// that actually has events. Without the GetUsage guard this panics in
// filterEvents (time.Time.In(nil)); with it, aggregation succeeds.
func TestGetUsageDefaultsNilLocation(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "claude")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"requestId":"r1","type":"assistant","timestamp":"2026-06-09T10:00:00Z","cwd":"/work/demo","sessionId":"s1","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	q := core.Query{
		Sources:     []string{core.SourceClaude},
		SourcePaths: map[string][]string{core.SourceClaude: {dir}},
		View:        "summary",
		Order:       "desc",
		Mode:        "auto",
		Speed:       "standard",
		// Location intentionally nil — exactly what the frontend sends.
	}
	res, err := UsageService{}.GetUsage(q)
	if err != nil {
		t.Fatalf("GetUsage returned error: %v", err)
	}
	if len(res.Rows) == 0 {
		t.Fatal("expected at least one row from the fixture")
	}
	if res.Totals.Input != 1000 || res.Totals.Output != 500 {
		t.Fatalf("unexpected totals: %+v", res.Totals)
	}
}
