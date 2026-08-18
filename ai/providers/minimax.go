package providers

// Ports: packages/ai/src/providers/minimax.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// MiniMaxProvider builds the MiniMax provider binding, over the
// anthropic-messages wire adapter.
func MiniMaxProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "minimax",
		Name:    "MiniMax",
		BaseURL: "https://api.minimax.io/anthropic",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("MiniMax API key", []string{"MINIMAX_API_KEY"})},
		Models:  catalog.BuiltinModels("minimax"),
		Api:     ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
