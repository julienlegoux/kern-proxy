package providers

// Ports: packages/ai/src/providers/zai-coding-cn.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// ZaiCodingCNProvider builds the Z.AI Coding CN provider binding, over the
// openai-completions wire adapter.
func ZaiCodingCNProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "zai-coding-cn",
		Name:    "Z.AI Coding CN",
		BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Z.AI Coding CN API key", []string{"ZAI_CODING_CN_API_KEY"})},
		Models:  catalog.BuiltinModels("zai-coding-cn"),
		Api:     ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
