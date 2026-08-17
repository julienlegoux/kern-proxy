package openaicompletions

// New Go tests (not literal upstream ports): upstream's openai-completions-*
// test files interleave compat-matrix/thinking-format assertions (issues
// 02/03) with core adapter behavior throughout, so there is no clean subset
// to port verbatim for the core-only scope of this issue — see the anthropic
// package's analogous note. These tests cover the same fidelity bar for the
// base adapter: request building, dual-map tool-call correlation, partial-JSON
// re-parsing, and prompt/cache usage math.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

// writeChunkedSSE formats OpenAI-style SSE: each event is "data: <json>\n\n",
// terminated by "data: [DONE]\n\n".
func writeChunkedSSE(chunks []string) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString("data: ")
		b.WriteString(c)
		b.WriteString("\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

func sseServer(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeChunkedSSE(chunks)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gpt-4o-mini",
		Name:          "GPT-4o mini",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openai",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 128000,
		MaxTokens:     4096,
	}
}

func minimalTextChunks() []string {
	return []string{
		`{"id":"chatcmpl-test","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		`{"id":"chatcmpl-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
}

func TestBuildParams_BasicRequestShape(t *testing.T) {
	srv := sseServer(t, minimalTextChunks())
	model := testModel(srv.URL)
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}

	var captured map[string]any
	opts := &ai.StreamOptions{
		APIKey: "sk-test",
		OnPayload: func(_ context.Context, payload any, _ *ai.Model) (any, error) {
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			if err := json.Unmarshal(raw, &captured); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			return nil, nil
		},
	}

	stream := Stream(context.Background(), model, chat, opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if captured["model"] != "gpt-4o-mini" {
		t.Errorf("model = %v, want gpt-4o-mini", captured["model"])
	}
	if captured["stream"] != true {
		t.Errorf("stream = %v, want true", captured["stream"])
	}
	messages, ok := captured["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %#v, want 2 entries (system, user)", captured["messages"])
	}
	sysMsg := messages[0].(map[string]any)
	if sysMsg["role"] != "system" || sysMsg["content"] != "be helpful" {
		t.Errorf("messages[0] = %#v, want system/be helpful", sysMsg)
	}
	userMsg := messages[1].(map[string]any)
	if userMsg["role"] != "user" || userMsg["content"] != "hi" {
		t.Errorf("messages[1] = %#v, want user/hi", userMsg)
	}
	if _, hasTools := captured["tools"]; hasTools {
		t.Errorf("tools present = %#v, want omitted when no tools", captured["tools"])
	}
}

// TestStream_CoalescesToolCallDeltasByStableIndexWhenIDMutates ports the
// upstream "coalesces tool call deltas by stable index when provider mutates
// ids mid-stream" fidelity case: index is the primary correlation key, so a
// changing id across deltas at the same index must not fork a second block.
func TestStream_CoalescesToolCallDeltasByStableIndexWhenIDMutates(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"edit","arguments":"{\"pa"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_b","function":{"arguments":"th\":\"f.txt\"}"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("edit"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want exactly 1 coalesced tool-call block", result.Content)
	}
	tc, ok := result.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %#v, want ai.ToolCall", result.Content[0])
	}
	if tc.Name != "edit" {
		t.Errorf("name = %q, want edit", tc.Name)
	}
	if tc.Arguments["path"] != "f.txt" {
		t.Errorf("arguments = %#v, want path=f.txt", tc.Arguments)
	}
}

// TestStream_AccumulatesMixedContentReasoningAndParallelToolCalls ports the
// upstream "accumulates mixed content, reasoning, and parallel tool call
// deltas independently" fidelity case: text, thinking, and two independent
// (id-keyed, distinct-index) tool calls must not cross-contaminate.
func TestStream_AccumulatesMixedContentReasoningAndParallelToolCalls(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"Sure, "}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"reasoning_content":"thinking..."}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"on it."}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"a","arguments":"{\"x\":1}"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","function":{"name":"b","arguments":"{\"y\":2}"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("go"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if len(result.Content) != 4 {
		t.Fatalf("content = %#v, want 4 blocks (text, thinking, 2 tool calls)", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Sure, on it." {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "Sure, on it.")
	}
	think, ok := result.Content[1].(ai.ThinkingContent)
	if !ok || think.Thinking != "thinking..." {
		t.Errorf("content[1] = %#v, want thinking %q", result.Content[1], "thinking...")
	}
	tc1, ok := result.Content[2].(ai.ToolCall)
	if !ok || tc1.Name != "a" || tc1.Arguments["x"] != float64(1) {
		t.Errorf("content[2] = %#v, want tool call a{x:1}", result.Content[2])
	}
	tc2, ok := result.Content[3].(ai.ToolCall)
	if !ok || tc2.Name != "b" || tc2.Arguments["y"] != float64(2) {
		t.Errorf("content[3] = %#v, want tool call b{y:2}", result.Content[3])
	}
}

// TestParseChunkUsage_DoesNotDoubleCountReasoningTokens ports the upstream
// "does not double-count reasoning tokens in completion usage" case:
// completion_tokens already includes reasoning tokens, so output must equal
// completion_tokens as-is.
func TestParseChunkUsage_DoesNotDoubleCountReasoningTokens(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":30}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.Usage.Output != 50 {
		t.Errorf("usage.Output = %d, want 50 (completion_tokens as-is)", result.Usage.Output)
	}
	if result.Usage.Reasoning == nil || *result.Usage.Reasoning != 30 {
		t.Errorf("usage.Reasoning = %v, want 30", result.Usage.Reasoning)
	}
}

// TestParseChunkUsage_PreservesCacheReadWriteFromChunkUsage ports the
// upstream "preserves prompt_tokens_details cache read/write fields from
// chunk usage" case.
func TestParseChunkUsage_PreservesCacheReadWriteFromChunkUsage(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":20,"cache_write_tokens":5},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.Usage.CacheRead != 20 {
		t.Errorf("usage.CacheRead = %d, want 20", result.Usage.CacheRead)
	}
	if result.Usage.CacheWrite != 5 {
		t.Errorf("usage.CacheWrite = %d, want 5", result.Usage.CacheWrite)
	}
	// input = prompt_tokens - cacheRead - cacheWrite = 100 - 20 - 5 = 75
	if result.Usage.Input != 75 {
		t.Errorf("usage.Input = %d, want 75", result.Usage.Input)
	}
}

// TestStream_MapsNonStandardFinishReasonToError ports the upstream "maps
// non-standard provider finish_reason values to stopReason error" case.
func TestStream_MapsNonStandardFinishReasonToError(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":"weird_reason"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "Provider finish_reason: weird_reason" {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, "Provider finish_reason: weird_reason")
	}
}

// TestStream_IgnoresNullStreamChunks ports the upstream "ignores null stream
// chunks from openai-compatible providers" case.
func TestStream_IgnoresNullStreamChunks(t *testing.T) {
	chunks := []string{
		"null",
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Errorf("stopReason = %q, want stop", result.StopReason)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "hi" {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "hi")
	}
}

// TestStream_ErrorsWhenStreamEndsAfterOnlyNullFinishReasonChunks ports the
// upstream "errors when a stream ends after only null finish_reason chunks"
// case: a stream with only null finish_reason must be reported as ended
// without finish_reason (retryable, per Epic 1's classifier).
func TestStream_ErrorsWhenStreamEndsAfterOnlyNullFinishReasonChunks(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "Stream ended without finish_reason" {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, "Stream ended without finish_reason")
	}
}

// TestConvertMessages_OmitsToolsFieldWhenEmpty and its sibling below port the
// upstream "empty tools handling" core behavior: empty tools arrays must not
// be serialized as `tools: []` (some OpenAI-compatible backends reject that),
// but tool history in the conversation still requires the (now-empty) tools
// field so proxies that need it see it.
func TestBuildParams_OmitsToolsFieldWhenContextToolsEmpty(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Tools != nil {
		t.Errorf("tools = %#v, want nil (omitted)", params.Tools)
	}
}

func TestBuildParams_SendsEmptyToolsWhenToolHistoryPresent(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "gpt-4o-mini",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
		},
		Tools: []ai.Tool{},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Tools == nil || len(*params.Tools) != 0 {
		t.Errorf("tools = %#v, want non-nil empty slice", params.Tools)
	}
}

// TestStream_NonOKStatusProducesErrorEvent verifies a non-2xx HTTP response
// terminates the stream with an in-band error event (never a Go panic/error
// from Stream itself), matching the adapter contract in ai.StreamFunc.
func TestStream_NonOKStatusProducesErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	t.Cleanup(srv.Close)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

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

// TestStream_ReparsesPartialToolArgsOnEveryDelta verifies the strict ->
// repair -> partial -> {} cascade runs on every delta (not just at the end),
// so a consumer reading Partial mid-stream sees usable (if incomplete)
// arguments, and the final block is the fully-parsed value with no scratch
// buffer leaking into the persisted ai.ToolCall.
func TestStream_ReparsesPartialToolArgsOnEveryDelta(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"edit","arguments":"{\"path\":\"a"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":".txt\"}"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
	}
	srv := sseServer(t, chunks)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("edit"), Timestamp: time.Now().UnixMilli()}}}

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
	tc, ok := result.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %#v, want ai.ToolCall", result.Content[0])
	}
	if tc.Arguments["path"] != "a.txt" {
		t.Errorf("final arguments = %#v, want path=a.txt", tc.Arguments)
	}
}

// TestBuildParams_RoundTripsAssistantToolCallAndToolResult verifies an
// assistant tool-call turn followed by its tool result round-trips into the
// OpenAI wire shape (assistant.tool_calls + a "tool" role message), which is
// the "chat-completions request building" half of this issue's scope.
func TestBuildParams_RoundTripsAssistantToolCallAndToolResult(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{"x": float64(1)}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "gpt-4o-mini",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
		},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})

	if len(params.Messages) != 3 {
		t.Fatalf("messages = %#v, want 3 (user, assistant, tool)", params.Messages)
	}
	assistant := params.Messages[1]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("messages[1] = %#v, want assistant with 1 tool call", assistant)
	}
	if assistant.ToolCalls[0].ID != "t1" || assistant.ToolCalls[0].Function.Name != "noop" {
		t.Errorf("tool call = %#v, want id=t1 name=noop", assistant.ToolCalls[0])
	}
	if assistant.ToolCalls[0].Function.Arguments != `{"x":1}` {
		t.Errorf("tool call arguments = %q, want %q", assistant.ToolCalls[0].Function.Arguments, `{"x":1}`)
	}
	toolMsg := params.Messages[2]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "t1" || toolMsg.Content != "done" {
		t.Errorf("messages[2] = %#v, want tool/t1/done", toolMsg)
	}
}

// TestStream_LiveSmoke exercises a real OpenAI request end to end. It is
// env-gated so it never runs in CI: set OPENAI_API_KEY to run it locally.
func TestStream_LiveSmoke(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping live smoke test")
	}

	model := &ai.Model{
		ID:            "gpt-4o-mini",
		Name:          "GPT-4o mini",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openai",
		BaseURL:       "https://api.openai.com/v1",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     64,
	}
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say the single word: pong"), Timestamp: time.Now().UnixMilli()}},
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
