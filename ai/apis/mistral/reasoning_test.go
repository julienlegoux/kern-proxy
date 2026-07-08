package mistral

// Ports test/mistral-reasoning-mode.test.ts: verifies StreamSimple's
// reasoning-mode selection per model family by capturing the built payload
// via OnPayload, exactly like the TS test's capturePayload helper.

import (
	"context"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func reasoningModel(id string, reasoning bool, thinkingLevelMap ai.ThinkingLevelMap) *ai.Model {
	return &ai.Model{
		ID:               id,
		Name:             id,
		Api:              ai.ApiMistralConversations,
		Provider:         "mistral",
		BaseURL:          "http://127.0.0.1:9",
		Reasoning:        reasoning,
		ThinkingLevelMap: thinkingLevelMap,
		Input:            []ai.Modality{ai.ModalityText},
		ContextWindow:    128000,
		MaxTokens:        4096,
	}
}

func capturePayload(t *testing.T, model *ai.Model, opts *ai.SimpleStreamOptions) *wireRequest {
	t.Helper()
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}

	var captured *wireRequest
	base := ai.SimpleStreamOptions{}
	if opts != nil {
		base = *opts
	}
	base.APIKey = "fake-key"
	base.OnPayload = func(ctx context.Context, payload any, model *ai.Model) (any, error) {
		captured = payload.(*wireRequest)
		return payload, nil
	}

	stream := StreamSimple(context.Background(), model, chat, &base)
	stream.Result(context.Background())

	if captured == nil {
		t.Fatal("expected payload to be captured before request failure")
	}
	return captured
}

func TestReasoningMode_UsesReasoningEffortForMistralSmall4(t *testing.T) {
	model := reasoningModel("mistral-small-2603", true, nil)
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{Reasoning: ai.ThinkingMedium})
	if payload.ReasoningEffort != "high" {
		t.Errorf("reasoningEffort = %q, want high", payload.ReasoningEffort)
	}
	if payload.PromptMode != "" {
		t.Errorf("promptMode = %q, want empty", payload.PromptMode)
	}
}

func TestReasoningMode_OmitsControlsForMistralSmall4WhenThinkingOff(t *testing.T) {
	model := reasoningModel("mistral-small-2603", true, nil)
	payload := capturePayload(t, model, nil)
	if payload.ReasoningEffort != "" {
		t.Errorf("reasoningEffort = %q, want empty", payload.ReasoningEffort)
	}
	if payload.PromptMode != "" {
		t.Errorf("promptMode = %q, want empty", payload.PromptMode)
	}
}

func TestReasoningMode_UsesPromptModeForMagistral(t *testing.T) {
	model := reasoningModel("magistral-medium-latest", true, nil)
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{Reasoning: ai.ThinkingMedium})
	if payload.PromptMode != "reasoning" {
		t.Errorf("promptMode = %q, want reasoning", payload.PromptMode)
	}
	if payload.ReasoningEffort != "" {
		t.Errorf("reasoningEffort = %q, want empty", payload.ReasoningEffort)
	}
}

func TestReasoningMode_UsesReasoningEffortForMistralMedium35(t *testing.T) {
	model := reasoningModel("mistral-medium-3.5", true, nil)
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{Reasoning: ai.ThinkingMedium})
	if payload.ReasoningEffort != "high" {
		t.Errorf("reasoningEffort = %q, want high", payload.ReasoningEffort)
	}
	if payload.PromptMode != "" {
		t.Errorf("promptMode = %q, want empty", payload.PromptMode)
	}
}

func TestReasoningMode_OmitsControlsForMistralMedium35WhenThinkingOff(t *testing.T) {
	model := reasoningModel("mistral-medium-3.5", true, nil)
	payload := capturePayload(t, model, nil)
	if payload.ReasoningEffort != "" {
		t.Errorf("reasoningEffort = %q, want empty", payload.ReasoningEffort)
	}
	if payload.PromptMode != "" {
		t.Errorf("promptMode = %q, want empty", payload.PromptMode)
	}
}

func TestReasoningMode_UsesSessionIDAsPromptCacheKey(t *testing.T) {
	model := reasoningModel("mistral-large-latest", false, nil)
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{StreamOptions: ai.StreamOptions{SessionID: "session-123"}})
	if payload.PromptCacheKey != "session-123" {
		t.Errorf("promptCacheKey = %q, want session-123", payload.PromptCacheKey)
	}
}

func TestReasoningMode_OmitsPromptCacheKeyWhenCacheRetentionDisabled(t *testing.T) {
	model := reasoningModel("mistral-large-latest", false, nil)
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{SessionID: "session-123", CacheRetention: ai.CacheRetentionNone},
	})
	if payload.PromptCacheKey != "" {
		t.Errorf("promptCacheKey = %q, want empty", payload.PromptCacheKey)
	}
}

// TestReasoningMode_ThinkingLevelMapOverridesDefault covers mapReasoningEffort's
// model.ThinkingLevelMap override path (default is "high" otherwise).
func TestReasoningMode_ThinkingLevelMapOverridesDefault(t *testing.T) {
	none := "none"
	model := reasoningModel("mistral-small-2603", true, ai.ThinkingLevelMap{ai.ThinkingMedium: &none})
	payload := capturePayload(t, model, &ai.SimpleStreamOptions{Reasoning: ai.ThinkingMedium})
	if payload.ReasoningEffort != "none" {
		t.Errorf("reasoningEffort = %q, want none (from ThinkingLevelMap override)", payload.ReasoningEffort)
	}
}
