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

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/providers"
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
	// The embedded catalog's baseUrl already includes "/v1beta"
	// (ai/catalog/data/models/google.json), but ai/apis/google's requestURL
	// appends "/v1beta/models/..." itself, doubling the path segment and
	// 404ing. Pass a corrected copy for this request only — fixing the
	// catalog/adapter mismatch itself is outside this CLI issue's scope; see
	// the PR description for a follow-up note.
	model := *catalogModel
	model.BaseURL = "https://generativelanguage.googleapis.com"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	final, err := Run(ctx, models, &model,
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
