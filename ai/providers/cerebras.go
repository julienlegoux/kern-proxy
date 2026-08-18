package providers

// Ports: packages/ai/src/providers/cerebras.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// CerebrasProvider builds the Cerebras provider binding, over the
// openai-completions wire adapter.
func CerebrasProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "cerebras",
		Name:    "Cerebras",
		BaseURL: "https://api.cerebras.ai/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Cerebras API key", []string{"CEREBRAS_API_KEY"})},
		Models:  catalog.BuiltinModels("cerebras"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
