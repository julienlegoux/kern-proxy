package mistral

// New Go tests exercising DecodeStream through the full Stream() round trip
// against an httptest SSE fixture server, matching this repo's established
// pattern in the sibling anthropic/google packages. Upstream has no
// dedicated SSE-decode test to port verbatim (mistral-conversations.ts
// delegates streaming to the @mistralai/mistralai SDK's own async iterator),
// so these fixtures model the documented chat/completions SSE response
// shape directly.

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
		b.WriteString("data: [DONE]\n\n")
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStream_TextRoundTrip(t *testing.T) {
	chunks := []string{
		`{"id":"resp-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
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
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
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

// TestStream_ThinkingContentArray covers Magistral's array-shaped delta
// content: a "thinking" part followed by a "text" part in the same delta.
func TestStream_ThinkingContentArray(t *testing.T) {
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"pondering"}]}]}}]}`,
		`{"choices":[{"index":0,"delta":{"content":[{"type":"text","text":"answer"}]},"finish_reason":"stop"}]}`,
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
	if !ok || thinking.Thinking != "pondering" {
		t.Errorf("content[0] = %#v, want thinking %q", result.Content[0], "pondering")
	}
	text, ok := result.Content[1].(ai.TextContent)
	if !ok || text.Text != "answer" {
		t.Errorf("content[1] = %#v, want text %q", result.Content[1], "answer")
	}
}

// TestStream_ToolCallRoundTrip covers incremental tool-call argument
// streaming keyed by index, matching OpenAI-style chat/completions delta
// framing.
func TestStream_ToolCallRoundTrip(t *testing.T) {
	// Mistral repeats the tool-call id on every delta for the same call
	// (unlike OpenAI's "id on the first chunk only" convention), which is
	// what the ported correlation key (id + index) assumes.
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_weather","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"arguments":"{\"city\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"arguments":"\"nyc\"}"}}]},"finish_reason":"tool_calls"}]}`,
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
	if !ok || tc.Name != "get_weather" || tc.ID != "call_1" {
		t.Fatalf("content[0] = %#v, want toolCall get_weather/call_1", result.Content[0])
	}
	if tc.Arguments["city"] != "nyc" {
		t.Errorf("arguments = %#v, want city=nyc", tc.Arguments)
	}
}

// TestStream_HTTPErrorSurfacesAsErrorEvent covers a non-2xx response.
func TestStream_HTTPErrorSurfacesAsErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad key"}`))
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

// TestStream_DoneSentinelStopsDecoding covers the "data: [DONE]" sentinel
// line being skipped rather than treated as a malformed chunk.
func TestStream_DoneSentinelStopsDecoding(t *testing.T) {
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
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
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
}
