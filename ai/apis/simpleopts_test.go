package apis

// Ports the behavior of packages/ai/src/api/simple-options.ts.

import (
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

func intPtr(v int) *int { return &v }

func TestClampMaxTokensToContext(t *testing.T) {
	model := &ai.Model{ContextWindow: 10000, MaxTokens: 8000}
	// ~25 tokens of context (100 chars) -> available = 10000 - 25 - 4096.
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText(makeText(100))},
	}}
	got := ClampMaxTokensToContext(model, chat, 8000)
	want := 10000 - 25 - 4096
	if got != want {
		t.Errorf("clamped = %d, want %d", got, want)
	}

	// Small request passes through.
	if got := ClampMaxTokensToContext(model, chat, 1000); got != 1000 {
		t.Errorf("clamped = %d, want 1000", got)
	}

	// No context window: only the floor applies.
	if got := ClampMaxTokensToContext(&ai.Model{}, chat, -5); got != 1 {
		t.Errorf("clamped = %d, want 1", got)
	}

	// Overfull context still returns at least the floor.
	huge := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText(makeText(100000))}}}
	if got := ClampMaxTokensToContext(model, huge, 8000); got != 1 {
		t.Errorf("clamped = %d, want 1", got)
	}
}

func TestBuildBaseOptions(t *testing.T) {
	model := &ai.Model{ContextWindow: 200000, MaxTokens: 4096}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	opts := &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "from-options", SessionID: "s"},
		Reasoning:     ai.ThinkingHigh,
	}
	base := BuildBaseOptions(model, chat, opts, "")
	if base.MaxTokens == nil || *base.MaxTokens != 4096 {
		t.Errorf("maxTokens = %v", base.MaxTokens)
	}
	if base.APIKey != "from-options" || base.SessionID != "s" {
		t.Errorf("base = %+v", base)
	}
	// Explicit apiKey argument wins.
	base = BuildBaseOptions(model, chat, opts, "resolved")
	if base.APIKey != "resolved" {
		t.Errorf("apiKey = %q", base.APIKey)
	}
}

func TestClampReasoning(t *testing.T) {
	if got := ClampReasoning(ai.ThinkingXHigh); got != ai.ThinkingHigh {
		t.Errorf("xhigh -> %v", got)
	}
	if got := ClampReasoning(ai.ThinkingLow); got != ai.ThinkingLow {
		t.Errorf("low -> %v", got)
	}
}

func TestAdjustMaxTokensForThinking(t *testing.T) {
	// No caller cap: model cap with the default budget inside it.
	maxTokens, budget := AdjustMaxTokensForThinking(nil, 32000, ai.ThinkingMedium, nil)
	if maxTokens != 32000 || budget != 8192 {
		t.Errorf("got %d/%d, want 32000/8192", maxTokens, budget)
	}

	// Caller cap: budget added on top, clamped to the model cap.
	maxTokens, budget = AdjustMaxTokensForThinking(intPtr(4096), 32000, ai.ThinkingHigh, nil)
	if maxTokens != 4096+16384 || budget != 16384 {
		t.Errorf("got %d/%d", maxTokens, budget)
	}

	// xhigh clamps to the high budget.
	_, budget = AdjustMaxTokensForThinking(nil, 64000, ai.ThinkingXHigh, nil)
	if budget != 16384 {
		t.Errorf("budget = %d, want high budget", budget)
	}

	// Budget shrinks to keep minimum output room.
	maxTokens, budget = AdjustMaxTokensForThinking(nil, 8192, ai.ThinkingHigh, nil)
	if maxTokens != 8192 || budget != 8192-1024 {
		t.Errorf("got %d/%d", maxTokens, budget)
	}

	// Custom budgets override the defaults.
	custom := &ai.ThinkingBudgets{Medium: intPtr(500)}
	_, budget = AdjustMaxTokensForThinking(nil, 32000, ai.ThinkingMedium, custom)
	if budget != 500 {
		t.Errorf("budget = %d, want 500", budget)
	}
}

func makeText(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}
