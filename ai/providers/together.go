package providers

// Ports: packages/ai/src/providers/together.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// TogetherProvider builds the Together provider binding, over the
// openai-completions wire adapter.
func TogetherProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "together",
		Name:    "Together",
		BaseURL: "https://api.together.ai/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Together API key", []string{"TOGETHER_API_KEY"})},
		Models:  catalog.BuiltinModels("together"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
