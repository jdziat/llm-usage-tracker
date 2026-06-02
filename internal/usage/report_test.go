package usage

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func sampleRows() []Row {
	return []Row{{
		Key:          "2026-05-18",
		Source:       "codex",
		SessionID:    "s1",
		Project:      "/work/repo",
		Start:        time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
		LastActivity: time.Date(2026, 5, 18, 10, 5, 0, 0, time.UTC),
		ModelsUsed:   []string{"gpt-5.5"},
		Tokens: Tokens{
			Input:     10,
			Output:    3,
			CacheRead: 20,
			CostUSD:   0.001,
		},
	}}
}

func TestWriteCSVReport(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCSV(&buf, Config{}, sampleRows()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "key,source,session_id") {
		t.Fatalf("missing CSV header: %s", out)
	}
	if !strings.Contains(out, "2026-05-18,codex,s1,/work/repo") {
		t.Fatalf("missing CSV row: %s", out)
	}
}

func TestWriteHTMLReport(t *testing.T) {
	var buf bytes.Buffer
	err := writeHTML(&buf, "Usage <Report>", Config{View: "daily"}, sampleRows(), []string{"warning <x>"})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!doctype html>") || !strings.Contains(out, "Usage &lt;Report&gt;") {
		t.Fatalf("missing escaped HTML title: %s", out)
	}
	if !strings.Contains(out, "warning &lt;x&gt;") {
		t.Fatalf("missing escaped warning: %s", out)
	}
}
