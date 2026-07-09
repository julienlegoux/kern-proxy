package openaicompletions

// TestConvertMessages_ThinkingAsText* ports
// openai-completions-thinking-as-text.test.ts: when compat.requiresThinkingAsText
// is set, same-model thinking replay serializes as plain text content parts
// (never as a native thinking/reasoning field), since some providers mimic
// whatever structure they're shown.
//
// TestStream_PreservesReasoningDetailsArrivingBeforeToolCall ports
// openai-completions-reasoning-details.test.ts: encrypted reasoning_details
// that arrive before their matching tool-call id must still attach once the
// id becomes known, and must replay back onto the assistant message's
// reasoning_details field on the next turn.

import (
	"context"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func thinkingAsTextModel() *ai.Model {
	return &ai.Model{
		ID:            "repro-model",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "repro-provider",
		BaseURL:       "https://example.invalid",
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     4096,
		Compat:        &ai.Compat{RequiresThinkingAsText: boolp(true)},
	}
}

func replayContext(assistant *ai.AssistantMessage) ai.Context {
	return ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("hello"), Timestamp: 1},
			assistant,
			&ai.UserMessage{Content: ai.UserText("continue"), Timestamp: 3},
		},
	}
}

func TestConvertMessages_ThinkingAsTextReplay_ThinkingPlusText(t *testing.T) {
	model := thinkingAsTextModel()
	compat := getCompat(model)
	assistant := &ai.AssistantMessage{
		Content: []ai.AssistantContentPart{
			ai.ThinkingContent{Thinking: "internal reasoning"},
			ai.TextContent{Text: "visible answer"},
		},
		Api: ai.ApiOpenAICompletions, Provider: "repro-provider", Model: "repro-model", Timestamp: 2,
	}

	messages := convertMessages(replayContext(assistant), model, compat)
	if len(messages) != 3 {
		t.Fatalf("messages = %#v, want 3", messages)
	}
	parts, ok := messages[1].Content.([]wireContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("messages[1].Content = %#v, want 2 text parts", messages[1].Content)
	}
	if parts[0].Type != "text" || parts[0].Text != "internal reasoning" {
		t.Errorf("parts[0] = %#v, want text %q", parts[0], "internal reasoning")
	}
	if parts[1].Type != "text" || parts[1].Text != "visible answer" {
		t.Errorf("parts[1] = %#v, want text %q", parts[1], "visible answer")
	}
}

func TestConvertMessages_ThinkingAsTextReplay_ThinkingOnly(t *testing.T) {
	model := thinkingAsTextModel()
	compat := getCompat(model)
	assistant := &ai.AssistantMessage{
		Content: []ai.AssistantContentPart{ai.ThinkingContent{Thinking: "internal reasoning"}},
		Api:     ai.ApiOpenAICompletions, Provider: "repro-provider", Model: "repro-model", Timestamp: 2,
	}

	messages := convertMessages(replayContext(assistant), model, compat)
	parts, ok := messages[1].Content.([]wireContentPart)
	if !ok || len(parts) != 1 || parts[0].Text != "internal reasoning" {
		t.Errorf("messages[1].Content = %#v, want 1 text part %q", messages[1].Content, "internal reasoning")
	}
}

// TestConvertMessages_ThinkingSignatureReplay verifies the default (non
// thinking-as-text) path: thinking replays under its own signature field
// (reasoning_content/reasoning/reasoning_text), with visible text kept as a
// plain string in `content`.
func TestConvertMessages_ThinkingSignatureReplay(t *testing.T) {
	model := thinkingAsTextModel()
	model.Compat = nil // default (non thinking-as-text) path
	compat := getCompat(model)
	assistant := &ai.AssistantMessage{
		Content: []ai.AssistantContentPart{
			ai.ThinkingContent{Thinking: "internal reasoning", ThinkingSignature: "reasoning_content"},
			ai.TextContent{Text: "visible answer"},
		},
		Api: ai.ApiOpenAICompletions, Provider: "repro-provider", Model: "repro-model", Timestamp: 2,
	}

	messages := convertMessages(replayContext(assistant), model, compat)
	msg := messages[1]
	if msg.Content != "visible answer" {
		t.Errorf("content = %#v, want plain string %q", msg.Content, "visible answer")
	}
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "internal reasoning" {
		t.Errorf("reasoning_content = %v, want internal reasoning", msg.ReasoningContent)
	}
}

// TestConvertMessages_ForcesEmptyReasoningContentWhenRequired ports the
// requiresReasoningContentOnAssistantMessages compat flag: replayed assistant
// messages must carry an explicit (possibly empty) reasoning_content field
// when reasoning is enabled, even with no thinking blocks to replay.
func TestConvertMessages_ForcesEmptyReasoningContentWhenRequired(t *testing.T) {
	model := thinkingAsTextModel()
	model.Compat = &ai.Compat{RequiresReasoningContentOnAssistantMessages: boolp(true)}
	compat := getCompat(model)
	assistant := &ai.AssistantMessage{
		Content: []ai.AssistantContentPart{ai.TextContent{Text: "visible answer"}},
		Api:     ai.ApiOpenAICompletions, Provider: "repro-provider", Model: "repro-model", Timestamp: 2,
	}

	messages := convertMessages(replayContext(assistant), model, compat)
	msg := messages[1]
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "" {
		t.Errorf("reasoning_content = %v, want explicit empty string", msg.ReasoningContent)
	}
}

func reasoningDetailsModel() *ai.Model {
	return &ai.Model{
		ID:            "google/gemini-test",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openrouter",
		BaseURL:       "https://openrouter.ai/api/v1",
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 100000,
		MaxTokens:     4096,
	}
}

// TestStream_PreservesReasoningDetailsArrivingBeforeToolCall ports the
// upstream reasoning_details streaming test: reasoning_details for a tool
// call id arriving *before* the tool_calls delta itself must still attach to
// the finished ai.ToolCall (buffered, then matched once the id appears).
func TestStream_PreservesReasoningDetailsArrivingBeforeToolCall(t *testing.T) {
	chunks := []string{
		`{"id":"chatcmpl-test","choices":[{"index":0,"delta":{"reasoning_details":[{"type":"reasoning.encrypted","id":"call_1","data":"encrypted-signature"}]}}]}`,
		`{"id":"chatcmpl-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read","arguments":"{\"path\":\"README.md\"}"}}]}}]}`,
		`{"id":"chatcmpl-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`,
	}
	srv := sseServer(t, chunks)
	model := reasoningDetailsModel()
	model.BaseURL = srv.URL
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("go"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{{Name: "read", Description: "Read a file", Parameters: []byte(`{"type":"object"}`)}},
	}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	var toolCall *ai.ToolCall
	for _, block := range result.Content {
		if tc, ok := block.(ai.ToolCall); ok {
			toolCall = &tc
		}
	}
	if toolCall == nil {
		t.Fatalf("content = %#v, want a tool call block", result.Content)
	}
	wantSignature := `{"type":"reasoning.encrypted","id":"call_1","data":"encrypted-signature"}`
	if toolCall.ThoughtSignature != wantSignature {
		t.Errorf("thoughtSignature = %q, want %q", toolCall.ThoughtSignature, wantSignature)
	}

	// Replay: the next turn's assistant message must carry reasoning_details.
	// (apis.TransformMessages synthesizes a "No result provided" tool result
	// for the still-unresolved tool call, so look up the assistant message by
	// role rather than assuming it is the only entry.)
	chat2 := ai.Context{Messages: []ai.Message{result}}
	compat := getCompat(model)
	messages := convertMessages(chat2, model, compat)
	var assistantMsg *wireMessage
	for i := range messages {
		if messages[i].Role == "assistant" {
			assistantMsg = &messages[i]
		}
	}
	if assistantMsg == nil {
		t.Fatalf("messages = %#v, want a replayed assistant message", messages)
	}
	if len(assistantMsg.ReasoningDetails) != 1 {
		t.Fatalf("reasoning_details = %#v, want 1 entry", assistantMsg.ReasoningDetails)
	}
	if string(assistantMsg.ReasoningDetails[0]) != wantSignature {
		t.Errorf("reasoning_details[0] = %s, want %s", assistantMsg.ReasoningDetails[0], wantSignature)
	}
}

// TestStream_IgnoresMalformedReasoningDetail verifies a reasoning_details
// entry missing required fields (id/data) is silently skipped rather than
// corrupting the tool call or aborting the stream.
func TestStream_IgnoresMalformedReasoningDetail(t *testing.T) {
	chunks := []string{
		`{"id":"c1","choices":[{"index":0,"delta":{"reasoning_details":[{"type":"reasoning.encrypted","id":""}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read","arguments":"{}"}}]}}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`,
	}
	srv := sseServer(t, chunks)
	model := reasoningDetailsModel()
	model.BaseURL = srv.URL
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("go"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	tc, ok := result.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %#v, want ai.ToolCall", result.Content[0])
	}
	if tc.ThoughtSignature != "" {
		t.Errorf("thoughtSignature = %q, want empty (malformed detail dropped)", tc.ThoughtSignature)
	}
}
