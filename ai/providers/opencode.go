package providers

// Ports: packages/ai/src/providers/opencode.ts

import (
	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/apis/google"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/apis/openairesponses"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// OpenCodeProvider builds the OpenCode Zen provider binding, dispatching on
// each model's api across the anthropic-messages, google-generative-ai,
// openai-completions, and openai-responses wire adapters.
func OpenCodeProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     "opencode",
		Name:   "OpenCode Zen",
		Auth:   ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("OpenCode API key", []string{"OPENCODE_API_KEY"})},
		Models: catalog.BuiltinModels("opencode"),
		ApiByProtocol: map[ai.Api]ai.ProviderStreams{
			ai.ApiAnthropicMessages:  ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
			ai.ApiGoogleGenerativeAI: ai.StreamFuncs{StreamFunc: google.Stream, StreamSimpleFunc: google.StreamSimple},
			ai.ApiOpenAICompletions:  ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
			ai.ApiOpenAIResponses:    ai.StreamFuncs{StreamFunc: openairesponses.Stream, StreamSimpleFunc: openairesponses.StreamSimple},
		},
	})
}
