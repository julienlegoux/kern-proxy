package openairesponses

// New Go tests exercising DecodeStream through the full Stream() round trip
// against an httptest SSE fixture server, matching this repo's established
// pattern in the sibling anthropic/openaicompletions packages. Upstream has
// no dedicated SSE-decode test file to port verbatim for openai-responses
// (its fidelity is exercised indirectly through higher-level tests), so
// these are new tests covering the same fidelity bar: text/thinking/tool-call
// streaming, usage accounting, stop-reason mapping, and error propagation.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// writeResponsesSSE formats each event as "event: <type>\ndata: <json>\n\n",
// matching real Responses API framing.
func writeResponsesSSE(events []string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: ")
		b.WriteString(e)
		b.WriteString("\n\n")
	}
	return b.String()
}

func sseServer(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeResponsesSSE(events)))
	}))
	t.Cleanup(srv.Close)
	return srv
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

func TestStream_TextRoundTrip(t *testing.T) {
	srv := sseServer(t, minimalTextEvents())
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop", result.StopReason)
	}
	if result.ResponseID != "resp_1" {
		t.Errorf("responseId = %q, want resp_1", result.ResponseID)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 text block", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello" {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "Hello")
	}
	if text.TextSignature == "" {
		t.Errorf("textSignature is empty, want the encoded v1 signature")
	}
	if result.Usage.Input != 12 || result.Usage.Output != 5 || result.Usage.TotalTokens != 17 {
		t.Errorf("usage = %#v, want input=12 output=5 total=17", result.Usage)
	}
}

// TestStream_ThinkingAndTextAccumulateIndependently covers the reasoning
// summary delta/done path and its interaction with a following text block.
func TestStream_ThinkingAndTextAccumulateIndependently(t *testing.T) {
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning","id":"rs_1"}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"thinking"}`,
		`{"type":"response.reasoning_summary_part.done","output_index":0}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"more"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"rs_1","encrypted_content":"enc"}}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":1,"delta":"answer"}`,
		`{"type":"response.output_item.done","output_index":1,"item":{"type":"message","id":"msg_1","content":[{"type":"output_text","text":"answer"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed"}}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if len(result.Content) != 2 {
		t.Fatalf("content = %#v, want 2 blocks (thinking, text)", result.Content)
	}
	think, ok := result.Content[0].(ai.ThinkingContent)
	if !ok || think.Thinking != "thinking\n\nmore" {
		t.Errorf("content[0] = %#v, want thinking %q", result.Content[0], "thinking\n\nmore")
	}
	if think.ThinkingSignature == "" || !strings.Contains(think.ThinkingSignature, "encrypted_content") {
		t.Errorf("thinkingSignature = %q, want the verbatim reasoning item JSON", think.ThinkingSignature)
	}
	text, ok := result.Content[1].(ai.TextContent)
	if !ok || text.Text != "answer" {
		t.Errorf("content[1] = %#v, want text %q", result.Content[1], "answer")
	}
}

// TestStream_ToolCallArgumentsReparseOnEveryDelta mirrors the
// openaicompletions package's equivalent fidelity case: partial-JSON
// re-parsing must run on every delta so a mid-stream consumer sees usable
// (if incomplete) arguments, and the finish reason upgrades to toolUse.
func TestStream_ToolCallArgumentsReparseOnEveryDelta(t *testing.T) {
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"edit","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"path\":\"a"}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"delta":".txt\"}"}`,
		`{"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"path\":\"a.txt\"}"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"edit","arguments":"{\"path\":\"a.txt\"}"}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed"}}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("edit"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})

	var sawPartialArg bool
	for ev := range stream.Events(context.Background()) {
		if d, ok := ev.(ai.ToolCallDeltaEvent); ok {
			tc := d.Partial.Content[d.ContentIndex].(ai.ToolCall)
			if path, ok := tc.Arguments["path"].(string); ok && path == "a" {
				sawPartialArg = true
			}
		}
	}
	if !sawPartialArg {
		t.Errorf("never observed the partially-parsed path=%q value mid-stream", "a")
	}

	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonToolUse {
		t.Errorf("stopReason = %q, want toolUse", result.StopReason)
	}
	tc, ok := result.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %#v, want ai.ToolCall", result.Content[0])
	}
	if tc.ID != "call_1|fc_1" {
		t.Errorf("id = %q, want call_1|fc_1", tc.ID)
	}
	if tc.Arguments["path"] != "a.txt" {
		t.Errorf("arguments = %#v, want path=a.txt", tc.Arguments)
	}
}

// TestStream_IncompleteStatusMapsToLength verifies the status -> StopReason
// mapping for the "incomplete" terminal state.
func TestStream_IncompleteStatusMapsToLength(t *testing.T) {
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"partial"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","content":[{"type":"output_text","text":"partial"}]}}`,
		`{"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete"}}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonLength {
		t.Errorf("stopReason = %q, want length", result.StopReason)
	}
}

// TestStream_ResponseFailedEventProducesErrorEvent covers the
// response.failed error path.
func TestStream_ResponseFailedEventProducesErrorEvent(t *testing.T) {
	events := []string{
		`{"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"code":"server_error","message":"boom"}}}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "server_error: boom" {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, "server_error: boom")
	}
}

// TestStream_ErrorEventPropagatesImmediately covers the bare "error" event
// type (distinct from response.failed).
func TestStream_ErrorEventPropagatesImmediately(t *testing.T) {
	events := []string{
		`{"type":"error","code":"rate_limited","message":"slow down"}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "rate_limited") || !strings.Contains(result.ErrorMessage, "slow down") {
		t.Errorf("errorMessage = %q, want it to contain the code and message", result.ErrorMessage)
	}
}

// TestStream_ErrorsWhenStreamEndsWithoutTerminalEvent covers the
// no-terminal-event guard.
func TestStream_ErrorsWhenStreamEndsWithoutTerminalEvent(t *testing.T) {
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"hi"}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "OpenAI Responses stream ended before a terminal response event" {
		t.Errorf("errorMessage = %q, want the terminal-event guard message", result.ErrorMessage)
	}
}

// TestStream_ServiceTierPricingAppliesMultiplier verifies the flex/priority
// pricing multiplier is applied to the calculated cost.
func TestStream_ServiceTierPricingAppliesMultiplier(t *testing.T) {
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"hi"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","content":[{"type":"output_text","text":"hi"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","service_tier":"flex","usage":{"input_tokens":100,"output_tokens":10,"total_tokens":110}}}`,
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	model.Cost = ai.ModelCost{Input: 1, Output: 1}
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test", ServiceTier: "flex"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	// Computed via runtime float64 vars in the same order as CalculateCost +
	// applyServiceTierPricing (input cost first, multiplier applied after):
	// using untyped constants here would let the Go compiler fold the whole
	// expression at arbitrary precision, which doesn't reproduce the
	// sequential float64 rounding the implementation actually performs.
	costPerMillion := model.Cost.Input
	inputTokens := 100.0
	wantInputCost := costPerMillion / 1e6 * inputTokens
	wantInputCost *= 0.5
	if result.Usage.Cost.Input != wantInputCost {
		t.Errorf("cost.input = %v, want %v (flex 0.5x)", result.Usage.Cost.Input, wantInputCost)
	}
}

func TestStream_NonOKStatusProducesErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	t.Cleanup(srv.Close)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "429") || !strings.Contains(result.ErrorMessage, "rate_limit_error") {
		t.Errorf("errorMessage = %q, want it to contain status 429 and the error body", result.ErrorMessage)
	}
}

func TestStream_MissingAPIKeyProducesErrorEvent(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

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

// TestStream_LiveSmoke exercises a real OpenAI Responses request end to end.
// It is env-gated so it never runs in CI: set OPENAI_API_KEY to run it
// locally.
func TestStream_LiveSmoke(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping live smoke test")
	}

	model := &ai.Model{
		ID:            "gpt-5-mini",
		Name:          "GPT-5 mini",
		Api:           ai.ApiOpenAIResponses,
		Provider:      "openai",
		BaseURL:       "https://api.openai.com/v1",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 400000,
		MaxTokens:     64,
	}
	chat := ai.Context{
		Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say the single word: pong"), Timestamp: time.Now().UnixMilli()}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: apiKey})
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
