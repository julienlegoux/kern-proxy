package vertex

// Env-gated live smoke test: only runs (and only makes a real network call,
// authenticating via real ambient Application Default Credentials) when
// GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION are both set. CI never sets
// these, so this test always skips there -- go test ./... and CI do not
// depend on live GCP access or credentials. Mirrors the fidelity bar of the
// sibling ai/apis/google package's live smoke test (google_live_smoke_test.go):
// a real streamed response, plain text content, and non-zero usage
// accounting -- here additionally exercising the real ADC token source
// (adcTokenFunc's default, not the fake one the rest of this package's tests
// inject) end-to-end.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestLiveSmoke_GoogleVertexADCStreamsPlainText(t *testing.T) {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if project == "" || location == "" {
		t.Skip("GOOGLE_CLOUD_PROJECT/GOOGLE_CLOUD_LOCATION not set; skipping live Vertex ADC smoke test")
	}

	model := &ai.Model{
		ID:            "gemini-2.5-flash",
		Name:          "Gemini 2.5 Flash",
		Api:           ai.ApiGoogleVertex,
		Provider:      "google-vertex",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 1000000,
		MaxTokens:     8192,
	}
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{
			Content:   ai.UserText("Reply with exactly the text: smoke-ok"),
			Timestamp: time.Now().UnixMilli(),
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxTokens := 64
	opts := &ai.StreamOptions{
		GoogleVertexProject:  project,
		GoogleVertexLocation: location,
		MaxTokens:            &maxTokens,
	}
	stream := Stream(ctx, model, chat, opts)
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
