package usage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

func TestRunRejectsInvalidLiveOutputModes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"json", []string{"--live", "--format", "json"}},
		{"csv", []string{"--live", "--format", "csv"}},
		{"html", []string{"--live", "--format", "html"}},
		{"output", []string{"--live", "--output", filepath.Join(t.TempDir(), "usage.txt")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "--live") {
				t.Fatalf("error = %q, want mention of --live", err.Error())
			}
		})
	}
}

func TestRunLiveRendersMultipleFrames(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(projectDir, "session.jsonl")
	data := `{"requestId":"req_1","type":"assistant","timestamp":"2026-05-18T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}
`
	if err := os.WriteFile(logPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	q := core.Query{
		View:          "daily",
		Sources:       []string{core.SourceClaude},
		Location:      time.UTC,
		Order:         "asc",
		Mode:          "display",
		SessionLength: 5 * time.Hour,
		SourcePaths:   map[string][]string{core.SourceClaude: {root}},
		Speed:         "standard",
		StartOfWeek:   "monday",
	}
	opts := RenderOptions{
		Format:          "table",
		progress:        true,
		refreshInterval: 10 * time.Millisecond,
	}
	var out bytes.Buffer
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runLive(q, opts, &out, bytes.NewBuffer(nil), stop)
	}()

	time.Sleep(35 * time.Millisecond)
	close(stop)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("live loop did not stop")
	}

	got := out.String()
	if count := strings.Count(got, terminalClear); count < 2 {
		t.Fatalf("rendered %d frame(s), want at least 2; output:\n%s", count, got)
	}
	if !strings.Contains(got, "Coding Agent Usage Report") {
		t.Fatalf("output did not include report title:\n%s", got)
	}
}
