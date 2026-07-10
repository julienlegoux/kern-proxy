package apis

// Ports: packages/ai/src/api/transform-messages.ts

import (
	"strings"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

const (
	nonVisionUserImagePlaceholder = "(image omitted: model does not support images)"
	nonVisionToolImagePlaceholder = "(tool image omitted: model does not support images)"
)

// NormalizeToolCallID adapts a tool-call id emitted by one provider so it is
// acceptable to another (e.g. OpenAI Responses generates 400+ char ids with
// `|` separators; Anthropic requires ids matching ^[a-zA-Z0-9_-]+$, max 64
// chars). model is the replay target; source is the (pre-transform) assistant
// message that produced the call. A nil NormalizeToolCallID disables
// cross-model id remapping.
type NormalizeToolCallID func(id string, model *ai.Model, source *ai.AssistantMessage) string

// TransformMessages normalizes a conversation before replaying it against
// model:
//
//   - Content left at its Go zero value (nil) by untyped callers (custom
//     tools, hand-built histories, old session files) is normalized to an
//     explicit empty value so downstream code can rely on the type contract.
//   - Images are downgraded to text placeholders when model doesn't accept
//     image input; consecutive images collapse into a single placeholder.
//   - Thinking blocks are kept (with signatures, for replay) when the
//     message's originating model matches model exactly; otherwise redacted
//     thinking is dropped and plain thinking is downgraded to text.
//   - Tool-call ids are remapped via normalizeToolCallID when replaying
//     cross-model (nil disables remapping); the Google-specific
//     ThoughtSignature is stripped cross-model.
//   - Assistant turns that ended in StopReasonError or StopReasonAborted are
//     skipped entirely: they are incomplete and unsafe to replay.
//   - Tool calls left without a matching tool result (because the assistant
//     turn was skipped, or the conversation was truncated) get a synthetic
//     "No result provided" error result so provider APIs that require a
//     result for every call still see one.
func TransformMessages(messages []ai.Message, model *ai.Model, normalizeToolCallID NormalizeToolCallID) []ai.Message {
	normalized := normalizeNilContent(messages)
	imageAware := downgradeUnsupportedImages(normalized, model)

	toolCallIDMap := make(map[string]string)
	transformed := make([]ai.Message, len(imageAware))
	for i, msg := range imageAware {
		transformed[i] = transformOne(msg, model, toolCallIDMap, normalizeToolCallID)
	}

	return insertOrphanToolResults(transformed)
}

// normalizeNilContent replaces a nil Content value with an explicit empty one
// so later passes never have to special-case "no content".
//
// Every branch that rewrites content copies the message first. Messages are
// pointers, so writing through one would edit the caller's own session history
// -- TransformMessages is a pure function over the slice it is handed.
func normalizeNilContent(messages []ai.Message) []ai.Message {
	out := make([]ai.Message, len(messages))
	for i, msg := range messages {
		switch m := msg.(type) {
		case *ai.UserMessage:
			if m.Content.Plain == nil && m.Content.Blocks == nil {
				clone := *m
				clone.Content.Blocks = []ai.UserContentPart{}
				out[i] = &clone
			} else {
				out[i] = m
			}
		case *ai.AssistantMessage:
			if m.Content == nil {
				clone := *m
				clone.Content = []ai.AssistantContentPart{}
				out[i] = &clone
			} else {
				out[i] = m
			}
		case *ai.ToolResultMessage:
			if m.Content == nil {
				clone := *m
				clone.Content = []ai.UserContentPart{}
				out[i] = &clone
			} else {
				out[i] = m
			}
		default:
			out[i] = msg
		}
	}
	return out
}

// replaceImagesWithPlaceholder replaces every image block with placeholder
// text, collapsing consecutive images into a single placeholder block.
func replaceImagesWithPlaceholder(content []ai.UserContentPart, placeholder string) []ai.UserContentPart {
	result := make([]ai.UserContentPart, 0, len(content))
	previousWasPlaceholder := false
	for _, block := range content {
		if _, ok := block.(ai.ImageContent); ok {
			if !previousWasPlaceholder {
				result = append(result, ai.TextContent{Text: placeholder})
			}
			previousWasPlaceholder = true
			continue
		}
		result = append(result, block)
		if text, ok := block.(ai.TextContent); ok {
			previousWasPlaceholder = text.Text == placeholder
		} else {
			previousWasPlaceholder = false
		}
	}
	return result
}

// downgradeUnsupportedImages replaces image content with text placeholders
// for models that don't accept image input. User messages with plain-string
// content (no block array) are left untouched, matching upstream's
// Array.isArray guard.
//
// Rewritten messages are copied first: they are pointers into the caller's
// session history, which this pass must not edit.
func downgradeUnsupportedImages(messages []ai.Message, model *ai.Model) []ai.Message {
	if model.SupportsImageInput() {
		return messages
	}

	out := make([]ai.Message, len(messages))
	for i, msg := range messages {
		switch m := msg.(type) {
		case *ai.UserMessage:
			if m.Content.Plain == nil {
				clone := *m
				clone.Content.Blocks = replaceImagesWithPlaceholder(m.Content.Blocks, nonVisionUserImagePlaceholder)
				out[i] = &clone
			} else {
				out[i] = m
			}
		case *ai.ToolResultMessage:
			clone := *m
			clone.Content = replaceImagesWithPlaceholder(m.Content, nonVisionToolImagePlaceholder)
			out[i] = &clone
		default:
			out[i] = msg
		}
	}
	return out
}

// transformOne applies the per-message thinking/tool-call transform. User
// messages pass through unchanged (image downgrade already ran); toolResult
// messages get their toolCallId remapped if a prior assistant message's tool
// call was remapped; assistant messages get their content transformed block
// by block.
func transformOne(msg ai.Message, model *ai.Model, toolCallIDMap map[string]string, normalizeToolCallID NormalizeToolCallID) ai.Message {
	switch m := msg.(type) {
	case *ai.UserMessage:
		return m
	case *ai.ToolResultMessage:
		if normalizedID, ok := toolCallIDMap[m.ToolCallID]; ok && normalizedID != m.ToolCallID {
			clone := *m
			clone.ToolCallID = normalizedID
			return &clone
		}
		return m
	case *ai.AssistantMessage:
		isSameModel := m.Provider == model.Provider && m.Api == model.Api && m.Model == model.ID
		content := make([]ai.AssistantContentPart, 0, len(m.Content))
		for _, block := range m.Content {
			content = append(content, transformAssistantBlock(block, model, m, isSameModel, toolCallIDMap, normalizeToolCallID)...)
		}
		out := *m
		out.Content = content
		return &out
	default:
		return msg
	}
}

// transformAssistantBlock transforms one assistant content block, returning
// zero, one, or (never more than one in practice, but the upstream flatMap
// shape is preserved) blocks to splice into the output content.
func transformAssistantBlock(
	block ai.AssistantContentPart,
	model *ai.Model,
	source *ai.AssistantMessage,
	isSameModel bool,
	toolCallIDMap map[string]string,
	normalizeToolCallID NormalizeToolCallID,
) []ai.AssistantContentPart {
	switch b := block.(type) {
	case ai.ThinkingContent:
		// Redacted thinking is opaque encrypted content, only valid for the
		// same model. Drop it for cross-model to avoid API errors.
		if b.Redacted {
			if isSameModel {
				return []ai.AssistantContentPart{b}
			}
			return nil
		}
		// For same model: keep thinking blocks with signatures (needed for
		// replay) even if the thinking text is empty (OpenAI encrypted
		// reasoning).
		if isSameModel && b.ThinkingSignature != "" {
			return []ai.AssistantContentPart{b}
		}
		// Skip empty thinking blocks, convert others to plain text.
		if strings.TrimSpace(b.Thinking) == "" {
			return nil
		}
		if isSameModel {
			return []ai.AssistantContentPart{b}
		}
		return []ai.AssistantContentPart{ai.TextContent{Text: b.Thinking}}

	case ai.TextContent:
		if isSameModel {
			return []ai.AssistantContentPart{b}
		}
		return []ai.AssistantContentPart{ai.TextContent{Text: b.Text}}

	case ai.ToolCall:
		toolCall := b
		if !isSameModel && toolCall.ThoughtSignature != "" {
			toolCall.ThoughtSignature = ""
		}
		if !isSameModel && normalizeToolCallID != nil {
			normalizedID := normalizeToolCallID(toolCall.ID, model, source)
			if normalizedID != toolCall.ID {
				toolCallIDMap[toolCall.ID] = normalizedID
				toolCall.ID = normalizedID
			}
		}
		return []ai.AssistantContentPart{toolCall}

	default:
		return []ai.AssistantContentPart{block}
	}
}

// insertOrphanToolResults synthesizes a "No result provided" error tool
// result for every tool call that isn't followed by a matching toolResult
// message before the next assistant turn, the next user turn, or the end of
// the conversation. Assistant turns that ended in error/aborted are skipped
// entirely (their tool calls, if any, are dropped along with them rather than
// synthesized).
func insertOrphanToolResults(messages []ai.Message) []ai.Message {
	result := make([]ai.Message, 0, len(messages))
	var pendingToolCalls []ai.ToolCall
	existingToolResultIDs := make(map[string]bool)

	flush := func() {
		for _, tc := range pendingToolCalls {
			if !existingToolResultIDs[tc.ID] {
				result = append(result, &ai.ToolResultMessage{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Content:    []ai.UserContentPart{ai.TextContent{Text: "No result provided"}},
					IsError:    true,
					Timestamp:  time.Now().UnixMilli(),
				})
			}
		}
		pendingToolCalls = nil
		existingToolResultIDs = make(map[string]bool)
	}

	for _, msg := range messages {
		switch m := msg.(type) {
		case *ai.AssistantMessage:
			// If we have pending orphaned tool calls from a previous
			// assistant, insert synthetic results now.
			flush()

			// Skip errored/aborted assistant messages entirely. These are
			// incomplete turns that shouldn't be replayed: they may have
			// partial content, and replaying them can cause API errors. The
			// model should retry from the last valid state.
			if m.StopReason == ai.StopReasonError || m.StopReason == ai.StopReasonAborted {
				continue
			}

			var toolCalls []ai.ToolCall
			for _, b := range m.Content {
				if tc, ok := b.(ai.ToolCall); ok {
					toolCalls = append(toolCalls, tc)
				}
			}
			if len(toolCalls) > 0 {
				pendingToolCalls = toolCalls
				existingToolResultIDs = make(map[string]bool)
			}

			result = append(result, m)
		case *ai.ToolResultMessage:
			existingToolResultIDs[m.ToolCallID] = true
			result = append(result, m)
		case *ai.UserMessage:
			// User message interrupts tool flow - insert synthetic results
			// for orphaned calls.
			flush()
			result = append(result, m)
		default:
			result = append(result, msg)
		}
	}

	// If the conversation ends with unresolved tool calls, synthesize
	// results now.
	flush()

	return result
}
