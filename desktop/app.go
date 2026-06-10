package main

import (
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

// UsageService is the Wails-bound bridge to the core data engine. Methods are
// exposed to the frontend as window.go.main.UsageService.*.
type UsageService struct{}

// GetUsage loads and aggregates usage for a query sent from the frontend.
//
// The frontend cannot send a *time.Location over the JSON bridge, so q.Location
// arrives nil; default it to local time the same way the CLI does in parseArgs.
// Without this, filterEvents calls time.Time.In(nil) and panics on the first
// event — invisible on a machine with no logs, fatal on one with real usage.
func (UsageService) GetUsage(q core.Query) (core.Result, error) {
	if q.Location == nil {
		q.Location = time.Local
	}
	return core.LoadAndAggregate(q, nil)
}

// ListSources returns the source identifiers the UI can filter by.
func (UsageService) ListSources() []string {
	return core.AllSources()
}

// P3: add CompareUsage(q, baseline core.Query) (core.DiffResult, error) over
// core.LoadAndCompare when the Trends/compare screen lands.
