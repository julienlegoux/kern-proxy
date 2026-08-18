package providers

// Ports: packages/ai/src/providers/xiaomi-token-plan-sgp.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// XiaomiTokenPlanSGPProvider builds the Xiaomi Token Plan SGP provider
// binding, over the openai-completions wire adapter.
func XiaomiTokenPlanSGPProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "xiaomi-token-plan-sgp",
		Name:    "Xiaomi Token Plan SGP",
		BaseURL: "https://token-plan-sgp.xiaomimimo.com/v1",
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("Xiaomi Token Plan SGP API key", []string{"XIAOMI_TOKEN_PLAN_SGP_API_KEY"}),
		},
		Models: catalog.BuiltinModels("xiaomi-token-plan-sgp"),
		Api:    ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
