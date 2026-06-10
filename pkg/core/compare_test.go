package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAndCompareJoinsRowsByKey(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "events.jsonl")
	data := `{"timestamp":"2026-06-15T12:00:00Z","model":"both","usage":{"inputTokens":1,"costUSD":10}}
{"timestamp":"2026-06-16T12:00:00Z","model":"current-only","usage":{"inputTokens":1,"costUSD":20}}
{"timestamp":"2026-05-15T12:00:00Z","model":"both","usage":{"inputTokens":1,"costUSD":4}}
{"timestamp":"2026-05-16T12:00:00Z","model":"baseline-only","usage":{"inputTokens":1,"costUSD":7}}
`
	if err := os.WriteFile(logPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	current := Query{
		View:        "summary",
		By:          "model",
		Sources:     []string{SourceOpenCode},
		SourcePaths: map[string][]string{SourceOpenCode: []string{root}},
		Since:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 6, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		Location:    time.UTC,
		Order:       "desc",
		Mode:        "display",
	}
	baseline := current
	baseline.Since = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	baseline.Until = time.Date(2026, 5, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)

	diff, err := LoadAndCompare(current, baseline, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Rows) != 3 {
		t.Fatalf("rows = %d, want 3: %+v", len(diff.Rows), diff.Rows)
	}
	got := map[string]DiffRow{}
	for _, row := range diff.Rows {
		got[row.Key] = row
	}
	if got["current-only"].Current.CostUSD != 20 || got["current-only"].Baseline.CostUSD != 0 {
		t.Fatalf("current-only row mismatch: %+v", got["current-only"])
	}
	if got["baseline-only"].Current.CostUSD != 0 || got["baseline-only"].Baseline.CostUSD != 7 {
		t.Fatalf("baseline-only row mismatch: %+v", got["baseline-only"])
	}
	if got["both"].Current.CostUSD != 10 || got["both"].Baseline.CostUSD != 4 {
		t.Fatalf("both row mismatch: %+v", got["both"])
	}
	if diff.Rows[0].Key != "current-only" || diff.Rows[1].Key != "both" || diff.Rows[2].Key != "baseline-only" {
		t.Fatalf("unexpected deterministic order: %+v", diff.Rows)
	}
}

func TestLoadAndCompareSummaryJoinsBeforeTop(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "events.jsonl")
	var data string
	currentTotal := 0.0
	for i := 0; i < 12; i++ {
		cost := float64(100 - i)
		currentTotal += cost
		data += fmt.Sprintf(`{"timestamp":"2026-06-15T12:00:00Z","model":"current-%02d","usage":{"inputTokens":1,"costUSD":%.0f}}`+"\n", i, cost)
	}
	baselineTotal := 1.0
	data += `{"timestamp":"2026-05-15T12:00:00Z","model":"current-00","usage":{"inputTokens":1,"costUSD":1}}` + "\n"
	for i := 0; i < 10; i++ {
		cost := float64(50 - i)
		baselineTotal += cost
		data += fmt.Sprintf(`{"timestamp":"2026-05-15T12:00:00Z","model":"baseline-%02d","usage":{"inputTokens":1,"costUSD":%.0f}}`+"\n", i, cost)
	}
	if err := os.WriteFile(logPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	current := Query{
		View:        "summary",
		By:          "model",
		Sources:     []string{SourceOpenCode},
		SourcePaths: map[string][]string{SourceOpenCode: []string{root}},
		Since:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 6, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		Location:    time.UTC,
		Order:       "desc",
		Mode:        "display",
	}
	baseline := current
	baseline.Since = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	baseline.Until = time.Date(2026, 5, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)

	diff, err := LoadAndCompare(current, baseline, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Rows) != 10 {
		t.Fatalf("rows = %d, want default top 10 after join", len(diff.Rows))
	}
	if diff.CurrentTotals.CostUSD != currentTotal {
		t.Fatalf("current total = %v, want %v", diff.CurrentTotals.CostUSD, currentTotal)
	}
	if diff.BaselineTotals.CostUSD != baselineTotal {
		t.Fatalf("baseline total = %v, want %v", diff.BaselineTotals.CostUSD, baselineTotal)
	}
	var joined DiffRow
	for _, row := range diff.Rows {
		if row.Key == "current-00" {
			joined = row
			break
		}
	}
	if joined.Key == "" {
		t.Fatalf("current-00 missing from joined top rows: %+v", diff.Rows)
	}
	if joined.Current.CostUSD != 100 || joined.Baseline.CostUSD != 1 || joined.BaselineCostZero() {
		t.Fatalf("current-00 was not fully joined before top: %+v", joined)
	}
}

func TestDiffRowDeltaAndPercent(t *testing.T) {
	cases := []struct {
		name  string
		row   DiffRow
		delta float64
		pct   float64
	}{
		{
			name:  "normal increase",
			row:   DiffRow{Current: Tokens{CostUSD: 15}, Baseline: Tokens{CostUSD: 10}},
			delta: 5,
			pct:   50,
		},
		{
			name:  "new from zero baseline",
			row:   DiffRow{Current: Tokens{CostUSD: 5}},
			delta: 5,
			pct:   100,
		},
		{
			name: "both zero",
			row:  DiffRow{},
			pct:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.DeltaCost(); got != tc.delta {
				t.Fatalf("DeltaCost = %v, want %v", got, tc.delta)
			}
			if got := tc.row.PctCost(); got != tc.pct {
				t.Fatalf("PctCost = %v, want %v", got, tc.pct)
			}
		})
	}
}
