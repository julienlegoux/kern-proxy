package providers

// Ports: packages/ai/src/providers/kimi-coding.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// KimiCodingProvider builds the Kimi For Coding provider binding, over the
// anthropic-messages wire adapter.
func KimiCodingProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "kimi-coding",
		Name:    "Kimi For Coding",
		BaseURL: "https://api.kimi.com/coding",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Kimi API key", []string{"KIMI_API_KEY"})},
		Models:  catalog.BuiltinModels("kimi-coding"),
		Api:     ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
