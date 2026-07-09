package azure

// New Go tests exercising Stream() end to end against an httptest SSE
// fixture server: this captures the actual outgoing request (URL, headers,
// body) that azure-openai-base-url.test.ts never observes (it only inspects
// the AzureOpenAI SDK client's constructor arguments before a request is
// built), matching the fidelity bar of the sibling openairesponses package's
// own Stream tests -- reusing the exact SSE fixture shape and
// openairesponses.DecodeStream this package delegates to.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

type capturedRequest struct {
	method  string
	path    string
	query   string
	headers http.Header
	body    map[string]any
}

func azureSSEServer(t *testing.T, events []string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = r.URL.RawQuery
		captured.headers = r.Header.Clone()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		captured.body = body

		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		var b strings.Builder
		for _, e := range events {
			b.WriteString("data: ")
			b.WriteString(e)
			b.WriteString("\n\n")
		}
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func minimalTextEvents() []string {
	return []string{
		`{"type":"response.created","response":{"id":"resp_1"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"Hello"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","status":"completed","content":[{"type":"output_text","text":"Hello"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":12,"output_tokens":5,"total_tokens":17}}}`,
	}
}

// TestStream_RequestURLShapeHeadersAndPayload is this issue's "URL shape,
// headers, payload" acceptance criterion: it drives a full Stream() call
// against a local httptest server standing in for a non-Azure-host proxy
// (whose base URL therefore passes normalizeAzureBaseURL unchanged) and
// inspects the actual outgoing HTTP request.
func TestStream_RequestURLShapeHeadersAndPayload(t *testing.T) {
	srv, captured := azureSSEServer(t, minimalTextEvents())
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "azure-key", AzureDeploymentName: "my-deployment"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if captured.method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured.method)
	}
	if captured.path != "/responses" {
		t.Errorf("path = %q, want /responses", captured.path)
	}
	if captured.query != "" {
		t.Errorf("query = %q, want empty (v1 default needs no api-version)", captured.query)
	}
	if got := captured.headers.Get("api-key"); got != "azure-key" {
		t.Errorf("api-key header = %q, want azure-key", got)
	}
	if got := captured.headers.Get("content-type"); got != "application/json" {
		t.Errorf("content-type header = %q", got)
	}
	if captured.body["model"] != "my-deployment" {
		t.Errorf("body.model = %v, want my-deployment", captured.body["model"])
	}
	if captured.body["stream"] != true {
		t.Errorf("body.stream = %v, want true", captured.body["stream"])
	}
}

// TestStream_DatedAPIVersionAppendsQueryParam covers the non-default
// AzureAPIVersion branch: a dated version is sent as an api-version query
// parameter (Azure's documented REST convention; see run()'s doc comment for
// why this isn't a captured upstream golden).
func TestStream_DatedAPIVersionAppendsQueryParam(t *testing.T) {
	srv, captured := azureSSEServer(t, minimalTextEvents())
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "azure-key", AzureAPIVersion: "2024-10-21"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	if captured.query != "api-version=2024-10-21" {
		t.Errorf("query = %q, want api-version=2024-10-21", captured.query)
	}
}

func TestStream_TextRoundTrip(t *testing.T) {
	srv, _ := azureSSEServer(t, minimalTextEvents())
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "azure-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop", result.StopReason)
	}
	if result.Api != ai.ApiAzureOpenAIResponses {
		t.Errorf("api = %q, want azure-openai-responses", result.Api)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 text block", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello" {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "Hello")
	}
}

func TestStream_MissingAPIKeyReturnsErrorEvent(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "No API key for provider") {
		t.Errorf("errorMessage = %q, want the missing-key message", result.ErrorMessage)
	}
}

func TestStream_MissingBaseURLReturnsErrorEvent(t *testing.T) {
	model := testModel("")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "azure-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "Azure OpenAI base URL is required") {
		t.Errorf("errorMessage = %q, want the missing-base-URL message", result.ErrorMessage)
	}
}

// TestStream_LiveSmoke exercises a real Azure OpenAI Responses request end to
// end. It is env-gated so it never runs in CI: set AZURE_OPENAI_API_KEY and
// either AZURE_OPENAI_BASE_URL or AZURE_OPENAI_RESOURCE_NAME to run it
// locally.
func TestStream_LiveSmoke(t *testing.T) {
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	baseURL := os.Getenv("AZURE_OPENAI_BASE_URL")
	resourceName := os.Getenv("AZURE_OPENAI_RESOURCE_NAME")
	if apiKey == "" || (baseURL == "" && resourceName == "") {
		t.Skip("AZURE_OPENAI_API_KEY and AZURE_OPENAI_BASE_URL/AZURE_OPENAI_RESOURCE_NAME not set; skipping live smoke test")
	}

	model := &ai.Model{
		ID:            "gpt-4o-mini",
		Name:          "GPT-4o mini",
		Api:           ai.ApiAzureOpenAIResponses,
		Provider:      "azure-openai-responses",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     64,
	}
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say the single word: pong"), Timestamp: time.Now().UnixMilli()}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: apiKey, AzureBaseURL: baseURL, AzureResourceName: resourceName})
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason == ai.StopReasonError {
		t.Fatalf("live smoke returned an error stop reason: %s", result.ErrorMessage)
	}
	if len(result.Content) == 0 {
		t.Fatalf("live smoke returned no content")
	}
}
