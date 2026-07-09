package providers

// Ports: packages/ai/src/providers/opencode-go.ts

import (
	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/anthropic"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// OpenCodeGoProvider builds the OpenCode Zen Go provider binding,
// dispatching on each model's api between the anthropic-messages and
// openai-completions wire adapters.
func OpenCodeGoProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     "opencode-go",
		Name:   "OpenCode Zen Go",
		Auth:   ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("OpenCode API key", []string{"OPENCODE_API_KEY"})},
		Models: catalog.BuiltinModels("opencode-go"),
		ApiByProtocol: map[ai.Api]ai.ProviderStreams{
			ai.ApiAnthropicMessages: ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
			ai.ApiOpenAICompletions: ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
		},
	})
}
