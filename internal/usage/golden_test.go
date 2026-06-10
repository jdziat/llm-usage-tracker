package usage

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// updateGolden regenerates the testdata/ golden files instead of comparing.
// Run: go test ./internal/usage -run Golden -update
var updateGolden = flag.Bool("update", false, "update golden files in testdata/")

// goldenFixture is a deterministic set of rows + warnings exercising every
// field the renderers emit (including the normally-hidden Reasoning/Credits/
// Start/LastActivity fields and per-model breakdowns). It exists so the
// rendered JSON/CSV/HTML output is a byte-for-byte regression tripwire across
// the pkg/core refactor (P0): if any renderer's bytes change, the matching
// golden test fails. When the data types move to pkg/core, update the type
// references here in lockstep but keep the golden files byte-identical.
func goldenFixture() (Config, []Row, []string) {
	t0 := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 6, 1, 17, 30, 0, 0, time.UTC)
	rows := []Row{
		{
			Key:          "claude-opus-4-8",
			Source:       "claude",
			SessionID:    "sess-1",
			Project:      "/work/alpha",
			Start:        t0,
			LastActivity: t1,
			ModelsUsed:   []string{"claude-opus-4-8"},
			Tokens:       Tokens{Input: 1000, Output: 2000, CacheCreation: 300, CacheRead: 4000, Reasoning: 50, CostUSD: 1.2345},
			ModelBreakdowns: []ModelBreakdown{
				{Model: "claude-opus-4-8", Tokens: Tokens{Input: 1000, Output: 2000, CacheCreation: 300, CacheRead: 4000, Reasoning: 50, CostUSD: 1.2345}},
			},
		},
		{
			Key:          "gpt-5.5",
			Source:       "codex",
			SessionID:    "sess-2",
			Project:      "/work/beta",
			Start:        t0,
			LastActivity: t1,
			ModelsUsed:   []string{"gpt-5.5"},
			Tokens:       Tokens{Input: 500, Output: 800, CacheRead: 1200, Reasoning: 30, CostUSD: 0.42, Credits: 2.5},
			ModelBreakdowns: []ModelBreakdown{
				{Model: "gpt-5.5", Tokens: Tokens{Input: 500, Output: 800, CacheRead: 1200, Reasoning: 30, CostUSD: 0.42, Credits: 2.5}},
			},
		},
	}
	cfg := Config{View: "summary", Breakdown: true}
	warnings := []string{"claude: could not read /x/bad.jsonl", "codex: malformed session"}
	return cfg, rows, warnings
}

// htmlGeneratedLine matches the non-deterministic "Generated <RFC3339>." line
// that writeHTML embeds, so it can be normalized before golden comparison.
var htmlGeneratedLine = regexp.MustCompile(`Generated [0-9T:.+\-Z]+\. View`)

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with: go test ./internal/usage -run Golden -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s mismatch — rendered output changed.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenJSON(t *testing.T) {
	cfg, rows, warnings := goldenFixture()
	var buf bytes.Buffer
	if err := writeJSON(&buf, cfg, rows, warnings); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "report.json", buf.Bytes())
}

func TestGoldenCSV(t *testing.T) {
	cfg, rows, _ := goldenFixture()
	var buf bytes.Buffer
	if err := writeCSV(&buf, cfg, rows); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "report.csv", buf.Bytes())
}

func TestGoldenHTML(t *testing.T) {
	cfg, rows, warnings := goldenFixture()
	var buf bytes.Buffer
	if err := writeHTML(&buf, "Golden Report", cfg, rows, warnings); err != nil {
		t.Fatal(err)
	}
	normalized := htmlGeneratedLine.ReplaceAll(buf.Bytes(), []byte("Generated TIMESTAMP. View"))
	assertGolden(t, "report.html", normalized)
}
