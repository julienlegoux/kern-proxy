package providers

// Ports: packages/ai/src/providers/ant-ling.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// AntLingProvider builds the Ant Ling provider binding, over the
// openai-completions wire adapter.
func AntLingProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "ant-ling",
		Name:    "Ant Ling",
		BaseURL: "https://api.ant-ling.com/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Ant Ling API key", []string{"ANT_LING_API_KEY"})},
		Models:  catalog.BuiltinModels("ant-ling"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
