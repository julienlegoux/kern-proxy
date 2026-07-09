package ai

// Ports the cost math asserted by packages/ai/test/anthropic-cache-write-1h-cost.test.ts
// and the thinking-level helpers from packages/ai/src/models.ts.

import (
	"math"
	"testing"
)

// opus48Cost mirrors claude-opus-4-8 pricing: input 5, cacheWrite (5m) 6.25 $/Mtok.
var opus48 = &Model{
	ID:       "claude-opus-4-8",
	Provider: "anthropic",
	Api:      ApiAnthropicMessages,
	Cost:     ModelCost{Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
}

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-10 }

func TestCalculateCost1hCacheWrite(t *testing.T) {
	// 600k at the 5m rate + 400k at 2x input:
	// 600k * 6.25/M + 400k * 10/M = 3.75 + 4.0 = 7.75
	oneHour := 400_000
	usage := Usage{Input: 100, Output: 5, CacheWrite: 1_000_000, CacheWrite1h: &oneHour}
	cost := CalculateCost(opus48, &usage)
	if !almostEqual(cost.CacheWrite, 7.75) {
		t.Errorf("cacheWrite cost = %v, want 7.75", cost.CacheWrite)
	}
}

func TestCalculateCost5mFallback(t *testing.T) {
	// No breakdown: all 1M tokens at the 5m rate = 6.25
	usage := Usage{Input: 100, Output: 5, CacheWrite: 1_000_000}
	cost := CalculateCost(opus48, &usage)
	if !almostEqual(cost.CacheWrite, 6.25) {
		t.Errorf("cacheWrite cost = %v, want 6.25", cost.CacheWrite)
	}
}

func TestCalculateCostTotals(t *testing.T) {
	usage := Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 0}
	cost := CalculateCost(opus48, &usage)
	if !almostEqual(cost.Input, 5) || !almostEqual(cost.Output, 25) || !almostEqual(cost.CacheRead, 0.5) {
		t.Errorf("unexpected component costs: %+v", cost)
	}
	if !almostEqual(cost.Total, 30.5) {
		t.Errorf("total = %v, want 30.5", cost.Total)
	}
	if usage.Cost != cost {
		t.Error("CalculateCost must write the cost back onto the usage")
	}
}

func strPtr(s string) *string { return &s }

func TestGetSupportedThinkingLevels(t *testing.T) {
	nonReasoning := &Model{Reasoning: false}
	got := GetSupportedThinkingLevels(nonReasoning)
	if len(got) != 1 || got[0] != ThinkingOff {
		t.Errorf("non-reasoning model levels = %v, want [off]", got)
	}

	// Reasoning model without a map: everything except xhigh.
	plain := &Model{Reasoning: true}
	got = GetSupportedThinkingLevels(plain)
	want := []ModelThinkingLevel{ThinkingOff, ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh}
	if len(got) != len(want) {
		t.Fatalf("levels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("levels = %v, want %v", got, want)
		}
	}

	// xhigh requires an explicit map entry; null entries disable a level.
	mapped := &Model{Reasoning: true, ThinkingLevelMap: ThinkingLevelMap{
		ThinkingXHigh:   strPtr("xhigh-value"),
		ThinkingMinimal: nil, // unsupported
	}}
	got = GetSupportedThinkingLevels(mapped)
	hasXHigh, hasMinimal := false, false
	for _, l := range got {
		if l == ThinkingXHigh {
			hasXHigh = true
		}
		if l == ThinkingMinimal {
			hasMinimal = true
		}
	}
	if !hasXHigh {
		t.Error("expected xhigh with explicit map entry")
	}
	if hasMinimal {
		t.Error("nil map entry must disable the level")
	}
}

func TestClampThinkingLevel(t *testing.T) {
	model := &Model{Reasoning: true, ThinkingLevelMap: ThinkingLevelMap{
		ThinkingMinimal: nil,
		ThinkingLow:     nil,
	}}
	// minimal unsupported -> clamps upward to medium.
	if got := ClampThinkingLevel(model, ThinkingMinimal); got != ThinkingMedium {
		t.Errorf("clamp(minimal) = %v, want medium", got)
	}
	// xhigh unsupported (no map entry) -> clamps downward to high.
	if got := ClampThinkingLevel(model, ThinkingXHigh); got != ThinkingHigh {
		t.Errorf("clamp(xhigh) = %v, want high", got)
	}
	// exact match passes through.
	if got := ClampThinkingLevel(model, ThinkingHigh); got != ThinkingHigh {
		t.Errorf("clamp(high) = %v, want high", got)
	}
	// unknown level -> first available.
	if got := ClampThinkingLevel(model, "bogus"); got != ThinkingOff {
		t.Errorf("clamp(bogus) = %v, want off", got)
	}
	// non-reasoning model always off.
	if got := ClampThinkingLevel(&Model{}, ThinkingHigh); got != ThinkingOff {
		t.Errorf("clamp on non-reasoning = %v, want off", got)
	}
}

func TestModelsAreEqual(t *testing.T) {
	a := &Model{ID: "m", Provider: "p"}
	b := &Model{ID: "m", Provider: "p"}
	c := &Model{ID: "m", Provider: "q"}
	if !ModelsAreEqual(a, b) {
		t.Error("same id+provider must be equal")
	}
	if ModelsAreEqual(a, c) {
		t.Error("different provider must not be equal")
	}
	if ModelsAreEqual(a, nil) || ModelsAreEqual(nil, b) {
		t.Error("nil is never equal")
	}
}
