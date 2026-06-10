package usage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jdziat/llm-usage-tracker/pkg/core"
)

func serveHTTP(q core.Query, opts RenderOptions, stderr io.Writer) error {
	host := opts.serveHost
	if host == "" {
		// An empty host binds every interface; never do that implicitly.
		host = "127.0.0.1"
	}
	if !isLoopbackHost(host) {
		fmt.Fprintf(stderr, "warning: binding to %s exposes the dashboard beyond this machine — it serves your local usage data with no authentication\n", host)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(opts.servePort))
	// TimeoutHandler caps a single request's wall-clock cost; WriteTimeout sits
	// just above it so the 503 can still be written. The handler itself bounds
	// concurrent disk scans with a semaphore (see newServeHandler).
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.TimeoutHandler(newServeHandler(q, opts, host), 30*time.Second, "request timed out"),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "listening on http://%s  (Ctrl-C to stop)\n", addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errc
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newServeHandler(base core.Query, opts RenderOptions, bindHost string) http.Handler {
	// Bound the number of concurrent disk scans so a burst of requests (e.g.
	// from a browser tab) can't pin CPU/IO walking the user's whole history.
	sem := make(chan struct{}, 4)
	load := func(q core.Query) (core.Result, error) {
		sem <- struct{}{}
		defer func() { <-sem }()
		return core.LoadAndAggregate(q, nil)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/api/usage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q, err := serveQuery(base, r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := load(q)
		if err != nil {
			http.Error(w, "failed to load usage: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := writeJSON(w, res); err != nil {
			http.Error(w, "failed to encode usage: "+err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q, err := serveQuery(base, r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := load(q)
		if err != nil {
			http.Error(w, "failed to load usage: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		htmlOpts := opts
		htmlOpts.Format = "html"
		if err := writeHTML(&buf, title(q), q, htmlOpts, res); err != nil {
			http.Error(w, "failed to render usage: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(addMetaRefresh(buf.Bytes(), opts.refreshInterval))
	})
	// Host-header allowlist: a loopback bind does not stop DNS rebinding, where
	// a malicious page rebinds its name to 127.0.0.1 and reads the dashboard.
	// Reject any request whose Host isn't loopback or the configured bind host.
	return hostGuard(mux, bindHost)
}

func hostGuard(next http.Handler, bindHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !isLoopbackHost(host) && host != bindHost {
			http.Error(w, "forbidden: host not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(h string) bool {
	switch h {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func addMetaRefresh(page []byte, interval time.Duration) []byte {
	seconds := int(interval.Round(time.Second) / time.Second)
	if seconds <= 0 {
		seconds = 5
	}
	meta := fmt.Sprintf(`<meta http-equiv="refresh" content="%d">`, seconds)
	return bytes.Replace(page, []byte("</head>"), []byte(meta+"</head>"), 1)
}

func serveQuery(base core.Query, values url.Values) (core.Query, error) {
	q := base
	q.Sources = append([]string(nil), base.Sources...)
	q.SourcePaths = cloneSourcePaths(base.SourcePaths)
	if q.Location == nil {
		q.Location = time.Local
	}

	if v := first(values, "view"); v != "" {
		if !views[v] {
			return q, fmt.Errorf("view must be daily, weekly, monthly, session, summary, blocks, statusline, or trend")
		}
		q.View = v
	}
	if v := first(values, "source"); v != "" {
		parts := strings.Split(v, ",")
		next := make([]string, 0, len(parts))
		for _, part := range parts {
			source := strings.TrimSpace(part)
			if source == "" {
				continue
			}
			if !sources[source] {
				return q, fmt.Errorf("source must be claude, codex, opencode, amp, or pi")
			}
			next = append(next, source)
		}
		if len(next) == 0 {
			return q, fmt.Errorf("source must be claude, codex, opencode, amp, or pi")
		}
		q.Sources = next
	}
	if v := first(values, "timezone"); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			return q, err
		}
		q.Location = loc
	}
	var err error
	if v := first(values, "since"); v != "" {
		q.Since, err = parseDateBound(v, q.Location, false)
		if err != nil {
			return q, err
		}
	}
	if v := first(values, "until"); v != "" {
		q.Until, err = parseDateBound(v, q.Location, true)
		if err != nil {
			return q, err
		}
	}
	if v := first(values, "by"); v != "" {
		if v != "model" && v != "project" && v != "source" {
			return q, fmt.Errorf("--by must be model, project, or source")
		}
		q.By = v
	}
	if v := first(values, "mode"); v != "" {
		if v != "auto" && v != "calculate" && v != "display" {
			return q, fmt.Errorf("--mode must be auto, calculate, or display")
		}
		q.Mode = v
	}
	if v := first(values, "speed"); v != "" {
		if v != "auto" && v != "standard" && v != "fast" {
			return q, fmt.Errorf("--speed must be auto, standard, or fast")
		}
		if v == "auto" {
			v = core.DetectCodexSpeed()
		}
		q.Speed = v
	}
	if v := first(values, "order"); v != "" {
		if v != "asc" && v != "desc" {
			return q, fmt.Errorf("--order must be asc or desc")
		}
		q.Order = v
	}
	if v := first(values, "start-of-week"); v != "" {
		if v != "monday" && v != "sunday" {
			return q, fmt.Errorf("--start-of-week must be monday or sunday")
		}
		q.StartOfWeek = v
	}
	if v := first(values, "project"); v != "" {
		q.Project = v
	}
	if v := first(values, "id"); v != "" {
		q.ID = v
	}
	if v := first(values, "top"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return q, fmt.Errorf("--top must be a non-negative integer")
		}
		q.Top = n
	}
	if v := first(values, "sparkline"); v != "" {
		if v != "cost" && v != "total" && v != "tokens" {
			return q, fmt.Errorf("--sparkline must be cost, total, or tokens")
		}
		q.Sparkline = v
	}
	if err := serveBool(values, "instances", &q.Instances); err != nil {
		return q, err
	}
	if err := serveBool(values, "active", &q.Active); err != nil {
		return q, err
	}
	if err := serveBool(values, "recent", &q.Recent); err != nil {
		return q, err
	}
	// Filesystem-path overrides (path / *-path) are deliberately NOT honored
	// over HTTP: letting a request repoint the scan roots would turn the server
	// into an arbitrary-directory reader. Requests may filter and slice within
	// the roots chosen at launch, never repoint them. The base query's
	// SourcePaths (set from CLI/config at startup) are the only roots scanned.
	if q.View == "statusline" {
		q.Sources = []string{core.SourceClaude}
		q.View = "blocks"
		q.Active = true
	}
	if q.View == "blocks" {
		q.Sources = []string{core.SourceClaude}
	}
	if q.View == "trend" && q.Sparkline == "" {
		q.Sparkline = "cost"
	}
	return q, nil
}

func first(values url.Values, key string) string {
	return strings.TrimSpace(values.Get(key))
}

func serveBool(values url.Values, key string, dst *bool) error {
	v := first(values, key)
	if v == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("--%s must be true or false", key)
	}
	*dst = parsed
	return nil
}

func cloneSourcePaths(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for source, paths := range in {
		out[source] = append([]string(nil), paths...)
	}
	return out
}
