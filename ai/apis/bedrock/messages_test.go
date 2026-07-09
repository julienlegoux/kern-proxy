package bedrock

// Ports test/bedrock-convert-messages.test.ts's blank/placeholder-handling
// assertions (the "skips unknown content type" cases have no Go equivalent
// input to construct -- see messages.go's package doc).

import (
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func testModel() *ai.Model {
	return &ai.Model{
		ID:            "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Name:          "Claude Sonnet 4.5",
		Api:           ai.ApiBedrockConverseStream,
		Provider:      "amazon-bedrock",
		BaseURL:       "https://bedrock-runtime.us-east-1.amazonaws.com",
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 200000,
		MaxTokens:     8192,
	}
}

func now() int64 { return time.Now().UnixMilli() }

func TestConvertMessages_PlainUserText(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText("hello"), Timestamp: now()},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(got))
	}
	if len(got[0].Content) != 1 || got[0].Content[0].Text != "hello" {
		t.Errorf("content = %+v, want [{Text: hello}]", got[0].Content)
	}
}

func TestConvertMessages_ReplacesBlankUserStringWithPlaceholder(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText("   "), Timestamp: now()},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 || len(got[0].Content) != 1 || got[0].Content[0].Text != emptyTextPlaceholder {
		t.Errorf("content = %+v, want [{Text: <empty>}]", got[0].Content)
	}
}

func TestConvertMessages_FiltersBlankUserTextBlocksWhenOtherContentRemains(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserBlocks(ai.TextContent{Text: ""}, ai.TextContent{Text: "hello"}), Timestamp: now()},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 || len(got[0].Content) != 1 || got[0].Content[0].Text != "hello" {
		t.Errorf("content = %+v, want [{Text: hello}]", got[0].Content)
	}
}

func TestConvertMessages_ReplacesUserContentEmptiedBySurrogateSanitizationWithPlaceholder(t *testing.T) {
	// WTF-8 for U+D83D (an unpaired high surrogate): Go strings are always
	// UTF-8 and cannot hold a raw surrogate code point directly (see
	// ai/sanitize_test.go's TestSanitizeSurrogates_UnpairedHighSurrogate).
	unpairedHighSurrogate := string([]byte{0xED, 0xA0, 0xBD})
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText(unpairedHighSurrogate), Timestamp: now()},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 || len(got[0].Content) != 1 || got[0].Content[0].Text != emptyTextPlaceholder {
		t.Errorf("content = %+v, want [{Text: <empty>}]", got[0].Content)
	}
}

func TestConvertMessages_SkipsAssistantTextBlocksEmptiedBySurrogateSanitization(t *testing.T) {
	unpairedHighSurrogate := string([]byte{0xED, 0xA0, 0xBD})
	msg := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{ai.TextContent{Text: unpairedHighSurrogate}},
		Api:        ai.ApiBedrockConverseStream,
		Provider:   "amazon-bedrock",
		Model:      testModel().ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  now(),
	}
	chat := ai.Context{Messages: []ai.Message{msg}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 0 {
		t.Errorf("len(messages) = %d, want 0 (assistant message with all-empty content dropped)", len(got))
	}
}

func TestConvertMessages_SkipsAssistantMessagesWithOnlyUnrecognizedContent(t *testing.T) {
	// Go's closed AssistantContentPart union can't hold an "unknown" block,
	// but an assistant message with genuinely empty content must still be
	// dropped entirely (upstream: "Bedrock rejects messages with empty
	// content arrays").
	msg := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        ai.ApiBedrockConverseStream,
		Provider:   "amazon-bedrock",
		Model:      testModel().ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  now(),
	}
	chat := ai.Context{Messages: []ai.Message{msg}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 0 {
		t.Errorf("len(messages) = %d, want 0", len(got))
	}
}

func TestConvertMessages_ReplacesBlankToolResultContentWithPlaceholder(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{
		&ai.ToolResultMessage{
			ToolCallID: "tool-1",
			ToolName:   "tool",
			Content:    []ai.UserContentPart{ai.TextContent{Text: ""}},
			IsError:    false,
			Timestamp:  now(),
		},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(got))
	}
	tr := got[0].Content[0].ToolResult
	if tr == nil || len(tr.Content) != 1 || tr.Content[0].Text != emptyTextPlaceholder {
		t.Errorf("toolResult content = %+v, want [{Text: <empty>}]", tr)
	}
}

func TestConvertMessages_MergesConsecutiveToolResultsIntoOneMessage(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{
		&ai.ToolResultMessage{ToolCallID: "t1", Content: []ai.UserContentPart{ai.TextContent{Text: "a"}}, Timestamp: now()},
		&ai.ToolResultMessage{ToolCallID: "t2", Content: []ai.UserContentPart{ai.TextContent{Text: "b"}}, Timestamp: now()},
	}}
	got := convertMessages(chat, testModel(), ai.CacheRetentionNone, nil)
	if len(got) != 1 {
		t.Fatalf("len(messages) = %d, want 1 (merged)", len(got))
	}
	if len(got[0].Content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(got[0].Content))
	}
	if got[0].Content[0].ToolResult.ToolUseID != "t1" || got[0].Content[1].ToolResult.ToolUseID != "t2" {
		t.Errorf("tool use ids = %q, %q, want t1, t2", got[0].Content[0].ToolResult.ToolUseID, got[0].Content[1].ToolResult.ToolUseID)
	}
}
