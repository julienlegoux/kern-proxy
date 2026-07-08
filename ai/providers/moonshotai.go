package providers

// Ports: packages/ai/src/providers/moonshotai.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// MoonshotAIProvider builds the Moonshot AI provider binding, over the
// openai-completions wire adapter.
func MoonshotAIProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "moonshotai",
		Name:    "Moonshot AI",
		BaseURL: "https://api.moonshot.ai/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Moonshot AI API key", []string{"MOONSHOT_API_KEY"})},
		Models:  catalog.BuiltinModels("moonshotai"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
