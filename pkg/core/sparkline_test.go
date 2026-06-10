package core

import (
	"reflect"
	"testing"
	"time"
)

func TestBucketSeriesDailyWindowTimezoneAndMetric(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{
		{Time: time.Date(2026, 6, 2, 3, 30, 0, 0, time.UTC), Tokens: Tokens{Input: 4, Output: 6, CostUSD: 1.25}},
		{Time: time.Date(2026, 6, 2, 5, 30, 0, 0, time.UTC), Tokens: Tokens{Input: 10, Output: 5, CostUSD: 2.50}},
	}
	q := Query{
		Since:    time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
		Until:    time.Date(2026, 6, 3, 23, 59, 59, int(time.Second-time.Nanosecond), loc),
		Location: loc,
	}
	if got, want := bucketSeries(events, q, "cost"), []float64{1.25, 2.50, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cost series = %v, want %v", got, want)
	}
	if got, want := bucketSeries(events, q, "total"), []float64{10, 15, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("token series = %v, want %v", got, want)
	}
}

func TestBucketSeriesDerivesMissingWindow(t *testing.T) {
	events := []Event{
		{Time: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC), Tokens: Tokens{CostUSD: 3}},
		{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), Tokens: Tokens{CostUSD: 1}},
	}
	got := bucketSeries(events, Query{Location: time.UTC}, "cost")
	want := []float64{1, 0, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %v, want %v", got, want)
	}
}

func TestSparklineGlyphMapping(t *testing.T) {
	if got := Sparkline(nil, false); got != "" {
		t.Fatalf("empty sparkline = %q, want empty", got)
	}
	if got := Sparkline([]float64{5}, false); got != "▁" {
		t.Fatalf("single unicode sparkline = %q, want lowest glyph", got)
	}
	if got := Sparkline([]float64{2, 2, 2}, false); got != "▁▁▁" {
		t.Fatalf("flat unicode sparkline = %q, want lowest flat glyphs", got)
	}
	if got := Sparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}, false); got != "▁▂▃▄▅▆▇█" {
		t.Fatalf("unicode ramp = %q", got)
	}
	if got := Sparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}, true); got != "._-=+*#@" {
		t.Fatalf("ascii ramp = %q", got)
	}
	// Lowest ASCII level must be a visible glyph, not a blank.
	if got := Sparkline([]float64{2, 2, 2}, true); got != "..." {
		t.Fatalf("flat ascii sparkline = %q, want visible lowest glyphs", got)
	}
}

func TestCalendarDayDiffIsDSTSafe(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tz data unavailable")
	}
	// Window spans the March 2026 spring-forward (a 23-hour day). The whole-day
	// offset must still count calendar days, not be off by the DST hour.
	start := time.Date(2026, 3, 6, 0, 0, 0, 0, ny)
	end := time.Date(2026, 3, 12, 0, 0, 0, 0, ny)
	if got := dayOffset(start, end); got != 6 {
		t.Fatalf("dayOffset across DST = %d, want 6", got)
	}
	if got := daysInclusive(start, end); got != 7 {
		t.Fatalf("daysInclusive across DST = %d, want 7", got)
	}
}
