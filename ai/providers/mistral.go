package providers

// Ports: packages/ai/src/providers/mistral.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/mistral"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// MistralProvider builds the Mistral provider binding, over the
// mistral-conversations wire adapter.
func MistralProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "mistral",
		Name:    "Mistral",
		BaseURL: "https://api.mistral.ai",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Mistral API key", []string{"MISTRAL_API_KEY"})},
		Models:  catalog.BuiltinModels("mistral"),
		Api:     ai.StreamFuncs{StreamFunc: mistral.Stream, StreamSimpleFunc: mistral.StreamSimple},
	})
}
