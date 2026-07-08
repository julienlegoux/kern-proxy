package auth

// Ports: packages/ai/src/auth/helpers.ts (envApiKeyAuth). lazyOAuth is not
// ported: it exists purely to keep Node-only OAuth flow code out of bundles
// via a dynamic import, a TS-bundler concern with no Go equivalent — Go
// providers construct their ai.OAuthAuth directly (see ai/providers).

import (
	"context"

	"github.com/julienlegoux/kern-proxy/ai"
)

// EnvAPIKeyAuth builds a standard api-key auth strategy: a stored credential
// key wins, otherwise the first set environment variable in envVars
// resolves, in order. Login prompts for the key as a secret.
func EnvAPIKeyAuth(name string, envVars []string) *ai.APIKeyAuth {
	return &ai.APIKeyAuth{
		Name: name,
		Login: func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.APIKeyCredential, error) {
			key, err := callbacks.Prompt(ctx, ai.AuthPrompt{
				Type:    ai.AuthPromptSecret,
				Message: "Enter " + name,
			})
			if err != nil {
				return nil, err
			}
			return &ai.APIKeyCredential{Key: key}, nil
		},
		Resolve: func(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
			if input.Credential != nil && input.Credential.Key != "" {
				return &ai.AuthResult{
					Auth:   ai.ModelAuth{APIKey: input.Credential.Key},
					Source: "stored credential",
				}, nil
			}
			for _, envVar := range envVars {
				if value := input.Ctx.Env(envVar); value != "" {
					return &ai.AuthResult{Auth: ai.ModelAuth{APIKey: value}, Source: envVar}, nil
				}
			}
			return nil, nil
		},
	}
}
