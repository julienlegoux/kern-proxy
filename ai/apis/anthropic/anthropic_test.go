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
			map[string]any{"role": "user", "content": "Hello"},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "Be concise."},
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
