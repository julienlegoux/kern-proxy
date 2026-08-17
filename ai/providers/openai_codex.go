package providers

// Ports: packages/ai/src/providers/openai-codex.ts. Upstream's only auth
// strategy is `lazyOAuth({ name: "OpenAI (ChatGPT Plus/Pro)", load:
// loadOpenAICodexOAuth })`, now bound to the real ai/auth/oauth strategy
// (oauth.CodexOAuth): the dual browser-PKCE / device-code login, refresh, and
// token derivation.

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/codex"
	"github.com/kern-ia/kern-link/ai/auth/oauth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// OpenAICodexProvider builds the OpenAI Codex provider binding.
func OpenAICodexProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "openai-codex",
		Name:    "OpenAI Codex",
		BaseURL: "https://chatgpt.com/backend-api",
		Auth:    ai.ProviderAuth{OAuth: oauth.CodexOAuth},
		Models:  catalog.BuiltinModels("openai-codex"),
		Api:     ai.StreamFuncs{StreamFunc: codex.Stream, StreamSimpleFunc: codex.StreamSimple},
	})
}
