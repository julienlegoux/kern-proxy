package providers

// Ports: packages/ai/src/providers/nvidia.ts (binding). RefreshModels is
// native: no upstream provider file wires a `refreshModels` hook, but
// upstream's build-time catalog generator's fetchNvidiaNimModelIds
// (packages/ai/scripts/generate-models.ts) documents the same
// https://integrate.api.nvidia.com/v1/models response shape, used there
// only to confirm/normalize static catalog entries' ids (the NIM /models
// listing carries no cost/context-window metadata to build a fresh Model
// from). This mirrors that: the live id list narrows the embedded catalog
// down to currently-available models rather than fabricating new ones, per
// docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md.

import (
	"context"
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openaicompletions"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// nvidiaModelsURL is a var so tests can point it at an httptest server.
var nvidiaModelsURL = "https://integrate.api.nvidia.com/v1/models"

type nvidiaModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// normalizeNvidiaModelID mirrors generate-models.ts's
// normalizeNvidiaModelId: NIM ids are case- and separator-inconsistent
// across catalog snapshots and the live listing.
func normalizeNvidiaModelID(id string) string {
	return strings.ReplaceAll(strings.ToLower(id), "_", ".")
}

// refreshNvidiaModels fetches NVIDIA NIM's live model id list and narrows
// the embedded catalog down to models still present live.
func refreshNvidiaModels(ctx context.Context) ([]*ai.Model, error) {
	var resp nvidiaModelsResponse
	if err := fetchJSON(ctx, nvidiaModelsURL, nil, &resp); err != nil {
		return nil, err
	}

	live := make(map[string]bool, len(resp.Data)*2)
	for _, m := range resp.Data {
		live[m.ID] = true
		live[normalizeNvidiaModelID(m.ID)] = true
	}

	catalogModels := catalog.BuiltinModels("nvidia")
	models := make([]*ai.Model, 0, len(catalogModels))
	for _, m := range catalogModels {
		if live[m.ID] || live[normalizeNvidiaModelID(m.ID)] {
			models = append(models, m)
		}
	}
	return models, nil
}

// NvidiaProvider builds the NVIDIA provider binding, over the
// openai-completions wire adapter. It is a dynamic provider: RefreshModels
// re-fetches the live model id list from NVIDIA NIM's /models endpoint.
func NvidiaProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:            "nvidia",
		Name:          "NVIDIA",
		BaseURL:       "https://integrate.api.nvidia.com/v1",
		Auth:          ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("NVIDIA API key", []string{"NVIDIA_API_KEY"})},
		Models:        catalog.BuiltinModels("nvidia"),
		RefreshModels: refreshNvidiaModels,
		Api:           ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
	})
}
