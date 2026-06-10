package core

import "time"

const (
	SourceClaude   = "claude"
	SourceCodex    = "codex"
	SourceOpenCode = "opencode"
	SourceAmp      = "amp"
	SourcePI       = "pi"
)

var allSources = []string{SourceClaude, SourceCodex, SourceOpenCode, SourceAmp, SourcePI}

func AllSources() []string {
	return append([]string(nil), allSources...)
}

type Tokens struct {
	Input         int64   `json:"inputTokens"`
	Output        int64   `json:"outputTokens"`
	CacheCreation int64   `json:"cacheCreationTokens"`
	CacheRead     int64   `json:"cacheReadTokens"`
	Reasoning     int64   `json:"reasoningTokens,omitempty"`
	CostUSD       float64 `json:"costUSD"`
	Credits       float64 `json:"credits,omitempty"`
}

func (t *Tokens) Add(o Tokens) {
	t.Input += o.Input
	t.Output += o.Output
	t.CacheCreation += o.CacheCreation
	t.CacheRead += o.CacheRead
	t.Reasoning += o.Reasoning
	t.CostUSD += o.CostUSD
	t.Credits += o.Credits
}

func (t Tokens) Total() int64 {
	return t.Input + t.Output + t.CacheCreation + t.CacheRead
}

type Event struct {
	Source       string    `json:"source"`
	SessionID    string    `json:"sessionId"`
	Project      string    `json:"project,omitempty"`
	Path         string    `json:"path,omitempty"`
	Model        string    `json:"model"`
	Time         time.Time `json:"time"`
	Tokens       Tokens    `json:"tokens"`
	IsFallback   bool      `json:"isFallback,omitempty"`
	RawCostKnown bool      `json:"-"`
}

type ModelBreakdown struct {
	Model string `json:"model"`
	Tokens
}

type Row struct {
	Key             string           `json:"key"`
	Source          string           `json:"source,omitempty"`
	SessionID       string           `json:"sessionId,omitempty"`
	Project         string           `json:"project,omitempty"`
	Start           time.Time        `json:"start,omitempty"`
	LastActivity    time.Time        `json:"lastActivity,omitempty"`
	ModelsUsed      []string         `json:"modelsUsed"`
	ModelBreakdowns []ModelBreakdown `json:"modelBreakdowns,omitempty"`
	Tokens
}
