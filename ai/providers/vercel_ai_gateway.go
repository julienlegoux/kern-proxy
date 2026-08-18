package providers

// Ports: packages/ai/src/providers/vercel-ai-gateway.ts (binding).
// RefreshModels is native: no upstream provider file wires a
// `refreshModels` hook, but upstream's build-time catalog generator's
// fetchAiGatewayModels (packages/ai/scripts/generate-models.ts) documents
// the same https://ai-gateway.vercel.sh/v1/models response shape and
// transform (only tool-use-tagged models); this reuses that shape as a
// runtime RefreshModels instead of a build-time script, per
// docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md.

import (
	"context"
	"strconv"
	"strings"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// vercelAIGatewayModelsURL is a var so tests can point it at an httptest
// server.
var vercelAIGatewayModelsURL = "https://ai-gateway.vercel.sh/v1/models"

const vercelAIGatewayBaseURL = "https://ai-gateway.vercel.sh"

type vercelAIGatewayModelsResponse struct {
	Data []vercelAIGatewayModel `json:"data"`
}

type vercelAIGatewayModel struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Tags          []string `json:"tags"`
	ContextWindow int      `json:"context_window"`
	MaxTokens     int      `json:"max_tokens"`
	Pricing       struct {
		Input           vercelAIGatewayNumber `json:"input"`
		Output          vercelAIGatewayNumber `json:"output"`
		InputCacheRead  vercelAIGatewayNumber `json:"input_cache_read"`
		InputCacheWrite vercelAIGatewayNumber `json:"input_cache_write"`
	} `json:"pricing"`
}

// vercelAIGatewayNumber decodes a JSON field that upstream's response may
// send as either a string or a number, mirroring generate-models.ts's
// toNumber helper.
type vercelAIGatewayNumber float64

func (n *vercelAIGatewayNumber) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = vercelAIGatewayNumber(v)
	return nil
}

func vercelAIGatewayHasTag(tags []string, name string) bool {
	for _, t := range tags {
		if t == name {
			return true
		}
	}
	return false
}

// refreshVercelAIGatewayModels fetches Vercel AI Gateway's live model list
// and decodes it into catalog model shapes, keeping only tool-use-tagged
// models (matching upstream's fetchAiGatewayModels).
func refreshVercelAIGatewayModels(ctx context.Context) ([]*ai.Model, error) {
	var resp vercelAIGatewayModelsResponse
	if err := fetchJSON(ctx, vercelAIGatewayModelsURL, nil, &resp); err != nil {
		return nil, err
	}

	models := make([]*ai.Model, 0, len(resp.Data))
	for _, m := range resp.Data {
		if !vercelAIGatewayHasTag(m.Tags, "tool-use") {
			continue
		}

		input := []ai.Modality{ai.ModalityText}
		if vercelAIGatewayHasTag(m.Tags, "vision") {
			input = append(input, ai.ModalityImage)
		}

		name := m.Name
		if name == "" {
			name = m.ID
		}
		contextWindow := m.ContextWindow
		if contextWindow == 0 {
			contextWindow = 4096
		}
		maxTokens := m.MaxTokens
		if maxTokens == 0 {
			maxTokens = 4096
		}

		models = append(models, &ai.Model{
			ID:        m.ID,
			Name:      name,
			Api:       ai.ApiAnthropicMessages,
			Provider:  "vercel-ai-gateway",
			BaseURL:   vercelAIGatewayBaseURL,
			Reasoning: vercelAIGatewayHasTag(m.Tags, "reasoning"),
			Input:     input,
			Cost: ai.ModelCost{
				Input:      roundCost(float64(m.Pricing.Input) * 1_000_000),
				Output:     roundCost(float64(m.Pricing.Output) * 1_000_000),
				CacheRead:  roundCost(float64(m.Pricing.InputCacheRead) * 1_000_000),
				CacheWrite: roundCost(float64(m.Pricing.InputCacheWrite) * 1_000_000),
			},
			ContextWindow: contextWindow,
			MaxTokens:     maxTokens,
		})
	}
	return models, nil
}

// VercelAIGatewayProvider builds the Vercel AI Gateway provider binding,
// over the anthropic-messages wire adapter. It is a dynamic provider:
// RefreshModels re-fetches the live model list from the gateway's /models
// endpoint.
func VercelAIGatewayProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "vercel-ai-gateway",
		Name:    "Vercel AI Gateway",
		BaseURL: vercelAIGatewayBaseURL,
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("Vercel AI Gateway API key", []string{"AI_GATEWAY_API_KEY"}),
		},
		Models:        catalog.BuiltinModels("vercel-ai-gateway"),
		RefreshModels: refreshVercelAIGatewayModels,
		Api:           ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
	})
}
