package providers

// Ports: packages/ai/src/providers/xiaomi-token-plan-cn.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// XiaomiTokenPlanCNProvider builds the Xiaomi Token Plan CN provider
// binding, over the openai-completions wire adapter.
func XiaomiTokenPlanCNProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "xiaomi-token-plan-cn",
		Name:    "Xiaomi Token Plan CN",
		BaseURL: "https://token-plan-cn.xiaomimimo.com/v1",
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("Xiaomi Token Plan CN API key", []string{"XIAOMI_TOKEN_PLAN_CN_API_KEY"}),
		},
		Models: catalog.BuiltinModels("xiaomi-token-plan-cn"),
		Api:    ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
