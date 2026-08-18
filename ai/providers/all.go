// Package providers holds the built-in provider bindings — thin
// declarations over each wire-protocol adapter in ai/apis, wiring the
// embedded catalog (ai/catalog) and an auth strategy to each adapter's
// Stream/StreamSimple functions — plus the in-process faux test provider
// (ai/providers/faux).
package providers

// Ports: packages/ai/src/providers/all.ts (builtinProviders/builtinModels;
// getBuiltinModel/getBuiltinModels/getBuiltinProviders's static-catalog-read
// half is ai/catalog, Issue 01). Issue 02 wired the 8 core bindings that
// front a first-party adapter package (anthropic, openai, azure, codex,
// google, vertex, mistral, bedrock). Issue 03 (this) appends the remaining
// ~27 compat-vendor bindings (mostly thin declarations over the
// openai-completions or anthropic-messages adapters) plus the native
// RefreshModels implementations for OpenRouter, Vercel AI Gateway, NVIDIA,
// and GitHub Copilot — see those bindings' own doc comments — bringing the
// total to upstream's ~35 built-in providers.

import "github.com/kern-ia/kern-link/ai"

// Providers returns every built-in provider binding, freshly constructed.
func Providers() []ai.Provider {
	return []ai.Provider{
		AmazonBedrockProvider(),
		AntLingProvider(),
		AnthropicProvider(),
		AzureOpenAIResponsesProvider(),
		CerebrasProvider(),
		CloudflareAIGatewayProvider(),
		CloudflareWorkersAIProvider(),
		DeepSeekProvider(),
		FireworksProvider(),
		GitHubCopilotProvider(),
		GoogleProvider(),
		GoogleVertexProvider(),
		GroqProvider(),
		HuggingFaceProvider(),
		KimiCodingProvider(),
		MiniMaxProvider(),
		MiniMaxCNProvider(),
		MistralProvider(),
		MoonshotAIProvider(),
		MoonshotAICNProvider(),
		NvidiaProvider(),
		OpenAIProvider(),
		OpenAICodexProvider(),
		OpenCodeProvider(),
		OpenCodeGoProvider(),
		OpenRouterProvider(),
		TogetherProvider(),
		VercelAIGatewayProvider(),
		XAIProvider(),
		XiaomiProvider(),
		XiaomiTokenPlanAMSProvider(),
		XiaomiTokenPlanCNProvider(),
		XiaomiTokenPlanSGPProvider(),
		ZaiProvider(),
		ZaiCodingCNProvider(),
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
