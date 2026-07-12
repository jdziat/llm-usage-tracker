package core

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
		// Sol: $5 input + $30 output + $5 cache write (defaults to input) + $0.50 cache read.
		{"gpt-5.6-sol", 40.5},
		{"gpt-5.6", 40.5},                // bare alias routes to Sol.
		{"gpt-5.6-sol-2026-07-09", 40.5}, // dated variant normalizes to Sol.
		// Terra: $2.50 input + $15 output + $2.50 cache write + $0.25 cache read.
		{"gpt-5.6-terra", 20.25},
		// Luna: $1 input + $6 output + $1 cache write + $0.10 cache read.
		{"gpt-5.6-luna", 8.1},
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
