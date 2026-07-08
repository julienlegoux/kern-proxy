package images

// Ports: packages/ai/src/image-models.ts (getImageModel/getImageModels/
// getImageProviders — the typed-decode half of the catalog reader; raw byte
// access is ai/catalog/images.go).

import "testing"

func TestCatalogModels_KnownProvider(t *testing.T) {
	models := CatalogModels("openrouter")
	if len(models) == 0 {
		t.Fatal(`CatalogModels("openrouter") returned no models`)
	}
	for _, m := range models {
		if m.Provider != "openrouter" {
			t.Errorf("model %s has provider %q, want \"openrouter\"", m.ID, m.Provider)
		}
		if m.Api != "openrouter-images" {
			t.Errorf("model %s has api %q, want \"openrouter-images\"", m.ID, m.Api)
		}
	}
}

func TestCatalogModels_UnknownProvider(t *testing.T) {
	if models := CatalogModels("does-not-exist"); models != nil {
		t.Fatalf("CatalogModels(unknown) = %v, want nil", models)
	}
}

func TestCatalogModel_FindsByID(t *testing.T) {
	model := CatalogModel("openrouter", "google/gemini-2.5-flash-image")
	if model == nil {
		t.Fatal(`CatalogModel("openrouter", "google/gemini-2.5-flash-image") = nil`)
	}
	if model.Name == "" {
		t.Error("Name is empty")
	}
}

func TestCatalogModel_UnknownID(t *testing.T) {
	if model := CatalogModel("openrouter", "does-not-exist"); model != nil {
		t.Fatalf("CatalogModel(unknown id) = %v, want nil", model)
	}
}
