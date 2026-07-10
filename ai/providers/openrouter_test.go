package providers

// Tests the native RefreshModels behavior described in openrouter.go's doc
// comment: fetch OpenRouter's live model list and decode it into catalog
// model shapes, keeping only tool-capable models.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienlegoux/kern-link/ai"
)

func TestOpenRouterProviderIsDynamic(t *testing.T) {
	provider := OpenRouterProvider()
	if !provider.CanRefreshModels() {
		t.Fatal("CanRefreshModels() = false, want true")
	}
}

func TestRefreshOpenRouterModelsKeepsOnlyToolCapableModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "anthropic/claude-x",
					"name": "Claude X",
					"context_length": 200000,
					"supported_parameters": ["tools", "reasoning"],
					"architecture": {"modality": "text+image->text"},
					"top_provider": {"max_completion_tokens": 8192},
					"pricing": {"prompt": "0.000003", "completion": "0.000015", "input_cache_read": "0.0000003", "input_cache_write": "0"}
				},
				{
					"id": "some/no-tools-model",
					"name": "No Tools",
					"context_length": 4096,
					"supported_parameters": ["reasoning"],
					"architecture": {"modality": "text->text"},
					"pricing": {"prompt": "0", "completion": "0"}
				}
			]
		}`))
	}))
	defer srv.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = srv.URL + "/api/v1/models"
	defer func() { openRouterModelsURL = orig }()

	models, err := refreshOpenRouterModels(context.Background())
	if err != nil {
		t.Fatalf("refreshOpenRouterModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}

	m := models[0]
	if m.ID != "anthropic/claude-x" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Provider != "openrouter" {
		t.Errorf("Provider = %q", m.Provider)
	}
	if m.Api != ai.ApiOpenAICompletions {
		t.Errorf("Api = %q", m.Api)
	}
	if m.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("BaseURL = %q", m.BaseURL)
	}
	if !m.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	if len(m.Input) != 2 || m.Input[0] != ai.ModalityText || m.Input[1] != ai.ModalityImage {
		t.Errorf("Input = %v", m.Input)
	}
	if m.Cost.Input != 3 || m.Cost.Output != 15 || m.Cost.CacheRead != 0.3 {
		t.Errorf("Cost = %+v", m.Cost)
	}
	if m.ContextWindow != 200000 || m.MaxTokens != 8192 {
		t.Errorf("ContextWindow/MaxTokens = %d/%d", m.ContextWindow, m.MaxTokens)
	}
}

func TestRefreshOpenRouterModelsPropagatesFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = srv.URL
	defer func() { openRouterModelsURL = orig }()

	if _, err := refreshOpenRouterModels(context.Background()); err == nil {
		t.Fatal("refreshOpenRouterModels error = nil, want non-nil")
	}
}
