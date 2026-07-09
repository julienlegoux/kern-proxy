package providers

// Ports: packages/ai/src/providers/anthropic.ts. Both auth strategies upstream
// wires are bound here: the api-key env strategy — ANTHROPIC_OAUTH_TOKEN takes
// precedence over ANTHROPIC_API_KEY, which is how upstream's own Claude Code
// impersonation mode authenticates without an interactive login — and the
// `lazyOAuth` "Anthropic (Claude Pro/Max)" interactive login flow, now provided
// by ai/auth/oauth (oauth.AnthropicOAuth).

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/anthropic"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/auth/oauth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// AnthropicProvider builds the Anthropic provider binding.
func AnthropicProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "anthropic",
		Name:    "Anthropic",
		BaseURL: "https://api.anthropic.com",
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("Anthropic API key", []string{"ANTHROPIC_OAUTH_TOKEN", "ANTHROPIC_API_KEY"}),
			OAuth:  oauth.AnthropicOAuth,
		},
		Models: catalog.BuiltinModels("anthropic"),
		Api:    ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
