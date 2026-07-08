package catalog

// Ports: packages/ai/test/bedrock-models.test.ts's always-run assertion
// ("should get all available Bedrock models"). The rest of that upstream
// file makes live Bedrock requests per catalog model, gated behind
// AWS credentials plus BEDROCK_EXTENSIVE_MODEL_TEST — that adapter-level
// smoke test belongs with ai/apis/bedrock (epic 10), not the catalog.

import "testing"

func TestBedrockModels_HasAvailableModels(t *testing.T) {
	models := BuiltinModels("amazon-bedrock")
	if len(models) == 0 {
		t.Fatal(`BuiltinModels("amazon-bedrock") is empty, want at least one model`)
	}
	for _, m := range models {
		if m.Provider != "amazon-bedrock" {
			t.Errorf("model %s has provider %q, want \"amazon-bedrock\"", m.ID, m.Provider)
		}
	}
}
