package providers

// Ports: packages/ai/src/providers/deepseek.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// DeepSeekProvider builds the DeepSeek provider binding, over the
// openai-completions wire adapter.
func DeepSeekProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "deepseek",
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("DeepSeek API key", []string{"DEEPSEEK_API_KEY"})},
		Models:  catalog.BuiltinModels("deepseek"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
