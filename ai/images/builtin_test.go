package images

// Ports: packages/ai/test/images-models.test.ts's "builtinImagesModels
// registers the openrouter provider with its catalog" case.

import (
	"context"
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

func TestBuiltinModels_RegistersOpenRouterProviderWithItsCatalog(t *testing.T) {
	models := BuiltinModels(&ai.CreateModelsOptions{AuthContext: fakeAuthContext{"OPENROUTER_API_KEY": "or-key"}})

	providers := models.GetProviders()
	if len(providers) != 1 || providers[0].ID() != "openrouter" {
		t.Fatalf("GetProviders() = %v, want exactly [openrouter]", providers)
	}

	list := models.GetModels("openrouter")
	if len(list) == 0 {
		t.Fatal(`GetModels("openrouter") is empty`)
	}
	for _, m := range list {
		if m.Api != "openrouter-images" {
			t.Errorf("model %s has api %q, want \"openrouter-images\"", m.ID, m.Api)
		}
	}

	auth, err := models.GetAuth(context.Background(), list[0])
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth == nil || auth.Auth.APIKey != "or-key" {
		t.Fatalf("GetAuth() = %v, want APIKey or-key", auth)
	}
}

func TestBuiltinProviders_ReturnsOpenRouter(t *testing.T) {
	providers := BuiltinProviders()
	if len(providers) != 1 || providers[0].ID() != "openrouter" {
		t.Fatalf("BuiltinProviders() = %v, want exactly [openrouter]", providers)
	}
}
