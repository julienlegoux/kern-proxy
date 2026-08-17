package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

// Ports the thinking-payload half of bedrock-converse-stream.ts:
// supportsAdaptiveThinking, supportsNativeXhighEffort, mapThinkingLevelToEffort,
// isGovCloudBedrockTarget, and buildAdditionalModelRequestFields.

import (
	"strings"

	"github.com/kern-ia/kern-link/ai"
)

// defaultThinkingBudgets are the per-level token budgets for non-adaptive
// (budget-based) Claude thinking on Bedrock. Ports the defaultBudgets literal
// inside buildAdditionalModelRequestFields.
var defaultThinkingBudgets = map[ai.ThinkingLevel]int{
	ai.ThinkingMinimal: 1024,
	ai.ThinkingLow:     2048,
	ai.ThinkingMedium:  8192,
	ai.ThinkingHigh:    16384,
	ai.ThinkingXHigh:   16384, // Claude doesn't support xhigh, clamp to high
}

// supportsAdaptiveThinking reports whether the model supports Claude's
// adaptive-thinking mode (Opus 4.6+, Sonnet 4.6+/5, Fable 5), checked via
// both id and name candidates (application inference profile ARNs don't
// contain the model name). Ports supportsAdaptiveThinking.
func supportsAdaptiveThinking(modelID, modelName string) bool {
	candidates := modelMatchCandidates(modelID, modelName)
	return containsAny(candidates, "opus-4-6") ||
		containsAny(candidates, "opus-4-7") ||
		containsAny(candidates, "opus-4-8") ||
		containsAny(candidates, "sonnet-4-6") ||
		containsAny(candidates, "sonnet-5") ||
		containsAny(candidates, "fable-5")
}

// supportsNativeXhighEffort reports whether the model natively supports
// output_config.effort="xhigh" (as opposed to clamping xhigh to high). Ports
// supportsNativeXhighEffort.
func supportsNativeXhighEffort(model *ai.Model) bool {
	candidates := modelMatchCandidates(model.ID, model.Name)
	return containsAny(candidates, "opus-4-7") || containsAny(candidates, "opus-4-8") || containsAny(candidates, "fable-5")
}

// mapThinkingLevelToEffort maps the abstract ThinkingLevel to Bedrock's
// output_config.effort value, preferring an explicit model.ThinkingLevelMap
// override. Ports mapThinkingLevelToEffort.
func mapThinkingLevelToEffort(model *ai.Model, level ai.ThinkingLevel) string {
	if level == ai.ThinkingXHigh && supportsNativeXhighEffort(model) {
		return "xhigh"
	}
	if model.ThinkingLevelMap != nil {
		if mapped, ok := model.ThinkingLevelMap[level]; ok && mapped != nil {
			return *mapped
		}
	}
	switch level {
	case ai.ThinkingMinimal, ai.ThinkingLow:
		return "low"
	case ai.ThinkingMedium:
		return "medium"
	case ai.ThinkingHigh:
		return "high"
	default:
		return "high"
	}
}

// getConfiguredBedrockRegion resolves the region for the GovCloud
// thinking-display check only: an explicit region option, else
// AWS_REGION/AWS_DEFAULT_REGION. Full region-resolution semantics (ARN
// extraction, endpoint pinning, client config) land with the auth-matrix
// issue. Ports the subset of getConfiguredBedrockRegion needed here.
func getConfiguredBedrockRegion(region string, env ai.ProviderEnv) string {
	if region != "" {
		return region
	}
	if v := providerEnvValue("AWS_REGION", env); v != "" {
		return v
	}
	return providerEnvValue("AWS_DEFAULT_REGION", env)
}

// isGovCloudBedrockTarget reports whether the resolved region or the model's
// id/ARN targets AWS GovCloud. Ports isGovCloudBedrockTarget.
func isGovCloudBedrockTarget(model *ai.Model, region string, env ai.ProviderEnv) bool {
	if r := getConfiguredBedrockRegion(region, env); strings.HasPrefix(strings.ToLower(r), "us-gov-") {
		return true
	}
	id := strings.ToLower(model.ID)
	return strings.HasPrefix(id, "us-gov.") || strings.HasPrefix(id, "arn:aws-us-gov:")
}

// buildAdditionalModelRequestFields builds the additionalModelRequestFields
// map for Claude thinking, or nil when reasoning is off/unrequested, the
// model doesn't advertise Reasoning, or it isn't an Anthropic Claude model
// (non-Claude Bedrock models have no equivalent knob upstream wires here).
// Ports buildAdditionalModelRequestFields.
func buildAdditionalModelRequestFields(
	model *ai.Model,
	reasoning ai.ThinkingLevel,
	thinkingBudgets *ai.ThinkingBudgets,
	interleavedThinking *bool,
	thinkingDisplay string,
	region string,
	env ai.ProviderEnv,
) map[string]any {
	if reasoning == "" || !model.Reasoning {
		return nil
	}
	if !isAnthropicClaudeModel(model) {
		return nil
	}

	// GovCloud Bedrock currently rejects the Claude thinking.display field.
	var display *string
	if !isGovCloudBedrockTarget(model, region, env) {
		d := thinkingDisplay
		if d == "" {
			d = "summarized"
		}
		display = &d
	}

	if supportsAdaptiveThinking(model.ID, model.Name) {
		thinking := map[string]any{"type": "adaptive"}
		if display != nil {
			thinking["display"] = *display
		}
		return map[string]any{
			"thinking":      thinking,
			"output_config": map[string]any{"effort": mapThinkingLevelToEffort(model, reasoning)},
		}
	}

	level := reasoning
	if level == ai.ThinkingXHigh {
		level = ai.ThinkingHigh
	}
	budget := defaultThinkingBudgets[level]
	if thinkingBudgets != nil {
		switch level {
		case ai.ThinkingMinimal:
			if thinkingBudgets.Minimal != nil {
				budget = *thinkingBudgets.Minimal
			}
		case ai.ThinkingLow:
			if thinkingBudgets.Low != nil {
				budget = *thinkingBudgets.Low
			}
		case ai.ThinkingMedium:
			if thinkingBudgets.Medium != nil {
				budget = *thinkingBudgets.Medium
			}
		case ai.ThinkingHigh:
			if thinkingBudgets.High != nil {
				budget = *thinkingBudgets.High
			}
		}
	}

	thinking := map[string]any{"type": "enabled", "budget_tokens": budget}
	if display != nil {
		thinking["display"] = *display
	}
	result := map[string]any{"thinking": thinking}

	interleaved := true
	if interleavedThinking != nil {
		interleaved = *interleavedThinking
	}
	if interleaved {
		result["anthropic_beta"] = []string{"interleaved-thinking-2025-05-14"}
	}
	return result
}
