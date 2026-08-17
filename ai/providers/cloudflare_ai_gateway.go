package providers

// Ports: packages/ai/src/providers/cloudflare-ai-gateway.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/apis/openairesponses"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// CloudflareAIGatewayProvider builds the Cloudflare AI Gateway provider
// binding, dispatching on each model's api between the anthropic-messages,
// openai-completions, and openai-responses wire adapters. No provider-level
// BaseURL: cloudflareAIGatewayAuth resolves the account+gateway-scoped
// endpoint per request from the model's catalog baseUrl template.
func CloudflareAIGatewayProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     "cloudflare-ai-gateway",
		Name:   "Cloudflare AI Gateway",
		Auth:   ai.ProviderAuth{APIKey: cloudflareAIGatewayAuth()},
		Models: catalog.BuiltinModels("cloudflare-ai-gateway"),
		ApiByProtocol: map[ai.Api]ai.ProviderStreams{
			ai.ApiAnthropicMessages: ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
			ai.ApiOpenAICompletions: ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
			ai.ApiOpenAIResponses:   ai.StreamFuncs{StreamFunc: openairesponses.Stream, StreamSimpleFunc: openairesponses.StreamSimple},
		},
	})
}
