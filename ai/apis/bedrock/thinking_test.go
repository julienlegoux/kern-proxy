package bedrock

// Ports test/bedrock-thinking-payload.test.ts's additionalModelRequestFields
// assertions directly against buildAdditionalModelRequestFields (the E2E
// "Claude max tokens" suite is env-gated and out of scope for unit tests).

import (
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

func opus48() *ai.Model {
	return &ai.Model{
		ID:        "global.anthropic.claude-opus-4-8-v1",
		Name:      "Claude Opus 4.8 (Global)",
		Reasoning: true,
	}
}

func TestBuildAdditionalModelRequestFields_AdaptiveThinkingForOpus48(t *testing.T) {
	got := buildAdditionalModelRequestFields(opus48(), ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" || thinking["display"] != "summarized" {
		t.Errorf("thinking = %+v", thinking)
	}
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "high" {
		t.Errorf("output_config = %+v", outputConfig)
	}
	if _, ok := got["anthropic_beta"]; ok {
		t.Errorf("anthropic_beta should be absent, got %+v", got["anthropic_beta"])
	}
}

func TestBuildAdditionalModelRequestFields_XHighMapsToEffortXHighForOpus48(t *testing.T) {
	got := buildAdditionalModelRequestFields(opus48(), ai.ThinkingXHigh, nil, nil, "", "", nil)
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "xhigh" {
		t.Errorf("output_config.effort = %v, want xhigh", outputConfig["effort"])
	}
}

func TestBuildAdditionalModelRequestFields_AdaptiveThinkingForFable5(t *testing.T) {
	model := &ai.Model{ID: "global.anthropic.claude-fable-5", Reasoning: true}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking = %+v", thinking)
	}
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "high" {
		t.Errorf("output_config = %+v", outputConfig)
	}
}

func TestBuildAdditionalModelRequestFields_XHighMapsToEffortXHighForFable5(t *testing.T) {
	model := &ai.Model{ID: "global.anthropic.claude-fable-5", Reasoning: true}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingXHigh, nil, nil, "", "", nil)
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "xhigh" {
		t.Errorf("output_config.effort = %v, want xhigh", outputConfig["effort"])
	}
}

func TestBuildAdditionalModelRequestFields_AdaptiveThinkingForSonnet5(t *testing.T) {
	model := &ai.Model{ID: "global.anthropic.claude-sonnet-5", Reasoning: true}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking = %+v", thinking)
	}
}

func TestBuildAdditionalModelRequestFields_OmitsDisplayForGovCloudModelID(t *testing.T) {
	model := &ai.Model{
		ID:        "us-gov.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Name:      "Claude Sonnet 4.5 (GovCloud)",
		Reasoning: true,
	}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || thinking["budget_tokens"] != 16384 {
		t.Errorf("thinking = %+v, want {type: enabled, budget_tokens: 16384}", thinking)
	}
	if _, ok := thinking["display"]; ok {
		t.Errorf("display should be omitted for GovCloud model id, got %+v", thinking)
	}
	beta, _ := got["anthropic_beta"].([]string)
	if len(beta) != 1 || beta[0] != "interleaved-thinking-2025-05-14" {
		t.Errorf("anthropic_beta = %+v", got["anthropic_beta"])
	}
}

func TestBuildAdditionalModelRequestFields_OmitsDisplayForGovCloudRegionOnAdaptiveThinking(t *testing.T) {
	got := buildAdditionalModelRequestFields(opus48(), ai.ThinkingHigh, nil, nil, "", "us-gov-west-1", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking = %+v", thinking)
	}
	if _, ok := thinking["display"]; ok {
		t.Errorf("display should be omitted for GovCloud region, got %+v", thinking)
	}
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "high" {
		t.Errorf("output_config = %+v", outputConfig)
	}
	if _, ok := got["anthropic_beta"]; ok {
		t.Errorf("anthropic_beta should be absent, got %+v", got["anthropic_beta"])
	}
}

func TestBuildAdditionalModelRequestFields_ApplicationInferenceProfileUsesNameForAdaptiveThinking(t *testing.T) {
	model := &ai.Model{
		ID:        "arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/my-profile",
		Name:      "Claude Opus 4.6",
		Reasoning: true,
	}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking = %+v", thinking)
	}
	outputConfig, _ := got["output_config"].(map[string]any)
	if outputConfig["effort"] != "high" {
		t.Errorf("output_config = %+v", outputConfig)
	}
}

func TestBuildAdditionalModelRequestFields_ApplicationInferenceProfileFallsBackToFixedBudgetForNonAdaptive(t *testing.T) {
	model := &ai.Model{
		ID:        "arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/my-profile",
		Name:      "Claude Sonnet 4.5",
		Reasoning: true,
	}
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil)
	thinking, _ := got["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Errorf("thinking = %+v", thinking)
	}
	if _, ok := thinking["budget_tokens"].(int); !ok {
		t.Errorf("budget_tokens missing/wrong type: %+v", thinking)
	}
	beta, _ := got["anthropic_beta"].([]string)
	if len(beta) != 1 || beta[0] != "interleaved-thinking-2025-05-14" {
		t.Errorf("anthropic_beta = %+v", got["anthropic_beta"])
	}
}

func TestBuildAdditionalModelRequestFields_NilWhenReasoningOff(t *testing.T) {
	if got := buildAdditionalModelRequestFields(opus48(), "", nil, nil, "", "", nil); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestBuildAdditionalModelRequestFields_NilForNonReasoningModel(t *testing.T) {
	model := &ai.Model{ID: "amazon.titan-text-express-v1", Reasoning: false}
	if got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, nil, "", "", nil); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestBuildAdditionalModelRequestFields_InterleavedThinkingCanBeDisabled(t *testing.T) {
	model := &ai.Model{
		ID:        "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Reasoning: true,
	}
	off := false
	got := buildAdditionalModelRequestFields(model, ai.ThinkingHigh, nil, &off, "", "", nil)
	if _, ok := got["anthropic_beta"]; ok {
		t.Errorf("anthropic_beta should be absent when interleavedThinking=false, got %+v", got["anthropic_beta"])
	}
}
