package usage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTrendEvents(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data := `{"timestamp":"2026-06-01T12:00:00Z","model":"m","usage":{"inputTokens":4,"outputTokens":6,"costUSD":1}}
{"timestamp":"2026-06-02T12:00:00Z","model":"m","usage":{"inputTokens":10,"outputTokens":5,"costUSD":2}}
`
	if err := os.WriteFile(filepath.Join(root, "events.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestTrendViewRendersSparklineAndStats(t *testing.T) {
	root := writeTrendEvents(t)
	var out, errOut bytes.Buffer
	err := Run([]string{"opencode", "trend", "--path", root, "--since", "2026-06-01", "--until", "2026-06-03", "--timezone", "UTC", "--mode", "display", "--no-progress"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Coding Agent Usage Report - Trend (cost) - opencode",
		"2026-06-01 .. 2026-06-03",
		"▅█▁",
		"min $0.0000",
		"max $2.0000",
		"total $3.0000",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("trend output missing %q:\n%s", want, got)
		}
	}
}

func TestTrendJSONCarriesSeriesAndStats(t *testing.T) {
	root := writeTrendEvents(t)
	var out, errOut bytes.Buffer
	err := Run([]string{"opencode", "trend", "--path", root, "--since", "2026-06-01", "--until", "2026-06-03", "--timezone", "UTC", "--mode", "display", "--format", "json", "--no-progress"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		View  string `json:"view"`
		Trend struct {
			Metric string    `json:"metric"`
			Series []float64 `json:"series"`
			Min    float64   `json:"min"`
			Max    float64   `json:"max"`
			Total  float64   `json:"total"`
		} `json:"trend"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.View != "trend" || got.Trend.Metric != "cost" {
		t.Fatalf("unexpected trend metadata: %+v", got)
	}
	if len(got.Trend.Series) != 3 || got.Trend.Series[0] != 1 || got.Trend.Series[1] != 2 || got.Trend.Series[2] != 0 {
		t.Fatalf("series = %v, want [1 2 0]", got.Trend.Series)
	}
	if got.Trend.Min != 0 || got.Trend.Max != 2 || got.Trend.Total != 3 {
		t.Fatalf("stats = min %v max %v total %v", got.Trend.Min, got.Trend.Max, got.Trend.Total)
	}
}

func TestNoUnicodeAndLocaleForceASCIISparkline(t *testing.T) {
	root := writeTrendEvents(t)
	var out, errOut bytes.Buffer
	err := Run([]string{"opencode", "trend", "--path", root, "--since", "2026-06-01", "--until", "2026-06-03", "--timezone", "UTC", "--mode", "display", "--no-unicode", "--no-progress"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "▁▂▃▄▅▆▇█") {
		t.Fatalf("--no-unicode did not force ASCII sparkline (found unicode glyph):\n%s", out.String())
	}

	t.Setenv("LANG", "C")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LC_ALL", "")
	out.Reset()
	err = Run([]string{"opencode", "trend", "--path", root, "--since", "2026-06-01", "--until", "2026-06-03", "--timezone", "UTC", "--mode", "display", "--no-progress"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "▁▂▃▄▅▆▇█") {
		t.Fatalf("non-UTF-8 locale did not force ASCII sparkline (found unicode glyph):\n%s", out.String())
	}
}

func TestParseArgsSparklineAndLocalePrecedence(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_CTYPE", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	q, opts, _, err := parseArgs([]string{"weekly", "--sparkline"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Sparkline != "cost" {
		t.Fatalf("sparkline = %q, want cost", q.Sparkline)
	}
	if opts.NoUnicode {
		t.Fatal("LC_CTYPE UTF-8 should override non-UTF-8 LANG")
	}

	t.Setenv("LC_ALL", "C")
	_, opts, _, err = parseArgs([]string{"weekly", "--sparkline", "total"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.NoUnicode {
		t.Fatal("LC_ALL non-UTF-8 should force ASCII")
	}
}
