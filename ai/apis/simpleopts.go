// Package apis holds the shared adapter layer: cross-provider message
// transformation, simple-option resolution, and the per-protocol adapters in
// its subpackages.
package apis

// Ports: packages/ai/src/api/simple-options.ts

import (
	"github.com/kern-ia/kern-link/ai"
)

const (
	contextSafetyTokens = 4096
	minMaxTokens        = 1
	minOutputTokens     = 1024
)

// ClampMaxTokensToContext caps maxTokens so the request fits the model's
// context window with a safety margin.
func ClampMaxTokensToContext(model *ai.Model, chat ai.Context, maxTokens int) int {
	if model.ContextWindow <= 0 {
		return maxInt(minMaxTokens, maxTokens)
	}
	available := model.ContextWindow - ai.EstimateContextTokens(chat).Tokens - contextSafetyTokens
	return minInt(maxTokens, maxInt(minMaxTokens, available))
}

// BuildBaseOptions derives the StreamOptions an adapter's Stream should use
// from SimpleStreamOptions, clamping maxTokens to the context window.
// apiKey, when non-empty, wins over the options' APIKey.
func BuildBaseOptions(model *ai.Model, chat ai.Context, options *ai.SimpleStreamOptions, apiKey string) ai.StreamOptions {
	var base ai.StreamOptions
	if options != nil {
		base = options.StreamOptions
	}
	requested := model.MaxTokens
	if base.MaxTokens != nil {
		requested = *base.MaxTokens
	}
	clamped := ClampMaxTokensToContext(model, chat, requested)
	base.MaxTokens = &clamped
	if apiKey != "" {
		base.APIKey = apiKey
	}
	return base
}

// ClampReasoning maps xhigh down to high for providers without an xhigh level.
func ClampReasoning(effort ai.ThinkingLevel) ai.ThinkingLevel {
	if effort == ai.ThinkingXHigh {
		return ai.ThinkingHigh
	}
	return effort
}

// defaultThinkingBudgets are the token budgets per thinking level for
// token-budget-based providers.
var defaultThinkingBudgets = map[ai.ThinkingLevel]int{
	ai.ThinkingMinimal: 1024,
	ai.ThinkingLow:     2048,
	ai.ThinkingMedium:  8192,
	ai.ThinkingHigh:    16384,
}

// AdjustMaxTokensForThinking fits a thinking budget inside the output cap.
// baseMaxTokens nil means no explicit caller cap: use the model cap and fit
// thinking inside it. Otherwise the budget is added on top of the caller cap,
// clamped to the model cap; if the result would leave no room for output the
// budget shrinks to keep minOutputTokens available.
func AdjustMaxTokensForThinking(
	baseMaxTokens *int,
	modelMaxTokens int,
	reasoningLevel ai.ThinkingLevel,
	customBudgets *ai.ThinkingBudgets,
) (maxTokens int, thinkingBudget int) {
	budgets := map[ai.ThinkingLevel]int{}
	for level, budget := range defaultThinkingBudgets {
		budgets[level] = budget
	}
	if customBudgets != nil {
		if customBudgets.Minimal != nil {
			budgets[ai.ThinkingMinimal] = *customBudgets.Minimal
		}
		if customBudgets.Low != nil {
			budgets[ai.ThinkingLow] = *customBudgets.Low
		}
		if customBudgets.Medium != nil {
			budgets[ai.ThinkingMedium] = *customBudgets.Medium
		}
		if customBudgets.High != nil {
			budgets[ai.ThinkingHigh] = *customBudgets.High
		}
	}

	level := ClampReasoning(reasoningLevel)
	thinkingBudget = budgets[level]
	if baseMaxTokens == nil {
		maxTokens = modelMaxTokens
	} else {
		maxTokens = minInt(*baseMaxTokens+thinkingBudget, modelMaxTokens)
	}

	if maxTokens <= thinkingBudget {
		thinkingBudget = maxInt(0, maxTokens-minOutputTokens)
	}

	return maxTokens, thinkingBudget
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
