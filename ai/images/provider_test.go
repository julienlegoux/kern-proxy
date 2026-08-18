package images

// Ports: packages/ai/test/images-models.test.ts (excluding the
// "builtinImagesModels registers the openrouter provider with its catalog"
// case, ported separately in builtin_test.go once BuiltinModels exists).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

type fakeAuthContext map[string]string

func (f fakeAuthContext) Env(name string) string    { return f[name] }
func (fakeAuthContext) FileExists(path string) bool { return false }

func testImageModel(provider, id string) *Model {
	return &Model{
		ID:       id,
		Name:     id,
		Api:      "test-images",
		Provider: provider,
		BaseURL:  "https://example.test/v1",
		Input:    []ai.Modality{ai.ModalityText},
		Output:   []ai.Modality{ai.ModalityImage},
	}
}

func okImagesResult(model *Model) *AssistantImages {
	return &AssistantImages{
		Api:        model.Api,
		Provider:   model.Provider,
		Model:      model.ID,
		Output:     []ai.UserContentPart{ai.ImageContent{Data: "aGk=", MimeType: "image/png"}},
		StopReason: StopReasonStop,
	}
}

type generateCall struct {
	model *Model
	opts  *Options
}

func testProvider(id string, models []*Model, envVar string, calls *[]generateCall) Provider {
	var apiKeyAuth *ai.APIKeyAuth
	if envVar != "" {
		apiKeyAuth = &ai.APIKeyAuth{
			Name: "Test key",
			Resolve: func(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
				key := input.Ctx.Env(envVar)
				if key == "" {
					return nil, nil
				}
				return &ai.AuthResult{Auth: ai.ModelAuth{APIKey: key}, Source: envVar}, nil
			},
		}
	} else {
		apiKeyAuth = &ai.APIKeyAuth{
			Name: "Test key",
			Resolve: func(context.Context, ai.APIKeyResolveInput) (*ai.AuthResult, error) {
				return &ai.AuthResult{}, nil
			},
		}
	}
	if models == nil {
		models = []*Model{testImageModel(id, "model-a")}
	}
	return CreateProvider(CreateProviderOptions{
		ID:     id,
		Auth:   ai.ProviderAuth{APIKey: apiKeyAuth},
		Models: models,
		API: func(_ context.Context, model *Model, _ Context, opts *Options) *AssistantImages {
			if calls != nil {
				*calls = append(*calls, generateCall{model: model, opts: opts})
			}
			return okImagesResult(model)
		},
	})
}

var testCtx = Context{Input: []ai.UserContentPart{ai.TextContent{Text: "a red circle"}}}

func TestImagesModels_RegistersProvidersAndReadsModelsSynchronously(t *testing.T) {
	models := CreateModels(nil)
	models.SetProvider(testProvider("p1", []*Model{testImageModel("p1", "m1"), testImageModel("p1", "m2")}, "", nil))
	models.SetProvider(testProvider("p2", []*Model{testImageModel("p2", "m3")}, "", nil))

	gotProviders := models.GetProviders()
	if len(gotProviders) != 2 || gotProviders[0].ID() != "p1" || gotProviders[1].ID() != "p2" {
		t.Fatalf("GetProviders() = %v, want [p1 p2]", gotProviders)
	}

	all := models.GetModels("")
	if len(all) != 3 {
		t.Fatalf("GetModels(\"\") = %d models, want 3", len(all))
	}

	p1Models := models.GetModels("p1")
	if len(p1Models) != 2 {
		t.Fatalf("GetModels(\"p1\") = %d models, want 2", len(p1Models))
	}

	if m := models.GetModel("p2", "m3"); m == nil || m.ID != "m3" {
		t.Fatalf("GetModel(\"p2\", \"m3\") = %v, want id m3", m)
	}
	if m := models.GetModel("p2", "missing"); m != nil {
		t.Fatalf("GetModel(\"p2\", \"missing\") = %v, want nil", m)
	}

	models.DeleteProvider("p1")
	if models.GetProvider("p1") != nil {
		t.Fatal("GetProvider(\"p1\") after delete should be nil")
	}
}

func TestImagesModels_ResolvesAuthAndMergesIntoRequests_ExplicitOptionsWin(t *testing.T) {
	var calls []generateCall
	models := CreateModels(&ai.CreateModelsOptions{AuthContext: fakeAuthContext{"TEST_KEY": "env-key"}})
	models.SetProvider(testProvider("p1", nil, "TEST_KEY", &calls))
	model := models.GetModel("p1", "model-a")

	auth, err := models.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth == nil || auth.Auth.APIKey != "env-key" {
		t.Fatalf("GetAuth() = %v, want APIKey env-key", auth)
	}

	result := models.GenerateImages(context.Background(), model, testCtx, nil)
	if result.StopReason != StopReasonStop {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonStop)
	}
	if calls[0].opts.APIKey != "env-key" {
		t.Fatalf("calls[0].opts.APIKey = %q, want env-key", calls[0].opts.APIKey)
	}

	models.GenerateImages(context.Background(), model, testCtx, &Options{APIKey: "explicit"})
	if calls[1].opts.APIKey != "explicit" {
		t.Fatalf("calls[1].opts.APIKey = %q, want explicit", calls[1].opts.APIKey)
	}
}

func TestImagesModels_MergesProviderResolvedEnvIntoImageOptions(t *testing.T) {
	var calls []generateCall
	models := CreateModels(nil)
	models.SetProvider(CreateProvider(CreateProviderOptions{
		ID: "p1",
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{
			Name: "Test key",
			Resolve: func(context.Context, ai.APIKeyResolveInput) (*ai.AuthResult, error) {
				return &ai.AuthResult{
					Auth: ai.ModelAuth{APIKey: "provider-key"},
					Env:  ai.ProviderEnv{"PROVIDER_ONLY": "provider", "SHARED": "provider"},
				}, nil
			},
		}},
		Models: []*Model{testImageModel("p1", "model-a")},
		API: func(_ context.Context, model *Model, _ Context, opts *Options) *AssistantImages {
			calls = append(calls, generateCall{model: model, opts: opts})
			return okImagesResult(model)
		},
	}))
	model := models.GetModel("p1", "model-a")

	models.GenerateImages(context.Background(), model, testCtx, &Options{
		APIKey: "request-key",
		Env:    ai.ProviderEnv{"REQUEST_ONLY": "request", "SHARED": "request"},
	})

	if calls[0].opts.APIKey != "request-key" {
		t.Fatalf("APIKey = %q, want request-key", calls[0].opts.APIKey)
	}
	wantEnv := ai.ProviderEnv{"PROVIDER_ONLY": "provider", "REQUEST_ONLY": "request", "SHARED": "request"}
	gotEnv := calls[0].opts.Env
	if len(gotEnv) != len(wantEnv) {
		t.Fatalf("Env = %v, want %v", gotEnv, wantEnv)
	}
	for k, v := range wantEnv {
		if gotEnv[k] != v {
			t.Fatalf("Env[%q] = %q, want %q", k, gotEnv[k], v)
		}
	}
}

func TestImagesModels_UnknownProviderAndUnconfiguredAuth(t *testing.T) {
	models := CreateModels(&ai.CreateModelsOptions{AuthContext: fakeAuthContext{}})
	ghost := models.GenerateImages(context.Background(), testImageModel("ghost", "m"), testCtx, nil)
	if ghost.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", ghost.StopReason, StopReasonError)
	}
	if !strings.Contains(ghost.ErrorMessage, "Unknown provider: ghost") {
		t.Fatalf("ErrorMessage = %q, want it to mention Unknown provider: ghost", ghost.ErrorMessage)
	}

	var calls []generateCall
	models.SetProvider(testProvider("p1", nil, "MISSING", &calls))
	model := models.GetModel("p1", "model-a")

	auth, err := models.GetAuth(context.Background(), model)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth != nil {
		t.Fatalf("GetAuth() = %v, want nil (unconfigured)", auth)
	}

	models.GenerateImages(context.Background(), model, testCtx, nil)
	if calls[0].opts != nil && calls[0].opts.APIKey != "" {
		t.Fatalf("calls[0].opts.APIKey = %q, want empty", calls[0].opts.APIKey)
	}
}

func TestImagesModels_SupportsDynamicProvidersViaRefreshWithInFlightDedupe(t *testing.T) {
	fetches := 0
	provider := CreateProvider(CreateProviderOptions{
		ID: "dyn",
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{Name: "Test", Resolve: func(context.Context, ai.APIKeyResolveInput) (*ai.AuthResult, error) {
			return &ai.AuthResult{}, nil
		}}},
		Models: nil,
		RefreshModels: func(context.Context) ([]*Model, error) {
			fetches++
			time.Sleep(5 * time.Millisecond)
			return []*Model{testImageModel("dyn", "listed")}, nil
		},
		API: func(_ context.Context, model *Model, _ Context, _ *Options) *AssistantImages {
			return okImagesResult(model)
		},
	})
	models := CreateModels(nil)
	models.SetProvider(provider)

	if got := models.GetModels("dyn"); len(got) != 0 {
		t.Fatalf("GetModels(\"dyn\") before refresh = %v, want empty", got)
	}

	done := make(chan error, 2)
	go func() { done <- models.Refresh(context.Background(), "dyn") }()
	go func() { done <- models.Refresh(context.Background(), "dyn") }()
	<-done
	<-done

	if fetches != 1 {
		t.Fatalf("fetches = %d, want 1 (in-flight dedupe)", fetches)
	}
	if m := models.GetModel("dyn", "listed"); m == nil {
		t.Fatal("GetModel(\"dyn\", \"listed\") = nil after refresh")
	}

	models.SetProvider(CreateProvider(CreateProviderOptions{
		ID: "flaky",
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{Name: "Test", Resolve: func(context.Context, ai.APIKeyResolveInput) (*ai.AuthResult, error) {
			return &ai.AuthResult{}, nil
		}}},
		Models: nil,
		RefreshModels: func(context.Context) ([]*Model, error) {
			return nil, errors.New("fetch failed")
		},
		API: func(_ context.Context, model *Model, _ Context, _ *Options) *AssistantImages {
			return okImagesResult(model)
		},
	}))
	err := models.Refresh(context.Background(), "flaky")
	var modelsErr *ai.ModelsError
	if err == nil {
		t.Fatal("Refresh(\"flaky\") = nil error, want a ModelsError")
	}
	if !errors.As(err, &modelsErr) || modelsErr.Code() != "model_source" {
		t.Fatalf("Refresh(\"flaky\") error = %v, want ModelsError code model_source", err)
	}

	if err := models.Refresh(context.Background(), ""); err != nil {
		t.Fatalf("Refresh(\"\") = %v, want nil (best-effort)", err)
	}
}
