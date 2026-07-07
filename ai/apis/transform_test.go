package apis

// Ports: packages/ai/test/transform-messages-copilot-openai-to-anthropic.test.ts
// Ports: packages/ai/test/lax-message-content.test.ts

import (
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// anthropicNormalizeToolCallID mirrors the normalizeToolCallId used by
// anthropic-messages.ts (and the TS test's local helper of the same name):
// Anthropic tool-call ids must match ^[a-zA-Z0-9_-]+$ and be at most 64 chars,
// so cross-model replay into Anthropic scrubs+truncates the incoming id.
func anthropicNormalizeToolCallID(id string, _ *ai.Model, _ *ai.AssistantMessage) string {
	out := make([]rune, 0, len(id))
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return string(out)
}

func makeCopilotClaudeModel() *ai.Model {
	return &ai.Model{
		ID:            "claude-sonnet-4.6",
		Name:          "Claude Sonnet 4.6",
		Api:           ai.ApiAnthropicMessages,
		Provider:      "github-copilot",
		BaseURL:       "https://api.individual.githubcopilot.com",
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 128000,
		MaxTokens:     16000,
	}
}

func makeAssistantMessage(content []ai.AssistantContentPart) *ai.AssistantMessage {
	return &ai.AssistantMessage{
		Content:    content,
		Api:        ai.ApiOpenAIResponses,
		Provider:   "github-copilot",
		Model:      "gpt-5",
		Usage:      ai.Usage{},
		StopReason: ai.StopReasonToolUse,
		Timestamp:  time.Now().UnixMilli(),
	}
}

func findAssistant(t *testing.T, messages []ai.Message) *ai.AssistantMessage {
	t.Helper()
	for _, m := range messages {
		if a, ok := m.(*ai.AssistantMessage); ok {
			return a
		}
	}
	t.Fatalf("no assistant message found in %#v", messages)
	return nil
}

func TestTransformMessagesConvertsThinkingToTextAcrossModels(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("hello"), Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Content: []ai.AssistantContentPart{
				ai.ThinkingContent{Thinking: "Let me think about this...", ThinkingSignature: "reasoning_content"},
				ai.TextContent{Text: "Hi there!"},
			},
			Api:        ai.ApiOpenAICompletions,
			Provider:   "github-copilot",
			Model:      "gpt-4o",
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, anthropicNormalizeToolCallID)
	assistant := findAssistant(t, result)

	var textBlocks, thinkingBlocks int
	for _, b := range assistant.Content {
		switch b.(type) {
		case ai.TextContent:
			textBlocks++
		case ai.ThinkingContent:
			thinkingBlocks++
		}
	}
	if thinkingBlocks != 0 {
		t.Errorf("thinkingBlocks = %d, want 0", thinkingBlocks)
	}
	if textBlocks < 2 {
		t.Errorf("textBlocks = %d, want >= 2", textBlocks)
	}
}

func TestTransformMessagesRemovesThoughtSignatureAcrossModels(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("run a command"), Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Content: []ai.AssistantContentPart{
				ai.ToolCall{
					ID:               "call_123",
					Name:             "bash",
					Arguments:        map[string]any{"command": "ls"},
					ThoughtSignature: `{"type":"reasoning.encrypted","id":"call_123","data":"encrypted"}`,
				},
			},
			Api:        ai.ApiOpenAIResponses,
			Provider:   "github-copilot",
			Model:      "gpt-5",
			StopReason: ai.StopReasonToolUse,
			Timestamp:  time.Now().UnixMilli(),
		},
		ai.ToolResultMessage{
			ToolCallID: "call_123",
			ToolName:   "bash",
			Content:    []ai.UserContentPart{ai.TextContent{Text: "output"}},
			IsError:    false,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, anthropicNormalizeToolCallID)
	assistant := findAssistant(t, result)

	var toolCall *ai.ToolCall
	for _, b := range assistant.Content {
		if tc, ok := b.(ai.ToolCall); ok {
			toolCall = &tc
		}
	}
	if toolCall == nil {
		t.Fatalf("no tool call found in transformed assistant content")
	}
	if toolCall.ThoughtSignature != "" {
		t.Errorf("ThoughtSignature = %q, want empty", toolCall.ThoughtSignature)
	}
}

func TestTransformMessagesSynthesizesToolResultForTrailingOrphan(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("read the file"), Timestamp: time.Now().UnixMilli()},
		makeAssistantMessage([]ai.AssistantContentPart{
			ai.ToolCall{ID: "call_123|fc_123", Name: "read", Arguments: map[string]any{"path": "README.md"}},
		}),
	}

	result := TransformMessages(messages, model, anthropicNormalizeToolCallID)
	last := result[len(result)-1]

	toolResult, ok := last.(ai.ToolResultMessage)
	if !ok {
		t.Fatalf("last message = %#v, want ToolResultMessage", last)
	}
	if toolResult.ToolCallID != "call_123_fc_123" {
		t.Errorf("ToolCallID = %q, want %q", toolResult.ToolCallID, "call_123_fc_123")
	}
	if toolResult.ToolName != "read" {
		t.Errorf("ToolName = %q, want %q", toolResult.ToolName, "read")
	}
	if !toolResult.IsError {
		t.Errorf("IsError = false, want true")
	}
	if len(toolResult.Content) != 1 {
		t.Fatalf("Content = %#v, want 1 block", toolResult.Content)
	}
	text, ok := toolResult.Content[0].(ai.TextContent)
	if !ok || text.Text != "No result provided" {
		t.Errorf("Content[0] = %#v, want text %q", toolResult.Content[0], "No result provided")
	}
}

func TestTransformMessagesSynthesizesOnlyForMissingResults(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("run commands"), Timestamp: time.Now().UnixMilli()},
		makeAssistantMessage([]ai.AssistantContentPart{
			ai.ToolCall{ID: "call_1|fc_1", Name: "read", Arguments: map[string]any{"path": "README.md"}},
			ai.ToolCall{ID: "call_2|fc_2", Name: "bash", Arguments: map[string]any{"command": "pwd"}},
		}),
		ai.ToolResultMessage{
			ToolCallID: "call_1|fc_1",
			ToolName:   "read",
			Content:    []ai.UserContentPart{ai.TextContent{Text: "done"}},
			IsError:    false,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, anthropicNormalizeToolCallID)

	var synthetic []ai.ToolResultMessage
	for _, m := range result {
		if tr, ok := m.(ai.ToolResultMessage); ok && tr.IsError {
			synthetic = append(synthetic, tr)
		}
	}
	if len(synthetic) != 1 {
		t.Fatalf("synthetic results = %#v, want exactly 1", synthetic)
	}
	if synthetic[0].ToolCallID != "call_2_fc_2" || synthetic[0].ToolName != "bash" {
		t.Errorf("synthetic[0] = %#v, want ToolCallID=call_2_fc_2 ToolName=bash", synthetic[0])
	}
}

func makeTextOnlyModel() *ai.Model {
	return &ai.Model{
		ID:            "test-model",
		Name:          "Test Model",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openai",
		BaseURL:       "https://example.invalid/v1",
		Reasoning:     false,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     16000,
	}
}

// TestTransformMessagesNormalizesNilContentToEmpty locks in the "lax message
// content" contract: untyped callers (custom tools, hand-built histories, old
// session files) may leave Content at its Go zero value (nil). transformMessages
// is the choke point before every provider request and must not panic on that
// input; it normalizes nil content to an explicit empty value instead.
func TestTransformMessagesNormalizesNilContentToEmpty(t *testing.T) {
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserContent{}, Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Api:        ai.ApiOpenAICompletions,
			Provider:   "openai",
			Model:      "test-model",
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
		ai.ToolResultMessage{
			ToolCallID: "call_1",
			ToolName:   "web_search",
			IsError:    false,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, makeTextOnlyModel(), nil)
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}
	for i, m := range result {
		switch v := m.(type) {
		case ai.UserMessage:
			if len(v.Content.Blocks) != 0 {
				t.Errorf("message[%d] UserMessage.Content.Blocks = %#v, want empty", i, v.Content.Blocks)
			}
		case *ai.AssistantMessage:
			if len(v.Content) != 0 {
				t.Errorf("message[%d] AssistantMessage.Content = %#v, want empty", i, v.Content)
			}
		case ai.ToolResultMessage:
			if len(v.Content) != 0 {
				t.Errorf("message[%d] ToolResultMessage.Content = %#v, want empty", i, v.Content)
			}
		default:
			t.Errorf("message[%d] has unexpected type %T", i, m)
		}
	}
}

func TestTransformMessagesKeepsThinkingAndSignatureForSameModel(t *testing.T) {
	model := &ai.Model{ID: "claude-x", Api: ai.ApiAnthropicMessages, Provider: "anthropic", Input: []ai.Modality{ai.ModalityText}}
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Content: []ai.AssistantContentPart{
				ai.ThinkingContent{Thinking: "reasoning...", ThinkingSignature: "sig-1"},
				ai.TextContent{Text: "answer"},
			},
			Api:        ai.ApiAnthropicMessages,
			Provider:   "anthropic",
			Model:      "claude-x",
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, nil)
	assistant := findAssistant(t, result)

	found := false
	for _, b := range assistant.Content {
		if th, ok := b.(ai.ThinkingContent); ok {
			found = true
			if th.ThinkingSignature != "sig-1" {
				t.Errorf("ThinkingSignature = %q, want %q", th.ThinkingSignature, "sig-1")
			}
			if th.Thinking != "reasoning..." {
				t.Errorf("Thinking = %q, want %q", th.Thinking, "reasoning...")
			}
		}
	}
	if !found {
		t.Errorf("same-model thinking block was dropped, want it kept with signature")
	}
}

func TestTransformMessagesDropsRedactedThinkingAcrossModelsKeepsSameModel(t *testing.T) {
	model := makeCopilotClaudeModel()
	redacted := ai.ThinkingContent{Redacted: true, ThinkingSignature: "opaque"}

	crossModel := []ai.Message{
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{redacted, ai.TextContent{Text: "answer"}},
			Api:        ai.ApiOpenAICompletions,
			Provider:   "github-copilot",
			Model:      "gpt-4o",
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
	}
	result := TransformMessages(crossModel, model, nil)
	assistant := findAssistant(t, result)
	for _, b := range assistant.Content {
		if _, ok := b.(ai.ThinkingContent); ok {
			t.Errorf("redacted thinking block survived cross-model transform: %#v", b)
		}
	}

	sameModel := []ai.Message{
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{redacted},
			Api:        model.Api,
			Provider:   model.Provider,
			Model:      model.ID,
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
	}
	result = TransformMessages(sameModel, model, nil)
	assistant = findAssistant(t, result)
	if len(assistant.Content) != 1 {
		t.Fatalf("same-model content = %#v, want redacted thinking block kept", assistant.Content)
	}
	if _, ok := assistant.Content[0].(ai.ThinkingContent); !ok {
		t.Errorf("same-model content[0] = %#v, want ThinkingContent", assistant.Content[0])
	}
}

func TestTransformMessagesSkipsErroredAndAbortedAssistantTurns(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.TextContent{Text: "partial"}},
			Api:        ai.ApiOpenAICompletions,
			Provider:   "github-copilot",
			Model:      "gpt-4o",
			StopReason: ai.StopReasonError,
			Timestamp:  time.Now().UnixMilli(),
		},
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.TextContent{Text: "aborted turn"}},
			Api:        ai.ApiOpenAICompletions,
			Provider:   "github-copilot",
			Model:      "gpt-4o",
			StopReason: ai.StopReasonAborted,
			Timestamp:  time.Now().UnixMilli(),
		},
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.TextContent{Text: "final answer"}},
			Api:        ai.ApiOpenAICompletions,
			Provider:   "github-copilot",
			Model:      "gpt-4o",
			StopReason: ai.StopReasonStop,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, nil)

	var assistants []*ai.AssistantMessage
	for _, m := range result {
		if a, ok := m.(*ai.AssistantMessage); ok {
			assistants = append(assistants, a)
		}
	}
	if len(assistants) != 1 {
		t.Fatalf("assistants = %d, want exactly 1 (errored/aborted turns dropped)", len(assistants))
	}
	if assistants[0].StopReason != ai.StopReasonStop {
		t.Errorf("surviving assistant StopReason = %q, want %q", assistants[0].StopReason, ai.StopReasonStop)
	}
}

func TestTransformMessagesDowngradesUnsupportedImagesToPlaceholder(t *testing.T) {
	model := &ai.Model{ID: "text-only", Api: ai.ApiOpenAICompletions, Provider: "openai", Input: []ai.Modality{ai.ModalityText}}
	messages := []ai.Message{
		ai.UserMessage{
			Content: ai.UserBlocks(
				ai.ImageContent{Data: "aaaa", MimeType: "image/png"},
				ai.ImageContent{Data: "bbbb", MimeType: "image/png"},
				ai.TextContent{Text: "what is this?"},
			),
			Timestamp: time.Now().UnixMilli(),
		},
		ai.ToolResultMessage{
			ToolCallID: "call_1",
			ToolName:   "screenshot",
			Content:    []ai.UserContentPart{ai.ImageContent{Data: "cccc", MimeType: "image/png"}},
			IsError:    false,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, nil)

	user, ok := result[0].(ai.UserMessage)
	if !ok {
		t.Fatalf("result[0] = %#v, want UserMessage", result[0])
	}
	// Two consecutive images collapse into a single placeholder block.
	if len(user.Content.Blocks) != 2 {
		t.Fatalf("UserMessage content = %#v, want 2 blocks (1 placeholder + 1 text)", user.Content.Blocks)
	}
	placeholder, ok := user.Content.Blocks[0].(ai.TextContent)
	if !ok || placeholder.Text != "(image omitted: model does not support images)" {
		t.Errorf("Content.Blocks[0] = %#v, want the user image placeholder", user.Content.Blocks[0])
	}
	trailing, ok := user.Content.Blocks[1].(ai.TextContent)
	if !ok || trailing.Text != "what is this?" {
		t.Errorf("Content.Blocks[1] = %#v, want the original text block", user.Content.Blocks[1])
	}

	toolResult, ok := result[1].(ai.ToolResultMessage)
	if !ok {
		t.Fatalf("result[1] = %#v, want ToolResultMessage", result[1])
	}
	if len(toolResult.Content) != 1 {
		t.Fatalf("ToolResultMessage content = %#v, want 1 block", toolResult.Content)
	}
	toolPlaceholder, ok := toolResult.Content[0].(ai.TextContent)
	if !ok || toolPlaceholder.Text != "(tool image omitted: model does not support images)" {
		t.Errorf("tool result content[0] = %#v, want the tool image placeholder", toolResult.Content[0])
	}
}

func TestTransformMessagesWithoutNormalizeCallbackKeepsToolCallIDsUnchanged(t *testing.T) {
	model := makeCopilotClaudeModel()
	messages := []ai.Message{
		ai.UserMessage{Content: ai.UserText("run a command"), Timestamp: time.Now().UnixMilli()},
		&ai.AssistantMessage{
			Content: []ai.AssistantContentPart{
				ai.ToolCall{ID: "call_123|fc_123", Name: "bash", Arguments: map[string]any{"command": "ls"}},
			},
			Api:        ai.ApiOpenAIResponses,
			Provider:   "github-copilot",
			Model:      "gpt-5",
			StopReason: ai.StopReasonToolUse,
			Timestamp:  time.Now().UnixMilli(),
		},
		ai.ToolResultMessage{
			ToolCallID: "call_123|fc_123",
			ToolName:   "bash",
			Content:    []ai.UserContentPart{ai.TextContent{Text: "output"}},
			IsError:    false,
			Timestamp:  time.Now().UnixMilli(),
		},
	}

	result := TransformMessages(messages, model, nil)
	assistant := findAssistant(t, result)
	toolCall, ok := assistant.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("assistant.Content[0] = %#v, want ToolCall", assistant.Content[0])
	}
	if toolCall.ID != "call_123|fc_123" {
		t.Errorf("ToolCall.ID = %q, want unchanged %q", toolCall.ID, "call_123|fc_123")
	}

	toolResult, ok := result[2].(ai.ToolResultMessage)
	if !ok {
		t.Fatalf("result[2] = %#v, want ToolResultMessage", result[2])
	}
	if toolResult.ToolCallID != "call_123|fc_123" {
		t.Errorf("ToolResultMessage.ToolCallID = %q, want unchanged %q", toolResult.ToolCallID, "call_123|fc_123")
	}
}
