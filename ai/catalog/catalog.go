// Package catalog embeds kern-proxy's built-in model catalog: the JSON data
// under data/, exported from upstream's models.generated.ts and
// image-models.generated.ts by tools/export-catalog (see that script's doc
// comment and docs/PORTING.md's upstream-sync procedure). data/images is
// embedded for byte-parity with upstream's catalog export but has no loader
// yet — ai/images (phase 13) adds one when it needs ImagesModel.
package catalog

// Ports: packages/ai/src/providers/all.ts (getBuiltinModel, getBuiltinModels,
// getBuiltinProviders — the static-catalog-read portion only; building actual
// runtime Providers from these reads is ai/providers, epic 11 issue 02),
// packages/ai/src/models.generated.ts, packages/ai/src/image-models.generated.ts,
// packages/ai/src/providers/*.models.ts (data, serialized to JSON by
// tools/export-catalog).

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/julienlegoux/kern-proxy/ai"
)

//go:embed data
var dataFS embed.FS

var loadOnce sync.Once
var loadErr error
var byProvider map[string][]*ai.Model
var providerIDs []string

// load parses the embedded catalog exactly once. A malformed embedded file
// is a build-time defect (the data is generated and committed alongside this
// package), so it panics rather than threading an error through every
// public function here.
func load() {
	loadOnce.Do(func() {
		entries, err := dataFS.ReadDir("data/models")
		if err != nil {
			loadErr = fmt.Errorf("catalog: reading embedded data/models: %w", err)
			return
		}
		byProvider = make(map[string][]*ai.Model, len(entries))
		providerIDs = make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			provider := strings.TrimSuffix(entry.Name(), ".json")
			data, err := dataFS.ReadFile("data/models/" + entry.Name())
			if err != nil {
				loadErr = fmt.Errorf("catalog: reading %s: %w", entry.Name(), err)
				return
			}
			var models []*ai.Model
			if err := json.Unmarshal(data, &models); err != nil {
				loadErr = fmt.Errorf("catalog: decoding %s: %w", entry.Name(), err)
				return
			}
			byProvider[provider] = models
			providerIDs = append(providerIDs, provider)
		}
		sort.Strings(providerIDs)
	})
	if loadErr != nil {
		panic(loadErr)
	}
}

// Providers returns every built-in provider id present in the catalog,
// sorted lexicographically.
func Providers() []string {
	load()
	out := make([]string, len(providerIDs))
	copy(out, providerIDs)
	return out
}

// BuiltinModels returns the catalog's models for provider, in the order
// upstream's generated catalog defines them, or nil if provider is unknown.
func BuiltinModels(provider string) []*ai.Model {
	load()
	return byProvider[provider]
}

// BuiltinModel looks up one model by provider and model id, or nil if
// either is unknown.
func BuiltinModel(provider, modelID string) *ai.Model {
	load()
	for _, m := range byProvider[provider] {
		if m.ID == modelID {
			return m
		}
	}
	return nil
}
