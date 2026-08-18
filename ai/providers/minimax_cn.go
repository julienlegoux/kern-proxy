package providers

// Ports: packages/ai/src/providers/minimax-cn.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// MiniMaxCNProvider builds the MiniMax CN provider binding, over the
// anthropic-messages wire adapter.
func MiniMaxCNProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "minimax-cn",
		Name:    "MiniMax CN",
		BaseURL: "https://api.minimaxi.com/anthropic",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("MiniMax CN API key", []string{"MINIMAX_CN_API_KEY"})},
		Models:  catalog.BuiltinModels("minimax-cn"),
		Api:     ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
