package providers

// Tests the native RefreshModels behavior described in
// vercel_ai_gateway.go's doc comment: fetch the gateway's live model list
// and decode it into catalog model shapes, keeping only tool-use-tagged
// models.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienlegoux/kern-link/ai"
)

func TestVercelAIGatewayProviderIsDynamic(t *testing.T) {
	provider := VercelAIGatewayProvider()
	if !provider.CanRefreshModels() {
		t.Fatal("CanRefreshModels() = false, want true")
	}
}

func TestRefreshVercelAIGatewayModelsKeepsOnlyToolUseTaggedModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "alibaba/qwen-3-14b",
					"name": "Qwen3-14B",
					"tags": ["tool-use", "reasoning", "vision"],
					"context_window": 40960,
					"max_tokens": 16384,
					"pricing": {"input": "0.00000012", "output": "0.00000024", "input_cache_read": 0.00000001, "input_cache_write": 0}
				},
				{
					"id": "no/tool-use",
					"name": "No Tool Use",
					"tags": ["reasoning"],
					"context_window": 4096,
					"max_tokens": 4096,
					"pricing": {"input": "0", "output": "0"}
				}
			]
		}`))
	}))
	defer srv.Close()

	orig := vercelAIGatewayModelsURL
	vercelAIGatewayModelsURL = srv.URL
	defer func() { vercelAIGatewayModelsURL = orig }()

	models, err := refreshVercelAIGatewayModels(context.Background())
	if err != nil {
		t.Fatalf("refreshVercelAIGatewayModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}

	m := models[0]
	if m.ID != "alibaba/qwen-3-14b" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Provider != "vercel-ai-gateway" {
		t.Errorf("Provider = %q", m.Provider)
	}
	if m.Api != ai.ApiAnthropicMessages {
		t.Errorf("Api = %q", m.Api)
	}
	if m.BaseURL != "https://ai-gateway.vercel.sh" {
		t.Errorf("BaseURL = %q", m.BaseURL)
	}
	if !m.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	if len(m.Input) != 2 || m.Input[1] != ai.ModalityImage {
		t.Errorf("Input = %v", m.Input)
	}
	if m.Cost.Input != 0.12 || m.Cost.Output != 0.24 || m.Cost.CacheRead != 0.01 {
		t.Errorf("Cost = %+v", m.Cost)
	}
	if m.ContextWindow != 40960 || m.MaxTokens != 16384 {
		t.Errorf("ContextWindow/MaxTokens = %d/%d", m.ContextWindow, m.MaxTokens)
	}
}

func TestRefreshVercelAIGatewayModelsPropagatesFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := vercelAIGatewayModelsURL
	vercelAIGatewayModelsURL = srv.URL
	defer func() { vercelAIGatewayModelsURL = orig }()

	if _, err := refreshVercelAIGatewayModels(context.Background()); err == nil {
		t.Fatal("refreshVercelAIGatewayModels error = nil, want non-nil")
	}
}
