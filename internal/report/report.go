package report

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/DevrajJain04/reqres/internal/model"
)

func WriteJSON(path string, data model.RunReport) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o644)
}

func WriteHTML(path string, data model.RunReport) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>ReqRes Report</title>")
	b.WriteString("<style>")
	b.WriteString("body{font-family:Segoe UI,Arial,sans-serif;background:#f5f7fb;color:#172033;padding:20px;}")
	b.WriteString(".card{background:#fff;border-radius:12px;padding:16px;margin-bottom:16px;box-shadow:0 8px 24px rgba(20,30,60,.08);}")
	b.WriteString("table{width:100%;border-collapse:collapse;}th,td{padding:8px;border-bottom:1px solid #e5e7ef;text-align:left;}")
	b.WriteString(".pass{color:#0a7b35;font-weight:600}.fail{color:#a40f2c;font-weight:600}.skip{color:#8a6c00;font-weight:600}")
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>ReqRes Run Report</h1>")
	b.WriteString("<div class=\"card\">")
	b.WriteString(fmt.Sprintf("<p><strong>Total:</strong> %d | <strong>Pass:</strong> %d | <strong>Fail:</strong> %d | <strong>Skip:</strong> %d</p>",
		data.Total, data.Passed, data.Failed, data.Skipped))
	b.WriteString(fmt.Sprintf("<p><strong>Duration:</strong> %d ms</p>", data.DurationMS))
	if len(data.Flaky) > 0 {
		b.WriteString("<p><strong>Flaky:</strong> " + html.EscapeString(strings.Join(data.Flaky, ", ")) + "</p>")
	}
	b.WriteString("</div>")

	for _, file := range data.Files {
		b.WriteString("<div class=\"card\">")
		b.WriteString(fmt.Sprintf("<h2>%s</h2>", html.EscapeString(file.File)))
		b.WriteString("<table><thead><tr><th>Test</th><th>Method</th><th>Path</th><th>Status</th><th>Message</th><th>Duration (ms)</th></tr></thead><tbody>")
		for _, test := range file.Tests {
			statusClass := string(test.Status)
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(test.Name) + "</td>")
			b.WriteString("<td>" + html.EscapeString(test.Method) + "</td>")
			b.WriteString("<td>" + html.EscapeString(test.Path) + "</td>")
			b.WriteString(fmt.Sprintf("<td class=\"%s\">%s</td>", statusClass, html.EscapeString(string(test.Status))))
			b.WriteString("<td>" + html.EscapeString(test.Message) + "</td>")
			b.WriteString(fmt.Sprintf("<td>%d</td>", test.DurationMS))
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
		b.WriteString("</div>")
	}

	if data.Load != nil {
		load := data.Load
		b.WriteString("<div class=\"card\">")
		b.WriteString("<h2>Load Test</h2>")
		b.WriteString(fmt.Sprintf("<p>%s %s | users=%d | requests=%d | success=%d | fail=%d</p>",
			html.EscapeString(load.Method), html.EscapeString(load.Path), load.Users, load.Requests, load.Successes, load.Failures))
		b.WriteString(fmt.Sprintf("<p>avg=%0.2fms p95=%0.2fms min=%0.2fms max=%0.2fms</p>", load.AvgMS, load.P95MS, load.MinMS, load.MaxMS))
		b.WriteString("</div>")
	}

	b.WriteString("</body></html>")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
