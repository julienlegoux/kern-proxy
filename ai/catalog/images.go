package catalog

// Ports: packages/ai/src/image-models.generated.ts (data access). This file
// adds the loader this package's doc comment (catalog.go) anticipated: raw
// embedded bytes only — typed decoding into ai/images.Model happens in that
// package (ai/images/catalog.go) so this package never needs to import
// ai/images (which would cycle back, since ai/images/builtin.go imports this
// package for the openrouter provider's initial model list).

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var imagesLoadOnce sync.Once
var imagesLoadErr error
var imagesData map[string][]byte
var imagesProviderIDs []string

func loadImages() {
	imagesLoadOnce.Do(func() {
		entries, err := dataFS.ReadDir("data/images")
		if err != nil {
			imagesLoadErr = fmt.Errorf("catalog: reading embedded data/images: %w", err)
			return
		}
		imagesData = make(map[string][]byte, len(entries))
		imagesProviderIDs = make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			provider := strings.TrimSuffix(entry.Name(), ".json")
			data, err := dataFS.ReadFile("data/images/" + entry.Name())
			if err != nil {
				imagesLoadErr = fmt.Errorf("catalog: reading %s: %w", entry.Name(), err)
				return
			}
			imagesData[provider] = data
			imagesProviderIDs = append(imagesProviderIDs, provider)
		}
		sort.Strings(imagesProviderIDs)
	})
	if imagesLoadErr != nil {
		panic(imagesLoadErr)
	}
}

// ImagesProviders returns every built-in image-generation provider id
// present in the embedded catalog (data/images/*.json), sorted
// lexicographically.
func ImagesProviders() []string {
	loadImages()
	out := make([]string, len(imagesProviderIDs))
	copy(out, imagesProviderIDs)
	return out
}

// ImagesData returns the raw embedded JSON catalog for one image-generation
// provider, or (nil, false) if unknown.
func ImagesData(provider string) ([]byte, bool) {
	loadImages()
	data, ok := imagesData[provider]
	return data, ok
}
