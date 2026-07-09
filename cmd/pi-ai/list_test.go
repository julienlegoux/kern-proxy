package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/providers/faux"
)

// fakeProvider builds a minimal ai.Provider with the given id/name/models,
// for deterministic list-command tests independent of the real catalog.
func fakeProvider(id, name string, modelIDs ...string) ai.Provider {
	h := faux.New(&faux.Options{Provider: id})
	models := make([]*ai.Model, 0, len(modelIDs))
	for _, mid := range modelIDs {
		models = append(models, &ai.Model{ID: mid, Name: mid, Provider: id})
	}
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:     id,
		Name:   name,
		Auth:   h.Provider.Auth(),
		Models: models,
		Api:    ai.StreamFuncs{StreamFunc: h.Provider.Stream, StreamSimpleFunc: h.Provider.StreamSimple},
	})
}

func TestRunList_SortsProvidersAndModels(t *testing.T) {
	providers := []ai.Provider{
		fakeProvider("zeta", "Zeta", "z-2", "z-1"),
		fakeProvider("alpha", "Alpha", "a-1"),
	}

	var buf bytes.Buffer
	if err := runList(&buf, providers); err != nil {
		t.Fatalf("runList: %v", err)
	}
	out := buf.String()

	alphaIdx := strings.Index(out, "alpha")
	zetaIdx := strings.Index(out, "zeta")
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("expected both providers listed, got:\n%s", out)
	}
	if alphaIdx > zetaIdx {
		t.Errorf("expected alpha before zeta (sorted by id), got:\n%s", out)
	}

	z1Idx := strings.Index(out, "z-1")
	z2Idx := strings.Index(out, "z-2")
	if z1Idx == -1 || z2Idx == -1 || z1Idx > z2Idx {
		t.Errorf("expected z-1 before z-2 (sorted by model id), got:\n%s", out)
	}

	if !strings.Contains(out, "Alpha") {
		t.Errorf("expected provider display name in output, got:\n%s", out)
	}
}

func TestRunList_NoProviders(t *testing.T) {
	var buf bytes.Buffer
	if err := runList(&buf, nil); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for no providers, got:\n%s", buf.String())
	}
}
