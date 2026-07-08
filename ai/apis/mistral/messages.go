package mistral

// Ports the message/tool conversion and tool-choice/stop-reason mapping
// halves of mistral-conversations.ts's buildChatPayload, toChatMessages,
// toFunctionTools, mapToolChoice, and mapChatStopReason.

import (
	"encoding/json"
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
)

// wireMessage is one entry of the Mistral chat completions `messages` array.
// Content is a string (system/user/tool-result text) or []map[string]any
// (multi-part content, e.g. text+image); ToolCalls is only set on assistant
// messages carrying tool invocations.
type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type wireToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function wireToolCallFunc `json:"function"`
}

type wireToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// wireTool is one entry of the Mistral chat completions `tools` array.
type wireTool struct {
	Type     string           `json:"type"`
	Function wireToolFunction `json:"function"`
}

type wireToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

// toFunctionTools converts tools to Mistral's function-tool wire shape.
// Ports toFunctionTools from mistral-conversations.ts.
func toFunctionTools(tools []ai.Tool) []wireTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{}`)
		}
		out = append(out, wireTool{
			Type: "function",
			Function: wireToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
				Strict:      false,
			},
		})
	}
	return out
}

// mapToolChoice maps the Mistral-specific tool-choice options to the wire
// `tool_choice` value: a named function pins the model to one tool
// (overriding the string enum), the string enum passes through as-is, and
// both empty means unset. Ports mapToolChoice from mistral-conversations.ts.
func mapToolChoice(choice, function string) any {
	if function != "" {
		return map[string]any{
			"type":     "function",
			"function": map[string]any{"name": function},
		}
	}
	if choice == "" {
		return nil
	}
	return choice
}

// mapChatStopReason maps a raw finish_reason string to ai.StopReason. Ports
// mapChatStopReason from mistral-conversations.ts.
func mapChatStopReason(reason string) ai.StopReason {
	switch reason {
	case "stop":
		return ai.StopReasonStop
	case "length", "model_length":
		return ai.StopReasonLength
	case "tool_calls":
		return ai.StopReasonToolUse
	case "error":
		return ai.StopReasonError
	default:
		return ai.StopReasonStop
	}
}

// toChatMessages transforms the unified conversation history (via
// apis.TransformMessages, remapping tool-call ids through normalizeID) and
// converts it to Mistral chat completions `messages` entries. Ports
// toChatMessages from mistral-conversations.ts (the system-prompt unshift is
// handled by buildParams, matching upstream's buildChatPayload).
func toChatMessages(messages []ai.Message, model *ai.Model, normalizeID apis.NormalizeToolCallID) []wireMessage {
	supportsImages := model.SupportsImageInput()
	transformed := apis.TransformMessages(messages, model, normalizeID)

	out := make([]wireMessage, 0, len(transformed))
	for _, msg := range transformed {
		switch m := msg.(type) {
		case ai.UserMessage:
			if wm, ok := userMessage(m, supportsImages); ok {
				out = append(out, wm)
			}
		case *ai.AssistantMessage:
			if wm, ok := assistantMessage(m); ok {
				out = append(out, wm)
			}
		case ai.ToolResultMessage:
			out = append(out, toolResultMessage(m, supportsImages))
		}
	}
	return out
}

func userMessage(m ai.UserMessage, supportsImages bool) (wireMessage, bool) {
	if m.Content.Plain != nil {
		return wireMessage{Role: "user", Content: ai.SanitizeSurrogates(*m.Content.Plain)}, true
	}

	hadImages := false
	var content []map[string]any
	for _, item := range m.Content.Blocks {
		switch b := item.(type) {
		case ai.TextContent:
			content = append(content, map[string]any{"type": "text", "text": ai.SanitizeSurrogates(b.Text)})
		case ai.ImageContent:
			hadImages = true
			if supportsImages {
				content = append(content, imagePart(b))
			}
		}
	}
	if len(content) > 0 {
		return wireMessage{Role: "user", Content: content}, true
	}
	if hadImages && !supportsImages {
		return wireMessage{Role: "user", Content: "(image omitted: model does not support images)"}, true
	}
	return wireMessage{}, false
}

func imagePart(b ai.ImageContent) map[string]any {
	return map[string]any{"type": "image_url", "image_url": "data:" + b.MimeType + ";base64," + b.Data}
}

func assistantMessage(m *ai.AssistantMessage) (wireMessage, bool) {
	var contentParts []map[string]any
	var toolCalls []wireToolCall

	for _, block := range m.Content {
		switch b := block.(type) {
		case ai.TextContent:
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			contentParts = append(contentParts, map[string]any{"type": "text", "text": ai.SanitizeSurrogates(b.Text)})
		case ai.ThinkingContent:
			if strings.TrimSpace(b.Thinking) == "" {
				continue
			}
			contentParts = append(contentParts, map[string]any{
				"type":     "thinking",
				"thinking": []map[string]any{{"type": "text", "text": ai.SanitizeSurrogates(b.Thinking)}},
			})
		case ai.ToolCall:
			args := b.Arguments
			if args == nil {
				args = map[string]any{}
			}
			argsJSON, _ := json.Marshal(args)
			toolCalls = append(toolCalls, wireToolCall{
				ID:       b.ID,
				Type:     "function",
				Function: wireToolCallFunc{Name: b.Name, Arguments: string(argsJSON)},
			})
		}
	}

	if len(contentParts) == 0 && len(toolCalls) == 0 {
		return wireMessage{}, false
	}
	wm := wireMessage{Role: "assistant"}
	if len(contentParts) > 0 {
		wm.Content = contentParts
	}
	if len(toolCalls) > 0 {
		wm.ToolCalls = toolCalls
	}
	return wm, true
}

func toolResultMessage(m ai.ToolResultMessage, supportsImages bool) wireMessage {
	var texts []string
	hasImages := false
	for _, part := range m.Content {
		if t, ok := part.(ai.TextContent); ok {
			texts = append(texts, ai.SanitizeSurrogates(t.Text))
		}
		if _, ok := part.(ai.ImageContent); ok {
			hasImages = true
		}
	}
	textResult := strings.Join(texts, "\n")
	toolText := buildToolResultText(textResult, hasImages, supportsImages, m.IsError)

	content := []map[string]any{{"type": "text", "text": toolText}}
	if supportsImages {
		for _, part := range m.Content {
			if img, ok := part.(ai.ImageContent); ok {
				content = append(content, imagePart(img))
			}
		}
	}

	return wireMessage{Role: "tool", ToolCallID: m.ToolCallID, Name: m.ToolName, Content: content}
}

// buildToolResultText composes the placeholder/prefixed text for a tool
// result. Ports buildToolResultText from mistral-conversations.ts.
func buildToolResultText(text string, hasImages, supportsImages, isError bool) string {
	trimmed := strings.TrimSpace(text)
	errorPrefix := ""
	if isError {
		errorPrefix = "[tool error] "
	}

	if trimmed != "" {
		imageSuffix := ""
		if hasImages && !supportsImages {
			imageSuffix = "\n[tool image omitted: model does not support images]"
		}
		return errorPrefix + trimmed + imageSuffix
	}

	if hasImages {
		if supportsImages {
			if isError {
				return "[tool error] (see attached image)"
			}
			return "(see attached image)"
		}
		if isError {
			return "[tool error] (image omitted: model does not support images)"
		}
		return "(image omitted: model does not support images)"
	}

	if isError {
		return "[tool error] (no tool output)"
	}
	return "(no tool output)"
}
