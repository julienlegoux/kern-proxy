package providers

// Ports: packages/ai/src/providers/groq.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// GroqProvider builds the Groq provider binding, over the openai-completions
// wire adapter.
func GroqProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "groq",
		Name:    "Groq",
		BaseURL: "https://api.groq.com/openai/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Groq API key", []string{"GROQ_API_KEY"})},
		Models:  catalog.BuiltinModels("groq"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
