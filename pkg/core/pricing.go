package core

import "strings"

type price struct {
	input, output, cacheWrite, cacheRead float64
}

// builtinPrices holds USD prices per million tokens for each model family.
// Last verified against provider pricing pages on 2026-06-09. When adding a
// model, also add a normalizeModel case if dated or suffixed variants of the
// ID exist in the wild.
var builtinPrices = map[string]price{
	"claude-fable-5":       {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1},
	"claude-mythos-5":      {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1},
	"claude-opus-4-8":      {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-7":      {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-6":      {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-5":      {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-1":      {input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5},
	"claude-opus-4":        {input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5},
	"claude-sonnet-4-6":    {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-sonnet-4-5":    {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-sonnet-4":      {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-haiku-4-5":     {input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.1},
	"claude-3-7-sonnet":    {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-3-5-sonnet":    {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-3-5-haiku":     {input: 0.8, output: 4, cacheWrite: 1, cacheRead: 0.08},
	"gpt-5.6-sol":          {input: 5, output: 30, cacheRead: 0.5},
	"gpt-5.6-terra":        {input: 2.5, output: 15, cacheRead: 0.25},
	"gpt-5.6-luna":         {input: 1, output: 6, cacheRead: 0.1},
	"gpt-5.5":              {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-5.4":              {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-5.3-codex":        {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-5.3-codex-spark":  {input: 0.25, output: 2, cacheRead: 0.025},
	"gpt-5.2":              {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-5":                {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-4.1":              {input: 2, output: 8, cacheRead: 0.5},
	"gpt-4.1-mini":         {input: 0.4, output: 1.6, cacheRead: 0.1},
	"gemini-3-pro-preview": {input: 2, output: 12, cacheRead: 0.2},
	"gemini-2.5-pro":       {input: 1.25, output: 10, cacheRead: 0.31},
	"gemini-2.5-flash":     {input: 0.3, output: 2.5, cacheRead: 0.075},
}

func calculateCost(model string, tok Tokens, speed string) float64 {
	p, ok := lookupPrice(model)
	if !ok {
		return 0
	}
	if p.cacheWrite == 0 {
		p.cacheWrite = p.input
	}
	if p.cacheRead == 0 {
		p.cacheRead = p.input
	}
	mult := 1.0
	if speed == "fast" {
		mult = fastMultiplier(model)
	}
	return (float64(tok.Input)*p.input + float64(tok.Output)*p.output + float64(tok.CacheCreation)*p.cacheWrite + float64(tok.CacheRead)*p.cacheRead) / 1_000_000 * mult
}

// fastMultipliers holds the fast-mode price multiplier for the models that
// offer Anthropic fast mode. Fast mode raises the base input/output rate, and
// prompt-caching multipliers apply on top of that fast base, so the multiplier
// applies uniformly across the input, output, and cache token categories:
//
//	Opus 4.8: $10/$50 fast vs $5/$25 standard  -> 2x
//	Opus 4.7: $30/$150 fast vs $5/$25 standard -> 6x
//	Opus 4.6: $30/$150 fast vs $5/$25 standard -> 6x
var fastMultipliers = map[string]float64{
	"claude-opus-4-8": 2,
	"claude-opus-4-7": 6,
	"claude-opus-4-6": 6,
}

// fastMultiplier returns the fast-mode price multiplier for a model, or 1 when
// the model does not offer fast-mode pricing.
func fastMultiplier(model string) float64 {
	if m, ok := fastMultipliers[normalizeModel(model)]; ok {
		return m
	}
	return 1
}

func lookupPrice(model string) (price, bool) {
	m := normalizeModel(model)
	if p, ok := builtinPrices[m]; ok {
		return p, true
	}
	for k, p := range builtinPrices {
		if strings.Contains(m, k) {
			return p, true
		}
	}
	return price{}, false
}

func normalizeModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "anthropic/")
	m = strings.TrimPrefix(m, "openai/")
	m = strings.TrimPrefix(m, "google/")
	m = strings.TrimPrefix(m, "google-gla/")
	m = strings.TrimPrefix(m, "vertex/")
	switch {
	case strings.Contains(m, "gemini-3-pro"):
		return "gemini-3-pro-preview"
	case strings.Contains(m, "gpt-5.6-terra"):
		return "gpt-5.6-terra"
	case strings.Contains(m, "gpt-5.6-luna"):
		return "gpt-5.6-luna"
	case strings.Contains(m, "gpt-5.6"):
		// The bare gpt-5.6 alias routes to the Sol flagship tier.
		return "gpt-5.6-sol"
	case strings.Contains(m, "fable-5"):
		return "claude-fable-5"
	case strings.Contains(m, "mythos-5"):
		return "claude-mythos-5"
	case strings.Contains(m, "opus-4-8"):
		return "claude-opus-4-8"
	case strings.Contains(m, "opus-4-7"):
		return "claude-opus-4-7"
	case strings.Contains(m, "opus-4-6"):
		return "claude-opus-4-6"
	case strings.Contains(m, "opus-4-5"):
		return "claude-opus-4-5"
	case strings.Contains(m, "opus-4-1"):
		return "claude-opus-4-1"
	case strings.Contains(m, "sonnet-4-6"):
		return "claude-sonnet-4-6"
	case strings.Contains(m, "sonnet-4-5"):
		return "claude-sonnet-4-5"
	case strings.Contains(m, "haiku-4-5"):
		return "claude-haiku-4-5"
	}
	return m
}
