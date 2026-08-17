package providers

// Ports: packages/ai/src/providers/xiaomi-token-plan-ams.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// XiaomiTokenPlanAMSProvider builds the Xiaomi Token Plan AMS provider
// binding, over the openai-completions wire adapter.
func XiaomiTokenPlanAMSProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "xiaomi-token-plan-ams",
		Name:    "Xiaomi Token Plan AMS",
		BaseURL: "https://token-plan-ams.xiaomimimo.com/v1",
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("Xiaomi Token Plan AMS API key", []string{"XIAOMI_TOKEN_PLAN_AMS_API_KEY"}),
		},
		Models: catalog.BuiltinModels("xiaomi-token-plan-ams"),
		Api:    ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
