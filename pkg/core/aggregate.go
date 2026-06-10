package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func aggregate(events []Event, cfg Query) []Row {
	filtered := filterEvents(events, cfg)
	switch cfg.View {
	case "session":
		return aggregateSessions(filtered, cfg)
	case "summary":
		return aggregateSummary(filtered, cfg, summaryKeyFunc(cfg))
	case "weekly":
		return aggregateByKey(filtered, cfg, func(t time.Time, loc *time.Location) string {
			return weekKey(t, loc, cfg.StartOfWeek)
		})
	case "monthly":
		return aggregateByKey(filtered, cfg, func(t time.Time, _ *time.Location) string {
			return t.In(cfg.Location).Format("2006-01")
		})
	case "blocks":
		return aggregateBlocks(filtered, cfg)
	case "trend":
		return aggregateByKey(filtered, cfg, func(t time.Time, _ *time.Location) string {
			return t.In(cfg.Location).Format("2006-01-02")
		})
	default:
		return aggregateByKey(filtered, cfg, func(t time.Time, _ *time.Location) string {
			return t.In(cfg.Location).Format("2006-01-02")
		})
	}
}

func aggregateSummary(events []Event, cfg Query, keyFn func(Event) string) []Row {
	rows := map[string]*Row{}
	for _, ev := range events {
		displayKey := firstNonEmpty(keyFn(ev), "unknown")
		key := summaryMapKey(displayKey, ev, cfg)
		row := rows[key]
		if row == nil {
			row = &Row{Key: displayKey, Source: sourceForRow(cfg, ev.Source), Start: ev.Time, LastActivity: ev.Time}
			rows[key] = row
		}
		addEvent(row, ev)
		if ev.Time.Before(row.Start) {
			row.Start = ev.Time
		}
		if ev.Time.After(row.LastActivity) {
			row.LastActivity = ev.Time
		}
	}
	out := sortedSummaryRows(rows, cfg.Order)
	limit := cfg.Top
	if limit == 0 && cfg.Compare == "" {
		limit = 10
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func summaryKeyFunc(cfg Query) func(Event) string {
	switch cfg.By {
	case "project":
		return func(ev Event) string { return compactProject(ev.Project) }
	case "source":
		return func(ev Event) string { return SourceLabel(ev.Source) }
	default:
		return func(ev Event) string { return firstNonEmpty(ev.Model, "unknown") }
	}
}

func summaryMapKey(displayKey string, ev Event, cfg Query) string {
	if cfg.By == "" || cfg.By == "model" {
		if len(cfg.Sources) != 1 {
			return SourceLabel(ev.Source) + ":" + displayKey
		}
	}
	return displayKey
}

func filterEvents(events []Event, cfg Query) []Event {
	var out []Event
	for _, ev := range events {
		when := ev.Time
		if when.IsZero() {
			continue
		}
		local := when.In(cfg.Location)
		if !cfg.Since.IsZero() && local.Before(cfg.Since) {
			continue
		}
		if !cfg.Until.IsZero() && local.After(cfg.Until) {
			continue
		}
		if cfg.Project != "" && !strings.Contains(strings.ToLower(ev.Project), strings.ToLower(cfg.Project)) {
			continue
		}
		if cfg.ID != "" && !strings.Contains(ev.SessionID, cfg.ID) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func aggregateByKey(events []Event, cfg Query, keyFn func(time.Time, *time.Location) string) []Row {
	rows := map[string]*Row{}
	for _, ev := range events {
		key := keyFn(ev.Time, cfg.Location)
		if cfg.Instances {
			key += " " + compactProject(ev.Project)
		}
		if len(cfg.Sources) != 1 {
			key += " " + SourceLabel(ev.Source)
		}
		row := rows[key]
		if row == nil {
			row = &Row{Key: key, Source: sourceForRow(cfg, ev.Source), Start: ev.Time, LastActivity: ev.Time}
			rows[key] = row
		}
		addEvent(row, ev)
	}
	return sortedRows(rows, cfg.Order)
}

func aggregateSessions(events []Event, cfg Query) []Row {
	rows := map[string]*Row{}
	for _, ev := range events {
		key := ev.SessionID
		if len(cfg.Sources) != 1 {
			key = SourceLabel(ev.Source) + ":" + key
		}
		row := rows[key]
		if row == nil {
			row = &Row{Key: displaySession(ev), Source: sourceForRow(cfg, ev.Source), SessionID: ev.SessionID, Project: ev.Project, Start: ev.Time, LastActivity: ev.Time}
			rows[key] = row
		}
		addEvent(row, ev)
		if ev.Time.Before(row.Start) {
			row.Start = ev.Time
		}
		if ev.Time.After(row.LastActivity) {
			row.LastActivity = ev.Time
		}
		if row.Project == "" {
			row.Project = ev.Project
		}
	}
	return sortedRows(rows, cfg.Order)
}

func aggregateBlocks(events []Event, cfg Query) []Row {
	if cfg.SessionLength == 0 {
		cfg.SessionLength = 5 * time.Hour
	}
	rows := map[string]*Row{}
	now := time.Now().In(cfg.Location)
	for _, ev := range events {
		local := ev.Time.In(cfg.Location)
		start := local.Truncate(cfg.SessionLength)
		if cfg.Recent && start.Before(now.Add(-72*time.Hour)) {
			continue
		}
		if cfg.Active && !(now.Equal(start) || (now.After(start) && now.Before(start.Add(cfg.SessionLength)))) {
			continue
		}
		key := start.Format(time.RFC3339)
		row := rows[key]
		if row == nil {
			row = &Row{Key: start.Format("2006-01-02 15:04"), Source: sourceForRow(cfg, SourceClaude), Start: start, LastActivity: ev.Time}
			rows[key] = row
		}
		addEvent(row, ev)
		if ev.Time.After(row.LastActivity) {
			row.LastActivity = ev.Time
		}
	}
	return sortedRows(rows, cfg.Order)
}

func addEvent(row *Row, ev Event) {
	row.Add(ev.Tokens)
	model := firstNonEmpty(ev.Model, "unknown")
	found := false
	for i := range row.ModelBreakdowns {
		if row.ModelBreakdowns[i].Model == model {
			row.ModelBreakdowns[i].Add(ev.Tokens)
			found = true
			break
		}
	}
	if !found {
		row.ModelBreakdowns = append(row.ModelBreakdowns, ModelBreakdown{Model: model, Tokens: ev.Tokens})
	}
}

func sortedRows(rows map[string]*Row, order string) []Row {
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		sortModelBreakdowns(r.ModelBreakdowns)
		r.ModelsUsed = rankedModelNames(r.ModelBreakdowns)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastActivity.IsZero() || !out[j].LastActivity.IsZero() {
			if order == "asc" {
				return out[i].LastActivity.Before(out[j].LastActivity)
			}
			return out[i].LastActivity.After(out[j].LastActivity)
		}
		if order == "asc" {
			return out[i].Key < out[j].Key
		}
		return out[i].Key > out[j].Key
	})
	return out
}

func sortedSummaryRows(rows map[string]*Row, order string) []Row {
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		sortModelBreakdowns(r.ModelBreakdowns)
		r.ModelsUsed = rankedModelNames(r.ModelBreakdowns)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CostUSD != out[j].CostUSD {
			if order == "asc" {
				return out[i].CostUSD < out[j].CostUSD
			}
			return out[i].CostUSD > out[j].CostUSD
		}
		if out[i].Total() != out[j].Total() {
			if order == "asc" {
				return out[i].Total() < out[j].Total()
			}
			return out[i].Total() > out[j].Total()
		}
		if order == "asc" {
			return out[i].Key < out[j].Key
		}
		return out[i].Key > out[j].Key
	})
	return out
}

func sortModelBreakdowns(models []ModelBreakdown) {
	sort.Slice(models, func(i, j int) bool {
		if models[i].CostUSD != models[j].CostUSD {
			return models[i].CostUSD > models[j].CostUSD
		}
		if models[i].Total() != models[j].Total() {
			return models[i].Total() > models[j].Total()
		}
		return models[i].Model < models[j].Model
	})
}

func rankedModelNames(models []ModelBreakdown) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		if model.Model != "" {
			out = append(out, model.Model)
		}
	}
	return out
}

func weekKey(t time.Time, loc *time.Location, startOfWeek string) string {
	local := t.In(loc)
	if startOfWeek == "sunday" {
		weekday := int(local.Weekday())
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -weekday)
		return start.Format("2006-01-02")
	}
	year, week := local.ISOWeek()
	return fmtKey("%04d-W%02d", year, week)
}

func fmtKey(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func sourceForRow(cfg Query, source string) string {
	if len(cfg.Sources) == 1 {
		return SourceLabel(source)
	}
	return ""
}

func displaySession(ev Event) string {
	if ev.Project != "" {
		return compactProject(ev.Project)
	}
	return ev.SessionID
}

func compactProject(project string) string {
	project = strings.TrimRight(project, "/")
	if project == "" {
		return ""
	}
	if i := strings.LastIndex(project, "/"); i >= 0 {
		return project[i+1:]
	}
	return project
}
