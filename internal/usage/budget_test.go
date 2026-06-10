package usage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

func TestEvaluateBudgetUnderAndOver(t *testing.T) {
	root := writeBudgetFixture(t)
	restoreBudgetNow(t, time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC))
	q := budgetQuery(root)

	under, ok, err := evaluateBudget(q, RenderOptions{Budget: 10, BudgetPeriod: "month"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || under.BudgetExceeded || under.CostUsed != 7.5 {
		t.Fatalf("under budget status = %+v ok=%v", under, ok)
	}

	over, ok, err := evaluateBudget(q, RenderOptions{TokenBudget: 10, BudgetPeriod: "month"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !over.BudgetExceeded || !over.TokenExceeded || over.TokenUsed != 15 {
		t.Fatalf("over budget status = %+v ok=%v", over, ok)
	}
}

func TestRunBudgetExitCode(t *testing.T) {
	root := writeBudgetFixture(t)
	restoreBudgetNow(t, time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC))

	var out, errOut bytes.Buffer
	err := Run(budgetArgs(root, "--budget", "1", "--budget-exit"), &out, &errOut)
	var exitErr ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want ExitError", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("exit code = %d, want 2", exitErr.Code)
	}
	if !strings.Contains(out.String(), "Coding Agent Usage Report") || !strings.Contains(out.String(), "warning: budget exceeded") {
		t.Fatalf("report did not render before exit:\nstdout:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "warning: budget exceeded") {
		t.Fatalf("stderr missing budget banner:\n%s", errOut.String())
	}

	out.Reset()
	errOut.Reset()
	err = Run(budgetArgs(root, "--budget", "10", "--budget-exit"), &out, &errOut)
	if err != nil {
		t.Fatalf("under budget Run error = %v", err)
	}
	if !strings.Contains(errOut.String(), "ON TRACK") {
		t.Fatalf("stderr missing under-budget banner:\n%s", errOut.String())
	}
}

func TestBudgetBannerGoesToStderrAndStructuredStdoutStaysClean(t *testing.T) {
	root := writeBudgetFixture(t)
	restoreBudgetNow(t, time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC))

	for _, format := range []string{"json", "csv"} {
		t.Run(format, func(t *testing.T) {
			var out, errOut bytes.Buffer
			args := budgetArgs(root, "--format", format, "--budget", "10")
			if err := Run(args, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "Budget:") || strings.Contains(out.String(), "budget exceeded") {
				t.Fatalf("%s stdout contains budget line:\n%s", format, out.String())
			}
			if !strings.Contains(errOut.String(), "Budget:") {
				t.Fatalf("%s stderr missing budget banner:\n%s", format, errOut.String())
			}
		})
	}
}

func writeBudgetFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"requestId":"req_1","type":"assistant","timestamp":"2026-06-01T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5,"costUSD":7.5}}}
{"requestId":"req_old","type":"assistant","timestamp":"2026-05-31T10:00:00Z","cwd":"/work/repo","sessionId":"s2","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5,"costUSD":99}}}
`
	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func restoreBudgetNow(t *testing.T, now time.Time) {
	t.Helper()
	prev := budgetNow
	budgetNow = func() time.Time { return now }
	t.Cleanup(func() { budgetNow = prev })
}

func budgetQuery(root string) core.Query {
	return core.Query{
		View:        "daily",
		Sources:     []string{core.SourceClaude},
		SourcePaths: map[string][]string{core.SourceClaude: {root}},
		Location:    time.UTC,
		Order:       "asc",
		Mode:        "display",
		Speed:       "standard",
		By:          "model",
	}
}

func budgetArgs(root string, extra ...string) []string {
	args := []string{"claude", "daily", "--path", root, "--mode", "display", "--timezone", "UTC", "--no-progress"}
	return append(args, extra...)
}
