package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func loadEvents(cfg Config) ([]Event, []string, error) {
	var events []Event
	var warnings []string
	for _, source := range cfg.Sources {
		paths := cfg.SourcePaths[source]
		if len(paths) == 0 {
			paths = defaultSourcePaths(source)
		}
		if cfg.progress != nil {
			cfg.progress.Set("Scanning " + sourceLabel(source) + ": discovering files")
		}
		var got []Event
		var err error
		switch source {
		case SourceClaude:
			got, err = readClaude(paths, cfg)
		case SourceCodex:
			got, err = readCodex(paths, cfg)
		default:
			got, err = readGenericSource(source, paths, cfg)
		}
		if err != nil {
			warnings = append(warnings, source+": "+err.Error())
			continue
		}
		if cfg.progress != nil {
			cfg.progress.Done(fmt.Sprintf("Scanned %s: %d events", sourceLabel(source), len(got)))
		}
		events = append(events, got...)
	}
	if cfg.progress != nil {
		cfg.progress.Set(fmt.Sprintf("Calculating costs for %d events", len(events)))
	}
	for i := range events {
		if events[i].Tokens.CostUSD == 0 || cfg.Mode == "calculate" || (cfg.Mode == "auto" && !events[i].RawCostKnown) {
			events[i].Tokens.CostUSD = calculateCost(events[i].Model, events[i].Tokens, cfg.Speed)
		}
	}
	if cfg.progress != nil {
		cfg.progress.Done(fmt.Sprintf("Calculated costs for %d events", len(events)))
	}
	return events, warnings, nil
}

func readClaude(roots []string, cfg Config) ([]Event, error) {
	files, err := discoverFiles(roots, func(p string) bool { return strings.HasSuffix(p, ".jsonl") })
	if err != nil {
		return nil, err
	}
	if cfg.progress != nil {
		cfg.progress.Set(fmt.Sprintf("Scanning Claude: 0/%d files", len(files)))
	}
	var out []Event
	seen := map[string]bool{}
	for i, file := range files {
		events, err := scanJSONL(file, func(m map[string]any) (Event, bool) {
			msg := asMap(m["message"])
			if msg == nil || stringAt(msg, "role") != "assistant" {
				return Event{}, false
			}
			usage := asMap(msg["usage"])
			if usage == nil {
				return Event{}, false
			}
			key := stringAt(m, "requestId")
			if key == "" {
				key = stringAt(msg, "id")
			}
			if key == "" {
				key = stringAt(m, "uuid")
			}
			if key != "" {
				key = file + ":" + key
				if seen[key] {
					return Event{}, false
				}
				seen[key] = true
			}
			tok := Tokens{
				Input:         intAt(usage, "input_tokens"),
				Output:        intAt(usage, "output_tokens"),
				CacheCreation: intAt(usage, "cache_creation_input_tokens"),
				CacheRead:     intAt(usage, "cache_read_input_tokens"),
				CostUSD:       floatAt(usage, "costUSD"),
			}
			if stringAt(msg, "model") == "<synthetic>" && tok.Total() == 0 && tok.CostUSD == 0 {
				return Event{}, false
			}
			when := parseTimeAny(stringAt(m, "timestamp"))
			return Event{
				Source:       SourceClaude,
				SessionID:    firstNonEmpty(stringAt(m, "sessionId"), basenameSession(file)),
				Project:      firstNonEmpty(stringAt(m, "cwd"), filepath.Base(filepath.Dir(file))),
				Path:         file,
				Model:        firstNonEmpty(stringAt(msg, "model"), "unknown"),
				Time:         when,
				Tokens:       tok,
				RawCostKnown: tok.CostUSD > 0,
			}, true
		})
		if err != nil && cfg.Debug {
			continue
		}
		out = append(out, events...)
		if cfg.progress != nil {
			cfg.progress.Set(fmt.Sprintf("Scanning Claude: %d/%d files, %d events", i+1, len(files), len(out)))
		}
	}
	return out, nil
}

func readCodex(roots []string, cfg Config) ([]Event, error) {
	files, err := discoverFiles(roots, func(p string) bool { return strings.HasSuffix(p, ".jsonl") })
	if err != nil {
		return nil, err
	}
	if cfg.progress != nil {
		cfg.progress.Set(fmt.Sprintf("Scanning Codex: 0/%d files", len(files)))
	}
	var out []Event
	for i, file := range files {
		sessionID := basenameSession(file)
		project := ""
		model := ""
		var prev Tokens
		events, err := scanJSONL(file, func(m map[string]any) (Event, bool) {
			typ := stringAt(m, "type")
			payload := asMap(m["payload"])
			switch typ {
			case "session_meta":
				if id := stringAt(payload, "id"); id != "" {
					sessionID = id
				}
				project = stringAt(payload, "cwd")
				model = stringAt(payload, "model")
				return Event{}, false
			case "turn_context":
				project = firstNonEmpty(stringAt(payload, "cwd"), project)
				model = firstNonEmpty(stringAt(payload, "model"), model)
				return Event{}, false
			case "event_msg":
				if stringAt(payload, "type") != "token_count" {
					return Event{}, false
				}
				info := asMap(payload["info"])
				if info == nil {
					return Event{}, false
				}
				usage := asMap(info["last_token_usage"])
				if usage == nil {
					total := codexTokens(asMap(info["total_token_usage"]))
					delta := Tokens{
						Input:     total.Input - prev.Input,
						Output:    total.Output - prev.Output,
						CacheRead: total.CacheRead - prev.CacheRead,
						Reasoning: total.Reasoning - prev.Reasoning,
					}
					prev = total
					return makeCodexEvent(file, sessionID, project, model, parseTimeAny(stringAt(m, "timestamp")), delta), delta.Total() > 0
				}
				tok := codexTokens(usage)
				return makeCodexEvent(file, sessionID, project, model, parseTimeAny(stringAt(m, "timestamp")), tok), tok.Total() > 0
			default:
				return Event{}, false
			}
		})
		if err != nil && cfg.Debug {
			continue
		}
		out = append(out, events...)
		if cfg.progress != nil {
			cfg.progress.Set(fmt.Sprintf("Scanning Codex: %d/%d files, %d events", i+1, len(files), len(out)))
		}
	}
	return out, nil
}

func codexTokens(usage map[string]any) Tokens {
	inputTotal := intAt(usage, "input_tokens")
	cache := intAt(usage, "cached_input_tokens")
	return Tokens{
		Input:     max64(0, inputTotal-cache),
		Output:    intAt(usage, "output_tokens"),
		CacheRead: cache,
		Reasoning: intAt(usage, "reasoning_output_tokens"),
	}
}

func makeCodexEvent(file, sessionID, project, model string, when time.Time, tok Tokens) Event {
	if model == "" {
		model = "gpt-5"
	}
	return Event{
		Source:     SourceCodex,
		SessionID:  firstNonEmpty(sessionID, basenameSession(file)),
		Project:    project,
		Path:       file,
		Model:      model,
		Time:       when,
		Tokens:     tok,
		IsFallback: model == "gpt-5",
	}
}

func readGenericSource(source string, roots []string, cfg Config) ([]Event, error) {
	files, err := discoverFiles(roots, func(p string) bool {
		return strings.HasSuffix(p, ".jsonl") || strings.HasSuffix(p, ".json")
	})
	if err != nil {
		return nil, err
	}
	if cfg.progress != nil {
		cfg.progress.Set(fmt.Sprintf("Scanning %s: 0/%d files", sourceLabel(source), len(files)))
	}
	var out []Event
	for i, file := range files {
		if strings.HasSuffix(file, ".jsonl") {
			events, _ := scanJSONL(file, func(m map[string]any) (Event, bool) {
				return genericEvent(source, file, m)
			})
			out = append(out, events...)
			if cfg.progress != nil {
				cfg.progress.Set(fmt.Sprintf("Scanning %s: %d/%d files, %d events", sourceLabel(source), i+1, len(files), len(out)))
			}
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil || !json.Valid(b) {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var m map[string]any
		if dec.Decode(&m) == nil {
			if ev, ok := genericEvent(source, file, m); ok {
				out = append(out, ev)
			}
		}
		if cfg.progress != nil {
			cfg.progress.Set(fmt.Sprintf("Scanning %s: %d/%d files, %d events", sourceLabel(source), i+1, len(files), len(out)))
		}
	}
	return out, nil
}

func genericEvent(source, file string, m map[string]any) (Event, bool) {
	usage := findUsageMap(m)
	if usage == nil {
		return Event{}, false
	}
	tok := Tokens{
		Input:         firstInt(usage, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens"),
		Output:        firstInt(usage, "output_tokens", "outputTokens", "completion_tokens", "completionTokens"),
		CacheCreation: firstInt(usage, "cache_creation_input_tokens", "cacheCreationInputTokens", "cache_creation_tokens", "cacheCreationTokens"),
		CacheRead:     firstInt(usage, "cache_read_input_tokens", "cacheReadInputTokens", "cached_input_tokens", "cachedInputTokens"),
		Reasoning:     firstInt(usage, "reasoning_output_tokens", "reasoningTokens"),
		CostUSD:       firstFloat(usage, "cost", "costUSD", "totalCost", "total_cost"),
		Credits:       firstFloat(usage, "credits", "credit"),
	}
	if tok.Total() == 0 && tok.CostUSD == 0 && tok.Credits == 0 {
		return Event{}, false
	}
	when := firstTime(m, usage)
	model := firstNonEmpty(
		stringAt(m, "model"),
		stringAt(m, "modelID"),
		stringAt(m, "modelId"),
		stringAt(usage, "model"),
		stringAt(asMap(m["message"]), "model"),
		"unknown",
	)
	return Event{
		Source:       source,
		SessionID:    firstNonEmpty(stringAt(m, "sessionID"), stringAt(m, "sessionId"), stringAt(m, "id"), basenameSession(file)),
		Project:      firstNonEmpty(stringAt(m, "cwd"), stringAt(m, "project"), filepath.Base(filepath.Dir(file))),
		Path:         file,
		Model:        model,
		Time:         when,
		Tokens:       tok,
		RawCostKnown: tok.CostUSD > 0,
	}, true
}

func scanJSONL(file string, fn func(map[string]any) (Event, bool)) ([]Event, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("{")) {
			continue
		}
		m, err := parseJSONLine(line)
		if err != nil {
			continue
		}
		if ev, ok := fn(m); ok {
			out = append(out, ev)
		}
	}
	return out, sc.Err()
}

func findUsageMap(m map[string]any) map[string]any {
	keys := []string{"usage", "tokenUsage", "token_usage", "total_token_usage", "last_token_usage"}
	for _, k := range keys {
		if u := asMap(m[k]); looksLikeUsage(u) {
			return u
		}
	}
	for _, v := range m {
		if cm := asMap(v); cm != nil {
			if u := findUsageMap(cm); u != nil {
				return u
			}
		}
	}
	return nil
}

func looksLikeUsage(m map[string]any) bool {
	if m == nil {
		return false
	}
	return firstInt(m, "input_tokens", "inputTokens", "prompt_tokens", "output_tokens", "outputTokens", "completion_tokens") > 0 ||
		firstFloat(m, "cost", "costUSD", "totalCost") > 0
}

func firstTime(ms ...map[string]any) time.Time {
	for _, m := range ms {
		for _, k := range []string{"timestamp", "time", "createdAt", "updatedAt", "lastActivity"} {
			if t := parseTimeAny(stringAt(m, k)); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

func firstInt(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if n := intAt(m, k); n != 0 {
			return n
		}
	}
	return 0
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if n := floatAt(m, k); n != 0 {
			return n
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
