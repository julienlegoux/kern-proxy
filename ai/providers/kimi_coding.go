package providers

// Ports: packages/ai/src/providers/kimi-coding.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/anthropic"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
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
