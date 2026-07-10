package providers

// Ports: packages/ai/test/providers.test.ts (the `describe("builtin
// providers", ...)` cases). Issue 02 ported the core, first-party-adapter
// cases: anthropic OAuth-token env precedence, bedrock ambient AWS
// credentials, and vertex ADC resolution/explicit-key override. Issue 03
// (this) ports the remaining compat-vendor cases — Cloudflare Workers
// AI/AI Gateway account+gateway env resolution — and extends the
// "builtinModels registers every builtin provider with models" case to the
// full ~35-provider set (upstream asserts `all.length > 500`; the embedded
// catalog here totals 1042). The `describe("createProvider", ...)`
// mechanics are already ported onto ai.CreateProvider directly, see
// ai/provider_test.go.

import (
	"context"
	"testing"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/auth/oauth"
)

type fakeAuthContext struct {
	env   map[string]string
	files map[string]bool
}

func (f fakeAuthContext) Env(name string) string      { return f.env[name] }
func (f fakeAuthContext) FileExists(path string) bool { return f.files[path] }

func isEmptyModelAuth(auth ai.ModelAuth) bool {
	return auth.APIKey == "" && auth.BaseURL == "" && len(auth.Headers) == 0
}

func newModels(env map[string]string, files ...string) ai.MutableModels {
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}
	return ai.CreateModels(&ai.CreateModelsOptions{
		AuthContext: fakeAuthContext{env: env, files: fileSet},
	})
}

// TestProvidersRegistersEveryCoreBindingWithModels checks the 8 core,
// first-party-adapter bindings from Issue 02 specifically: each declares the
// expected single api on every model it owns. Total provider/model counts
// across the full ~35-provider set are TestBuiltinModelsRegistersEveryProviderWithModels's
// job below, now that Issue 03 has appended the remaining compat-vendor
// bindings to Providers().
func TestProvidersRegistersEveryCoreBindingWithModels(t *testing.T) {
	wantAPI := map[string]ai.Api{
		"anthropic":              ai.ApiAnthropicMessages,
		"openai":                 ai.ApiOpenAIResponses,
		"azure-openai-responses": ai.ApiAzureOpenAIResponses,
		"openai-codex":           ai.ApiOpenAICodexResponses,
		"google":                 ai.ApiGoogleGenerativeAI,
		"google-vertex":          ai.ApiGoogleVertex,
		"mistral":                ai.ApiMistralConversations,
		"amazon-bedrock":         ai.ApiBedrockConverseStream,
	}

	byID := map[string]ai.Provider{}
	for _, provider := range Providers() {
		byID[provider.ID()] = provider
	}

	for id, api := range wantAPI {
		provider, ok := byID[id]
		if !ok {
			t.Errorf("missing core provider binding %q", id)
			continue
		}
		list := provider.GetModels()
		if len(list) == 0 {
			t.Errorf("provider %q has no models", id)
		}
		for _, m := range list {
			if m.Provider != id {
				t.Errorf("provider %q lists model %q owned by %q", id, m.ID, m.Provider)
			}
			if m.Api != api {
				t.Errorf("provider %q model %q api = %q, want %q", id, m.ID, m.Api, api)
			}
		}
	}
}

// TestBuiltinModelsRegistersEveryProviderWithModels ports providers.test.ts's
// "builtinModels registers every builtin provider with models": the full
// ~35-provider set (this repo's embedded catalog totals 1042 models across
// 35 providers, comfortably above upstream's `toBeGreaterThan(500)`), every
// provider owns only its own models, and none is empty.
func TestBuiltinModelsRegistersEveryProviderWithModels(t *testing.T) {
	const wantProviderCount = 35

	all := Providers()
	if len(all) != wantProviderCount {
		t.Fatalf("Providers() returned %d providers, want %d", len(all), wantProviderCount)
	}

	models := Models(nil)
	providers := models.GetProviders()
	if len(providers) != wantProviderCount {
		t.Fatalf("Models(nil).GetProviders() = %d, want %d", len(providers), wantProviderCount)
	}

	total := 0
	for _, provider := range providers {
		list := models.GetModels(provider.ID())
		if len(list) == 0 {
			t.Errorf("provider %q has no models", provider.ID())
		}
		for _, m := range list {
			if m.Provider != provider.ID() {
				t.Errorf("provider %q lists model %q owned by %q", provider.ID(), m.ID, m.Provider)
			}
		}
		total += len(list)
	}
	if total <= 500 {
		t.Errorf("total models across all providers = %d, want > 500", total)
	}

	anthropicModel := models.GetModel("anthropic", "claude-haiku-4-5")
	if anthropicModel == nil || anthropicModel.Api != ai.ApiAnthropicMessages {
		t.Errorf("anthropic claude-haiku-4-5 = %+v, want api %q", anthropicModel, ai.ApiAnthropicMessages)
	}
}

// TestCloudflareWorkersAIAuthRequiresAccountConfigAndScopesEnv ports
// providers.test.ts's "requires Cloudflare Workers AI account config and
// returns scoped env".
func TestCloudflareWorkersAIAuthRequiresAccountConfigAndScopesEnv(t *testing.T) {
	missingAccount := newModels(map[string]string{"CLOUDFLARE_API_KEY": "cf-key"})
	missingAccount.SetProvider(CloudflareWorkersAIProvider())
	model := missingAccount.GetModels("cloudflare-workers-ai")[0]
	result, err := missingAccount.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result != nil {
		t.Fatalf("GetAuth (missing account) = %+v, want nil", result)
	}

	configured := newModels(map[string]string{
		"CLOUDFLARE_API_KEY":    "cf-key",
		"CLOUDFLARE_ACCOUNT_ID": "account-id",
	})
	configured.SetProvider(CloudflareWorkersAIProvider())
	result, err = configured.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result == nil {
		t.Fatal("GetAuth (configured) = nil")
	}
	if result.Auth.APIKey != "cf-key" {
		t.Errorf("Auth.APIKey = %q, want cf-key", result.Auth.APIKey)
	}
	if result.Auth.BaseURL != "https://api.cloudflare.com/client/v4/accounts/account-id/ai/v1" {
		t.Errorf("Auth.BaseURL = %q", result.Auth.BaseURL)
	}
	if result.Env["CLOUDFLARE_ACCOUNT_ID"] != "account-id" {
		t.Errorf("Env = %+v", result.Env)
	}
}

// TestCloudflareAIGatewayAuthRequiresGatewayConfigAndScopesEnvHeaders ports
// providers.test.ts's "requires Cloudflare AI Gateway account and gateway
// config and returns scoped env headers".
func TestCloudflareAIGatewayAuthRequiresGatewayConfigAndScopesEnvHeaders(t *testing.T) {
	missingGateway := newModels(map[string]string{
		"CLOUDFLARE_API_KEY":    "cf-key",
		"CLOUDFLARE_ACCOUNT_ID": "account-id",
	})
	missingGateway.SetProvider(CloudflareAIGatewayProvider())
	model := missingGateway.GetModels("cloudflare-ai-gateway")[0]
	result, err := missingGateway.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result != nil {
		t.Fatalf("GetAuth (missing gateway) = %+v, want nil", result)
	}

	configured := newModels(map[string]string{
		"CLOUDFLARE_API_KEY":    "cf-key",
		"CLOUDFLARE_ACCOUNT_ID": "account-id",
		"CLOUDFLARE_GATEWAY_ID": "gateway-id",
	})
	configured.SetProvider(CloudflareAIGatewayProvider())
	result, err = configured.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result == nil {
		t.Fatal("GetAuth (configured) = nil")
	}
	if got := result.Auth.Headers["cf-aig-authorization"]; got == nil || *got != "Bearer cf-key" {
		t.Errorf(`Auth.Headers["cf-aig-authorization"] = %v, want "Bearer cf-key"`, got)
	}
	if got, ok := result.Auth.Headers["Authorization"]; !ok || got != nil {
		t.Errorf(`Auth.Headers["Authorization"] = %v, want present nil`, got)
	}
	if got, ok := result.Auth.Headers["x-api-key"]; !ok || got != nil {
		t.Errorf(`Auth.Headers["x-api-key"] = %v, want present nil`, got)
	}
	if result.Auth.BaseURL != "https://gateway.ai.cloudflare.com/v1/account-id/gateway-id/anthropic" {
		t.Errorf("Auth.BaseURL = %q", result.Auth.BaseURL)
	}
	if result.Env["CLOUDFLARE_ACCOUNT_ID"] != "account-id" || result.Env["CLOUDFLARE_GATEWAY_ID"] != "gateway-id" {
		t.Errorf("Env = %+v", result.Env)
	}
}

func TestAnthropicAuthResolvesOAuthTokenPrecedence(t *testing.T) {
	models := newModels(map[string]string{
		"ANTHROPIC_API_KEY":     "key",
		"ANTHROPIC_OAUTH_TOKEN": "oauth-token",
	})
	models.SetProvider(AnthropicProvider())
	model := models.GetModel("anthropic", "claude-haiku-4-5")
	if model == nil {
		t.Fatal("anthropic model claude-haiku-4-5 not found in catalog")
	}

	result, err := models.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result == nil || result.Auth.APIKey != "oauth-token" || result.Source != "ANTHROPIC_OAUTH_TOKEN" {
		t.Fatalf("GetAuth = %+v", result)
	}
}

func TestBedrockAuthResolvesFromAmbientCredentialsWithoutAPIKey(t *testing.T) {
	models := newModels(map[string]string{"AWS_PROFILE": "dev"})
	models.SetProvider(AmazonBedrockProvider())
	list := models.GetModels("amazon-bedrock")
	if len(list) == 0 {
		t.Fatal("amazon-bedrock has no models")
	}
	model := list[0]

	result, err := models.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result == nil || !isEmptyModelAuth(result.Auth) || result.Source != "AWS_PROFILE" {
		t.Fatalf("GetAuth = %+v", result)
	}

	unconfigured := newModels(map[string]string{})
	unconfigured.SetProvider(AmazonBedrockProvider())
	result, err = unconfigured.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth (unconfigured): %v", err)
	}
	if result != nil {
		t.Fatalf("GetAuth (unconfigured) = %+v, want nil", result)
	}
}

func TestGoogleVertexAuthResolvesADCThenExplicitKeyOverride(t *testing.T) {
	const adc = "~/.config/gcloud/application_default_credentials.json"

	configured := newModels(map[string]string{
		"GOOGLE_CLOUD_PROJECT":  "proj",
		"GOOGLE_CLOUD_LOCATION": "us-central1",
	}, adc)
	configured.SetProvider(GoogleVertexProvider())
	model := configured.GetModels("google-vertex")[0]

	result, err := configured.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if result == nil || !isEmptyModelAuth(result.Auth) {
		t.Fatalf("GetAuth = %+v", result)
	}
	if result.Source != "gcloud application default credentials" {
		t.Errorf("Source = %q", result.Source)
	}

	partial := newModels(map[string]string{"GOOGLE_CLOUD_PROJECT": "proj"}, adc)
	partial.SetProvider(GoogleVertexProvider())
	result, err = partial.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth (partial ADC): %v", err)
	}
	if result != nil {
		t.Fatalf("GetAuth (partial ADC) = %+v, want nil", result)
	}

	keyed := newModels(map[string]string{"GOOGLE_CLOUD_API_KEY": "vertex-key"})
	keyed.SetProvider(GoogleVertexProvider())
	result, err = keyed.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth (explicit key): %v", err)
	}
	if result == nil || result.Auth.APIKey != "vertex-key" {
		t.Fatalf("GetAuth (explicit key) = %+v", result)
	}
}

func TestOpenAICodexProviderWiresRealCodexOAuth(t *testing.T) {
	provider := OpenAICodexProvider()
	if got := provider.Auth().OAuth; got != oauth.CodexOAuth {
		t.Fatalf("Auth().OAuth = %p, want the real oauth.CodexOAuth strategy %p", got, oauth.CodexOAuth)
	}
	if got := provider.Auth().OAuth.Name; got != "OpenAI (ChatGPT Plus/Pro)" {
		t.Errorf("OAuth.Name = %q, want %q", got, "OpenAI (ChatGPT Plus/Pro)")
	}
}

func TestAnthropicProviderWiresRealAnthropicOAuth(t *testing.T) {
	provider := AnthropicProvider()
	if got := provider.Auth().OAuth; got != oauth.AnthropicOAuth {
		t.Fatalf("Auth().OAuth = %p, want the real oauth.AnthropicOAuth strategy %p", got, oauth.AnthropicOAuth)
	}
	// The api-key env strategy (and its ANTHROPIC_OAUTH_TOKEN precedence) must
	// survive alongside the new OAuth binding.
	if provider.Auth().APIKey == nil {
		t.Error("Auth().APIKey = nil, want the env api-key strategy kept")
	}
}

// TestAnthropicStoredOAuthResolvesWithoutBlockingFallback is the regression
// test for the CRITICAL bug: before this fix Anthropic had no OAuth binding, so
// a stored *OAuthCredential fell through ResolveProviderAuth to nil, nil —
// resolving to no auth and (worse) blocking the ambient API-key fallback.
func TestAnthropicStoredOAuthResolvesWithoutBlockingFallback(t *testing.T) {
	ctx := context.Background()
	store := ai.NewInMemoryCredentialStore()
	// Expires far in the future so resolution derives auth via ToAuth without
	// attempting a (network) refresh.
	if _, err := store.Modify(ctx, "anthropic", func(ai.Credential) (ai.Credential, error) {
		return &ai.OAuthCredential{Access: "anthropic-access-token", Refresh: "r", Expires: 9e15}, nil
	}); err != nil {
		t.Fatalf("seed credential: %v", err)
	}

	result, err := ai.ResolveProviderAuth(ctx, "anthropic",
		AnthropicProvider().Auth(), nil, store, fakeAuthContext{}, nil)
	if err != nil {
		t.Fatalf("ResolveProviderAuth: %v", err)
	}
	if result == nil {
		t.Fatal("result = nil; a stored Anthropic OAuth credential must resolve, not fall through to nil")
	}
	if result.Source != "OAuth" {
		t.Errorf("Source = %q, want \"OAuth\"", result.Source)
	}
	if result.Auth.APIKey != "anthropic-access-token" {
		t.Errorf("Auth.APIKey = %q, want the OAuth access token", result.Auth.APIKey)
	}
}
