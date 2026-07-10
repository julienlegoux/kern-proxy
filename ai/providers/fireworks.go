package providers

// Ports: packages/ai/src/providers/fireworks.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/anthropic"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// FireworksProvider builds the Fireworks provider binding, dispatching on
// each model's api between the anthropic-messages and openai-completions
// wire adapters.
func FireworksProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "fireworks",
		Name:    "Fireworks",
		BaseURL: "https://api.fireworks.ai/inference",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Fireworks API key", []string{"FIREWORKS_API_KEY"})},
		Models:  catalog.BuiltinModels("fireworks"),
		ApiByProtocol: map[ai.Api]ai.ProviderStreams{
			ai.ApiAnthropicMessages: ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
			ai.ApiOpenAICompletions: ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
		},
	})
}
