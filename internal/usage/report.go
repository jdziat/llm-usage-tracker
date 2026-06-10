package usage

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

type RenderOptions struct {
	Format       string
	OutputPath   string
	Breakdown    bool
	Compact      bool
	TokenLimit   int64
	Debug        bool
	Fields       []string
	Budget       float64
	BudgetPeriod string
	TokenBudget  int64
	BudgetExit   bool
	BudgetStatus *BudgetStatus

	progress        bool
	refreshInterval time.Duration
}

func Render(w io.Writer, res core.Result, q core.Query, opts RenderOptions) error {
	switch opts.Format {
	case "json":
		return writeJSON(w, res)
	case "csv":
		return writeCSV(w, opts, res.Rows)
	case "html":
		return writeHTML(w, title(q), q, opts, res)
	}
	renderReport(w, res, q, opts)
	return nil
}

func writeJSON(w io.Writer, res core.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func renderReport(out io.Writer, res core.Result, q core.Query, opts RenderOptions) {
	if q.View == "blocks" && opts.Compact {
		writeStatusline(out, res.Rows, q, opts)
		return
	}
	if len(res.Rows) == 0 {
		fmt.Fprintln(out, "No usage data found.")
		return
	}
	if opts.Format == "pretty" {
		writePrettyTable(out, title(q), res, q, opts)
		return
	}
	writeTable(out, title(q), res, q, opts)
}

func writeTable(w io.Writer, title string, res core.Result, q core.Query, opts RenderOptions) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("-", len(title)))
	cols := selectedColumns(q, opts)
	headers := columnHeaders(cols, q)
	widths := reportWidths(q, opts, cols)
	printColumnRow(w, widths, cols, headers)
	printSep(w, widths)
	for _, row := range res.Rows {
		printColumnRow(w, widths, cols, rowCells(cols, row, q, false, false, widths))
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				breakdown := row
				breakdown.Key = b.Model
				breakdown.ModelsUsed = nil
				breakdown.Tokens = b.Tokens
				printColumnRow(w, widths, cols, rowCells(cols, breakdown, q, true, false, widths))
			}
		}
	}
	printSep(w, widths)
	printColumnRow(w, widths, cols, totalCells(cols, res.Totals))
	if opts.BudgetStatus != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, budgetBanner(*opts.BudgetStatus))
	}
}

func writePrettyTable(w io.Writer, title string, res core.Result, q core.Query, opts RenderOptions) {
	fmt.Fprintln(w, title)
	cols := selectedColumns(q, opts)
	widths := reportWidths(q, opts, cols)
	headers := columnHeaders(cols, q)
	printBoxBorder(w, widths, "top")
	printBoxColumnRow(w, widths, cols, headers)
	printBoxBorder(w, widths, "mid")
	for _, row := range res.Rows {
		printBoxColumnRow(w, widths, cols, rowCells(cols, row, q, false, false, widths))
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				breakdown := row
				breakdown.Key = b.Model
				breakdown.ModelsUsed = nil
				breakdown.Tokens = b.Tokens
				printBoxColumnRow(w, widths, cols, rowCells(cols, breakdown, q, true, false, widths))
			}
		}
	}
	printBoxBorder(w, widths, "mid")
	printBoxColumnRow(w, widths, cols, totalCells(cols, res.Totals))
	printBoxBorder(w, widths, "bottom")
	if opts.BudgetStatus != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, budgetBanner(*opts.BudgetStatus))
	}
}

func writeCSV(w io.Writer, opts RenderOptions, rows []core.Row) error {
	cw := csv.NewWriter(w)
	if len(opts.Fields) > 0 {
		cols := selectedColumns(core.Query{}, opts)
		if err := cw.Write(csvHeaders(cols)); err != nil {
			return err
		}
		for _, row := range rows {
			if err := cw.Write(rowCells(cols, row, core.Query{}, false, true, nil)); err != nil {
				return err
			}
			if opts.Breakdown {
				for _, b := range row.ModelBreakdowns {
					breakdown := row
					breakdown.Key = "  " + b.Model
					breakdown.ModelsUsed = []string{b.Model}
					breakdown.Tokens = b.Tokens
					if err := cw.Write(rowCells(cols, breakdown, core.Query{}, false, true, nil)); err != nil {
						return err
					}
				}
			}
		}
		cw.Flush()
		return cw.Error()
	}
	header := []string{"key", "source", "session_id", "project", "start", "last_activity", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "reasoning_tokens", "cost_usd", "credits", "models"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(csvRow(row)); err != nil {
			return err
		}
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				breakdown := row
				breakdown.Key = "  " + b.Model
				breakdown.ModelsUsed = []string{b.Model}
				breakdown.Tokens = b.Tokens
				if err := cw.Write(csvRow(breakdown)); err != nil {
					return err
				}
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeHTML(w io.Writer, title string, q core.Query, opts RenderOptions, res core.Result) error {
	fmt.Fprintln(w, "<!doctype html>")
	fmt.Fprintln(w, `<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">`)
	fmt.Fprintf(w, "<title>%s</title>\n", html.EscapeString(title))
	fmt.Fprintln(w, `<style>
body{font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:32px;background:#f7f8fa;color:#1f2933}
main{max-width:1200px;margin:0 auto}
h1{font-size:24px;margin:0 0 6px}
.meta{color:#64748b;margin:0 0 24px}
.summary{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px;margin:0 0 24px}
.metric{background:white;border:1px solid #d9e2ec;border-radius:8px;padding:14px}
.metric span{display:block;color:#64748b;font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.metric strong{display:block;margin-top:6px;font-size:20px}
table{width:100%;border-collapse:collapse;background:white;border:1px solid #d9e2ec}
th,td{padding:10px 12px;border-bottom:1px solid #e5eaf0;text-align:right;white-space:nowrap}
th:first-child,td:first-child,td:last-child{text-align:left}
th{background:#eef2f6;color:#334e68;font-size:12px;text-transform:uppercase;letter-spacing:.04em}
tr.breakdown td:first-child{padding-left:28px;color:#52606d}
.warnings{margin-top:20px;color:#9a3412}
</style></head><body><main>`)
	fmt.Fprintf(w, "<h1>%s</h1>\n", html.EscapeString(title))
	fmt.Fprintf(w, `<p class="meta">Generated %s. View: %s.</p>`+"\n", html.EscapeString(time.Now().Format(time.RFC3339)), html.EscapeString(q.View))
	fmt.Fprintln(w, `<section class="summary">`)
	htmlMetric(w, "Input", formatInt(res.Totals.Input))
	htmlMetric(w, "Output", formatInt(res.Totals.Output))
	htmlMetric(w, "Cache Read", formatInt(res.Totals.CacheRead))
	htmlMetric(w, "Total Tokens", formatInt(res.Totals.Total()))
	htmlMetric(w, "Cost", formatCost(res.Totals.CostUSD))
	fmt.Fprintln(w, `</section>`)
	cols := selectedColumns(q, opts)
	fmt.Fprintln(w, htmlHeaderRow(cols, q))
	for _, row := range res.Rows {
		writeHTMLRow(w, row, "", cols, q)
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				breakdown := row
				breakdown.Key = b.Model
				breakdown.ModelsUsed = nil
				breakdown.Tokens = b.Tokens
				writeHTMLRow(w, breakdown, ` class="breakdown"`, cols, q)
			}
		}
	}
	fmt.Fprintln(w, `</tbody></table>`)
	if len(res.Warnings) > 0 {
		fmt.Fprintln(w, `<div class="warnings"><strong>Warnings</strong><ul>`)
		for _, warning := range res.Warnings {
			fmt.Fprintf(w, "<li>%s</li>\n", html.EscapeString(warningString(warning)))
		}
		fmt.Fprintln(w, `</ul></div>`)
	}
	fmt.Fprintln(w, `</main></body></html>`)
	return nil
}

func writeStatusline(w io.Writer, rows []core.Row, q core.Query, opts RenderOptions) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "Claude: no active usage")
		return
	}
	r := rows[0]
	end := r.Start.Add(q.SessionLength)
	remaining := time.Until(end)
	if remaining < 0 {
		remaining = 0
	}
	limit := ""
	if opts.TokenLimit > 0 {
		limit = fmt.Sprintf(" %.0f%%", float64(r.Total())/float64(opts.TokenLimit)*100)
	}
	fmt.Fprintf(w, "Claude %s tokens %s %s left%s\n", formatInt(r.Total()), formatCost(r.CostUSD), remaining.Round(time.Minute), limit)
}

func printColumnRow(w io.Writer, widths []int, cols []fieldColumn, cells []string) {
	for i, cell := range cells {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		if cols[i].left {
			fmt.Fprintf(w, "%-*s", widths[i], truncate(cell, widths[i]))
		} else {
			fmt.Fprintf(w, "%*s", widths[i], truncate(cell, widths[i]))
		}
	}
	fmt.Fprintln(w)
}

func printSep(w io.Writer, widths []int) {
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, strings.Repeat("-", width))
	}
	fmt.Fprintln(w)
}

func printBoxBorder(w io.Writer, widths []int, pos string) {
	left, mid, right := "+", "+", "+"
	_ = pos
	fmt.Fprint(w, left)
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, mid)
		}
		fmt.Fprint(w, strings.Repeat("-", width+2))
	}
	fmt.Fprintln(w, right)
}

func printBoxColumnRow(w io.Writer, widths []int, cols []fieldColumn, cells []string) {
	fmt.Fprint(w, "|")
	for i, cell := range cells {
		if cols[i].left {
			fmt.Fprintf(w, " %-*s |", widths[i], truncate(cell, widths[i]))
		} else {
			fmt.Fprintf(w, " %*s |", widths[i], truncate(cell, widths[i]))
		}
	}
	fmt.Fprintln(w)
}

func csvRow(row core.Row) []string {
	return []string{
		row.Key,
		row.Source,
		row.SessionID,
		row.Project,
		formatTime(row.Start),
		formatTime(row.LastActivity),
		fmt.Sprintf("%d", row.Input),
		fmt.Sprintf("%d", row.Output),
		fmt.Sprintf("%d", row.CacheCreation),
		fmt.Sprintf("%d", row.CacheRead),
		fmt.Sprintf("%d", row.Total()),
		fmt.Sprintf("%d", row.Reasoning),
		fmt.Sprintf("%.6f", row.CostUSD),
		fmt.Sprintf("%.6f", row.Credits),
		strings.Join(row.ModelsUsed, ";"),
	}
}

func writeHTMLRow(w io.Writer, row core.Row, class string, cols []fieldColumn, q core.Query) {
	fmt.Fprintf(w, "<tr%s>", class)
	cells := rowCells(cols, row, q, false, false, nil)
	for i, col := range cols {
		cell := cells[i]
		if col.name == "models" {
			cell = strings.Join(row.ModelsUsed, ", ")
		}
		fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(cell))
	}
	fmt.Fprintln(w, "</tr>")
}

func htmlMetric(w io.Writer, label, value string) {
	fmt.Fprintf(w, `<div class="metric"><span>%s</span><strong>%s</strong></div>`+"\n", html.EscapeString(label), html.EscapeString(value))
}

func keyHeader(q core.Query) string {
	switch q.View {
	case "weekly":
		return "Week"
	case "monthly":
		return "Month"
	case "session":
		return "Session"
	case "summary":
		switch q.By {
		case "project":
			return "Project"
		case "source":
			return "Source"
		}
		return "Model"
	case "blocks":
		return "Block Start"
	default:
		return "Date"
	}
}

func minWidth(q core.Query, opts RenderOptions) int {
	if opts.Breakdown {
		return 34
	}
	if q.View == "summary" {
		return 30
	}
	if q.View == "session" {
		return 30
	}
	if q.View == "blocks" {
		return 16
	}
	return 18
}

func modelWidth(opts RenderOptions) int {
	if opts.Breakdown {
		return 34
	}
	return 30
}

func reportWidths(q core.Query, opts RenderOptions, cols []fieldColumn) []int {
	widths := make([]int, len(cols))
	for i, col := range cols {
		switch col.name {
		case "key":
			widths[i] = minWidth(q, opts)
		case "models":
			widths[i] = modelWidth(opts)
		default:
			widths[i] = col.width
		}
	}
	return widths
}

type fieldColumn struct {
	name      string
	header    string
	csvHeader string
	width     int
	left      bool
	value     func(core.Row) string
	total     func(core.Tokens) string
}

var fieldRegistry = map[string]fieldColumn{
	"key":        {name: "key", csvHeader: "key", left: true, value: func(r core.Row) string { return r.Key }, total: func(core.Tokens) string { return "Total" }},
	"source":     {name: "source", header: "Source", csvHeader: "source", width: 10, left: true, value: func(r core.Row) string { return r.Source }},
	"session_id": {name: "session_id", header: "Session ID", csvHeader: "session_id", width: 18, left: true, value: func(r core.Row) string { return r.SessionID }},
	"project":    {name: "project", header: "Project", csvHeader: "project", width: 24, left: true, value: func(r core.Row) string { return r.Project }},
	"start":      {name: "start", header: "Start", csvHeader: "start", width: 20, left: true, value: func(r core.Row) string { return formatTime(r.Start) }},
	"last":       {name: "last", header: "Last Activity", csvHeader: "last_activity", width: 20, left: true, value: func(r core.Row) string { return formatTime(r.LastActivity) }},
	"duration":   {name: "duration", header: "Duration", csvHeader: "duration", width: 10, value: func(r core.Row) string { return formatDuration(r.LastActivity.Sub(r.Start)) }},
	"input":      {name: "input", header: "Input", csvHeader: "input_tokens", width: 15, value: func(r core.Row) string { return formatInt(r.Input) }, total: func(t core.Tokens) string { return formatInt(t.Input) }},
	"output":     {name: "output", header: "Output", csvHeader: "output_tokens", width: 12, value: func(r core.Row) string { return formatInt(r.Output) }, total: func(t core.Tokens) string { return formatInt(t.Output) }},
	"cache_cr":   {name: "cache_cr", header: "Cache Cr.", csvHeader: "cache_creation_tokens", width: 12, value: func(r core.Row) string { return formatInt(r.CacheCreation) }, total: func(t core.Tokens) string { return formatInt(t.CacheCreation) }},
	"cache_rd":   {name: "cache_rd", header: "Cache Rd.", csvHeader: "cache_read_tokens", width: 15, value: func(r core.Row) string { return formatInt(r.CacheRead) }, total: func(t core.Tokens) string { return formatInt(t.CacheRead) }},
	"cache":      {name: "cache", header: "Cache", csvHeader: "cache_tokens", width: 15, value: func(r core.Row) string { return formatInt(r.CacheCreation + r.CacheRead) }, total: func(t core.Tokens) string { return formatInt(t.CacheCreation + t.CacheRead) }},
	"reasoning":  {name: "reasoning", header: "Reasoning", csvHeader: "reasoning_tokens", width: 15, value: func(r core.Row) string { return formatInt(r.Reasoning) }, total: func(t core.Tokens) string { return formatInt(t.Reasoning) }},
	"total":      {name: "total", header: "Total", csvHeader: "total_tokens", width: 15, value: func(r core.Row) string { return formatInt(r.Total()) }, total: func(t core.Tokens) string { return formatInt(t.Total()) }},
	"cost":       {name: "cost", header: "Cost", csvHeader: "cost_usd", width: 11, value: func(r core.Row) string { return formatCost(r.CostUSD) }, total: func(t core.Tokens) string { return formatCost(t.CostUSD) }},
	"credits":    {name: "credits", header: "Credits", csvHeader: "credits", width: 11, value: func(r core.Row) string { return fmt.Sprintf("%.6f", r.Credits) }, total: func(t core.Tokens) string { return fmt.Sprintf("%.6f", t.Credits) }},
	"models":     {name: "models", header: "Models", csvHeader: "models", left: true, value: func(r core.Row) string { return strings.Join(r.ModelsUsed, ",") }},
}

var fieldAliases = map[string]string{
	"cache_creation": "cache_cr",
	"cache_read":     "cache_rd",
	"model":          "models",
}

var defaultTableFields = []string{"key", "input", "output", "cache_cr", "cache_rd", "total", "cost", "models"}

func validFieldNames() []string {
	return []string{"key", "input", "output", "cache_cr", "cache_rd", "cache", "reasoning", "total", "cost", "credits", "models", "source", "session_id", "project", "start", "last", "duration"}
}

// formatDuration renders a row's active span (LastActivity-Start) compactly,
// e.g. "8h30m". Negative or zero spans render as "0m".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	s := d.Round(time.Minute).String()
	if trimmed := strings.TrimSuffix(s, "0s"); trimmed != "" {
		s = trimmed
	}
	return s
}

func parseFields(s string) ([]string, error) {
	var fields []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if alias, ok := fieldAliases[name]; ok {
			name = alias
		}
		if _, ok := fieldRegistry[name]; !ok {
			return nil, fmt.Errorf("unknown field %q; valid fields: %s", name, strings.Join(validFieldNames(), ", "))
		}
		fields = append(fields, name)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("--fields must include at least one field; valid fields: %s", strings.Join(validFieldNames(), ", "))
	}
	return fields, nil
}

func selectedColumns(q core.Query, opts RenderOptions) []fieldColumn {
	fields := opts.Fields
	if len(fields) == 0 {
		fields = defaultTableFields
	}
	cols := make([]fieldColumn, 0, len(fields))
	for _, name := range fields {
		col := fieldRegistry[name]
		if name == "key" {
			col.header = keyHeader(q)
		}
		cols = append(cols, col)
	}
	return cols
}

func columnHeaders(cols []fieldColumn, q core.Query) []string {
	headers := make([]string, len(cols))
	for i, col := range cols {
		if col.name == "key" {
			headers[i] = keyHeader(q)
			continue
		}
		headers[i] = col.header
	}
	return headers
}

func csvHeaders(cols []fieldColumn) []string {
	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = col.csvHeader
	}
	return headers
}

func rowCells(cols []fieldColumn, row core.Row, _ core.Query, breakdown bool, csvMode bool, widths []int) []string {
	cells := make([]string, len(cols))
	for i, col := range cols {
		if breakdown && col.name == "models" {
			cells[i] = ""
			continue
		}
		if csvMode {
			cells[i] = csvFieldValue(col.name, row)
		} else {
			cells[i] = col.value(row)
		}
		if breakdown && col.name == "key" && !strings.HasPrefix(cells[i], "  ") {
			if !csvMode && len(widths) > i {
				cells[i] = "  " + truncate(cells[i], widths[i]-2)
			} else {
				cells[i] = "  " + cells[i]
			}
		}
	}
	return cells
}

func csvFieldValue(name string, row core.Row) string {
	switch name {
	case "input":
		return fmt.Sprintf("%d", row.Input)
	case "output":
		return fmt.Sprintf("%d", row.Output)
	case "cache_cr":
		return fmt.Sprintf("%d", row.CacheCreation)
	case "cache_rd":
		return fmt.Sprintf("%d", row.CacheRead)
	case "reasoning":
		return fmt.Sprintf("%d", row.Reasoning)
	case "total":
		return fmt.Sprintf("%d", row.Total())
	case "cost":
		return fmt.Sprintf("%.6f", row.CostUSD)
	case "credits":
		return fmt.Sprintf("%.6f", row.Credits)
	case "models":
		return strings.Join(row.ModelsUsed, ";")
	default:
		return fieldRegistry[name].value(row)
	}
}

func totalCells(cols []fieldColumn, totals core.Tokens) []string {
	cells := make([]string, len(cols))
	for i, col := range cols {
		if col.total != nil {
			cells[i] = col.total(totals)
		}
	}
	return cells
}

func htmlHeaderRow(cols []fieldColumn, q core.Query) string {
	var b strings.Builder
	b.WriteString(`<table><thead><tr>`)
	for _, header := range columnHeaders(cols, q) {
		b.WriteString(`<th>`)
		b.WriteString(html.EscapeString(header))
		b.WriteString(`</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	return b.String()
}

func formatCost(f float64) string {
	return fmt.Sprintf("$%.4f", f)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatInt(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return sign + strings.Join(parts, ",")
}

func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}
