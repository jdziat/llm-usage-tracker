package usage

import "testing"

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

func TestFastModeIsPerModel(t *testing.T) {
	tok := Tokens{Input: 1_000_000, Output: 1_000_000, CacheCreation: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model string
		want  float64
	}{
		{"claude-opus-4-8-20260514", 73.5}, // 2x: $10/$50 fast base.
		{"claude-opus-4-7", 220.5},         // 6x: $30/$150 fast base.
		{"claude-opus-4-6", 220.5},         // 6x: $30/$150 fast base.
		{"gpt-5", 12.625},                  // no fast mode: 1x (unchanged from standard).
	}
	for _, c := range cases {
		if got := calculateCost(c.model, tok, "fast"); got != c.want {
			t.Errorf("%s fast cost = %.3f, want %.3f", c.model, got, c.want)
		}
	}
}
