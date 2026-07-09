package images

// Ports: packages/ai/src/providers/openrouter-images.ts (openrouterImagesProvider),
// packages/ai/src/providers/all.ts (builtinImagesProviders/builtinImagesModels
// — the image-generation half; the text-side Providers/Models equivalents
// live in ai/providers, epic 11).

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/auth"
)

// OpenRouterProvider builds the OpenRouter image-generation provider
// binding: the embedded catalog's models, env-key auth
// (OPENROUTER_API_KEY), and the openrouter-images adapter.
func OpenRouterProvider() Provider {
	return CreateProvider(CreateProviderOptions{
		ID:     "openrouter",
		Name:   "OpenRouter",
		Auth:   ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("OpenRouter API key", []string{"OPENROUTER_API_KEY"})},
		Models: CatalogModels("openrouter"),
		API:    generateImagesOpenRouter,
	})
}

// BuiltinProviders returns every built-in image-generation provider,
// freshly constructed.
func BuiltinProviders() []Provider {
	return []Provider{OpenRouterProvider()}
}

// BuiltinModels returns a Models collection with every built-in
// image-generation provider registered.
func BuiltinModels(options *ai.CreateModelsOptions) MutableModels {
	models := CreateModels(options)
	for _, provider := range BuiltinProviders() {
		models.SetProvider(provider)
	}
	return models
}
