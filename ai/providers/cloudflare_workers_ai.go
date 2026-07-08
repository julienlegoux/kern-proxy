package providers

// Ports: packages/ai/src/providers/cloudflare-workers-ai.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// CloudflareWorkersAIProvider builds the Cloudflare Workers AI provider
// binding, over the openai-completions wire adapter. No provider-level
// BaseURL: cloudflareWorkersAIAuth resolves the account-scoped endpoint per
// request from the model's {CLOUDFLARE_ACCOUNT_ID}-templated catalog
// baseUrl.
func CloudflareWorkersAIProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     "cloudflare-workers-ai",
		Name:   "Cloudflare Workers AI",
		Auth:   ai.ProviderAuth{APIKey: cloudflareWorkersAIAuth()},
		Models: catalog.BuiltinModels("cloudflare-workers-ai"),
		Api:    ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
