package usage

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

func sampleRows() []core.Row {
	return []core.Row{{
		Key:          "2026-05-18",
		Source:       "codex",
		SessionID:    "s1",
		Project:      "/work/repo",
		Start:        time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
		LastActivity: time.Date(2026, 5, 18, 10, 5, 0, 0, time.UTC),
		ModelsUsed:   []string{"gpt-5.5"},
		Tokens: core.Tokens{
			Input:     10,
			Output:    3,
			CacheRead: 20,
			CostUSD:   0.001,
		},
	}}
}

func TestWriteCSVReport(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCSV(&buf, RenderOptions{}, sampleRows()); err != nil {
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
	err := writeHTML(&buf, "Usage <Report>", core.Query{View: "daily"}, RenderOptions{}, core.Result{
		View:     "daily",
		Rows:     sampleRows(),
		Totals:   core.Tokens{Input: 10, Output: 3, CacheRead: 20, CostUSD: 0.001},
		Warnings: []core.Warning{{Message: "warning <x>"}},
	})
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

func TestFieldsSelectionAndOrdering(t *testing.T) {
	fields, err := parseFields("models,cost,total")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeCSV(&buf, RenderOptions{Fields: fields}, sampleRows()); err != nil {
		t.Fatal(err)
	}
	want := "models,cost_usd,total_tokens\ngpt-5.5,0.001000,33\n"
	if got := buf.String(); got != want {
		t.Fatalf("selected CSV fields mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}

	buf.Reset()
	Render(&buf, core.Result{Rows: sampleRows(), Totals: core.Tokens{Input: 10, Output: 3, CacheRead: 20, CostUSD: 0.001}}, core.Query{View: "daily"}, RenderOptions{Fields: fields})
	out := buf.String()
	modelIdx := strings.Index(out, "Models")
	costIdx := strings.Index(out, "Cost")
	totalIdx := strings.Index(out, "Total")
	if modelIdx < 0 || costIdx < 0 || totalIdx < 0 || !(modelIdx < costIdx && costIdx < totalIdx) {
		t.Fatalf("table did not honor selected field order:\n%s", out)
	}
}
