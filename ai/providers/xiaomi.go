package providers

// Ports: packages/ai/src/providers/xiaomi.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// XiaomiProvider builds the Xiaomi provider binding, over the
// openai-completions wire adapter.
func XiaomiProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "xiaomi",
		Name:    "Xiaomi",
		BaseURL: "https://api.xiaomimimo.com/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Xiaomi API key", []string{"XIAOMI_API_KEY"})},
		Models:  catalog.BuiltinModels("xiaomi"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
