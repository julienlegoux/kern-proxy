package providers

// Ports: packages/ai/src/providers/xai.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// XAIProvider builds the xAI provider binding, over the openai-completions
// wire adapter.
func XAIProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "xai",
		Name:    "xAI",
		BaseURL: "https://api.x.ai/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("xAI API key", []string{"XAI_API_KEY"})},
		Models:  catalog.BuiltinModels("xai"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
