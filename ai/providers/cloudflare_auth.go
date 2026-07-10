package providers

// Ports: packages/ai/src/providers/cloudflare-auth.ts. Both Cloudflare
// bindings (cloudflare-workers-ai, cloudflare-ai-gateway) need an account id
// (and, for the AI Gateway, a gateway id too) alongside the API key, so
// neither can use the plain auth.EnvAPIKeyAuth helper: the resolved baseUrl
// is templated per-model from Model.BaseURL's {CLOUDFLARE_ACCOUNT_ID}/
// {CLOUDFLARE_GATEWAY_ID} placeholders, and the AI Gateway additionally
// swaps the auth scheme onto a custom header instead of api_key/Authorization.

import (
	"context"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
)

const (
	cloudflareAPIKeyEnv    = "CLOUDFLARE_API_KEY"
	cloudflareAccountIDEnv = "CLOUDFLARE_ACCOUNT_ID"
	cloudflareGatewayIDEnv = "CLOUDFLARE_GATEWAY_ID"
)

func resolveCloudflareBaseURL(model *ai.Model, accountID, gatewayID string) string {
	url := strings.ReplaceAll(model.BaseURL, "{"+cloudflareAccountIDEnv+"}", accountID)
	url = strings.ReplaceAll(url, "{"+cloudflareGatewayIDEnv+"}", gatewayID)
	return url
}

type resolvedCloudflareEnv struct {
	apiKey  string
	env     ai.ProviderEnv
	baseURL string
	source  string
}

// resolveCloudflareEnv reads the api key/account id (and, for the AI
// Gateway, the gateway id) from the stored credential's scoped env falling
// back to ambient env vars, matching upstream's resolveCloudflareEnv. A
// stored credential's Env values take precedence over ambient env, same as
// EnvAPIKeyAuth's Key precedence.
func resolveCloudflareResolvedEnv(needsGateway bool, model *ai.Model, ctx ai.AuthContext, credential *ai.APIKeyCredential) *resolvedCloudflareEnv {
	resolveValue := func(name string) string {
		if credential != nil {
			if name == cloudflareAPIKeyEnv {
				return credential.Key
			}
			return credential.Env[name]
		}
		return ctx.Env(name)
	}

	apiKey := resolveValue(cloudflareAPIKeyEnv)
	accountID := resolveValue(cloudflareAccountIDEnv)
	var gatewayID string
	if needsGateway {
		gatewayID = resolveValue(cloudflareGatewayIDEnv)
	}

	if apiKey == "" || accountID == "" || (needsGateway && gatewayID == "") {
		return nil
	}

	env := ai.ProviderEnv{cloudflareAccountIDEnv: accountID}
	if gatewayID != "" {
		env[cloudflareGatewayIDEnv] = gatewayID
	}

	source := cloudflareAPIKeyEnv
	if credential != nil {
		source = "stored credential"
	}

	return &resolvedCloudflareEnv{
		apiKey:  apiKey,
		env:     env,
		baseURL: resolveCloudflareBaseURL(model, accountID, gatewayID),
		source:  source,
	}
}

// cloudflareWorkersAIAuth resolves plain api-key auth scoped to an account
// id, templated into the model's baseUrl.
func cloudflareWorkersAIAuth() *ai.APIKeyAuth {
	return &ai.APIKeyAuth{
		Name: "Cloudflare API key",
		Resolve: func(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
			resolved := resolveCloudflareResolvedEnv(false, input.Model, input.Ctx, input.Credential)
			if resolved == nil {
				return nil, nil
			}
			return &ai.AuthResult{
				Auth:   ai.ModelAuth{APIKey: resolved.apiKey, BaseURL: resolved.baseURL},
				Env:    resolved.env,
				Source: resolved.source,
			}, nil
		},
	}
}

// cloudflareAIGatewayAuth resolves api-key auth scoped to an account+gateway
// id pair, swapping the auth scheme onto the `cf-aig-authorization` header
// (with Authorization/x-api-key explicitly cleared) instead of a plain api
// key, matching upstream's gateway-specific header shape.
func cloudflareAIGatewayAuth() *ai.APIKeyAuth {
	return &ai.APIKeyAuth{
		Name: "Cloudflare API key",
		Resolve: func(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
			resolved := resolveCloudflareResolvedEnv(true, input.Model, input.Ctx, input.Credential)
			if resolved == nil {
				return nil, nil
			}
			return &ai.AuthResult{
				Auth: ai.ModelAuth{
					Headers: ai.ProviderHeaders{
						"cf-aig-authorization": ai.HeaderValue("Bearer " + resolved.apiKey),
						"Authorization":        nil,
						"x-api-key":            nil,
					},
					BaseURL: resolved.baseURL,
				},
				Env:    resolved.env,
				Source: resolved.source,
			}, nil
		},
	}
}
