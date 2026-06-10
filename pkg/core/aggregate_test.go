package core

import (
	"fmt"
	"testing"
	"time"
)

func TestAggregateRanksModelsByCostThenTokens(t *testing.T) {
	when := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: SourceClaude, SessionID: "s", Model: "claude-haiku-4-5-20251001", Time: when, Tokens: Tokens{Input: 260, Output: 12053, CacheCreation: 220333, CacheRead: 2699365}},
		{Source: SourceClaude, SessionID: "s", Model: "claude-opus-4-7", Time: when, Tokens: Tokens{Input: 4154, Output: 1543155, CacheCreation: 7250092, CacheRead: 1293732557}},
	}
	for i := range events {
		events[i].Tokens.CostUSD = calculateCost(events[i].Model, events[i].Tokens, "standard")
	}

	rows := aggregate(events, Query{View: "daily", Sources: []string{SourceClaude}, Location: time.UTC, Order: "asc"})
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if got := rows[0].ModelsUsed[0]; got != "claude-opus-4-7" {
		t.Fatalf("first model = %q, want costliest model", got)
	}
	if rows[0].ModelBreakdowns[0].Model != "claude-opus-4-7" {
		t.Fatalf("first breakdown = %q", rows[0].ModelBreakdowns[0].Model)
	}
	if rows[0].ModelBreakdowns[1].CostUSD <= 0 {
		t.Fatalf("haiku 4.5 cost was not calculated: %+v", rows[0].ModelBreakdowns[1])
	}
}

func TestAggregateSummaryReturnsTopModelsByCost(t *testing.T) {
	when := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: SourceClaude, SessionID: "s1", Project: "/work/cheap", Model: "claude-haiku-4-5-20251001", Time: when, Tokens: Tokens{Input: 100, CostUSD: 1}},
		{Source: SourceClaude, SessionID: "s2", Project: "/work/expensive", Model: "claude-opus-4-7", Time: when, Tokens: Tokens{Input: 100, CostUSD: 10}},
		{Source: SourceCodex, SessionID: "s3", Project: "/work/mid", Model: "gpt-5.5", Time: when, Tokens: Tokens{Input: 100, CostUSD: 5}},
	}

	rows := aggregate(events, Query{View: "summary", Sources: []string{SourceClaude, SourceCodex}, Location: time.UTC, Order: "desc", Top: 2})
	if len(rows) != 2 {
		t.Fatalf("expected 2 summary rows, got %d", len(rows))
	}
	if rows[0].Key != "claude-opus-4-7" || rows[1].Key != "gpt-5.5" {
		t.Fatalf("unexpected summary order: %+v", rows)
	}
}

func TestAggregateSummaryMergesProjectsPerModel(t *testing.T) {
	when := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: SourceClaude, SessionID: "s1", Project: "/work/a", Model: "claude-opus-4-8", Time: when, Tokens: Tokens{Input: 100, CostUSD: 2}},
		{Source: SourceClaude, SessionID: "s2", Project: "/work/b", Model: "claude-opus-4-8", Time: when.Add(time.Hour), Tokens: Tokens{Input: 50, CostUSD: 1}},
	}

	rows := aggregate(events, Query{View: "summary", Sources: []string{SourceClaude}, Location: time.UTC, Order: "desc"})
	if len(rows) != 1 {
		t.Fatalf("expected one row per model, got %d", len(rows))
	}
	if rows[0].Key != "claude-opus-4-8" || rows[0].Input != 150 || rows[0].CostUSD != 3 {
		t.Fatalf("unexpected merged row: %+v", rows[0])
	}
}

func TestAggregateSummaryDefaultsToTopTen(t *testing.T) {
	when := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	var events []Event
	for i := 0; i < 12; i++ {
		events = append(events, Event{
			Source:    SourceClaude,
			SessionID: "s",
			Project:   "/work/repo",
			Model:     fmt.Sprintf("model-%02d", i),
			Time:      when,
			Tokens:    Tokens{Input: 100, CostUSD: float64(i)},
		})
	}

	rows := aggregate(events, Query{View: "summary", Sources: []string{SourceClaude}, Location: time.UTC, Order: "desc"})
	if len(rows) != 10 {
		t.Fatalf("expected default top 10, got %d", len(rows))
	}
	if rows[0].Key != "model-11" {
		t.Fatalf("first row = %q, want model-11", rows[0].Key)
	}
}

func TestAggregateSummaryByProject(t *testing.T) {
	when := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: SourceClaude, SessionID: "s1", Project: "/work/alpha", Model: "claude-opus-4-8", Time: when, Tokens: Tokens{Input: 100, CostUSD: 2}},
		{Source: SourceCodex, SessionID: "s2", Project: "/tmp/alpha", Model: "gpt-5.5", Time: when.Add(time.Hour), Tokens: Tokens{Input: 50, CostUSD: 1}},
		{Source: SourceClaude, SessionID: "s3", Project: "/work/beta", Model: "claude-haiku-4-5-20251001", Time: when, Tokens: Tokens{Input: 20, CostUSD: 5}},
	}

	rows := aggregate(events, Query{View: "summary", By: "project", Sources: []string{SourceClaude, SourceCodex}, Location: time.UTC, Order: "desc"})
	if len(rows) != 2 {
		t.Fatalf("expected 2 project rows, got %d", len(rows))
	}
	if rows[0].Key != "beta" || rows[1].Key != "alpha" {
		t.Fatalf("unexpected project order: %+v", rows)
	}
	if rows[1].Input != 150 || rows[1].CostUSD != 3 {
		t.Fatalf("project alpha was not merged: %+v", rows[1])
	}
	if got := rows[1].ModelsUsed; len(got) != 2 || got[0] != "claude-opus-4-8" || got[1] != "gpt-5.5" {
		t.Fatalf("models = %v, want cost-ranked project models", got)
	}
}

func TestAggregateSummaryBySource(t *testing.T) {
	when := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: SourceClaude, SessionID: "s1", Project: "/work/a", Model: "claude-opus-4-8", Time: when, Tokens: Tokens{Input: 100, CostUSD: 2}},
		{Source: SourceCodex, SessionID: "s2", Project: "/work/b", Model: "gpt-5.5", Time: when, Tokens: Tokens{Input: 50, CostUSD: 5}},
		{Source: SourceClaude, SessionID: "s3", Project: "/work/c", Model: "claude-haiku-4-5-20251001", Time: when, Tokens: Tokens{Input: 20, CostUSD: 1}},
	}

	rows := aggregate(events, Query{View: "summary", By: "source", Sources: []string{SourceClaude, SourceCodex}, Location: time.UTC, Order: "desc"})
	if len(rows) != 2 {
		t.Fatalf("expected 2 source rows, got %d", len(rows))
	}
	if rows[0].Key != "codex" || rows[1].Key != "claude" {
		t.Fatalf("unexpected source order: %+v", rows)
	}
	if rows[1].Input != 120 || rows[1].CostUSD != 3 {
		t.Fatalf("source claude was not merged: %+v", rows[1])
	}
}
