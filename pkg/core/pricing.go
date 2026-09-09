package core

import "strings"

type price struct {
	input, output, cacheWrite, cacheRead float64
}

// builtinPrices holds USD prices per million tokens for each model family.
// Last verified against provider pricing pages on 2026-09-08. When adding a
// model, also add a normalizeModel case if dated or suffixed variants of the
// ID exist in the wild.
//
// Anthropic cache reads are 0.1x base input, except Fable 5.1 and Mythos 5.1
// at 0.025x. OpenAI publishes cache-write rates only for the GPT-5.6 family
// and GPT-6 Astra (1.25x input); the rest leave cacheWrite zero and fall back
// to the input rate in calculateCost.
var builtinPrices = map[string]price{
	"claude-fable-5-1":       {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 0.25},
	"claude-mythos-5-1":      {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 0.25},
	"claude-fable-5":         {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1},
	"claude-mythos-5":        {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1},
	"claude-opus-5":          {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-8":        {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-7":        {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-6":        {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-5":        {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-1":        {input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5},
	"claude-opus-4":          {input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5},
	"claude-sonnet-5":        {input: 2, output: 10, cacheWrite: 2.5, cacheRead: 0.2},
	"claude-sonnet-4-6":      {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-sonnet-4-5":      {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-sonnet-4":        {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-haiku-4-5":       {input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.1},
	"claude-3-7-sonnet":      {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-3-5-sonnet":      {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-3-5-haiku":       {input: 0.8, output: 4, cacheWrite: 1, cacheRead: 0.08},
	"gpt-6-astra":            {input: 10, output: 50, cacheWrite: 12.5, cacheRead: 1},
	"gpt-5.6-sol":            {input: 4, output: 20, cacheWrite: 5, cacheRead: 0.4},
	"gpt-5.6-terra":          {input: 2, output: 12, cacheWrite: 2.5, cacheRead: 0.2},
	"gpt-5.6-luna":           {input: 0.2, output: 1.2, cacheWrite: 0.25, cacheRead: 0.02},
	"gpt-5.5":                {input: 5, output: 30, cacheRead: 0.5},
	"gpt-5.4":                {input: 2.5, output: 15, cacheRead: 0.25},
	"gpt-5.3-codex":          {input: 1.75, output: 14, cacheRead: 0.175},
	"gpt-5.3-codex-spark":    {input: 0.25, output: 2, cacheRead: 0.025},
	"gpt-5.2":                {input: 1.75, output: 14, cacheRead: 0.175},
	"gpt-5":                  {input: 1.25, output: 10, cacheRead: 0.125},
	"gpt-4.1":                {input: 2, output: 8, cacheRead: 0.5},
	"gpt-4.1-mini":           {input: 0.4, output: 1.6, cacheRead: 0.1},
	"gemini-3.1-pro-preview": {input: 2, output: 12, cacheRead: 0.2},
	"gemini-3-pro-preview":   {input: 2, output: 12, cacheRead: 0.2},
	"gemini-3.8-flash":       {input: 0.75, output: 3.75, cacheRead: 0.075},
	"gemini-3.7-flash":       {input: 0.75, output: 3.75, cacheRead: 0.075},
	"gemini-3.6-flash":       {input: 0.75, output: 3.75, cacheRead: 0.075},
	"gemini-3-flash-preview": {input: 0.5, output: 3, cacheRead: 0.05},
	"gemini-2.5-pro":         {input: 1.25, output: 10, cacheRead: 0.125},
	"gemini-2.5-flash":       {input: 0.3, output: 2.5, cacheRead: 0.03},
}

func calculateCost(model string, tok Tokens, speed string) float64 {
	p, kind := lookupPrice(model)
	if kind == priceMatchNone {
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
// offer one. Anthropic fast mode and OpenAI fast mode (the tier formerly
// called priority processing, which Codex selects with service_tier) both
// raise the base input/output rate uniformly, and cache multipliers apply on
// top of that fast base, so a single multiplier covers the input, output, and
// cache token categories:
//
//	Opus 5:        $10/$50 fast vs $5/$25 standard      -> 2x
//	Opus 4.8:      $10/$50 fast vs $5/$25 standard      -> 2x
//	Opus 4.7:      $30/$150 fast vs $5/$25 standard     -> 6x
//	Opus 4.6:      $30/$150 fast vs $5/$25 standard     -> 6x
//	GPT-6 Astra:   $20/$100 fast vs $10/$50 standard    -> 2x
//	GPT-5.5:       $12.50/$75 fast vs $5/$30 standard   -> 2.5x
//	GPT-4.1(mini): $3.50/$14 fast vs $2/$8 standard     -> 1.75x
//
// GPT-5.3 Codex Spark has no published fast-mode rate and is omitted.
var fastMultipliers = map[string]float64{
	"claude-opus-5":   2,
	"claude-opus-4-8": 2,
	"claude-opus-4-7": 6,
	"claude-opus-4-6": 6,
	"gpt-6-astra":     2,
	"gpt-5.6-sol":     2,
	"gpt-5.6-terra":   2,
	"gpt-5.6-luna":    2,
	"gpt-5.5":         2.5,
	"gpt-5.4":         2,
	"gpt-5.3-codex":   2,
	"gpt-5.2":         2,
	"gpt-5":           2,
	"gpt-4.1":         1.75,
	"gpt-4.1-mini":    1.75,
}

// fastMultiplier returns the fast-mode price multiplier for a model, or 1 when
// the model does not offer fast-mode pricing. It resolves the model the same
// way lookupPrice does, so a dated ID like gpt-5-2025-08-07 is multiplied like
// the gpt-5 it is priced as. The family rung is excluded: a tier the table
// does not know is not assumed to carry its flagship's fast premium.
func fastMultiplier(model string) float64 {
	key, kind := resolvePriceKey(model)
	if kind == priceMatchNone || kind == priceMatchFamily {
		return 1
	}
	if m, ok := fastMultipliers[key]; ok {
		return m
	}
	return 1
}

// priceMatchKind records how a model ID resolved against builtinPrices, so
// callers can tell a quoted rate from an inferred one.
type priceMatchKind int

const (
	// priceMatchNone means no key and no family matched: cost is reported as 0.
	priceMatchNone priceMatchKind = iota
	// priceMatchExact means the normalized ID is a table key.
	priceMatchExact
	// priceMatchVariant means the ID contains a table key, as a dated or
	// suffixed variant does.
	priceMatchVariant
	// priceMatchFamily means no key matched and the price came from the
	// model's family default. The number is an estimate of the right order of
	// magnitude, not a quoted rate.
	priceMatchFamily
)

// familyDefaults prices a model whose exact ID is not in builtinPrices yet.
// Without it a newly shipped tier reports $0 and silently understates a whole
// month; claude-opus-5 did exactly that before it was added to the table. Each
// entry maps a family to that family's current flagship key, and every token
// listed must be present. Order is most specific first.
var familyDefaults = []struct {
	tokens []string
	key    string
}{
	{[]string{"fable"}, "claude-fable-5-1"},
	{[]string{"mythos"}, "claude-mythos-5-1"},
	{[]string{"opus"}, "claude-opus-5"},
	{[]string{"sonnet"}, "claude-sonnet-5"},
	{[]string{"haiku"}, "claude-haiku-4-5"},
	{[]string{"codex"}, "gpt-5.3-codex"},
	{[]string{"gpt-4"}, "gpt-4.1"},
	// No gpt-5 or gpt-6 entry: "gpt-5" is itself a table key and any gpt-6 ID
	// is rewritten by normalizeModel, so those generations are always claimed
	// by an earlier rung. This catches a later generation instead.
	{[]string{"gpt"}, "gpt-6-astra"},
	{[]string{"gemini", "flash"}, "gemini-3-flash-preview"},
	{[]string{"gemini"}, "gemini-3.1-pro-preview"},
}

// PriceMatch reports how a model ID was priced: "exact", "variant", "family",
// or "none". A "family" result is an estimate from familyDefaults, not a rate
// the provider published for that ID.
func PriceMatch(model string) string {
	switch _, kind := lookupPrice(model); kind {
	case priceMatchExact:
		return "exact"
	case priceMatchVariant:
		return "variant"
	case priceMatchFamily:
		return "family"
	default:
		return "none"
	}
}

func lookupPrice(model string) (price, priceMatchKind) {
	key, kind := resolvePriceKey(model)
	if kind == priceMatchNone {
		return price{}, kind
	}
	return builtinPrices[key], kind
}

// resolvePriceKey maps a logged model ID to a builtinPrices key and reports
// which rung answered.
func resolvePriceKey(model string) (string, priceMatchKind) {
	m := normalizeModel(model)
	if _, ok := builtinPrices[m]; ok {
		return m, priceMatchExact
	}
	// Longest match wins. Map iteration order is randomized, so returning the
	// first containment hit would price an ID that contains two keys (say
	// "gpt-4.1-mini-2025-04-14", holding both gpt-4.1 and gpt-4.1-mini)
	// differently from run to run.
	best := ""
	for k := range builtinPrices {
		if !strings.Contains(m, k) {
			continue
		}
		if len(k) > len(best) || (len(k) == len(best) && k < best) {
			best = k
		}
	}
	if best != "" {
		return best, priceMatchVariant
	}
	for _, f := range familyDefaults {
		if containsAll(m, f.tokens) {
			return f.key, priceMatchFamily
		}
	}
	return "", priceMatchNone
}

func containsAll(m string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(m, t) {
			return false
		}
	}
	return true
}

func normalizeModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "anthropic/")
	m = strings.TrimPrefix(m, "openai/")
	m = strings.TrimPrefix(m, "google/")
	m = strings.TrimPrefix(m, "google-gla/")
	m = strings.TrimPrefix(m, "vertex/")
	// Order matters: each case must precede any case whose pattern is a
	// substring of it, or the shorter pattern would claim the longer ID.
	switch {
	case strings.Contains(m, "gemini-3.1-pro"):
		return "gemini-3.1-pro-preview"
	case strings.Contains(m, "gemini-3-pro"):
		return "gemini-3-pro-preview"
	case strings.Contains(m, "gemini-3.8-flash"):
		return "gemini-3.8-flash"
	case strings.Contains(m, "gemini-3.7-flash"):
		return "gemini-3.7-flash"
	case strings.Contains(m, "gemini-3.6-flash"):
		return "gemini-3.6-flash"
	case strings.Contains(m, "gemini-3-flash"):
		return "gemini-3-flash-preview"
	case strings.Contains(m, "gpt-6-astra"), strings.Contains(m, "gpt-6"):
		// The bare gpt-6 alias routes to the Astra flagship tier.
		return "gpt-6-astra"
	case strings.Contains(m, "gpt-5.6-terra"):
		return "gpt-5.6-terra"
	case strings.Contains(m, "gpt-5.6-luna"):
		return "gpt-5.6-luna"
	case strings.Contains(m, "gpt-5.6"):
		// The bare gpt-5.6 alias routes to the Sol flagship tier.
		return "gpt-5.6-sol"
	case strings.Contains(m, "gpt-5.3-codex-spark"):
		return "gpt-5.3-codex-spark"
	case strings.Contains(m, "gpt-5.3-codex"):
		return "gpt-5.3-codex"
	case strings.Contains(m, "fable-5-1"):
		return "claude-fable-5-1"
	case strings.Contains(m, "mythos-5-1"):
		return "claude-mythos-5-1"
	case strings.Contains(m, "fable-5"):
		return "claude-fable-5"
	case strings.Contains(m, "mythos-5"):
		return "claude-mythos-5"
	case strings.Contains(m, "opus-5"):
		return "claude-opus-5"
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
	case strings.Contains(m, "sonnet-5"):
		return "claude-sonnet-5"
	case strings.Contains(m, "sonnet-4-6"):
		return "claude-sonnet-4-6"
	case strings.Contains(m, "sonnet-4-5"):
		return "claude-sonnet-4-5"
	case strings.Contains(m, "haiku-4-5"):
		return "claude-haiku-4-5"
	}
	return m
}
