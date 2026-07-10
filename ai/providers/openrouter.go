package providers

// Ports: packages/ai/src/providers/openrouter.ts (binding). RefreshModels is
// native: no upstream provider file wires a `refreshModels` hook, but
// upstream's build-time catalog generator's fetchOpenRouterModels
// (packages/ai/scripts/generate-models.ts) documents the same
// https://openrouter.ai/api/v1/models response shape and transform (only
// tool-capable models, $/token pricing rescaled to $/million tokens); this
// reuses that shape as a runtime RefreshModels instead of a build-time
// script, per docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md.

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// openRouterModelsURL is a var so tests can point it at an httptest server.
var openRouterModelsURL = "https://openrouter.ai/api/v1/models"

const openRouterBaseURL = "https://openrouter.ai/api/v1"

type openRouterModelsResponse struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	ContextLength       int      `json:"context_length"`
	SupportedParameters []string `json:"supported_parameters"`
	Architecture        struct {
		Modality string `json:"modality"`
	} `json:"architecture"`
	TopProvider struct {
		MaxCompletionTokens int `json:"max_completion_tokens"`
	} `json:"top_provider"`
	Pricing struct {
		Prompt          string `json:"prompt"`
		Completion      string `json:"completion"`
		InputCacheRead  string `json:"input_cache_read"`
		InputCacheWrite string `json:"input_cache_write"`
	} `json:"pricing"`
}

func openRouterSupports(params []string, name string) bool {
	for _, p := range params {
		if p == name {
			return true
		}
	}
	return false
}

func roundCost(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func parseCost(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// refreshOpenRouterModels fetches OpenRouter's live model list and decodes
// it into catalog model shapes, keeping only tool-capable models (matching
// upstream's fetchOpenRouterModels).
func refreshOpenRouterModels(ctx context.Context) ([]*ai.Model, error) {
	var resp openRouterModelsResponse
	if err := fetchJSON(ctx, openRouterModelsURL, nil, &resp); err != nil {
		return nil, err
	}

	models := make([]*ai.Model, 0, len(resp.Data))
	for _, m := range resp.Data {
		if !openRouterSupports(m.SupportedParameters, "tools") {
			continue
		}

		input := []ai.Modality{ai.ModalityText}
		if strings.Contains(m.Architecture.Modality, "image") {
			input = append(input, ai.ModalityImage)
		}

		contextWindow := m.ContextLength
		if contextWindow == 0 {
			contextWindow = 4096
		}
		maxTokens := m.TopProvider.MaxCompletionTokens
		if maxTokens == 0 {
			maxTokens = 4096
		}

		models = append(models, &ai.Model{
			ID:        m.ID,
			Name:      m.Name,
			Api:       ai.ApiOpenAICompletions,
			Provider:  "openrouter",
			BaseURL:   openRouterBaseURL,
			Reasoning: openRouterSupports(m.SupportedParameters, "reasoning"),
			Input:     input,
			Cost: ai.ModelCost{
				Input:      roundCost(parseCost(m.Pricing.Prompt) * 1_000_000),
				Output:     roundCost(parseCost(m.Pricing.Completion) * 1_000_000),
				CacheRead:  roundCost(parseCost(m.Pricing.InputCacheRead) * 1_000_000),
				CacheWrite: roundCost(parseCost(m.Pricing.InputCacheWrite) * 1_000_000),
			},
			ContextWindow: contextWindow,
			MaxTokens:     maxTokens,
		})
	}
	return models, nil
}

// OpenRouterProvider builds the OpenRouter provider binding, over the
// openai-completions wire adapter. It is a dynamic provider: RefreshModels
// re-fetches the live model list from OpenRouter's /models endpoint.
func OpenRouterProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:            "openrouter",
		Name:          "OpenRouter",
		BaseURL:       openRouterBaseURL,
		Auth:          ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("OpenRouter API key", []string{"OPENROUTER_API_KEY"})},
		Models:        catalog.BuiltinModels("openrouter"),
		RefreshModels: refreshOpenRouterModels,
		Api:           ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
