package providers

// Ports: packages/ai/src/providers/zai.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// ZaiProvider builds the Z.AI provider binding, over the openai-completions
// wire adapter.
func ZaiProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "zai",
		Name:    "Z.AI",
		BaseURL: "https://api.z.ai/api/coding/paas/v4",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Z.AI API key", []string{"ZAI_API_KEY"})},
		Models:  catalog.BuiltinModels("zai"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
