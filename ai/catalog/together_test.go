package catalog

// Ports: packages/ai/test/together-models.test.ts's catalog-shape
// assertions (via getBuiltinModel, upstream's compat.ts alias for
// getBuiltinModel — compat.ts itself isn't ported, see docs/PORTING.md's
// "Intentional deviations"). The upstream file's third case,
// "resolves TOGETHER_API_KEY from the environment", exercises
// env-api-keys.ts (ai/auth/env.go), not the catalog, so it's out of this
// issue's scope; that behavior is already covered by ai/auth/env's own
// tests.

import (
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestTogetherModels_KimiK2_6ViaOpenAICompletions(t *testing.T) {
	model := BuiltinModel("together", "moonshotai/Kimi-K2.6")
	if model == nil {
		t.Fatal(`BuiltinModel("together", "moonshotai/Kimi-K2.6") = nil`)
	}
	if model.Api != ai.ApiOpenAICompletions {
		t.Errorf("Api = %q, want %q", model.Api, ai.ApiOpenAICompletions)
	}
	if model.Provider != "together" {
		t.Errorf("Provider = %q, want \"together\"", model.Provider)
	}
	if model.BaseURL != "https://api.together.ai/v1" {
		t.Errorf("BaseURL = %q, want \"https://api.together.ai/v1\"", model.BaseURL)
	}
	if !model.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	wantThinking := ai.ThinkingLevelMap{"minimal": nil, "low": nil, "medium": nil}
	if !thinkingLevelMapsEqual(model.ThinkingLevelMap, wantThinking) {
		t.Errorf("ThinkingLevelMap = %v, want %v", model.ThinkingLevelMap, wantThinking)
	}
	if len(model.Input) != 2 || model.Input[0] != ai.ModalityText || model.Input[1] != ai.ModalityImage {
		t.Errorf("Input = %v, want [text image]", model.Input)
	}
	if model.ContextWindow != 262144 {
		t.Errorf("ContextWindow = %d, want 262144", model.ContextWindow)
	}
	if model.MaxTokens != 131000 {
		t.Errorf("MaxTokens = %d, want 131000", model.MaxTokens)
	}
	wantCost := ai.ModelCost{Input: 1.2, Output: 4.5, CacheRead: 0.2, CacheWrite: 0}
	if model.Cost != wantCost {
		t.Errorf("Cost = %+v, want %+v", model.Cost, wantCost)
	}
	if model.Compat == nil {
		t.Fatal("Compat = nil, want a compat block")
	}
	wantCompat := ai.Compat{
		SupportsStore:              boolPtr(false),
		SupportsDeveloperRole:      boolPtr(false),
		SupportsReasoningEffort:    boolPtr(false),
		MaxTokensField:             "max_tokens",
		ThinkingFormat:             ai.ThinkingFormatTogether,
		SupportsStrictMode:         boolPtr(false),
		SupportsLongCacheRetention: boolPtr(false),
	}
	if !compatsEqual(*model.Compat, wantCompat) {
		t.Errorf("Compat = %+v, want %+v", *model.Compat, wantCompat)
	}
}

func TestTogetherModels_ReasoningControlsAcrossVariants(t *testing.T) {
	gptOss := BuiltinModel("together", "openai/gpt-oss-120b")
	if gptOss == nil {
		t.Fatal(`BuiltinModel("together", "openai/gpt-oss-120b") = nil`)
	}
	wantGptOssThinking := ai.ThinkingLevelMap{"off": nil, "minimal": nil}
	if !thinkingLevelMapsEqual(gptOss.ThinkingLevelMap, wantGptOssThinking) {
		t.Errorf("gpt-oss-120b ThinkingLevelMap = %v, want %v", gptOss.ThinkingLevelMap, wantGptOssThinking)
	}
	if gptOss.Compat == nil || gptOss.Compat.SupportsReasoningEffort == nil || !*gptOss.Compat.SupportsReasoningEffort {
		t.Error("gpt-oss-120b Compat.SupportsReasoningEffort != true")
	}
	if gptOss.Compat == nil || gptOss.Compat.ThinkingFormat != ai.ThinkingFormatOpenAI {
		t.Errorf("gpt-oss-120b Compat.ThinkingFormat = %v, want %q", gptOss.Compat, ai.ThinkingFormatOpenAI)
	}

	deepSeekV4 := BuiltinModel("together", "deepseek-ai/DeepSeek-V4-Pro")
	if deepSeekV4 == nil {
		t.Fatal(`BuiltinModel("together", "deepseek-ai/DeepSeek-V4-Pro") = nil`)
	}
	high := "high"
	wantDeepSeekThinking := ai.ThinkingLevelMap{"minimal": nil, "low": nil, "medium": nil, "high": &high, "xhigh": nil}
	if !thinkingLevelMapsEqual(deepSeekV4.ThinkingLevelMap, wantDeepSeekThinking) {
		t.Errorf("DeepSeek-V4-Pro ThinkingLevelMap = %v, want %v", deepSeekV4.ThinkingLevelMap, wantDeepSeekThinking)
	}
	if deepSeekV4.Compat == nil || deepSeekV4.Compat.SupportsReasoningEffort == nil || !*deepSeekV4.Compat.SupportsReasoningEffort {
		t.Error("DeepSeek-V4-Pro Compat.SupportsReasoningEffort != true")
	}
	if deepSeekV4.Compat == nil || deepSeekV4.Compat.ThinkingFormat != ai.ThinkingFormatTogether {
		t.Errorf("DeepSeek-V4-Pro Compat.ThinkingFormat = %v, want %q", deepSeekV4.Compat, ai.ThinkingFormatTogether)
	}

	minimax := BuiltinModel("together", "MiniMaxAI/MiniMax-M2.7")
	if minimax == nil {
		t.Fatal(`BuiltinModel("together", "MiniMaxAI/MiniMax-M2.7") = nil`)
	}
	wantMinimaxThinking := ai.ThinkingLevelMap{"off": nil, "minimal": nil, "low": nil, "medium": nil}
	if !thinkingLevelMapsEqual(minimax.ThinkingLevelMap, wantMinimaxThinking) {
		t.Errorf("MiniMax-M2.7 ThinkingLevelMap = %v, want %v", minimax.ThinkingLevelMap, wantMinimaxThinking)
	}
	if minimax.Compat != nil && minimax.Compat.ThinkingFormat != "" {
		t.Errorf("MiniMax-M2.7 Compat.ThinkingFormat = %q, want unset", minimax.Compat.ThinkingFormat)
	}
	if minimax.Compat == nil || minimax.Compat.SupportsReasoningEffort == nil || *minimax.Compat.SupportsReasoningEffort {
		t.Error("MiniMax-M2.7 Compat.SupportsReasoningEffort != false")
	}
}

func boolPtr(b bool) *bool { return &b }

func thinkingLevelMapsEqual(a, b ai.ThinkingLevelMap) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if (av == nil) != (bv == nil) {
			return false
		}
		if av != nil && bv != nil && *av != *bv {
			return false
		}
	}
	return true
}

func compatsEqual(a, b ai.Compat) bool {
	// Compares only the fields both callers here populate; ai.Compat holds
	// pointers and slices/maps so it isn't comparable with ==.
	eqBoolPtr := func(x, y *bool) bool {
		if (x == nil) != (y == nil) {
			return false
		}
		return x == nil || *x == *y
	}
	return eqBoolPtr(a.SupportsStore, b.SupportsStore) &&
		eqBoolPtr(a.SupportsDeveloperRole, b.SupportsDeveloperRole) &&
		eqBoolPtr(a.SupportsReasoningEffort, b.SupportsReasoningEffort) &&
		a.MaxTokensField == b.MaxTokensField &&
		a.ThinkingFormat == b.ThinkingFormat &&
		eqBoolPtr(a.SupportsStrictMode, b.SupportsStrictMode) &&
		eqBoolPtr(a.SupportsLongCacheRetention, b.SupportsLongCacheRetention)
}
