package providers

// Ports: packages/ai/src/providers/moonshotai-cn.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// MoonshotAICNProvider builds the Moonshot AI CN provider binding, over the
// openai-completions wire adapter.
func MoonshotAICNProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "moonshotai-cn",
		Name:    "Moonshot AI CN",
		BaseURL: "https://api.moonshot.cn/v1",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Moonshot AI API key", []string{"MOONSHOT_API_KEY"})},
		Models:  catalog.BuiltinModels("moonshotai-cn"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
