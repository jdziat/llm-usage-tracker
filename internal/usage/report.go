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
	Format     string
	OutputPath string
	Breakdown  bool
	Compact    bool
	TokenLimit int64
	Debug      bool

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
	headers := []string{keyHeader(q), "Input", "Output", "Cache Cr.", "Cache Rd.", "Total", "Cost", "Models"}
	widths := reportWidths(q, opts)
	printRow(w, widths, headers)
	printSep(w, widths)
	for _, row := range res.Rows {
		printRow(w, widths, []string{
			row.Key,
			formatInt(row.Input),
			formatInt(row.Output),
			formatInt(row.CacheCreation),
			formatInt(row.CacheRead),
			formatInt(row.Total()),
			formatCost(row.CostUSD),
			truncate(strings.Join(row.ModelsUsed, ","), widths[7]),
		})
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				printRow(w, widths, []string{
					"  " + truncate(b.Model, widths[0]-2),
					formatInt(b.Input),
					formatInt(b.Output),
					formatInt(b.CacheCreation),
					formatInt(b.CacheRead),
					formatInt(b.Total()),
					formatCost(b.CostUSD),
					"",
				})
			}
		}
	}
	printSep(w, widths)
	printRow(w, widths, []string{"Total", formatInt(res.Totals.Input), formatInt(res.Totals.Output), formatInt(res.Totals.CacheCreation), formatInt(res.Totals.CacheRead), formatInt(res.Totals.Total()), formatCost(res.Totals.CostUSD), ""})
}

func writePrettyTable(w io.Writer, title string, res core.Result, q core.Query, opts RenderOptions) {
	fmt.Fprintln(w, title)
	widths := reportWidths(q, opts)
	headers := []string{keyHeader(q), "Input", "Output", "Cache Cr.", "Cache Rd.", "Total", "Cost", "Models"}
	printBoxBorder(w, widths, "top")
	printBoxRow(w, widths, headers)
	printBoxBorder(w, widths, "mid")
	for _, row := range res.Rows {
		printBoxRow(w, widths, []string{
			row.Key,
			formatInt(row.Input),
			formatInt(row.Output),
			formatInt(row.CacheCreation),
			formatInt(row.CacheRead),
			formatInt(row.Total()),
			formatCost(row.CostUSD),
			truncate(strings.Join(row.ModelsUsed, ","), widths[7]),
		})
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				printBoxRow(w, widths, []string{
					"  " + truncate(b.Model, widths[0]-2),
					formatInt(b.Input),
					formatInt(b.Output),
					formatInt(b.CacheCreation),
					formatInt(b.CacheRead),
					formatInt(b.Total()),
					formatCost(b.CostUSD),
					"",
				})
			}
		}
	}
	printBoxBorder(w, widths, "mid")
	printBoxRow(w, widths, []string{"Total", formatInt(res.Totals.Input), formatInt(res.Totals.Output), formatInt(res.Totals.CacheCreation), formatInt(res.Totals.CacheRead), formatInt(res.Totals.Total()), formatCost(res.Totals.CostUSD), ""})
	printBoxBorder(w, widths, "bottom")
}

func writeCSV(w io.Writer, opts RenderOptions, rows []core.Row) error {
	cw := csv.NewWriter(w)
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
	fmt.Fprintln(w, `<table><thead><tr><th>`+html.EscapeString(keyHeader(q))+`</th><th>Input</th><th>Output</th><th>Cache Cr.</th><th>Cache Rd.</th><th>Total</th><th>Cost</th><th>Models</th></tr></thead><tbody>`)
	for _, row := range res.Rows {
		writeHTMLRow(w, row, "")
		if opts.Breakdown {
			for _, b := range row.ModelBreakdowns {
				breakdown := row
				breakdown.Key = b.Model
				breakdown.ModelsUsed = nil
				breakdown.Tokens = b.Tokens
				writeHTMLRow(w, breakdown, ` class="breakdown"`)
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

func printRow(w io.Writer, widths []int, cols []string) {
	for i, col := range cols {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		if i == 0 || i == len(cols)-1 {
			fmt.Fprintf(w, "%-*s", widths[i], truncate(col, widths[i]))
		} else {
			fmt.Fprintf(w, "%*s", widths[i], truncate(col, widths[i]))
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

func printBoxRow(w io.Writer, widths []int, cols []string) {
	fmt.Fprint(w, "|")
	for i, col := range cols {
		if i == 0 || i == len(cols)-1 {
			fmt.Fprintf(w, " %-*s |", widths[i], truncate(col, widths[i]))
		} else {
			fmt.Fprintf(w, " %*s |", widths[i], truncate(col, widths[i]))
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

func writeHTMLRow(w io.Writer, row core.Row, class string) {
	fmt.Fprintf(w, "<tr%s>", class)
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(row.Key))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatInt(row.Input)))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatInt(row.Output)))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatInt(row.CacheCreation)))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatInt(row.CacheRead)))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatInt(row.Total())))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(formatCost(row.CostUSD)))
	fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(strings.Join(row.ModelsUsed, ", ")))
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

func reportWidths(q core.Query, opts RenderOptions) []int {
	return []int{minWidth(q, opts), 15, 12, 12, 15, 15, 11, modelWidth(opts)}
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
