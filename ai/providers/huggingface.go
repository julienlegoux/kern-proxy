package providers

// Ports: packages/ai/src/providers/huggingface.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// HuggingFaceProvider builds the Hugging Face provider binding, over the
// openai-completions wire adapter.
func HuggingFaceProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "huggingface",
		Name:    "Hugging Face",
		BaseURL: "https://router.huggingface.co/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Hugging Face token", []string{"HF_TOKEN"})},
		Models:  catalog.BuiltinModels("huggingface"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
