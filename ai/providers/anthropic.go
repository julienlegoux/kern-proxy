package providers

// Ports: packages/ai/src/providers/anthropic.ts. Only the api-key auth
// strategy is wired here — ANTHROPIC_OAUTH_TOKEN still takes precedence over
// ANTHROPIC_API_KEY, which is how upstream's own Claude Code impersonation
// mode authenticates without an interactive login. The `lazyOAuth`
// "Anthropic (Claude Pro/Max)" interactive login flow is ai/auth/oauth,
// deferred to Epic 12.

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/anthropic"
	"github.com/julienlegoux/kern-proxy/ai/auth"
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
		},
		Models: catalog.BuiltinModels("anthropic"),
		Api:    ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
