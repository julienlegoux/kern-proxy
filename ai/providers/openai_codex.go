package providers

// Ports: packages/ai/src/providers/openai-codex.ts. Upstream's only auth
// strategy is `lazyOAuth({ name: "OpenAI (ChatGPT Plus/Pro)", load:
// loadOpenAICodexOAuth })`. ai/auth/oauth (Epic 12) has not landed yet, so
// this binding advertises the same OAuth strategy shape (Name, for status
// UI) with Login/Refresh/ToAuth stubs that report the flow as not yet
// implemented. Until Epic 12 replaces this stub, ai.ResolveProviderAuth
// already treats "no stored credential" as unconfigured — the same
// observable behavior as upstream before a user has logged in.

import (
	"context"
	"errors"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/codex"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

var errCodexOAuthPending = errors.New("providers: OpenAI Codex OAuth login is not implemented yet (Epic 12)")

// OpenAICodexProvider builds the OpenAI Codex provider binding.
func OpenAICodexProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "openai-codex",
		Name:    "OpenAI Codex",
		BaseURL: "https://chatgpt.com/backend-api",
		Auth: ai.ProviderAuth{OAuth: &ai.OAuthAuth{
			Name: "OpenAI (ChatGPT Plus/Pro)",
			Login: func(context.Context, ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
				return nil, errCodexOAuthPending
			},
			Refresh: func(context.Context, *ai.OAuthCredential) (*ai.OAuthCredential, error) {
				return nil, errCodexOAuthPending
			},
			ToAuth: func(context.Context, *ai.OAuthCredential) (ai.ModelAuth, error) {
				return ai.ModelAuth{}, errCodexOAuthPending
			},
		}},
		Models: catalog.BuiltinModels("openai-codex"),
		Api:    ai.StreamFuncs{StreamFunc: codex.Stream, StreamSimpleFunc: codex.StreamSimple},
	})
}
