package catalog

// Ports: packages/ai/src/providers/all.ts (getBuiltinModel, getBuiltinModels,
// getBuiltinProviders — the static-catalog-read portion; provider
// construction from these reads is ai/providers, epic 11 issue 02).

import (
	"sort"
	"testing"
)

func TestProviders_ReturnsSortedNonEmptyList(t *testing.T) {
	providers := Providers()
	if len(providers) < 30 {
		t.Fatalf("Providers() returned %d entries, want at least 30", len(providers))
	}
	if !sort.StringsAreSorted(providers) {
		t.Fatalf("Providers() not sorted: %v", providers)
	}
	found := map[string]bool{}
	for _, p := range providers {
		found[p] = true
	}
	for _, want := range []string{"anthropic", "openai", "together", "amazon-bedrock", "mistral"} {
		if !found[want] {
			t.Errorf("Providers() missing %q", want)
		}
	}
}

func TestBuiltinModels_KnownProvider(t *testing.T) {
	models := BuiltinModels("anthropic")
	if len(models) == 0 {
		t.Fatal("BuiltinModels(\"anthropic\") returned no models")
	}
	for _, m := range models {
		if m.Provider != "anthropic" {
			t.Errorf("model %s has provider %q, want \"anthropic\"", m.ID, m.Provider)
		}
	}
}

func TestBuiltinModels_UnknownProvider(t *testing.T) {
	if models := BuiltinModels("does-not-exist"); models != nil {
		t.Fatalf("BuiltinModels(unknown) = %v, want nil", models)
	}
}

func TestBuiltinModel_FindsByID(t *testing.T) {
	model := BuiltinModel("anthropic", "claude-opus-4-5")
	if model == nil {
		t.Fatal("BuiltinModel(\"anthropic\", \"claude-opus-4-5\") = nil")
	}
	if model.Name != "Claude Opus 4.5 (latest)" {
		t.Errorf("Name = %q, want %q", model.Name, "Claude Opus 4.5 (latest)")
	}
	if model.Api != "anthropic-messages" {
		t.Errorf("Api = %q, want \"anthropic-messages\"", model.Api)
	}
}

func TestBuiltinModel_UnknownID(t *testing.T) {
	if model := BuiltinModel("anthropic", "does-not-exist"); model != nil {
		t.Fatalf("BuiltinModel(unknown id) = %v, want nil", model)
	}
}

func TestBuiltinModel_UnknownProvider(t *testing.T) {
	if model := BuiltinModel("does-not-exist", "claude-opus-4-5"); model != nil {
		t.Fatalf("BuiltinModel(unknown provider) = %v, want nil", model)
	}
}
