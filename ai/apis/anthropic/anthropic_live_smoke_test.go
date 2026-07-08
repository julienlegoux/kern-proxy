package anthropic

// Ports: packages/ai/test/anthropic-opus-4-8-smoke.test.ts
//
// Env-gated live smoke test mirroring upstream's
// `describe.skipIf(!process.env.ANTHROPIC_API_KEY)`: it only runs (and only
// makes a real network call to the live Anthropic API) when ANTHROPIC_API_KEY
// is set in the environment. CI never sets that secret, so this test always
// skips there — go test ./... and CI do not depend on live network access.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestLiveSmoke_AnthropicOpus48StreamsWithReasoning(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping live Anthropic smoke test")
	}

	yes := true
	model := &ai.Model{
		ID:            "claude-opus-4-8",
		Name:          "Claude Opus 4.8",
		Api:           ai.ApiAnthropicMessages,
		Provider:      "anthropic",
		BaseURL:       "https://api.anthropic.com",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 200000,
		MaxTokens:     32000,
		Reasoning:     true,
		Compat:        &ai.Compat{ForceAdaptiveThinking: &yes},
	}
	chat := ai.Context{
		SystemPrompt: "You are a precise assistant. Follow the user's instructions exactly.",
		Messages: []ai.Message{ai.UserMessage{
			Content: ai.UserText("Compute 48291 * 7317 and 90844 - 17729, add the results, and determine " +
				"whether the sum is divisible by 11. Reply with exactly this format and nothing else: " +
				"sum=<sum>; divisibleBy11=<yes|no>"),
			Timestamp: time.Now().UnixMilli(),
		}},
	}

	var capturedPayload map[string]any
	maxTokens := 1024
	opts := &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{
			APIKey:    apiKey,
			MaxTokens: &maxTokens,
			OnPayload: func(_ context.Context, payload any, _ *ai.Model) (any, error) {
				if m, ok := payload.(map[string]any); ok {
					capturedPayload = m
				}
				return payload, nil
			},
		},
		Reasoning: ai.ThinkingHigh,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream := StreamSimple(ctx, model, chat, opts)

	sawThinking := false
	for ev := range stream.Events() {
		switch ev.EventKind() {
		case ai.EventThinkingStart, ai.EventThinkingDelta, ai.EventThinkingEnd:
			sawThinking = true
		}
	}

	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q (errorMessage=%q), want stop", result.StopReason, result.ErrorMessage)
	}
	if result.ErrorMessage != "" {
		t.Errorf("errorMessage = %q, want empty", result.ErrorMessage)
	}

	// Unlike the fixture-backed unit tests above, this test hits the live
	// API: assert on the specific keys the issue cares about rather than
	// exact whole-map equality, since the live payload's exact key set is
	// not something this port controls end-to-end (the SDK-shaped request
	// body may carry additional fields upstream's own test doesn't pin
	// either).
	if capturedPayload == nil {
		t.Fatal("OnPayload was never called")
	}
	thinking, _ := capturedPayload["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("payload.thinking.type = %v, want %q", thinking["type"], "adaptive")
	}
	outputConfig, _ := capturedPayload["output_config"].(map[string]any)
	if outputConfig["effort"] != "high" {
		t.Errorf("payload.output_config.effort = %v, want %q", outputConfig["effort"], "high")
	}
	if !sawThinking {
		t.Error("sawThinking = false, want true")
	}

	var thinkingBlock *ai.ThinkingContent
	for _, block := range result.Content {
		if tc, ok := block.(ai.ThinkingContent); ok {
			thinkingBlock = &tc
			break
		}
	}
	if thinkingBlock == nil {
		t.Fatal("expected a thinking block in the response content")
	}
	if thinkingBlock.ThinkingSignature == "" {
		t.Error("thinkingBlock.ThinkingSignature is empty, want a signature")
	}

	var text strings.Builder
	for _, block := range result.Content {
		if tc, ok := block.(ai.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	got := strings.TrimSpace(text.String())
	want := "sum=353418362; divisibleBy11=yes"
	if got != want {
		t.Errorf("final text = %q, want %q", got, want)
	}
}
