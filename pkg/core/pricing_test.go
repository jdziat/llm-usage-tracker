package core

import (
	"strings"
	"testing"
)

func TestHaiku45PricingAlias(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	got := calculateCost("claude-haiku-4-5-20251001", tok, "standard")
	want := 7.35 // $1 input + $5 output + $1.25 cache write + $0.10 cache read.
	if got != want {
		t.Fatalf("cost = %.2f, want %.2f", got, want)
	}
}

func TestOpus47DoesNotUseDeprecatedOpus4Pricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	got := calculateCost("claude-opus-4-7", tok, "standard")
	want := 36.75 // $5 input + $25 output + $6.25 cache write + $0.50 cache read.
	if got != want {
		t.Fatalf("cost = %.2f, want %.2f", got, want)
	}
}

func TestOpus48DoesNotUseDeprecatedOpus4Pricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	// Dated variant must normalize to claude-opus-4-8, not fall through to the
	// deprecated claude-opus-4 substring match (which would triple the cost).
	got := calculateCost("claude-opus-4-8-20260514", tok, "standard")
	want := 36.75 // $5 input + $25 output + $6.25 cache write + $0.50 cache read.
	if got != want {
		t.Fatalf("cost = %.2f, want %.2f", got, want)
	}
}

func TestFable5AndMythos5Pricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	want := 73.5 // $10 input + $50 output + $12.50 cache write + $1 cache read.
	cases := []string{
		"claude-fable-5",
		"claude-fable-5[1m]", // long-context variant must normalize to the base ID.
		"claude-mythos-5",
	}
	for _, model := range cases {
		if got := calculateCost(model, tok, "standard"); got != want {
			t.Errorf("%s cost = %.2f, want %.2f", model, got, want)
		}
	}
}

func TestGPT56FamilyPricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model string
		want  float64
	}{
		// Sol: $4 input + $20 output + $5 cache write + $0.40 cache read.
		{"gpt-5.6-sol", 29.4},
		{"gpt-5.6", 29.4},                // bare alias routes to Sol.
		{"gpt-5.6-sol-2026-07-09", 29.4}, // dated variant normalizes to Sol.
		// Terra: $2 input + $12 output + $2.50 cache write + $0.20 cache read.
		{"gpt-5.6-terra", 16.7},
		// Luna: $0.20 input + $1.20 output + $0.25 cache write + $0.02 cache read.
		{"gpt-5.6-luna", 1.67},
	}
	for _, c := range cases {
		if got := calculateCost(c.model, tok, "standard"); got != c.want {
			t.Errorf("%s cost = %.2f, want %.2f", c.model, got, c.want)
		}
	}
}

func TestGPT6AstraPricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	want := 73.5 // $10 input + $50 output + $12.50 cache write + $1 cache read.
	cases := []string{
		"gpt-6-astra",
		"gpt-6-astra-2026-09-03", // dated variant normalizes to the base ID.
		"gpt-6",                  // bare alias routes to Astra.
		"openai/gpt-6-astra",
	}
	for _, model := range cases {
		if got := calculateCost(model, tok, "standard"); got != want {
			t.Errorf("%s cost = %.2f, want %.2f", model, got, want)
		}
	}
}

func TestFable51AndMythos51Pricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	// Cache reads on the 5.1 tier are 0.025x base input, not 0.1x, so these
	// must not fall through to the claude-fable-5 substring match.
	want := 72.75 // $10 input + $50 output + $12.50 cache write + $0.25 cache read.
	cases := []string{
		"claude-fable-5-1",
		"claude-fable-5-1[1m]",
		"claude-mythos-5-1",
	}
	for _, model := range cases {
		if got := calculateCost(model, tok, "standard"); got != want {
			t.Errorf("%s cost = %.2f, want %.2f", model, got, want)
		}
	}
}

func TestOpus5AndSonnet5Pricing(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model string
		want  float64
	}{
		// Opus 5: $5 input + $25 output + $6.25 cache write + $0.50 cache read.
		{"claude-opus-5", 36.75},
		{"claude-opus-5[1m]", 36.75},
		// Must not fall through to the retired claude-opus-4 pricing.
		{"anthropic/claude-opus-5", 36.75},
		// Sonnet 5: $2 input + $10 output + $2.50 cache write + $0.20 cache read.
		{"claude-sonnet-5", 14.7},
		{"claude-sonnet-5[1m]", 14.7},
	}
	for _, c := range cases {
		if got := calculateCost(c.model, tok, "standard"); got != c.want {
			t.Errorf("%s cost = %.2f, want %.2f", c.model, got, c.want)
		}
	}
}

func TestFastModeIsPerModel(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model string
		want  float64
	}{
		{"claude-opus-5", 73.5},            // 2x: $10/$50 fast base.
		{"claude-opus-4-8-20260514", 73.5}, // 2x: $10/$50 fast base.
		{"claude-opus-4-7", 220.5},         // 6x: $30/$150 fast base.
		{"claude-opus-4-6", 220.5},         // 6x: $30/$150 fast base.
		{"gpt-6-astra", 147},               // 2x: $20/$100 fast base.
		{"gpt-5.3-codex", 35.35},           // 2x: $3.50/$28 fast base.
		{"gpt-5", 25.25},                   // 2x: $2.50/$20 fast base.
		{"gpt-4.1", 21.875},                // 1.75x: $3.50/$14 fast base.
		{"claude-sonnet-5", 14.7},          // no fast mode: 1x (unchanged from standard).
	}
	for _, c := range cases {
		if got := calculateCost(c.model, tok, "fast"); got != c.want {
			t.Errorf("%s fast cost = %.3f, want %.3f", c.model, got, c.want)
		}
	}
}

func TestLookupPrefersLongestKey(t *testing.T) {
	// These IDs have no normalizeModel case, so they reach the containment
	// loop, and each contains two table keys. Map iteration is randomized, so
	// taking the first hit would price them differently from run to run.
	cases := []struct {
		model string
		want  string
	}{
		{"gpt-4.1-mini-2025-04-14", "gpt-4.1-mini"},   // also contains gpt-4.1.
		{"gpt-4.1-mini-preview", "gpt-4.1-mini"},      // also contains gpt-4.1.
		{"gemini-2.5-pro-exp-0827", "gemini-2.5-pro"}, // no alias case for -exp.
	}
	for _, c := range cases {
		want := builtinPrices[c.want]
		for i := 0; i < 50; i++ {
			got, kind := lookupPrice(c.model)
			if kind != priceMatchVariant {
				t.Fatalf("%s matched at rung %v, want variant (the containment loop)", c.model, kind)
			}
			if got != want {
				t.Fatalf("%s resolved to %+v, want %s (%+v)", c.model, got, c.want, want)
			}
		}
	}
}

func TestNormalizeClaimsAmbiguousIDsBeforeLookup(t *testing.T) {
	// The IDs that would otherwise be ambiguous at the containment rung are
	// pinned by a normalizeModel case instead, so they resolve exactly.
	cases := []struct{ model, want string }{
		{"claude-fable-5-1-preview", "claude-fable-5-1"},          // not claude-fable-5.
		{"gpt-5.3-codex-spark-2026-01-01", "gpt-5.3-codex-spark"}, // not gpt-5.3-codex.
		{"gpt-5.6-terra-preview", "gpt-5.6-terra"},                // not gpt-5.6-sol.
	}
	for _, c := range cases {
		if got := normalizeModel(c.model); got != c.want {
			t.Errorf("normalizeModel(%q) = %s, want %s", c.model, got, c.want)
		}
		if _, kind := lookupPrice(c.model); kind != priceMatchExact {
			t.Errorf("%s matched at rung %v, want exact", c.model, kind)
		}
	}
}

func TestFamilyFallbackKeepsUnknownTiersPriced(t *testing.T) {
	// A tier that ships before the table is updated must not report $0, which
	// is how claude-opus-5 silently halved reported spend before it was added.
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model  string
		family string // the family default it should be priced from
	}{
		{"claude-opus-6", "claude-opus-5"},
		{"claude-sonnet-6-2", "claude-sonnet-5"},
		{"claude-fable-6", "claude-fable-5-1"},
		{"claude-haiku-6", "claude-haiku-4-5"},
		{"gemini-4-flash", "gemini-3-flash-preview"},
		{"gemini-4-pro", "gemini-3.1-pro-preview"},
	}
	for _, c := range cases {
		if got, want := calculateCost(c.model, tok, "standard"), calculateCost(c.family, tok, "standard"); got != want {
			t.Errorf("%s cost = %.2f, want %.2f (from %s)", c.model, got, want, c.family)
		}
		if got := PriceMatch(c.model); got != "family" {
			t.Errorf("PriceMatch(%q) = %s, want family", c.model, got)
		}
	}
	// Truly unrecognizable models stay unpriced rather than being guessed at.
	for _, m := range []string{"llama-4-70b", "mistral-large", ""} {
		if got := calculateCost(m, tok, "standard"); got != 0 {
			t.Errorf("%s priced at %.2f, want 0", m, got)
		}
		if got := PriceMatch(m); got != "none" {
			t.Errorf("PriceMatch(%q) = %s, want none", m, got)
		}
	}
}

func TestPriceMatchKinds(t *testing.T) {
	cases := []struct{ model, want string }{
		{"claude-opus-5", "exact"},
		{"claude-opus-5[1m]", "exact"},    // normalizeModel collapses it.
		{"gpt-4.1-2025-04-14", "variant"}, // dated, resolved by containment.
		{"claude-opus-6", "family"},       // unknown tier, family default.
		{"llama-4-70b", "none"},
	}
	for _, c := range cases {
		if got := PriceMatch(c.model); got != c.want {
			t.Errorf("PriceMatch(%q) = %s, want %s", c.model, got, c.want)
		}
	}
}

func TestEveryFamilyDefaultIsReachable(t *testing.T) {
	// A family entry claimed by an earlier rung is dead weight: a maintainer
	// repointing it when a flagship is renamed would change nothing.
	for _, f := range familyDefaults {
		probe := "zz-" + strings.Join(f.tokens, "-") + "-zz"
		got, kind := lookupPrice(probe)
		if kind != priceMatchFamily {
			t.Errorf("familyDefaults %v unreachable: %q matched at rung %v", f.tokens, probe, kind)
			continue
		}
		if want := builtinPrices[f.key]; got != want {
			t.Errorf("familyDefaults %v priced %+v, want %s (%+v)", f.tokens, got, f.key, want)
		}
	}
}

func TestEveryFamilyDefaultKeyExists(t *testing.T) {
	for _, f := range familyDefaults {
		if _, ok := builtinPrices[f.key]; !ok {
			t.Errorf("familyDefaults %v points at %q, which is not in builtinPrices", f.tokens, f.key)
		}
	}
}

func TestFastMultiplierFollowsPriceResolution(t *testing.T) {
	cases := []struct {
		model string
		want  float64
	}{
		{"gpt-5", 2},                   // exact.
		{"gpt-5-2025-08-07", 2},        // dated variant, priced as gpt-5.
		{"gpt-4.1-mini-preview", 1.75}, // longest-key variant.
		{"claude-opus-5[1m]", 2},       // alias case.
		{"gpt-5.3-codex-2026-01-01", 2},
		{"claude-opus-6", 1},   // family rung: no inferred premium.
		{"claude-sonnet-5", 1}, // priced, but no fast tier.
		{"llama-4-70b", 1},     // unpriced.
	}
	for _, c := range cases {
		if got := fastMultiplier(c.model); got != c.want {
			t.Errorf("fastMultiplier(%q) = %g, want %g", c.model, got, c.want)
		}
	}
}
