package google

// Env-gated live smoke test: only runs (and only makes a real network call to
// the live Gemini API) when GEMINI_API_KEY is set in the environment. CI
// never sets that secret, so this test always skips there -- go test ./...
// and CI do not depend on live network access. Upstream has no dedicated
// google-generative-ai smoke test to port verbatim (google-specific
// assertions live only inside broader multi-provider integration suites
// such as test/stream.test.ts and test/tokens.test.ts, gated the same way on
// GEMINI_API_KEY/GOOGLE_API_KEY but not isolated to this adapter), so this is
// a new minimal smoke test covering the same fidelity bar as the sibling
// anthropic package's live smoke test: a real streamed response, plain text
// content, and non-zero usage accounting.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestLiveSmoke_GoogleGeminiStreamsPlainText(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set; skipping live Gemini smoke test")
	}

	model := &ai.Model{
		ID:            "gemini-2.5-flash",
		Name:          "Gemini 2.5 Flash",
		Api:           ai.ApiGoogleGenerativeAI,
		Provider:      "google",
		BaseURL:       "https://generativelanguage.googleapis.com",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 1000000,
		MaxTokens:     8192,
	}
	chat := ai.Context{
		Messages: []ai.Message{ai.UserMessage{
			Content:   ai.UserText("Reply with exactly the text: smoke-ok"),
			Timestamp: time.Now().UnixMilli(),
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxTokens := 64
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: apiKey, MaxTokens: &maxTokens})
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q (errorMessage=%q), want stop", result.StopReason, result.ErrorMessage)
	}
	if len(result.Content) == 0 {
		t.Fatal("content is empty, want at least one text block")
	}
	if result.Usage.TotalTokens == 0 {
		t.Error("usage.totalTokens = 0, want non-zero")
	}
}
