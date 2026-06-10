package core

import "sort"

// Period comparison uses the CLI-derived current and baseline windows. For
// "--compare previous" with no explicit dates, the CLI sets the current window
// from the view before calling LoadAndCompare: monthly means this calendar
// month vs the previous calendar month, weekly means this calendar week vs the
// previous week using StartOfWeek, and daily or other views mean today vs
// yesterday. A baseline cost of zero with positive current cost is reported as
// 100 percent so JSON has a finite value; renderers can label that case "new".

type DiffResult struct {
	View           string    `json:"view"`
	Rows           []DiffRow `json:"data"`
	CurrentTotals  Tokens    `json:"currentTotals"`
	BaselineTotals Tokens    `json:"baselineTotals"`
	Warnings       []Warning `json:"warnings,omitempty"`
}

type DiffRow struct {
	Key      string `json:"key"`
	Current  Tokens `json:"current"`
	Baseline Tokens `json:"baseline"`
}

func (d DiffRow) DeltaCost() float64 {
	return d.Current.CostUSD - d.Baseline.CostUSD
}

func (d DiffRow) PctCost() float64 {
	return pctCost(d.Current.CostUSD, d.Baseline.CostUSD)
}

func (d DiffRow) BaselineCostZero() bool {
	return d.Baseline.CostUSD == 0
}

func LoadAndCompare(q Query, baseline Query, onProgress Progress) (DiffResult, error) {
	top := 0
	if q.View == "summary" {
		top = q.Top
		if top == 0 {
			top = 10
		}
	}
	q.Top = 0
	baseline.Top = 0
	if q.Compare == "" {
		q.Compare = "compare"
	}
	if baseline.Compare == "" {
		baseline.Compare = q.Compare
	}
	current, err := LoadAndAggregate(q, onProgress)
	if err != nil {
		return DiffResult{}, err
	}
	base, err := LoadAndAggregate(baseline, onProgress)
	if err != nil {
		return DiffResult{}, err
	}
	rows := joinDiffRows(current.Rows, base.Rows)
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	warnings := append([]Warning{}, current.Warnings...)
	warnings = append(warnings, base.Warnings...)
	return DiffResult{
		View:           current.View,
		Rows:           rows,
		CurrentTotals:  current.Totals,
		BaselineTotals: base.Totals,
		Warnings:       warnings,
	}, nil
}

func pctCost(current, baseline float64) float64 {
	switch {
	case baseline == 0 && current == 0:
		return 0
	case baseline == 0:
		return 100
	default:
		return (current - baseline) / baseline * 100
	}
}

func joinDiffRows(current, baseline []Row) []DiffRow {
	joined := map[string]*DiffRow{}
	for _, row := range current {
		diff := joined[row.Key]
		if diff == nil {
			diff = &DiffRow{Key: row.Key}
			joined[row.Key] = diff
		}
		diff.Current = row.Tokens
	}
	for _, row := range baseline {
		diff := joined[row.Key]
		if diff == nil {
			diff = &DiffRow{Key: row.Key}
			joined[row.Key] = diff
		}
		diff.Baseline = row.Tokens
	}
	out := make([]DiffRow, 0, len(joined))
	for _, row := range joined {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current.CostUSD != out[j].Current.CostUSD {
			return out[i].Current.CostUSD > out[j].Current.CostUSD
		}
		if out[i].Baseline.CostUSD != out[j].Baseline.CostUSD {
			return out[i].Baseline.CostUSD > out[j].Baseline.CostUSD
		}
		return out[i].Key < out[j].Key
	})
	return out
}
