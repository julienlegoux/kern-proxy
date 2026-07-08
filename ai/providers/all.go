// Package providers holds the built-in provider bindings — thin
// declarations over each wire-protocol adapter in ai/apis, wiring the
// embedded catalog (ai/catalog) and an auth strategy to each adapter's
// Stream/StreamSimple functions — plus the in-process faux test provider
// (ai/providers/faux).
package providers

// Ports: packages/ai/src/providers/all.ts (builtinProviders/builtinModels;
// getBuiltinModel/getBuiltinModels/getBuiltinProviders's static-catalog-read
// half is ai/catalog, Issue 01). This issue wires the 8 core bindings that
// front a first-party adapter package (anthropic, openai, azure, codex,
// google, vertex, mistral, bedrock); the ~25 remaining compat-vendor
// bindings and their RefreshModels implementations are
// docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md,
// appended to the Providers() list there.

import "github.com/julienlegoux/kern-proxy/ai"

// Providers returns every built-in provider binding, freshly constructed.
func Providers() []ai.Provider {
	return []ai.Provider{
		AnthropicProvider(),
		OpenAIProvider(),
		AzureOpenAIResponsesProvider(),
		OpenAICodexProvider(),
		GoogleProvider(),
		GoogleVertexProvider(),
		MistralProvider(),
		AmazonBedrockProvider(),
	}
}

// Models returns a Models collection with every built-in provider
// registered.
func Models(options *ai.CreateModelsOptions) ai.MutableModels {
	models := ai.CreateModels(options)
	for _, provider := range Providers() {
		models.SetProvider(provider)
	}
	return models
}
