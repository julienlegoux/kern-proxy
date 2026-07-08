package providers

// Ports: packages/ai/test/providers.test.ts (the `describe("builtin
// providers", ...)` cases covering the core, first-party-adapter bindings
// this issue wires: anthropic OAuth-token env precedence, bedrock ambient
// AWS credentials, and vertex ADC resolution / explicit-key override. The
// ~25 remaining compat-vendor cases (Cloudflare, etc.) land with Issue 03's
// bindings. The `describe("createProvider", ...)` mechanics are already
// ported onto ai.CreateProvider directly, see ai/provider_test.go.

import (
	"context"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
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

func TestProvidersRegistersEveryCoreBindingWithModels(t *testing.T) {
	all := Providers()
	if len(all) != 8 {
		t.Fatalf("Providers() returned %d providers, want 8", len(all))
	}

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

	models := Models(nil)
	if got := len(models.GetProviders()); got != 8 {
		t.Fatalf("Models(nil).GetProviders() = %d, want 8", got)
	}

	seen := map[string]bool{}
	for _, provider := range all {
		seen[provider.ID()] = true

		wantAPI, ok := wantAPI[provider.ID()]
		if !ok {
			t.Fatalf("unexpected provider id %q", provider.ID())
		}
		list := provider.GetModels()
		if len(list) == 0 {
			t.Fatalf("provider %q has no models", provider.ID())
		}
		for _, m := range list {
			if m.Provider != provider.ID() {
				t.Errorf("provider %q lists model %q owned by %q", provider.ID(), m.ID, m.Provider)
			}
			if m.Api != wantAPI {
				t.Errorf("provider %q model %q api = %q, want %q", provider.ID(), m.ID, m.Api, wantAPI)
			}
		}
	}
	for id := range wantAPI {
		if !seen[id] {
			t.Errorf("missing core provider binding %q", id)
		}
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

func TestOpenAICodexProviderAdvertisesOAuthPendingEpic12(t *testing.T) {
	provider := OpenAICodexProvider()
	oauth := provider.Auth().OAuth
	if oauth == nil {
		t.Fatal("Auth().OAuth is nil, want a pending-epic-12 stub")
	}
	if oauth.Name != "OpenAI (ChatGPT Plus/Pro)" {
		t.Errorf("oauth.Name = %q", oauth.Name)
	}
	if _, err := oauth.Login(context.Background(), ai.AuthLoginCallbacks{}); err == nil {
		t.Error("Login() = nil error, want a not-implemented error")
	}
	if _, err := oauth.Refresh(context.Background(), &ai.OAuthCredential{}); err == nil {
		t.Error("Refresh() = nil error, want a not-implemented error")
	}
	if _, err := oauth.ToAuth(context.Background(), &ai.OAuthCredential{}); err == nil {
		t.Error("ToAuth() = nil error, want a not-implemented error")
	}
}
