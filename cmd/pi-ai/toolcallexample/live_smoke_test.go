package toolcallexample

// Env-gated live smoke test: only runs (and only makes a real network call)
// when GEMINI_API_KEY is set. CI never sets that secret, so this always
// skips there; go test ./... does not depend on live network access. This
// is the "env-gated variant against a real provider" the issue's acceptance
// criteria call for — the same Run function as the faux test above, wired
// to the real Google provider binding (ai/providers, Epic 11) instead of
// faux. Because a live model's decision to call a tool isn't fully
// controllable, this only asserts the round trip completes without error or
// abort and returns content — not that a tool call necessarily happened.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/providers"
)

func TestLiveSmoke_GeminiToolCallRoundTrip(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set; skipping live Gemini smoke test")
	}

	models := providers.Models(nil)
	catalogModel := models.GetModel("google", "gemini-2.5-flash")
	if catalogModel == nil {
		t.Fatal("gemini-2.5-flash not found in the built-in catalog")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	final, err := Run(ctx, models, catalogModel,
		"What is the weather in Paris? Use the get_weather tool to find out, then answer in one short sentence.",
		nil, apiKey)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if final.StopReason != ai.StopReasonStop && final.StopReason != ai.StopReasonToolUse {
		t.Fatalf("stopReason = %q (errorMessage=%q), want stop or toolUse", final.StopReason, final.ErrorMessage)
	}
	if len(final.Content) == 0 {
		t.Fatal("content is empty, want at least one block")
	}
}
