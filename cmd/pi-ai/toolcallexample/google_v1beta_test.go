package toolcallexample

// Regression test for the doubled-/v1beta bug (issue #78). A google model from
// the real embedded catalog carries a baseURL ending in /v1beta, and
// ai/apis/google's requestURL appends "/v1beta/models/..." itself. Because
// requestURL is unexported and the catalog lives behind ai/providers (which
// would be an import cycle inside the google package), this drives the real
// StreamSimple code path with the catalog model — its host redirected to a test
// server with the /v1beta suffix preserved — and asserts the request path
// carries a single /v1beta segment rather than the doubled one.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/google"
	"github.com/kern-ia/kern-link/ai/providers"
)

func TestGoogleCatalogModelBuildsSingleV1BetaPath(t *testing.T) {
	catalogModel := providers.Models(nil).GetModel("google", "gemini-2.5-flash")
	if catalogModel == nil {
		t.Fatal("gemini-2.5-flash not found in the built-in catalog")
	}
	// Precondition that caused the doubling: the catalog baseURL ends in /v1beta.
	if !strings.HasSuffix(strings.TrimRight(catalogModel.BaseURL, "/"), "/v1beta") {
		t.Fatalf("catalog google baseURL = %q, want a trailing /v1beta", catalogModel.BaseURL)
	}

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer srv.Close()

	// Mirror the catalog model's /v1beta-suffixed baseURL on a controllable host
	// so the real requestURL output is observable without a live call.
	model := *catalogModel
	model.BaseURL = srv.URL + "/v1beta"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := google.StreamSimple(ctx, &model, chat, &ai.SimpleStreamOptions{StreamOptions: ai.StreamOptions{APIKey: "test-key"}})
	if _, err := stream.Result(ctx); err != nil {
		t.Fatalf("StreamSimple Result: %v", err)
	}

	if strings.Contains(gotPath, "/v1beta/v1beta") {
		t.Errorf("request path = %q doubles the /v1beta segment", gotPath)
	}
	if want := "/v1beta/models/gemini-2.5-flash:streamGenerateContent"; gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}
