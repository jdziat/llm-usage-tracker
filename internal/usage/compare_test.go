package usage

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

func TestBaselineQueryPreviousExplicitWindow(t *testing.T) {
	q, _, _, err := parseArgs([]string{"daily", "--since", "2026-05-10", "--until", "2026-05-12", "--compare", "previous", "--timezone", "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	current, baseline, err := baselineQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Since.Equal(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("current since = %v", current.Since)
	}
	if !current.Until.Equal(time.Date(2026, 5, 12, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)) {
		t.Fatalf("current until = %v", current.Until)
	}
	if want := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC); !baseline.Since.Equal(want) {
		t.Fatalf("baseline since = %v, want %v", baseline.Since, want)
	}
	if want := time.Date(2026, 5, 9, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC); !baseline.Until.Equal(want) {
		t.Fatalf("baseline until = %v, want %v", baseline.Until, want)
	}
}

func TestBaselineQueryPreviousExplicitWindowAcrossDST(t *testing.T) {
	q, _, _, err := parseArgs([]string{"daily", "--since", "2026-03-09", "--until", "2026-03-15", "--compare", "previous", "--timezone", "America/New_York"})
	if err != nil {
		t.Fatal(err)
	}
	_, baseline, err := baselineQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 2, 0, 0, 0, 0, q.Location)
	if !baseline.Since.Equal(want) {
		t.Fatalf("baseline since = %v, want %v", baseline.Since, want)
	}
	if baseline.Since.Hour() != 0 {
		t.Fatalf("baseline since hour = %d, want midnight: %v", baseline.Since.Hour(), baseline.Since)
	}
}

func TestBaselineQueryExplicitRange(t *testing.T) {
	q, _, _, err := parseArgs([]string{"daily", "--since", "2026-06-01", "--until", "2026-06-30", "--compare", "2026-05-01..2026-05-31", "--timezone", "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	current, baseline, err := baselineQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Since.Equal(q.Since) || !current.Until.Equal(q.Until) {
		t.Fatalf("explicit range changed current window: current=%v..%v q=%v..%v", current.Since, current.Until, q.Since, q.Until)
	}
	if want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC); !baseline.Since.Equal(want) {
		t.Fatalf("baseline since = %v, want %v", baseline.Since, want)
	}
	if want := time.Date(2026, 5, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC); !baseline.Until.Equal(want) {
		t.Fatalf("baseline until = %v, want %v", baseline.Until, want)
	}
}

func TestParseArgsRejectsCompareForBlocksAndStatusline(t *testing.T) {
	for _, view := range []string{"blocks", "statusline"} {
		t.Run(view, func(t *testing.T) {
			_, _, _, err := parseArgs([]string{view, "--compare", "previous"})
			if err == nil || !strings.Contains(err.Error(), "--compare is not supported") {
				t.Fatalf("err = %v, want unsupported compare error", err)
			}
		})
	}
}

func TestRenderDiffTableShape(t *testing.T) {
	diff := core.DiffResult{
		View: "daily",
		Rows: []core.DiffRow{{
			Key:      "2026-06-01",
			Current:  core.Tokens{CostUSD: 12},
			Baseline: core.Tokens{CostUSD: 10},
		}},
		CurrentTotals:  core.Tokens{CostUSD: 12},
		BaselineTotals: core.Tokens{CostUSD: 10},
	}
	var buf bytes.Buffer
	if err := RenderDiff(&buf, diff, core.Query{View: "daily", Compare: "previous"}, RenderOptions{Format: "table"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Date", "Cost", "Prev", "Δ Cost", "Δ %", "Total"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff table missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "+$2.0000") || !strings.Contains(out, "+20.0%") {
		t.Fatalf("diff table missing formatted deltas:\n%s", out)
	}
}

func TestRenderDiffJSONComparisonObject(t *testing.T) {
	diff := core.DiffResult{
		View: "summary",
		Rows: []core.DiffRow{{
			Key:      "model-a",
			Current:  core.Tokens{Input: 1, CostUSD: 5},
			Baseline: core.Tokens{Input: 2, CostUSD: 0},
		}},
		CurrentTotals:  core.Tokens{Input: 1, CostUSD: 5},
		BaselineTotals: core.Tokens{Input: 2, CostUSD: 0},
	}
	var buf bytes.Buffer
	if err := RenderDiff(&buf, diff, core.Query{View: "summary", Compare: "previous"}, RenderOptions{Format: "json"}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Totals     core.Tokens `json:"totals"`
		Comparison struct {
			BaselineTotals core.Tokens `json:"baselineTotals"`
			Rows           []struct {
				Key          string      `json:"key"`
				Current      core.Tokens `json:"current"`
				Baseline     core.Tokens `json:"baseline"`
				DeltaCost    float64     `json:"deltaCost"`
				PctCost      float64     `json:"pctCost"`
				BaselineZero bool        `json:"baselineZero"`
			} `json:"rows"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Comparison.BaselineTotals.Input != 2 || len(got.Comparison.Rows) != 1 {
		t.Fatalf("comparison object missing baseline totals/rows: %+v", got)
	}
	row := got.Comparison.Rows[0]
	if row.Key != "model-a" || row.Current.CostUSD != 5 || row.Baseline.CostUSD != 0 || row.DeltaCost != 5 || row.PctCost != 100 || !row.BaselineZero {
		t.Fatalf("unexpected comparison row: %+v", row)
	}
}

func TestDiffFormattingZeroUnsigned(t *testing.T) {
	row := core.DiffRow{Current: core.Tokens{CostUSD: 5}, Baseline: core.Tokens{CostUSD: 5}}
	cells := diffCells(row)
	if cells[3] != "$0.0000" || cells[4] != "0.0%" {
		t.Fatalf("zero deltas should be unsigned: %v", cells)
	}
}

func TestDiffCurrentRowsSkipsBaselineOnlyRows(t *testing.T) {
	rows := diffCurrentRows(core.DiffResult{Rows: []core.DiffRow{
		{Key: "current", Current: core.Tokens{Input: 1}},
		{Key: "baseline", Baseline: core.Tokens{Input: 1}},
	}})
	if len(rows) != 1 || rows[0].Key != "current" {
		t.Fatalf("fallback rows = %+v, want current rows only", rows)
	}
}

func TestRunCompareCSVWarnsAndKeepsStdoutClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"opencode", "summary", "--path", t.TempDir(), "--since", "2026-06-01", "--until", "2026-06-30", "--compare", "previous", "--format", "csv", "--no-progress"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "warning: --compare is not supported for csv/html output; emitting current period only.") {
		t.Fatalf("stderr missing compare fallback warning: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "warning:") {
		t.Fatalf("stdout should contain data only, got %q", stdout.String())
	}
}
