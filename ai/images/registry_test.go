package images

// Ports: packages/ai/test/openrouter-images.test.ts (indirectly, via the
// top-level generateImages entrypoint), packages/ai/src/images.ts,
// packages/ai/src/images-api-registry.ts.

import (
	"context"
	"testing"
)

func TestGenerateImages_DispatchesToRegisteredAdapter(t *testing.T) {
	const testAPI = "test-registry-images"
	var gotModel *Model
	RegisterAPIProvider(testAPI, func(_ context.Context, model *Model, _ Context, _ *Options) *AssistantImages {
		gotModel = model
		return &AssistantImages{Api: model.Api, Provider: model.Provider, Model: model.ID, StopReason: StopReasonStop}
	})

	model := &Model{ID: "m1", Api: testAPI, Provider: "p1"}
	result := GenerateImages(context.Background(), model, Context{}, nil)

	if result.StopReason != StopReasonStop {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonStop)
	}
	if gotModel != model {
		t.Fatalf("adapter did not receive the dispatched model")
	}
}

func TestGenerateImages_UnregisteredAPIReturnsErrorResult(t *testing.T) {
	model := &Model{ID: "m1", Api: "no-such-api-registered", Provider: "p1"}
	result := GenerateImages(context.Background(), model, Context{}, nil)

	if result.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonError)
	}
	if result.ErrorMessage == "" {
		t.Fatalf("ErrorMessage is empty, want a message naming the unregistered api")
	}
}
