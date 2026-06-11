package main

import (
	"io/fs"
	"path/filepath"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// watchInterval is how often the live watcher polls source directory mtimes. Kept
// short enough to feel live but long enough to avoid churning the disk.
const watchInterval = 3 * time.Second

// UsageService is the Wails-bound bridge to the core data engine. Methods are
// exposed to the frontend through Wails v3 bound service calls.
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

// CompareUsage diffs a current query against a baseline query, mirroring the
// CLI's --compare path. Both queries arrive over the JSON bridge with a nil
// Location, so default each to local time before handing off to core — without
// it filterEvents panics in time.Time.In(nil) exactly as in GetUsage.
func (UsageService) CompareUsage(q core.Query, baseline core.Query) (core.DiffResult, error) {
	if q.Location == nil {
		q.Location = time.Local
	}
	if baseline.Location == nil {
		baseline.Location = time.Local
	}
	return core.LoadAndCompare(q, baseline, nil)
}

// ListSources returns the source identifiers the UI can filter by.
func (UsageService) ListSources() []string {
	return core.AllSources()
}

// startUsageWatch polls the resolved source directories on an interval and emits a
// "usage:changed" event to the frontend whenever any source's newest mtime
// advances, so the UI can refetch. It returns immediately; the poll loop runs
// in a background goroutine until the Wails application context is cancelled.
func startUsageWatch(app *application.App) {
	go func() {
		ticker := time.NewTicker(watchInterval)
		defer ticker.Stop()
		// The candidate root set is fixed for the run (it derives from the home
		// dir + built-in source layout), so resolve it once; only the mtimes
		// beneath those roots change tick to tick.
		roots := resolvedSourceRoots()
		prev := snapshotMtimes(roots)
		for {
			select {
			case <-app.Context().Done():
				return
			case <-ticker.C:
				cur := snapshotMtimes(roots)
				if mtimesChanged(prev, cur) {
					prev = cur
					app.Event.Emit("usage:changed", nil)
				}
			}
		}
	}()
}

// resolvedSourceRoots returns the deduplicated set of directories that back all
// known sources, using the same resolution the core engine uses for reads.
func resolvedSourceRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	for _, source := range core.AllSources() {
		for _, p := range core.DefaultSourcePaths(source) {
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			roots = append(roots, p)
		}
	}
	return roots
}

// snapshotMtimes walks each root and records the newest file modification time
// seen beneath it, keyed by root. Roots that don't exist or are empty are
// recorded as the zero time so they still participate in change detection (a
// directory appearing or its first file landing is a change). Pure and
// display-free so it can be unit tested.
func snapshotMtimes(roots []string) map[string]time.Time {
	out := make(map[string]time.Time, len(roots))
	for _, root := range roots {
		if root == "" {
			continue
		}
		var newest time.Time
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules", "memory":
					return filepath.SkipDir
				}
				return nil
			}
			info, statErr := d.Info()
			if statErr != nil {
				return nil
			}
			if mt := info.ModTime(); mt.After(newest) {
				newest = mt
			}
			return nil
		})
		out[root] = newest
	}
	return out
}

// mtimesChanged reports whether two mtime snapshots differ: any root added,
// removed, or whose newest mtime advanced counts as a change. Pure and
// display-free so it can be unit tested.
func mtimesChanged(prev, cur map[string]time.Time) bool {
	if len(prev) != len(cur) {
		return true
	}
	for root, mt := range cur {
		old, ok := prev[root]
		if !ok || !mt.Equal(old) {
			return true
		}
	}
	return false
}
