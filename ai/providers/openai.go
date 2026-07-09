package providers

// Ports: packages/ai/src/providers/openai.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openairesponses"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// OpenAIProvider builds the OpenAI provider binding, over the
// openai-responses wire adapter.
func OpenAIProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "openai",
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("OpenAI API key", []string{"OPENAI_API_KEY"})},
		Models:  catalog.BuiltinModels("openai"),
		Api:     ai.StreamFuncs{StreamFunc: openairesponses.Stream, StreamSimpleFunc: openairesponses.StreamSimple},
	})
}
