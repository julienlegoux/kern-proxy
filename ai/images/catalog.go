package images

// Ports: packages/ai/src/image-models.ts (getImageModel, getImageModels;
// getImageProviders is ai/catalog's ImagesProviders — that half needs no
// typed Model, so it stays in ai/catalog to avoid this package importing
// itself indirectly through a re-export).

import (
	"encoding/json"
	"fmt"

	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

// CatalogModels decodes the embedded image-model catalog for provider (e.g.
// "openrouter"), or nil if the provider has no embedded catalog.
func CatalogModels(provider string) []*Model {
	data, ok := catalog.ImagesData(provider)
	if !ok {
		return nil
	}
	var models []*Model
	if err := json.Unmarshal(data, &models); err != nil {
		panic(fmt.Errorf("images: decoding embedded catalog for %s: %w", provider, err))
	}
	return models
}

// CatalogModel looks up one embedded catalog model by provider and model id,
// or nil if either is unknown.
func CatalogModel(provider, id string) *Model {
	for _, m := range CatalogModels(provider) {
		if m.ID == id {
			return m
		}
	}
	return nil
}
