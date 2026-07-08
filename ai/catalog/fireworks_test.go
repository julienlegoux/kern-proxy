package catalog

// Ports: packages/ai/test/fireworks-models.test.ts's catalog-shape
// assertions only (via getBuiltinModel/getBuiltinModels, upstream's
// compat.ts aliases — compat.ts itself isn't ported, see docs/PORTING.md's
// "Intentional deviations"). Not ported: the file's
// "resolves FIREWORKS_API_KEY from the environment" case (env-api-keys.ts /
// ai/auth/env.go territory, already covered by that package's own tests)
// and its HTTP-integration suite exercising anthropic-messages session
// affinity and tool compat against a mock server (adapter behavior, ported
// alongside ai/apis/anthropic in epic 5, not the catalog).

import (
	"strings"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestFireworksModels_KimiK2_6ViaAnthropicMessages(t *testing.T) {
	model := BuiltinModel("fireworks", "accounts/fireworks/models/kimi-k2p6")
	if model == nil {
		t.Fatal(`BuiltinModel("fireworks", "accounts/fireworks/models/kimi-k2p6") = nil`)
	}
	if model.Api != ai.ApiAnthropicMessages {
		t.Errorf("Api = %q, want %q", model.Api, ai.ApiAnthropicMessages)
	}
	if model.Provider != "fireworks" {
		t.Errorf("Provider = %q, want \"fireworks\"", model.Provider)
	}
	if model.BaseURL != "https://api.fireworks.ai/inference" {
		t.Errorf("BaseURL = %q, want \"https://api.fireworks.ai/inference\"", model.BaseURL)
	}
	if !model.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	if len(model.Input) != 2 || model.Input[0] != ai.ModalityText || model.Input[1] != ai.ModalityImage {
		t.Errorf("Input = %v, want [text image]", model.Input)
	}
	if model.ContextWindow != 262000 {
		t.Errorf("ContextWindow = %d, want 262000", model.ContextWindow)
	}
	if model.MaxTokens != 262000 {
		t.Errorf("MaxTokens = %d, want 262000", model.MaxTokens)
	}
	wantCost := ai.ModelCost{Input: 0.95, Output: 4, CacheRead: 0.16, CacheWrite: 0}
	if model.Cost != wantCost {
		t.Errorf("Cost = %+v, want %+v", model.Cost, wantCost)
	}
}

func TestFireworksModels_FirePassTurboRouterModel(t *testing.T) {
	var found *ai.Model
	for _, m := range BuiltinModels("fireworks") {
		if strings.HasPrefix(m.ID, "accounts/fireworks/routers/") && strings.HasSuffix(m.ID, "-turbo") {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatal("no fireworks router model id matches accounts/fireworks/routers/*-turbo")
	}
	if found.Api != ai.ApiAnthropicMessages {
		t.Errorf("Api = %q, want %q", found.Api, ai.ApiAnthropicMessages)
	}
	if found.BaseURL != "https://api.fireworks.ai/inference" {
		t.Errorf("BaseURL = %q, want \"https://api.fireworks.ai/inference\"", found.BaseURL)
	}
	if len(found.Input) != 2 || found.Input[0] != ai.ModalityText || found.Input[1] != ai.ModalityImage {
		t.Errorf("Input = %v, want [text image]", found.Input)
	}
}

func TestFireworksModels_GLM5_2FastAlignsWithBase(t *testing.T) {
	base := BuiltinModel("fireworks", "accounts/fireworks/models/glm-5p2")
	fast := BuiltinModel("fireworks", "accounts/fireworks/routers/glm-5p2-fast")
	if base == nil || fast == nil {
		t.Fatal("expected both glm-5p2 and glm-5p2-fast in the fireworks catalog")
	}
	if fast.Api != base.Api {
		t.Errorf("fast.Api = %q, base.Api = %q", fast.Api, base.Api)
	}
	if fast.BaseURL != base.BaseURL {
		t.Errorf("fast.BaseURL = %q, base.BaseURL = %q", fast.BaseURL, base.BaseURL)
	}
	if !compatsEqual(derefCompat(fast.Compat), derefCompat(base.Compat)) {
		t.Errorf("fast.Compat = %+v, base.Compat = %+v", fast.Compat, base.Compat)
	}
	if !thinkingLevelMapsEqual(fast.ThinkingLevelMap, base.ThinkingLevelMap) {
		t.Errorf("fast.ThinkingLevelMap = %v, base.ThinkingLevelMap = %v", fast.ThinkingLevelMap, base.ThinkingLevelMap)
	}
}

func TestFireworksModels_SessionAffinityAndToolCompat(t *testing.T) {
	model := BuiltinModel("fireworks", "accounts/fireworks/models/kimi-k2p6")
	if model == nil || model.Compat == nil {
		t.Fatal("expected kimi-k2p6 with a compat block")
	}
	c := model.Compat
	if c.SendSessionAffinityHeaders == nil || !*c.SendSessionAffinityHeaders {
		t.Error("SendSessionAffinityHeaders != true")
	}
	if c.SupportsEagerToolInputStreaming == nil || *c.SupportsEagerToolInputStreaming {
		t.Error("SupportsEagerToolInputStreaming != false")
	}
	if c.SupportsCacheControlOnTools == nil || *c.SupportsCacheControlOnTools {
		t.Error("SupportsCacheControlOnTools != false")
	}
	if c.SupportsLongCacheRetention == nil || *c.SupportsLongCacheRetention {
		t.Error("SupportsLongCacheRetention != false")
	}
}

func derefCompat(c *ai.Compat) ai.Compat {
	if c == nil {
		return ai.Compat{}
	}
	return *c
}
