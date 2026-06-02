package usage

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func getPath(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		cm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = cm[p]
	}
	return cur
}

func stringAt(m map[string]any, path ...string) string {
	v := getPath(m, path...)
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		return ""
	}
}

func intAt(m map[string]any, path ...string) int64 {
	v := getPath(m, path...)
	switch x := v.(type) {
	case json.Number:
		n, _ := x.Int64()
		return n
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		return 0
	}
}

func floatAt(m map[string]any, path ...string) float64 {
	v := getPath(m, path...)
	switch x := v.(type) {
	case json.Number:
		f, _ := x.Float64()
		return f
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	default:
		return 0
	}
}

func parseTimeAny(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseJSONLine(line []byte) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(line)))
	dec.UseNumber()
	var m map[string]any
	err := dec.Decode(&m)
	return m, err
}

func basenameSession(path string) string {
	name := path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".jsonl")
	name = strings.TrimSuffix(name, ".json")
	if strings.HasPrefix(name, "rollout-") {
		parts := strings.Split(name, "-")
		if len(parts) >= 7 {
			return strings.Join(parts[4:], "-")
		}
	}
	return name
}

func sourceLabel(s string) string {
	if s == SourcePI {
		return "pi-agent"
	}
	return s
}
