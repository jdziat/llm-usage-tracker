package core

import (
	"math"
	"time"
)

const (
	unicodeSparklineLevels = "▁▂▃▄▅▆▇█"
	// Ascending visual density. The lowest level is a visible glyph (not a
	// blank) so a min-value bucket and an all-equal series stay distinguishable
	// from missing output on a non-UTF-8 terminal.
	asciiSparklineLevels = "._-=+*#@"
)

func bucketSeries(events []Event, q Query, metric string) []float64 {
	if q.Location == nil {
		q.Location = time.Local
	}
	filtered := filterEvents(events, q)
	start, end, ok := seriesWindow(filtered, q)
	if !ok {
		return nil
	}
	days := daysInclusive(start, end)
	if days <= 0 {
		return nil
	}
	series := make([]float64, days)
	for _, ev := range filtered {
		day := dayStart(ev.Time, q.Location)
		if day.Before(start) || day.After(end) {
			continue
		}
		idx := dayOffset(start, day)
		if idx >= 0 && idx < len(series) {
			series[idx] += metricValue(ev, metric)
		}
	}
	return series
}

func Sparkline(series []float64, ascii bool) string {
	if len(series) == 0 {
		return ""
	}
	min, max, _ := seriesStats(series)
	if min == max {
		if ascii {
			return repeatByte(asciiSparklineLevels[0], len(series))
		}
		return repeatRune([]rune(unicodeSparklineLevels)[0], len(series))
	}
	if ascii {
		out := make([]byte, len(series))
		for i, v := range series {
			out[i] = asciiSparklineLevels[levelIndex(v, min, max)]
		}
		return string(out)
	}
	levels := []rune(unicodeSparklineLevels)
	out := make([]rune, len(series))
	for i, v := range series {
		out[i] = levels[levelIndex(v, min, max)]
	}
	return string(out)
}

func buildTrend(events []Event, q Query, metric string) *Trend {
	series := bucketSeries(events, q, metric)
	filtered := filterEvents(events, q)
	start, end, ok := seriesWindow(filtered, q)
	if !ok {
		return &Trend{Metric: normalizeSparklineMetric(metric), Series: series}
	}
	min, max, total := seriesStats(series)
	return &Trend{
		Metric: normalizeSparklineMetric(metric),
		Start:  start,
		End:    end,
		Series: series,
		Min:    min,
		Max:    max,
		Total:  total,
	}
}

func rowSparklineSeries(events []Event, q Query, metric string) map[string][]float64 {
	if q.Location == nil {
		q.Location = time.Local
	}
	filtered := filterEvents(events, q)
	out := map[string][]float64{}
	var keyFn func(time.Time, *time.Location) string
	var startFn func(time.Time) time.Time
	var daysFn func(time.Time) int
	switch q.View {
	case "weekly":
		keyFn = func(t time.Time, loc *time.Location) string { return weekKey(t, loc, q.StartOfWeek) }
		startFn = func(t time.Time) time.Time { return weekStartTime(t, q.Location, q.StartOfWeek) }
		daysFn = func(time.Time) int { return 7 }
	case "monthly":
		keyFn = func(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006-01") }
		startFn = func(t time.Time) time.Time {
			local := t.In(q.Location)
			return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, q.Location)
		}
		daysFn = func(start time.Time) int {
			return daysInclusive(start, start.AddDate(0, 1, -1))
		}
	default:
		return nil
	}
	starts := map[string]time.Time{}
	for _, ev := range filtered {
		key := rowSparklineKey(ev, q, keyFn)
		start := startFn(ev.Time)
		if _, ok := out[key]; !ok {
			out[key] = make([]float64, daysFn(start))
			starts[key] = start
		}
		idx := dayOffset(starts[key], dayStart(ev.Time, q.Location))
		if idx >= 0 && idx < len(out[key]) {
			out[key][idx] += metricValue(ev, metric)
		}
	}
	return out
}

func rowSparklineKey(ev Event, q Query, keyFn func(time.Time, *time.Location) string) string {
	key := keyFn(ev.Time, q.Location)
	if q.Instances {
		key += " " + compactProject(ev.Project)
	}
	if len(q.Sources) != 1 {
		key += " " + SourceLabel(ev.Source)
	}
	return key
}

func seriesWindow(events []Event, q Query) (time.Time, time.Time, bool) {
	var start, end time.Time
	if !q.Since.IsZero() {
		start = dayStart(q.Since, q.Location)
	}
	if !q.Until.IsZero() {
		end = dayStart(q.Until, q.Location)
	}
	if start.IsZero() || end.IsZero() {
		for _, ev := range events {
			day := dayStart(ev.Time, q.Location)
			if start.IsZero() || day.Before(start) {
				start = day
			}
			if end.IsZero() || day.After(end) {
				end = day
			}
		}
	}
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func dayStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

// calendarDayDiff returns whole calendar days between two day-start times,
// DST-safe: both are projected onto fixed-offset UTC midnights so a 23h/25h
// DST day doesn't perturb the /24. O(1), replacing an O(days) day-stepping loop
// that became a per-event cliff for long trend windows.
func calendarDayDiff(start, day time.Time) int {
	a := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours()) / 24
}

func daysInclusive(start, end time.Time) int {
	return calendarDayDiff(start, end) + 1
}

func dayOffset(start, day time.Time) int {
	return calendarDayDiff(start, day)
}

func metricValue(ev Event, metric string) float64 {
	switch normalizeSparklineMetric(metric) {
	case "total":
		return float64(ev.Tokens.Total())
	default:
		return ev.Tokens.CostUSD
	}
}

func normalizeSparklineMetric(metric string) string {
	switch metric {
	case "total", "tokens":
		return "total"
	default:
		return "cost"
	}
}

func seriesStats(series []float64) (float64, float64, float64) {
	if len(series) == 0 {
		return 0, 0, 0
	}
	min, max := series[0], series[0]
	total := 0.0
	for _, v := range series {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		total += v
	}
	return min, max, total
}

func levelIndex(v, min, max float64) int {
	if max == min {
		return 0
	}
	scaled := (v - min) / (max - min)
	idx := int(math.Round(scaled * 7))
	if idx < 0 {
		return 0
	}
	if idx > 7 {
		return 7
	}
	return idx
}

func repeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

func repeatByte(b byte, n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return string(out)
}

func weekStartTime(t time.Time, loc *time.Location, startOfWeek string) time.Time {
	local := t.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	weekday := int(start.Weekday())
	if startOfWeek == "sunday" {
		return start.AddDate(0, 0, -weekday)
	}
	if weekday == 0 {
		weekday = 7
	}
	return start.AddDate(0, 0, -(weekday - 1))
}
