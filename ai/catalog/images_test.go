package catalog

// Ports: packages/ai/src/image-models.generated.ts (data access only — typed
// decode into ai/images.Model lives in that package, avoiding an
// ai/catalog <-> ai/images import cycle; see ai/images/catalog.go).

import "testing"

func TestImagesProviders_ReturnsSortedNonEmptyList(t *testing.T) {
	providers := ImagesProviders()
	if len(providers) == 0 {
		t.Fatal("ImagesProviders() is empty, want at least one provider")
	}
	found := false
	for _, p := range providers {
		if p == "openrouter" {
			found = true
		}
	}
	if !found {
		t.Errorf("ImagesProviders() = %v, want it to include \"openrouter\"", providers)
	}
}

func TestImagesData_KnownProvider(t *testing.T) {
	data, ok := ImagesData("openrouter")
	if !ok {
		t.Fatal(`ImagesData("openrouter") ok = false, want true`)
	}
	if len(data) == 0 {
		t.Fatal(`ImagesData("openrouter") returned no bytes`)
	}
}

func TestImagesData_UnknownProvider(t *testing.T) {
	if _, ok := ImagesData("does-not-exist"); ok {
		t.Fatal(`ImagesData("does-not-exist") ok = true, want false`)
	}
}
