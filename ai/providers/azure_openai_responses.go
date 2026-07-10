package providers

// Ports: packages/ai/src/providers/azure-openai-responses.ts. No provider-
// level BaseURL: the adapter resolves the Azure endpoint per request from
// AZURE_OPENAI_BASE_URL/AZURE_OPENAI_RESOURCE_NAME/AZURE_OPENAI_API_VERSION/
// AZURE_OPENAI_DEPLOYMENT_NAME_MAP (see ai/apis/azure), matching the empty
// baseUrl on this provider's catalog models.

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/azure"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// AzureOpenAIResponsesProvider builds the Azure OpenAI provider binding.
func AzureOpenAIResponsesProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     "azure-openai-responses",
		Name:   "Azure OpenAI",
		Auth:   ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Azure OpenAI API key", []string{"AZURE_OPENAI_API_KEY"})},
		Models: catalog.BuiltinModels("azure-openai-responses"),
		Api:    ai.StreamFuncs{StreamFunc: azure.Stream, StreamSimpleFunc: azure.StreamSimple},
	})
}
