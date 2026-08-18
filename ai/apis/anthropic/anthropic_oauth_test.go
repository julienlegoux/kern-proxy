package anthropic

// Ports: packages/ai/src/api/anthropic-messages.ts (the OAuth branch of
// createClient: isOAuthToken, the claude-cli identity headers/beta list, and
// buildParams' isOAuthToken system-block/tool-name branches) plus
// packages/ai/test/anthropic-tool-name-normalization.test.ts (toClaudeCodeName
// / fromClaudeCodeName round-tripping). Upstream's own coverage for this mode
// is either folded into the untested createClient branch or gated behind a
// live OAuth credential (test/anthropic-tool-name-normalization.test.ts uses
// describe.skipIf(!oauthToken)); these are new offline golden-request Go
// tests covering the same fidelity bar via captured request bodies/headers,
// matching the convention already established in anthropic_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

const oauthTestToken = "sk-ant-oat01-test-token"

func TestStream_OAuthTokenUsesBearerAuthInsteadOfAPIKeyHeader(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got := gotHeaders.Get("x-api-key"); got != "" {
		t.Errorf("x-api-key = %q, want empty for OAuth token", got)
	}
	if got := gotHeaders.Get("authorization"); got != "Bearer "+oauthTestToken {
		t.Errorf("authorization = %q, want %q", got, "Bearer "+oauthTestToken)
	}
}

func TestStream_OAuthTokenSetsImpersonationHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got := gotHeaders.Get("anthropic-dangerous-direct-browser-access"); got != "true" {
		t.Errorf("anthropic-dangerous-direct-browser-access = %q, want true", got)
	}
	if got := gotHeaders.Get("user-agent"); got != "claude-cli/2.1.75" {
		t.Errorf("user-agent = %q, want claude-cli/2.1.75", got)
	}
	if got := gotHeaders.Get("x-app"); got != "cli" {
		t.Errorf("x-app = %q, want cli", got)
	}
	if got := gotHeaders.Get("anthropic-beta"); got != "claude-code-20250219,oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q, want claude-code-20250219,oauth-2025-04-20", got)
	}
}

func TestStream_OAuthTokenCombinesFineGrainedToolStreamingBetaWithImpersonationBetas(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	notSupported := false
	model.Compat = &ai.Compat{SupportsEagerToolInputStreaming: &notSupported}
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{{Name: "search", Description: "Search.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	want := "claude-code-20250219,oauth-2025-04-20,fine-grained-tool-streaming-2025-05-14"
	if got := gotHeaders.Get("anthropic-beta"); got != want {
		t.Errorf("anthropic-beta = %q, want %q", got, want)
	}
}

func TestStream_OAuthTokenInjectsIdentitySystemBlockBeforeUserSystemPrompt(t *testing.T) {
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
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	want := []any{
		map[string]any{"type": "text", "text": "You are Claude Code, Anthropic's official CLI for Claude.", "cache_control": map[string]any{"type": "ephemeral"}},
		map[string]any{"type": "text", "text": "Be concise.", "cache_control": map[string]any{"type": "ephemeral"}},
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(captured["system"])
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("system = %s, want %s", gotJSON, wantJSON)
	}
}

func TestStream_OAuthTokenInjectsIdentitySystemBlockWhenNoUserSystemPrompt(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	systemBlocks, ok := captured["system"].([]any)
	if !ok || len(systemBlocks) != 1 {
		t.Fatalf("system = %#v, want 1 identity block", captured["system"])
	}
	block := systemBlocks[0].(map[string]any)
	if block["text"] != "You are Claude Code, Anthropic's official CLI for Claude." {
		t.Errorf("system[0].text = %q", block["text"])
	}
}

func TestStream_APIKeyRequestsAreUnaffectedByOAuthImpersonation(t *testing.T) {
	var gotHeaders http.Header
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-api03-test"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got := gotHeaders.Get("x-api-key"); got != "sk-ant-api03-test" {
		t.Errorf("x-api-key = %q, want the plain API key", got)
	}
	if got := gotHeaders.Get("authorization"); got != "" {
		t.Errorf("authorization = %q, want empty for API-key auth", got)
	}
	if got := gotHeaders.Get("anthropic-dangerous-direct-browser-access"); got != "" {
		t.Errorf("anthropic-dangerous-direct-browser-access = %q, want empty", got)
	}
	if got := gotHeaders.Get("x-app"); got != "" {
		t.Errorf("x-app = %q, want empty", got)
	}
	if _, hasSystem := captured["system"]; hasSystem {
		t.Errorf("system = %#v, want no system block (no systemPrompt, no OAuth identity)", captured["system"])
	}
}

func TestConvertTools_OAuthTokenRemapsMatchingToolNamesToClaudeCodeCasing(t *testing.T) {
	var captured struct {
		Tools []map[string]any `json:"tools"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{
			{Name: "read", Description: "Read a file.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
			{Name: "find", Description: "Find files.", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
		},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if len(captured.Tools) != 2 {
		t.Fatalf("tools = %#v, want 2", captured.Tools)
	}
	if captured.Tools[0]["name"] != "Read" {
		t.Errorf(`tools[0].name = %q, want "Read" (matches CC casing)`, captured.Tools[0]["name"])
	}
	if captured.Tools[1]["name"] != "find" {
		t.Errorf(`tools[1].name = %q, want "find" unchanged (no CC tool named "Find")`, captured.Tools[1]["name"])
	}
}

func TestConvertAssistantBlocks_OAuthTokenRemapsHistoricalToolCallNameOutbound(t *testing.T) {
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
			&ai.UserMessage{Content: ai.UserText("Add a todo"), Timestamp: now},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "toolu_1", Name: "todowrite", Arguments: map[string]any{"task": "buy milk"}}},
				Api:        ai.ApiAnthropicMessages,
				Provider:   "anthropic",
				Model:      model.ID,
				StopReason: ai.StopReasonToolUse,
				Timestamp:  now,
			},
			&ai.ToolResultMessage{ToolCallID: "toolu_1", ToolName: "todowrite", Content: []ai.UserContentPart{ai.TextContent{Text: "ok"}}, Timestamp: now},
		},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	assistantBlocks, ok := captured.Messages[1]["content"].([]any)
	if !ok || len(assistantBlocks) != 1 {
		t.Fatalf("assistant content = %#v, want 1 block", captured.Messages[1]["content"])
	}
	toolUse := assistantBlocks[0].(map[string]any)
	if toolUse["name"] != "TodoWrite" {
		t.Errorf(`tool_use.name = %q, want "TodoWrite"`, toolUse["name"])
	}
}

func TestStream_OAuthTokenRemapsInboundToolCallNameBackToOriginalCasing(t *testing.T) {
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":5,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_test","name":"TodoWrite","input":{}}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":5,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Add a todo: buy milk. Use the todowrite tool."), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{{
			Name:        "todowrite",
			Description: "Write a todo item",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"task":{"type":"string"}}}`),
		}},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	var toolCall *ai.ToolCall
	for _, block := range result.Content {
		if tc, ok := block.(ai.ToolCall); ok {
			toolCall = &tc
			break
		}
	}
	if toolCall == nil {
		t.Fatalf("no tool call in %#v", result.Content)
	}
	if toolCall.Name != "todowrite" {
		t.Errorf("toolCall.Name = %q, want %q (original user casing)", toolCall.Name, "todowrite")
	}
}

func TestStream_OAuthTokenLeavesInboundToolCallNameUnchangedWhenNoMatchingUserTool(t *testing.T) {
	events := []sseEvent{
		{event: "message_start", data: `{"type":"message_start","message":{"id":"msg_test","usage":{"input_tokens":5,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`},
		{event: "content_block_start", data: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_test","name":"Bash","input":{}}}`},
		{event: "content_block_stop", data: `{"type":"content_block_stop","index":0}`},
		{event: "message_delta", data: `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":5,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`},
		{event: "message_stop", data: `{"type":"message_stop"}`},
	}
	srv := sseServer(t, events)
	model := testModel(srv.URL)
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("run a command"), Timestamp: time.Now().UnixMilli()}},
	}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: oauthTestToken})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}

	var toolCall *ai.ToolCall
	for _, block := range result.Content {
		if tc, ok := block.(ai.ToolCall); ok {
			toolCall = &tc
			break
		}
	}
	if toolCall == nil {
		t.Fatalf("no tool call in %#v", result.Content)
	}
	if toolCall.Name != "Bash" {
		t.Errorf("toolCall.Name = %q, want %q unchanged (no matching user tool)", toolCall.Name, "Bash")
	}
}
