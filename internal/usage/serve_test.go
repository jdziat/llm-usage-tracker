package usage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

const testBindHost = "127.0.0.1"

// loopbackReq builds a request with a loopback Host so it passes the
// DNS-rebinding host guard (httptest defaults Host to "example.com").
func loopbackReq(method, target string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.Host = testBindHost
	return r
}

func TestServeAPIUsageJSON(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 5 * time.Second}, testBindHost)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, loopbackReq(http.MethodGet, "/api/usage"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var out struct {
		View   string           `json:"view"`
		Data   []map[string]any `json:"data"`
		Totals map[string]any   `json:"totals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if out.View != "daily" {
		t.Fatalf("view = %q, want daily", out.View)
	}
	if len(out.Data) != 2 {
		t.Fatalf("data rows = %d, want 2; body = %s", len(out.Data), rec.Body.String())
	}
	if out.Totals == nil {
		t.Fatalf("totals missing in body: %s", rec.Body.String())
	}
}

func TestServeHTMLAndHealthz(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 3 * time.Second}, testBindHost)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, loopbackReq(http.MethodGet, "/healthz"))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", health.Code, health.Body.String())
	}
	if strings.TrimSpace(health.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("health body = %q", health.Body.String())
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, loopbackReq(http.MethodGet, "/"))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d, body = %s", page.Code, page.Body.String())
	}
	if got := page.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}
	body := page.Body.String()
	for _, want := range []string{"Coding Agent Usage Report", `<meta http-equiv="refresh" content="3">`, "<table"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q:\n%s", want, body)
		}
	}
}

func TestServeQueryParamOverride(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 5 * time.Second}, testBindHost)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, loopbackReq(http.MethodGet, "/api/usage?since=2026-05-19&view=summary&by=source"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out core.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if out.View != "summary" {
		t.Fatalf("view = %q, want summary", out.View)
	}
	if len(out.Rows) != 1 {
		t.Fatalf("rows = %d, want 1; body = %s", len(out.Rows), rec.Body.String())
	}
	if out.Rows[0].Key != "claude" {
		t.Fatalf("row key = %q, want claude", out.Rows[0].Key)
	}
	if out.Totals.Input != 30 {
		t.Fatalf("total input = %d, want 30", out.Totals.Input)
	}
}

func TestServeBadParamDoesNotPanic(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 5 * time.Second}, testBindHost)
	for _, path := range []string{"/api/usage?view=bogus", "/?top=bad"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, loopbackReq(http.MethodGet, path))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if strings.TrimSpace(rec.Body.String()) == "" {
				t.Fatal("empty error response")
			}
		})
	}
}

// TestServeRejectsRebindingHost verifies the DNS-rebinding guard: a request
// whose Host is not loopback (or the bind host) is forbidden.
func TestServeRejectsRebindingHost(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 5 * time.Second}, testBindHost)
	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback Host status = %d, want 403", rec.Code)
	}
}

// TestServeIgnoresPathOverride verifies the security fix: filesystem-path query
// params can NOT repoint the scan roots. Pointing claude-path at a bogus dir
// must not change what's scanned — the response still reflects the launch-time
// root, proving the param is ignored (no arbitrary-directory read).
func TestServeIgnoresPathOverride(t *testing.T) {
	handler := newServeHandler(testServeQuery(t), RenderOptions{refreshInterval: 5 * time.Second}, testBindHost)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, loopbackReq(http.MethodGet, "/api/usage?claude-path=/nonexistent-dir&path=/etc"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out core.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	// Still 2 rows from the launch-time fixture root; the path params were ignored.
	if len(out.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (path override must be ignored); body = %s", len(out.Rows), rec.Body.String())
	}
}

func TestParseArgsServeMode(t *testing.T) {
	q, opts, live, err := parseArgs([]string{"claude", "serve", "--view", "summary", "--port", "9999", "--host", "127.0.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Fatal("live = true, want false")
	}
	if !opts.serve {
		t.Fatal("serve mode was not enabled")
	}
	if opts.servePort != 9999 || opts.serveHost != "127.0.0.2" {
		t.Fatalf("serve addr = %s:%d, want 127.0.0.2:9999", opts.serveHost, opts.servePort)
	}
	if len(q.Sources) != 1 || q.Sources[0] != core.SourceClaude {
		t.Fatalf("sources = %v, want claude", q.Sources)
	}
	if q.View != "summary" {
		t.Fatalf("view = %q, want summary", q.View)
	}
}

func testServeQuery(t *testing.T) core.Query {
	t.Helper()
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"requestId":"req_1","type":"assistant","timestamp":"2026-05-18T10:00:00Z","cwd":"/work/repo","sessionId":"s1","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4,"costUSD":1.25}}}
{"requestId":"req_2","type":"assistant","timestamp":"2026-05-19T10:00:00Z","cwd":"/work/repo","sessionId":"s2","message":{"role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":30,"cache_creation_input_tokens":6,"cache_read_input_tokens":9,"output_tokens":12,"costUSD":2.50}}}
`
	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.Query{
		View:          "daily",
		Sources:       []string{core.SourceClaude},
		Location:      time.UTC,
		Order:         "asc",
		Mode:          "display",
		SessionLength: 5 * time.Hour,
		SourcePaths:   map[string][]string{core.SourceClaude: {root}},
		Speed:         "standard",
		StartOfWeek:   "monday",
		By:            "model",
	}
}
