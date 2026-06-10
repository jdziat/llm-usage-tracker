package usage

import (
	"fmt"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

var budgetNow = time.Now

type BudgetStatus struct {
	Period         string
	CostLimit      float64
	TokenLimit     int64
	CostUsed       float64
	TokenUsed      int64
	CostPercent    float64
	TokenPercent   float64
	CostExceeded   bool
	TokenExceeded  bool
	BudgetExceeded bool
}

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e ExitError) Unwrap() error {
	return e.Err
}

func evaluateBudget(q core.Query, opts RenderOptions) (BudgetStatus, bool, error) {
	if opts.Budget <= 0 && opts.TokenBudget <= 0 {
		return BudgetStatus{}, false, nil
	}
	since, until := budgetWindow(opts.BudgetPeriod, q.Location, budgetNow())
	// The budget is evaluated over the whole calendar window (day/week/month),
	// not the view's --since/--until/--top/--active filters, so a narrowed view
	// can't understate spend. Source/project/id filters ARE inherited, so a
	// budget scoped with --project tracks that project's spend for the period.
	bq := q
	bq.Since = since
	bq.Until = until
	bq.View = "daily"
	bq.Top = 0
	bq.Active = false
	bq.Recent = false
	res, err := core.LoadAndAggregate(bq, nil)
	if err != nil {
		return BudgetStatus{}, false, err
	}
	status := BudgetStatus{
		Period:     opts.BudgetPeriod,
		CostLimit:  opts.Budget,
		TokenLimit: opts.TokenBudget,
		CostUsed:   res.Totals.CostUSD,
		TokenUsed:  res.Totals.Total(),
	}
	if status.CostLimit > 0 {
		status.CostPercent = status.CostUsed / status.CostLimit * 100
		status.CostExceeded = status.CostUsed > status.CostLimit
	}
	if status.TokenLimit > 0 {
		status.TokenPercent = float64(status.TokenUsed) / float64(status.TokenLimit) * 100
		status.TokenExceeded = status.TokenUsed > status.TokenLimit
	}
	status.BudgetExceeded = status.CostExceeded || status.TokenExceeded
	return status, true, nil
}

func budgetWindow(period string, loc *time.Location, now time.Time) (time.Time, time.Time) {
	if loc == nil {
		loc = time.Local
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch period {
	case "day":
	case "week":
		weekday := int(start.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = start.AddDate(0, 0, -(weekday - 1))
	case "month":
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	}
	switch period {
	case "day":
		return start, start.AddDate(0, 0, 1).Add(-time.Nanosecond)
	case "week":
		return start, start.AddDate(0, 0, 7).Add(-time.Nanosecond)
	case "month":
		return start, start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	default:
		return start, start.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
}

func budgetBanner(status BudgetStatus) string {
	switch {
	case status.CostLimit > 0 && status.CostExceeded:
		return fmt.Sprintf("warning: budget exceeded: $%.2f / $%.2f %s (%.1f%%)", status.CostUsed, status.CostLimit, status.Period, status.CostPercent)
	case status.TokenLimit > 0 && status.TokenExceeded:
		return fmt.Sprintf("warning: token budget exceeded: %s / %s %s (%.1f%%)", formatInt(status.TokenUsed), formatInt(status.TokenLimit), status.Period, status.TokenPercent)
	case status.CostLimit > 0:
		return fmt.Sprintf("Budget: $%.2f / $%.2f %s  (%.1f%%)  ON TRACK", status.CostUsed, status.CostLimit, status.Period, status.CostPercent)
	case status.TokenLimit > 0:
		return fmt.Sprintf("Budget: %s / %s tokens %s  (%.1f%%)  ON TRACK", formatInt(status.TokenUsed), formatInt(status.TokenLimit), status.Period, status.TokenPercent)
	default:
		return ""
	}
}
