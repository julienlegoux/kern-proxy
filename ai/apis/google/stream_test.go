package google

// New Go tests exercising DecodeStream through the full Stream() round trip
// against an httptest SSE fixture server, matching this repo's established
// pattern in the sibling anthropic/openairesponses packages. Upstream has no
// dedicated SSE-decode test to port verbatim (google-generative-ai.ts
// delegates streaming to the @google/genai SDK's own async iterator), so
// these fixtures model the Gemini REST API's documented
// streamGenerateContent?alt=sse response shape directly.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func sseServer(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		var b strings.Builder
		for _, c := range chunks {
			b.WriteString("data: ")
			b.WriteString(c)
			b.WriteString("\n\n")
		}
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStream_TextRoundTrip(t *testing.T) {
	chunks := []string{
		`{"responseId":"resp-1","candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"index":0}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":" world"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2,"totalTokenCount":12}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "test-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop", result.StopReason)
	}
	if result.ResponseID != "resp-1" {
		t.Errorf("responseId = %q, want resp-1", result.ResponseID)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 text block", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello world" {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "Hello world")
	}
	if result.Usage.Input != 10 || result.Usage.Output != 2 || result.Usage.TotalTokens != 12 {
		t.Errorf("usage = %#v, want input=10 output=2 total=12", result.Usage)
	}
}

// TestStream_ThinkingAndTextAccumulateAsSeparateBlocks covers the
// thought/non-thought Part discrimination: consecutive thinking deltas
// accumulate into one thinking block, then a following non-thinking delta
// starts a new text block.
func TestStream_ThinkingAndTextAccumulateAsSeparateBlocks(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"pondering","thought":true,"thoughtSignature":"c2ln"}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"answer"}]},"finishReason":"STOP"}],"usageMetadata":{}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "test-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if len(result.Content) != 2 {
		t.Fatalf("content = %#v, want [thinking, text]", result.Content)
	}
	thinking, ok := result.Content[0].(ai.ThinkingContent)
	if !ok || thinking.Thinking != "pondering" || thinking.ThinkingSignature != "c2ln" {
		t.Errorf("content[0] = %#v, want thinking %q sig c2ln", result.Content[0], "pondering")
	}
	text, ok := result.Content[1].(ai.TextContent)
	if !ok || text.Text != "answer" {
		t.Errorf("content[1] = %#v, want text %q", result.Content[1], "answer")
	}
}

// TestStream_ToolCallRoundTrip covers functionCall streaming into a ToolCall
// block with a synthesized id (Gemini's own function calls have no id).
func TestStream_ToolCallRoundTrip(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"nyc"}}}]},"finishReason":"STOP"}]}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "test-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonToolUse {
		t.Errorf("stopReason = %q, want toolUse", result.StopReason)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 toolCall block", result.Content)
	}
	tc, ok := result.Content[0].(ai.ToolCall)
	if !ok || tc.Name != "get_weather" || tc.ID == "" {
		t.Errorf("content[0] = %#v, want toolCall get_weather with a synthesized id", result.Content[0])
	}
	if tc.Arguments["city"] != "nyc" {
		t.Errorf("arguments = %#v, want city=nyc", tc.Arguments)
	}
}

// TestStream_HTTPErrorSurfacesAsErrorEvent covers a non-2xx response.
func TestStream_HTTPErrorSurfacesAsErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	t.Cleanup(srv.Close)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "bad-key"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "401") {
		t.Errorf("errorMessage = %q, want it to mention the 401 status", result.ErrorMessage)
	}
}
