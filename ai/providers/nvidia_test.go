package providers

// Tests the native RefreshModels behavior described in nvidia.go's doc
// comment: fetch NVIDIA NIM's live model id list and narrow the embedded
// catalog down to models still present live (matching by raw id or a
// normalized lowercase/dot-separated variant).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

func TestNvidiaProviderIsDynamic(t *testing.T) {
	provider := NvidiaProvider()
	if !provider.CanRefreshModels() {
		t.Fatal("CanRefreshModels() = false, want true")
	}
}

func TestRefreshNvidiaModelsNarrowsCatalogToLiveIDs(t *testing.T) {
	catalogModels := catalog.BuiltinModels("nvidia")
	if len(catalogModels) < 2 {
		t.Fatalf("nvidia catalog has %d models, need >= 2 to exercise filtering", len(catalogModels))
	}
	keepID := catalogModels[0].ID
	dropID := catalogModels[1].ID

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "` + keepID + `"}, {"id": "some/unrelated-model"}]}`))
	}))
	defer srv.Close()

	orig := nvidiaModelsURL
	nvidiaModelsURL = srv.URL
	defer func() { nvidiaModelsURL = orig }()

	models, err := refreshNvidiaModels(context.Background())
	if err != nil {
		t.Fatalf("refreshNvidiaModels: %v", err)
	}

	found := map[string]bool{}
	for _, m := range models {
		found[m.ID] = true
	}
	if !found[keepID] {
		t.Errorf("expected live-listed model %q to be kept", keepID)
	}
	if found[dropID] {
		t.Errorf("expected non-live-listed model %q to be dropped", dropID)
	}
}

func TestRefreshNvidiaModelsMatchesNormalizedID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Live NIM ids can use underscores/mixed case; the normalized form
		// (lowercase, underscore->dot) must still match a dotted catalog id.
		_, _ = w.Write([]byte(`{"data": [{"id": "Meta/Llama-3_1-70B-Instruct"}]}`))
	}))
	defer srv.Close()

	orig := nvidiaModelsURL
	nvidiaModelsURL = srv.URL
	defer func() { nvidiaModelsURL = orig }()

	catalogModels := catalog.BuiltinModels("nvidia")
	var target string
	for _, m := range catalogModels {
		if normalizeNvidiaModelID(m.ID) == normalizeNvidiaModelID("Meta/Llama-3_1-70B-Instruct") {
			target = m.ID
			break
		}
	}
	if target == "" {
		t.Skip("no catalog model normalizes to the fixture id; adjust fixture if the catalog changes")
	}

	models, err := refreshNvidiaModels(context.Background())
	if err != nil {
		t.Fatalf("refreshNvidiaModels: %v", err)
	}
	found := false
	for _, m := range models {
		if m.ID == target {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q to match via id normalization", target)
	}
}

func TestRefreshNvidiaModelsPropagatesFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := nvidiaModelsURL
	nvidiaModelsURL = srv.URL
	defer func() { nvidiaModelsURL = orig }()

	if _, err := refreshNvidiaModels(context.Background()); err == nil {
		t.Fatal("refreshNvidiaModels error = nil, want non-nil")
	}
}
