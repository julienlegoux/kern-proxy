package anthropic

// Ports: packages/ai/test/anthropic-sse-parsing.test.ts (raw SSE
// parsing/decoding cases; request-building cases are new Go tests covering
// the same fidelity bar via OnPayload, since upstream has no dedicated
// request-building test file for the non-thinking/non-cache/non-oauth core).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// sseEvent is one event to serialize into a test SSE response body.
type sseEvent struct {
	event string
	data  string
}

// writeSSE formats events as "event: X\ndata: Y\n\n", always terminating the
// last event with a trailing blank line (unlike the upstream test helper,
// which relies on iterateSseMessages' forced end-of-stream flush). Real
// Anthropic responses always terminate every event, including the last, with
// a blank line, so this keeps the fixture realistic while working with
// ai/internal/sse's blank-line dispatch.
func writeSSE(events []sseEvent) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("event: ")
		b.WriteString(e.event)
		b.WriteString("\ndata: ")
		b.WriteString(e.data)
		b.WriteString("\n\n")
	}
	return b.String()
}

func sseServer(t *testing.T, events []sseEvent) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(events)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "claude-haiku-4-5",
		Name:          "Claude Haiku 4.5",
		Api:           ai.ApiAnthropicMessages,
		Provider:      "anthropic",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 200000,
		MaxTokens:     8192,
	}
}

const minimalEventsToolCall = `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"A\H\",\"text\":\"col1	col2\"}"}}`

func minimalAnthropicEvents() []sseEvent {
	return []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":12,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
}

func TestStream_RepairsMalformedSSEJSONAndStreamedToolJSON(t *testing.T) {
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_test","name":"edit","input":{}}}`},
		{event: "content_block_delta", data: minimalEventsToolCall},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":12,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{
		Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Use the edit tool."), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{{
			Name:        "edit",
			Description: "Edit a file.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"text":{"type":"string"}},"required":["path","text"]}`),
		}},
	}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonToolUse {
		t.Errorf("stopReason = %q, want toolUse", result.StopReason)
	}
	if result.ErrorMessage != "" {
		t.Errorf("errorMessage = %q, want empty", result.ErrorMessage)
	}

	var toolCall *ai.ToolCall
	for _, block := range result.Content {
		if tc, ok := block.(ai.ToolCall); ok {
			toolCall = &tc
			break
		}
	}
	if toolCall == nil {
		t.Fatalf("no tool call block found in %#v", result.Content)
	}
	want := map[string]any{"path": `A\H`, "text": "col1\tcol2"}
	got := map[string]any{"path": toolCall.Arguments["path"], "text": toolCall.Arguments["text"]}
	if got["path"] != want["path"] || got["text"] != want["text"] {
		t.Errorf("arguments = %#v, want %#v", toolCall.Arguments, want)
	}
}

func TestStream_PreservesRefusalStopDetails(t *testing.T) {
	explanation := "This request triggered restrictions on violative cyber content and was blocked under Anthropic's Usage Policy."
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_01XFUDYJgAACzvnptvVoYEL","usage":{"input_tokens":412,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber","explanation":"` + explanation + `"}},"usage":{"input_tokens":412,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("blocked request"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != explanation {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, explanation)
	}
}

func TestStream_IgnoresUnknownSSEEventsAfterMessageStop(t *testing.T) {
	events := append(minimalAnthropicEvents(),
		sseEvent{event: "done", data: "[DONE]"},
		sseEvent{event: "proxy.stats", data: "not json"},
	)
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello."), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Errorf("stopReason = %q, want stop", result.StopReason)
	}
	if result.ErrorMessage != "" {
		t.Errorf("errorMessage = %q, want empty", result.ErrorMessage)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 block", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello" {
		t.Errorf("content[0] = %#v, want text %q", result.Content[0], "Hello")
	}
}

func TestStream_StreamEndedBeforeMessageStopIsAnError(t *testing.T) {
	// message_start with no message_stop: iterateAnthropicEvents' upstream
	// equivalent throws "Anthropic stream ended before message_stop" — the
	// exact text ai/retry.go already classifies as retryable.
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":1,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "Anthropic stream ended before message_stop" {
		t.Errorf("errorMessage = %q", result.ErrorMessage)
	}
	if !ai.IsRetryableAssistantError(result) {
		t.Errorf("IsRetryableAssistantError(result) = false, want true")
	}
}

func TestStream_MissingAPIKeyProducesError(t *testing.T) {
	model := testModel("http://unused.invalid")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	want := "No API key for provider: anthropic"
	if result.ErrorMessage != want {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
}

func TestStream_ExplicitAuthorizationHeaderSkipsAPIKeyCheck(t *testing.T) {
	events := minimalAnthropicEvents()
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		Headers: ai.ProviderHeaders{"authorization": ai.HeaderValue("Bearer xyz")},
	})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Errorf("stopReason = %q, want stop (no auth error)", result.StopReason)
	}
}

// --- request building -------------------------------------------------------

func TestStream_SendsAPIKeyAndVersionHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test-key"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got := gotHeaders.Get("x-api-key"); got != "sk-ant-test-key" {
		t.Errorf("x-api-key = %q", got)
	}
	if got := gotHeaders.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q", got)
	}
	if got := gotHeaders.Get("accept"); got != "text/event-stream" {
		t.Errorf("accept = %q", got)
	}
}

func TestStream_RequestBodyMatchesExpectedShape(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{
		SystemPrompt: "Be concise.",
		Messages: []ai.Message{
			ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()},
		},
		Tools: []ai.Tool{{
			Name:        "search",
			Description: "Search the web.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		}},
	}
	temperature := 0.5
	maxTokens := 1024
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey:      "sk-ant-test",
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	want := map[string]any{
		"model":      "claude-haiku-4-5",
		"stream":     true,
		"max_tokens": float64(1024),
		"messages": []any{
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": "Hello", "cache_control": map[string]any{"type": "ephemeral"}},
			}},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "Be concise.", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"temperature": 0.5,
		"tools": []any{
			map[string]any{
				"name":                  "search",
				"description":           "Search the web.",
				"eager_input_streaming": true,
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"query": map[string]any{"type": "string"}},
					"required":   []any{"query"},
				},
				"cache_control": map[string]any{"type": "ephemeral"},
			},
		},
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(captured)
	var wantNorm, gotNorm any
	_ = json.Unmarshal(wantJSON, &wantNorm)
	_ = json.Unmarshal(gotJSON, &gotNorm)
	if string(mustJSON(wantNorm)) != string(mustJSON(gotNorm)) {
		t.Errorf("request body =\n%s\nwant\n%s", mustJSON(gotNorm), mustJSON(wantNorm))
	}
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

func TestStream_ConvertsImagesToolCallsAndThinkingHistory(t *testing.T) {
	var captured struct {
		Messages []map[string]any `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	now := time.Now().UnixMilli()
	chat := ai.Context{
		Messages: []ai.Message{
			ai.UserMessage{
				Content:   ai.UserBlocks(ai.TextContent{Text: "Look at this:"}, ai.ImageContent{Data: "YWJj", MimeType: "image/png"}),
				Timestamp: now,
			},
			&ai.AssistantMessage{
				Content: []ai.AssistantContentPart{
					ai.ThinkingContent{Thinking: "reasoning with signature", ThinkingSignature: "sig-1"},
					ai.ThinkingContent{Thinking: "reasoning without signature"},
					ai.TextContent{Text: "Here you go."},
					ai.ToolCall{ID: "toolu_1", Name: "search", Arguments: map[string]any{"query": "cats"}},
				},
				Api:        ai.ApiAnthropicMessages,
				Provider:   "anthropic",
				Model:      model.ID,
				StopReason: ai.StopReasonToolUse,
				Timestamp:  now,
			},
			ai.ToolResultMessage{
				ToolCallID: "toolu_1",
				ToolName:   "search",
				Content:    []ai.UserContentPart{ai.TextContent{Text: "cat pictures"}},
				Timestamp:  now,
			},
		},
	}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if len(captured.Messages) != 3 {
		t.Fatalf("messages = %#v, want 3", captured.Messages)
	}

	userBlocks, ok := captured.Messages[0]["content"].([]any)
	if !ok || len(userBlocks) != 2 {
		t.Fatalf("user content = %#v, want 2 blocks", captured.Messages[0]["content"])
	}
	imgBlock := userBlocks[1].(map[string]any)
	if imgBlock["type"] != "image" {
		t.Errorf("second user block = %#v, want image", imgBlock)
	}

	assistantBlocks, ok := captured.Messages[1]["content"].([]any)
	if !ok || len(assistantBlocks) != 4 {
		t.Fatalf("assistant content = %#v, want 4 blocks", captured.Messages[1]["content"])
	}
	thinkingWithSig := assistantBlocks[0].(map[string]any)
	if thinkingWithSig["type"] != "thinking" || thinkingWithSig["signature"] != "sig-1" {
		t.Errorf("block[0] = %#v, want thinking with signature", thinkingWithSig)
	}
	thinkingNoSig := assistantBlocks[1].(map[string]any)
	if thinkingNoSig["type"] != "text" || thinkingNoSig["text"] != "reasoning without signature" {
		t.Errorf("block[1] = %#v, want downgraded text", thinkingNoSig)
	}
	toolUse := assistantBlocks[3].(map[string]any)
	if toolUse["type"] != "tool_use" || toolUse["id"] != "toolu_1" || toolUse["name"] != "search" {
		t.Errorf("block[3] = %#v, want tool_use", toolUse)
	}

	toolResultMsg := captured.Messages[2]
	if toolResultMsg["role"] != "user" {
		t.Errorf("tool result message role = %v, want user", toolResultMsg["role"])
	}
	resultBlocks, ok := toolResultMsg["content"].([]any)
	if !ok || len(resultBlocks) != 1 {
		t.Fatalf("tool result content = %#v, want 1 block", toolResultMsg["content"])
	}
	resultBlock := resultBlocks[0].(map[string]any)
	if resultBlock["type"] != "tool_result" || resultBlock["tool_use_id"] != "toolu_1" || resultBlock["content"] != "cat pictures" {
		t.Errorf("tool_result block = %#v", resultBlock)
	}
}

func TestStream_OnPayloadCanReplaceRequestBody(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	var onPayloadModel *ai.Model
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey: "sk-ant-test",
		OnPayload: func(ctx context.Context, payload any, m *ai.Model) (any, error) {
			onPayloadModel = m
			return payload, nil
		},
	})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	if onPayloadModel == nil || onPayloadModel.ID != "claude-haiku-4-5" {
		t.Errorf("onPayloadModel = %#v", onPayloadModel)
	}
	if captured["model"] != "claude-haiku-4-5" {
		t.Errorf("captured model = %v", captured["model"])
	}
}

// --- cache_control -----------------------------------------------------------
//
// Ports: packages/ai/test/cache-retention.test.ts (Anthropic Provider
// describe block only; OpenAI Responses/Completions cases are out of scope).
// Upstream captures the payload via onPayload against a request that's
// allowed to fail past that point; these ports instead point at a real
// httptest SSE server so the request completes successfully, which is
// simpler and matches this package's existing test style.

// capturePayload runs model/chat/opts through Stream against a local SSE
// server that always succeeds, and returns the JSON request body it sent, as
// a generic map for flexible assertions.
func capturePayload(t *testing.T, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) map[string]any {
	t.Helper()
	srv := sseServer(t, minimalAnthropicEvents())
	m := *model
	m.BaseURL = srv.URL

	var captured any
	opts.OnPayload = func(_ context.Context, payload any, _ *ai.Model) (any, error) {
		captured = payload
		return payload, nil
	}

	stream := Stream(context.Background(), &m, chat, opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	raw, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured payload: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal captured payload: %v", err)
	}
	return out
}

// captureSimplePayload is capturePayload's StreamSimple counterpart.
func captureSimplePayload(t *testing.T, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) map[string]any {
	t.Helper()
	srv := sseServer(t, minimalAnthropicEvents())
	m := *model
	m.BaseURL = srv.URL

	var captured any
	opts.OnPayload = func(_ context.Context, payload any, _ *ai.Model) (any, error) {
		captured = payload
		return payload, nil
	}

	stream := StreamSimple(context.Background(), &m, chat, opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	raw, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured payload: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal captured payload: %v", err)
	}
	return out
}

func cacheControlContext() ai.Context {
	return ai.Context{
		SystemPrompt: "You are a helpful assistant.",
		Messages:     []ai.Message{ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}},
	}
}

func systemCacheControl(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	system, ok := payload["system"].([]any)
	if !ok || len(system) == 0 {
		t.Fatalf("system = %#v, want non-empty array", payload["system"])
	}
	block, ok := system[0].(map[string]any)
	if !ok {
		t.Fatalf("system[0] = %#v, want object", system[0])
	}
	cc, _ := block["cache_control"].(map[string]any)
	return cc
}

func TestBuildParams_CacheControlDefaultRetentionOmitsTTL(t *testing.T) {
	model := testModel("")
	payload := capturePayload(t, model, cacheControlContext(), &ai.StreamOptions{APIKey: "fake-key"})
	cc := systemCacheControl(t, payload)
	want := map[string]any{"type": "ephemeral"}
	if !mapsEqual(cc, want) {
		t.Errorf("system cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlLongRetentionAddsOneHourTTL(t *testing.T) {
	model := testModel("")
	payload := capturePayload(t, model, cacheControlContext(), &ai.StreamOptions{
		APIKey:         "fake-key",
		CacheRetention: ai.CacheRetentionLong,
	})
	cc := systemCacheControl(t, payload)
	want := map[string]any{"type": "ephemeral", "ttl": "1h"}
	if !mapsEqual(cc, want) {
		t.Errorf("system cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlLongRetentionOmitsTTLWhenModelDoesNotSupportIt(t *testing.T) {
	model := testModel("")
	no := false
	model.Compat = &ai.Compat{SupportsLongCacheRetention: &no}
	payload := capturePayload(t, model, cacheControlContext(), &ai.StreamOptions{
		APIKey:         "fake-key",
		CacheRetention: ai.CacheRetentionLong,
	})
	cc := systemCacheControl(t, payload)
	want := map[string]any{"type": "ephemeral"}
	if !mapsEqual(cc, want) {
		t.Errorf("system cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlNoneOmitsCacheControlEntirely(t *testing.T) {
	model := testModel("")
	chat := cacheControlContext()
	chat.Tools = []ai.Tool{{
		Name:        "search",
		Description: "Search the web.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
	}}
	payload := capturePayload(t, model, chat, &ai.StreamOptions{
		APIKey:         "fake-key",
		CacheRetention: ai.CacheRetentionNone,
	})

	system := payload["system"].([]any)
	sysBlock := system[0].(map[string]any)
	if _, has := sysBlock["cache_control"]; has {
		t.Errorf("system[0] = %#v, want no cache_control", sysBlock)
	}

	tools := payload["tools"].([]any)
	toolBlock := tools[0].(map[string]any)
	if _, has := toolBlock["cache_control"]; has {
		t.Errorf("tools[0] = %#v, want no cache_control", toolBlock)
	}

	messages := payload["messages"].([]any)
	lastMsg := messages[len(messages)-1].(map[string]any)
	if _, isString := lastMsg["content"].(string); !isString {
		t.Errorf("last message content = %#v, want plain string (untouched)", lastMsg["content"])
	}
}

func TestBuildParams_CacheControlAppliesToStringUserMessage(t *testing.T) {
	model := testModel("")
	payload := capturePayload(t, model, cacheControlContext(), &ai.StreamOptions{APIKey: "fake-key"})

	messages := payload["messages"].([]any)
	lastMsg := messages[len(messages)-1].(map[string]any)
	blocks, ok := lastMsg["content"].([]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("last message content = %#v, want 1-block array", lastMsg["content"])
	}
	block := blocks[0].(map[string]any)
	cc, _ := block["cache_control"].(map[string]any)
	want := map[string]any{"type": "ephemeral"}
	if !mapsEqual(cc, want) {
		t.Errorf("last block cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlEnvVarSelectsLongRetention(t *testing.T) {
	model := testModel("")
	payload := capturePayload(t, model, cacheControlContext(), &ai.StreamOptions{
		APIKey: "fake-key",
		Env:    ai.ProviderEnv{"PI_CACHE_RETENTION": "long"},
	})
	cc := systemCacheControl(t, payload)
	want := map[string]any{"type": "ephemeral", "ttl": "1h"}
	if !mapsEqual(cc, want) {
		t.Errorf("system cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlOnToolsAppliesToLastToolOnly(t *testing.T) {
	model := testModel("")
	chat := cacheControlContext()
	chat.Tools = []ai.Tool{
		{Name: "first", Description: "First tool.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "second", Description: "Second tool.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
	}
	payload := capturePayload(t, model, chat, &ai.StreamOptions{APIKey: "fake-key"})

	tools := payload["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools = %#v, want 2", tools)
	}
	first := tools[0].(map[string]any)
	if _, has := first["cache_control"]; has {
		t.Errorf("tools[0] = %#v, want no cache_control", first)
	}
	second := tools[1].(map[string]any)
	cc, _ := second["cache_control"].(map[string]any)
	want := map[string]any{"type": "ephemeral"}
	if !mapsEqual(cc, want) {
		t.Errorf("tools[1] cache_control = %#v, want %#v", cc, want)
	}
}

func TestBuildParams_CacheControlOnToolsOmittedWhenCompatDisallows(t *testing.T) {
	model := testModel("")
	no := false
	model.Compat = &ai.Compat{SupportsCacheControlOnTools: &no}
	chat := cacheControlContext()
	chat.Tools = []ai.Tool{{Name: "search", Description: "Search.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}}
	payload := capturePayload(t, model, chat, &ai.StreamOptions{APIKey: "fake-key"})

	tools := payload["tools"].([]any)
	tool := tools[0].(map[string]any)
	if _, has := tool["cache_control"]; has {
		t.Errorf("tools[0] = %#v, want no cache_control", tool)
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}

// --- adaptive thinking -------------------------------------------------------
//
// Ports: packages/ai/test/anthropic-force-adaptive-thinking.test.ts and
// anthropic-thinking-disable.test.ts, adapted to hand-built models (this
// package has no catalog yet — epic 11 — so tests that upstream drives via
// getModel("anthropic", "claude-...") use an equivalent custom Model here).

func thinkingModel(compat *ai.Compat) *ai.Model {
	return &ai.Model{
		ID:            "vendor--claude-opus-latest",
		Name:          "Vendor Proxy Opus Latest",
		Api:           ai.ApiAnthropicMessages,
		Provider:      "vendor-proxy",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 200000,
		MaxTokens:     32000,
		Reasoning:     true,
		Compat:        compat,
	}
}

func thinkingContext() ai.Context {
	return ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}
}

func TestStreamSimple_SendsLegacyThinkingPayloadByDefaultForCustomModel(t *testing.T) {
	model := thinkingModel(nil)
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingMedium,
	})
	thinking, ok := payload["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Errorf("thinking = %#v, want type=enabled", payload["thinking"])
	}
	if _, has := payload["output_config"]; has {
		t.Errorf("output_config = %#v, want absent", payload["output_config"])
	}
}

func TestStreamSimple_SendsAdaptiveThinkingPayloadWhenForceAdaptiveThinkingCompatTrue(t *testing.T) {
	yes := true
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingMedium,
	})
	want := map[string]any{"type": "adaptive", "display": "summarized"}
	if got, _ := payload["thinking"].(map[string]any); !mapsEqual(got, want) {
		t.Errorf("thinking = %#v, want %#v", payload["thinking"], want)
	}
	wantOutputConfig := map[string]any{"effort": "medium"}
	if got, _ := payload["output_config"].(map[string]any); !mapsEqual(got, wantOutputConfig) {
		t.Errorf("output_config = %#v, want %#v", payload["output_config"], wantOutputConfig)
	}
}

func TestStreamSimple_UsesExplicitThinkingLevelMapOverrideForEffort(t *testing.T) {
	yes := true
	xhigh := "xhigh"
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingXHigh: &xhigh}
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingXHigh,
	})
	wantOutputConfig := map[string]any{"effort": "xhigh"}
	if got, _ := payload["output_config"].(map[string]any); !mapsEqual(got, wantOutputConfig) {
		t.Errorf("output_config = %#v, want %#v", payload["output_config"], wantOutputConfig)
	}
}

func TestStreamSimple_XHighWithoutExplicitMapFallsBackToHighEffort(t *testing.T) {
	yes := true
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingXHigh,
	})
	wantOutputConfig := map[string]any{"effort": "high"}
	if got, _ := payload["output_config"].(map[string]any); !mapsEqual(got, wantOutputConfig) {
		t.Errorf("output_config = %#v, want %#v", payload["output_config"], wantOutputConfig)
	}
}

func TestStreamSimple_AllowsOptOutWithForceAdaptiveThinkingFalse(t *testing.T) {
	no := false
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &no})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingMedium,
	})
	thinking, ok := payload["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Errorf("thinking = %#v, want type=enabled", payload["thinking"])
	}
	if _, has := payload["output_config"]; has {
		t.Errorf("output_config = %#v, want absent", payload["output_config"])
	}
}

func TestStreamSimple_PreservesDisabledThinkingWhenReasoningOffRegardlessOfOverride(t *testing.T) {
	yes := true
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
	})
	want := map[string]any{"type": "disabled"}
	if got, _ := payload["thinking"].(map[string]any); !mapsEqual(got, want) {
		t.Errorf("thinking = %#v, want %#v", payload["thinking"], want)
	}
	if _, has := payload["output_config"]; has {
		t.Errorf("output_config = %#v, want absent", payload["output_config"])
	}
}

func TestStreamSimple_ThinkingDisabledForBudgetBasedModelWhenReasoningOff(t *testing.T) {
	model := thinkingModel(nil)
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
	})
	want := map[string]any{"type": "disabled"}
	if got, _ := payload["thinking"].(map[string]any); !mapsEqual(got, want) {
		t.Errorf("thinking = %#v, want %#v", payload["thinking"], want)
	}
}

func TestStreamSimple_OmitsDisabledThinkingWhenModelMarksOffUnsupported(t *testing.T) {
	yes := true
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: nil}
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
	})
	if _, has := payload["thinking"]; has {
		t.Errorf("thinking = %#v, want absent", payload["thinking"])
	}
	if _, has := payload["output_config"]; has {
		t.Errorf("output_config = %#v, want absent", payload["output_config"])
	}
}

func TestStreamSimple_AdaptiveThinkingForHighReasoning(t *testing.T) {
	yes := true
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key"},
		Reasoning:     ai.ThinkingHigh,
	})
	want := map[string]any{"type": "adaptive", "display": "summarized"}
	if got, _ := payload["thinking"].(map[string]any); !mapsEqual(got, want) {
		t.Errorf("thinking = %#v, want %#v", payload["thinking"], want)
	}
	wantOutputConfig := map[string]any{"effort": "high"}
	if got, _ := payload["output_config"].(map[string]any); !mapsEqual(got, wantOutputConfig) {
		t.Errorf("output_config = %#v, want %#v", payload["output_config"], wantOutputConfig)
	}
}

// TestStreamSimple_TemperatureOmittedWhenThinkingEnabled is a new (non-ported)
// test: upstream's buildParams guards `!options?.thinkingEnabled` before
// setting temperature, but no dedicated upstream test exercises it directly.
func TestStreamSimple_TemperatureOmittedWhenThinkingEnabled(t *testing.T) {
	yes := true
	temp := 0.7
	model := thinkingModel(&ai.Compat{ForceAdaptiveThinking: &yes})
	payload := captureSimplePayload(t, model, thinkingContext(), &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "fake-key", Temperature: &temp},
		Reasoning:     ai.ThinkingHigh,
	})
	if _, has := payload["temperature"]; has {
		t.Errorf("temperature = %#v, want absent when thinking is enabled", payload["temperature"])
	}
}

// --- empty thinking signature compat -----------------------------------------
//
// Ports: packages/ai/test/anthropic-empty-thinking-signature-compat.test.ts

func TestConvertAssistantBlocks_EmptySignatureThinkingDowngradesToTextByDefault(t *testing.T) {
	model := &ai.Model{ID: "mimo-v2.5-pro", Api: ai.ApiAnthropicMessages, Provider: "xiaomi-token-plan-ams", Reasoning: true}
	blocks := convertAssistantBlocks([]ai.AssistantContentPart{
		ai.ThinkingContent{Thinking: "internal reasoning", ThinkingSignature: ""},
	}, model, false)
	want := []map[string]any{{"type": "text", "text": "internal reasoning"}}
	if len(blocks) != 1 || !mapsEqual(blocks[0], want[0]) {
		t.Errorf("blocks = %#v, want %#v", blocks, want)
	}
}

func TestConvertAssistantBlocks_PreservesEmptySignatureThinkingWhenAllowEmptySignatureCompatEnabled(t *testing.T) {
	yes := true
	model := &ai.Model{
		ID: "mimo-v2.5-pro", Api: ai.ApiAnthropicMessages, Provider: "xiaomi-token-plan-ams", Reasoning: true,
		Compat: &ai.Compat{AllowEmptySignature: &yes},
	}
	// Upstream's fixture uses a whitespace-only signature (" ") to prove the
	// check trims before deciding "empty", not just a strict "" comparison.
	blocks := convertAssistantBlocks([]ai.AssistantContentPart{
		ai.ThinkingContent{Thinking: "internal reasoning", ThinkingSignature: " "},
	}, model, false)
	want := []map[string]any{{"type": "thinking", "thinking": "internal reasoning", "signature": ""}}
	if len(blocks) != 1 || !mapsEqual(blocks[0], want[0]) {
		t.Errorf("blocks = %#v, want %#v", blocks, want)
	}
}

// --- 1h cache-write cost ------------------------------------------------------
//
// Ports: packages/ai/test/anthropic-cache-write-1h-cost.test.ts

// costTestModel mirrors the comment in the upstream fixture: "claude-opus-4-8:
// input 5, cacheWrite (5m) 6.25 per Mtok. 1h write = 2x input = 10." This
// package has no catalog (epic 11) so the price sheet is hand-built here
// rather than fetched via getModel.
func costTestModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "claude-opus-4-8",
		Api:           ai.ApiAnthropicMessages,
		Provider:      "anthropic",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 200000,
		MaxTokens:     32000,
		Cost:          ai.ModelCost{Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	}
}

func eventsWithCacheCreation(cacheCreationJSON string) []sseEvent {
	startUsage := `"input_tokens":100,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000000`
	if cacheCreationJSON != "" {
		startUsage += `,"cache_creation":` + cacheCreationJSON
	}
	return []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{` + startUsage + `}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":100,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000000}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
}

func TestStream_PricesOneHourCacheWriteAtTwiceInputRate(t *testing.T) {
	events := eventsWithCacheCreation(`{"ephemeral_5m_input_tokens":600000,"ephemeral_1h_input_tokens":400000}`)
	srv := sseServer(t, events)
	model := costTestModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if result.Usage.CacheWrite != 1_000_000 {
		t.Errorf("cacheWrite = %d, want 1000000", result.Usage.CacheWrite)
	}
	if result.Usage.CacheWrite1h == nil || *result.Usage.CacheWrite1h != 400_000 {
		t.Errorf("cacheWrite1h = %v, want 400000", result.Usage.CacheWrite1h)
	}
	// 600k * 6.25/Mtok + 400k * (2*5)/Mtok = 3.75 + 4.0 = 7.75
	want := 7.75
	if diff := result.Usage.Cost.CacheWrite - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("cost.cacheWrite = %v, want %v", result.Usage.Cost.CacheWrite, want)
	}
}

func TestStream_CacheWriteFallsBackToFiveMinuteRateWhenNoBreakdownReported(t *testing.T) {
	events := eventsWithCacheCreation("")
	srv := sseServer(t, events)
	model := costTestModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if result.Usage.CacheWrite != 1_000_000 {
		t.Errorf("cacheWrite = %d, want 1000000", result.Usage.CacheWrite)
	}
	if result.Usage.CacheWrite1h == nil || *result.Usage.CacheWrite1h != 0 {
		t.Errorf("cacheWrite1h = %v, want 0", result.Usage.CacheWrite1h)
	}
	// 1M * 6.25/Mtok = 6.25
	want := 6.25
	if diff := result.Usage.Cost.CacheWrite - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("cost.cacheWrite = %v, want %v", result.Usage.Cost.CacheWrite, want)
	}
}

// --- thinking-block SSE decode ------------------------------------------------
//
// New Go-native tests: no upstream unit-test file exercises the raw SSE
// decode path for thinking content blocks in isolation (the upstream tests
// above all assert request-building via onPayload against a fake client that
// never streams thinking content back). These fill that gap directly against
// ai/internal/sse, mirroring the fidelity bar of anthropic-sse-parsing.test.ts
// for the non-thinking blocks issue 01 already ported.

func TestStream_DecodesThinkingBlockWithSignatureDeltas(t *testing.T) {
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me "}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think."}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-part-1"}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-part-2"}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`},
		{event: "content_block_delta", data: `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Answer."}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":1}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":12,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})

	var thinkingDeltaCount, thinkingStartCount, thinkingEndCount int
	for ev := range stream.Events(context.Background()) {
		switch ev.EventKind() {
		case ai.EventThinkingStart:
			thinkingStartCount++
		case ai.EventThinkingDelta:
			thinkingDeltaCount++
		case ai.EventThinkingEnd:
			thinkingEndCount++
		}
	}
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if thinkingStartCount != 1 || thinkingEndCount != 1 {
		t.Errorf("thinkingStart/End counts = %d/%d, want 1/1", thinkingStartCount, thinkingEndCount)
	}
	if thinkingDeltaCount != 2 {
		t.Errorf("thinkingDelta count = %d, want 2 (no event for signature_delta)", thinkingDeltaCount)
	}
	if len(result.Content) != 2 {
		t.Fatalf("content = %#v, want 2 blocks", result.Content)
	}
	thinking, ok := result.Content[0].(ai.ThinkingContent)
	if !ok {
		t.Fatalf("content[0] = %#v, want ThinkingContent", result.Content[0])
	}
	if thinking.Thinking != "Let me think." {
		t.Errorf("thinking.Thinking = %q, want %q", thinking.Thinking, "Let me think.")
	}
	if thinking.ThinkingSignature != "sig-part-1sig-part-2" {
		t.Errorf("thinking.ThinkingSignature = %q, want %q", thinking.ThinkingSignature, "sig-part-1sig-part-2")
	}
	if thinking.Redacted {
		t.Errorf("thinking.Redacted = true, want false")
	}
}

func TestStream_DecodesRedactedThinkingBlock(t *testing.T) {
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"opaque-payload"}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":12,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 block", result.Content)
	}
	thinking, ok := result.Content[0].(ai.ThinkingContent)
	if !ok {
		t.Fatalf("content[0] = %#v, want ThinkingContent", result.Content[0])
	}
	if !thinking.Redacted {
		t.Errorf("thinking.Redacted = false, want true")
	}
	if thinking.ThinkingSignature != "opaque-payload" {
		t.Errorf("thinking.ThinkingSignature = %q, want %q", thinking.ThinkingSignature, "opaque-payload")
	}
	if thinking.Thinking != "[Reasoning redacted]" {
		t.Errorf("thinking.Thinking = %q, want %q", thinking.Thinking, "[Reasoning redacted]")
	}
}
