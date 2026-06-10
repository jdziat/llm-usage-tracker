package core

import (
	"encoding/json"
	"fmt"
	"time"
)

type Query struct {
	Sources       []string
	SourcePaths   map[string][]string
	Since, Until  time.Time
	Location      *time.Location
	View          string
	Sparkline     string
	Compare       string
	By            string
	Project, ID   string
	Order         string
	Top           int
	StartOfWeek   string
	Mode          string
	Speed         string
	Instances     bool
	Active        bool
	Recent        bool
	SessionLength time.Duration
	Tolerant      bool
}

type Warning struct{ Source, Message string }

func (w Warning) MarshalJSON() ([]byte, error) {
	s := w.Message
	if w.Source != "" {
		s = w.Source + ": " + s
	}
	return json.Marshal(s)
}

type Progress func(stage string)

type Result struct {
	View          string               `json:"view"`
	Rows          []Row                `json:"data"`
	Totals        Tokens               `json:"totals"`
	Trend         *Trend               `json:"trend,omitempty"`
	SparklineRows map[string][]float64 `json:"sparklines,omitempty"`
	Warnings      []Warning            `json:"warnings,omitempty"`
}

type Trend struct {
	Metric string    `json:"metric"`
	Start  time.Time `json:"start,omitempty"`
	End    time.Time `json:"end,omitempty"`
	Series []float64 `json:"series"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
	Total  float64   `json:"total"`
}

func LoadAndAggregate(q Query, onProgress Progress) (Result, error) {
	var events []Event
	var warnings []Warning
	for _, source := range q.Sources {
		paths := q.SourcePaths[source]
		if len(paths) == 0 {
			paths = DefaultSourcePaths(source)
		}
		progress(onProgress, "Scanning "+SourceLabel(source)+": discovering files")
		var got []Event
		var err error
		switch source {
		case SourceClaude:
			got, err = readClaude(paths, q, onProgress)
		case SourceCodex:
			got, err = readCodex(paths, q, onProgress)
		default:
			got, err = readGenericSource(source, paths, q, onProgress)
		}
		if err != nil {
			warnings = append(warnings, Warning{Source: source, Message: err.Error()})
			continue
		}
		progress(onProgress, fmt.Sprintf("Scanned %s: %d events", SourceLabel(source), len(got)))
		events = append(events, got...)
	}
	progress(onProgress, fmt.Sprintf("Calculating costs for %d events", len(events)))
	for i := range events {
		if events[i].Tokens.CostUSD == 0 || q.Mode == "calculate" || (q.Mode == "auto" && !events[i].RawCostKnown) {
			events[i].Tokens.CostUSD = calculateCost(events[i].Model, events[i].Tokens, q.Speed)
		}
	}
	progress(onProgress, fmt.Sprintf("Calculated costs for %d events", len(events)))
	rows := aggregate(events, q)
	res := Result{View: q.View, Rows: rows, Warnings: warnings}
	if q.View == "trend" {
		res.Trend = buildTrend(events, q, q.Sparkline)
	}
	if q.Sparkline != "" && (q.View == "weekly" || q.View == "monthly") {
		res.SparklineRows = rowSparklineSeries(events, q, q.Sparkline)
	}
	for _, row := range rows {
		res.Totals.Add(row.Tokens)
	}
	return res, nil
}

func progress(onProgress Progress, stage string) {
	if onProgress != nil {
		onProgress(stage)
	}
}
