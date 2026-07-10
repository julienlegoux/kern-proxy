package openairesponses

// Ports: packages/ai/src/api/openai-responses-shared.ts (convertResponsesMessages,
// convertResponsesTools, and the id-normalization helpers in their closure).
//
// Exported so the Azure and Codex variants (epic 7, issues 02-04) can reuse
// this conversion without duplicating it: each variant only differs in its
// allowed-tool-call-call-provider set and (for Codex) its own transport, not
// in how a Context becomes Responses API input items.

import (
	"encoding/json"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis"
)

// wireInputItem is any Responses API input item: an easy-input message
// ({role,content}), a typed item (function_call/function_call_output/
// message/reasoning), or a raw passthrough (verbatim thinking-signature
// replay). Represented as `any` since the union has no common Go struct
// shape, mirroring TS's ResponseInput union.
type wireInputItem = any

type wireEasyMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type wireInputTextPart struct {
	Type string `json:"type"` // "input_text"
	Text string `json:"text"`
}

type wireInputImagePart struct {
	Type     string `json:"type"` // "input_image"
	Detail   string `json:"detail"`
	ImageURL string `json:"image_url"`
}

type wireOutputMessage struct {
	Type    string                  `json:"type"` // "message"
	Role    string                  `json:"role"` // "assistant"
	Content []wireOutputContentPart `json:"content"`
	Status  string                  `json:"status"`
	ID      string                  `json:"id"`
	Phase   string                  `json:"phase,omitempty"`
}

type wireOutputContentPart struct {
	Type        string `json:"type"` // "output_text"
	Text        string `json:"text"`
	Annotations []any  `json:"annotations"`
}

type wireFunctionCall struct {
	Type      string `json:"type"` // "function_call"
	ID        string `json:"id,omitempty"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireFunctionCallOutput struct {
	Type   string `json:"type"` // "function_call_output"
	CallID string `json:"call_id"`
	// Output is a string, or a []wireFunctionCallOutputPart when the tool
	// result carries images the model can accept.
	Output any `json:"output"`
}

type wireFunctionCallOutputPart struct {
	Type     string `json:"type"` // "input_text" | "input_image"
	Text     string `json:"text,omitempty"`
	Detail   string `json:"detail,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type wireTool struct {
	Type        string         `json:"type"` // "function"
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict"`
}

// ConvertToolsOptions mirrors ConvertResponsesToolsOptions.
type ConvertToolsOptions struct {
	// Strict selects the `strict` flag on every converted tool; the zero
	// value (false) matches upstream's default.
	Strict bool
}

// ConvertTools ports convertResponsesTools.
func ConvertTools(tools []ai.Tool, options ConvertToolsOptions) []wireTool {
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
		var params map[string]any
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		if params == nil {
			params = map[string]any{}
		}
		out = append(out, wireTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
			Strict:      options.Strict,
		})
	}
	return out
}

// ConvertMessagesOptions mirrors ConvertResponsesMessagesOptions.
type ConvertMessagesOptions struct {
	// IncludeSystemPrompt matches upstream's default-true option.
	IncludeSystemPrompt bool
	// SystemPromptRole is the role used for the system prompt item ("system"
	// or "developer"); callers resolve the compat gate before calling.
	SystemPromptRole string
	// AllowedToolCallProviders is the set of model.Provider values that use
	// the pipe-separated `{callId}|{itemId}` tool-call id convention this
	// family's streaming decode produces; every other provider's ids are
	// normalized without a pipe split. Ports OPENAI_TOOL_CALL_PROVIDERS /
	// AZURE_TOOL_CALL_PROVIDERS (each variant supplies its own set).
	AllowedToolCallProviders map[string]bool
}

// ConvertMessages ports convertResponsesMessages.
func ConvertMessages(model *ai.Model, chat ai.Context, options ConvertMessagesOptions) []wireInputItem {
	var items []wireInputItem

	normalize := buildNormalizeToolCallID(options.AllowedToolCallProviders)
	transformed := apis.TransformMessages(chat.Messages, model, normalize)

	includeSystemPrompt := options.IncludeSystemPrompt
	if chat.SystemPrompt == "" {
		includeSystemPrompt = false
	}
	if includeSystemPrompt {
		role := options.SystemPromptRole
		if role == "" {
			role = "system"
		}
		items = append(items, wireEasyMessage{Role: role, Content: ai.SanitizeSurrogates(chat.SystemPrompt)})
	}

	msgIndex := 0
	for i := 0; i < len(transformed); i++ {
		msg := transformed[i]
		switch m := msg.(type) {
		case *ai.UserMessage:
			item, ok := convertUserMessage(m)
			if !ok {
				continue
			}
			items = append(items, item)

		case *ai.AssistantMessage:
			isDifferentModel := m.Model != model.ID && m.Provider == model.Provider && m.Api == model.Api
			output, textBlockIndex := convertAssistantMessage(m, msgIndex, isDifferentModel)
			if len(output) == 0 {
				continue
			}
			_ = textBlockIndex
			items = append(items, output...)

		case *ai.ToolResultMessage:
			items = append(items, convertToolResultMessage(m, model))

		default:
			continue
		}
		msgIndex++
	}

	return items
}

func convertUserMessage(m *ai.UserMessage) (wireInputItem, bool) {
	if m.Content.Plain != nil {
		return wireEasyMessage{
			Role:    "user",
			Content: []wireInputTextPart{{Type: "input_text", Text: ai.SanitizeSurrogates(*m.Content.Plain)}},
		}, true
	}
	var parts []any
	for _, block := range m.Content.Blocks {
		switch b := block.(type) {
		case ai.TextContent:
			parts = append(parts, wireInputTextPart{Type: "input_text", Text: ai.SanitizeSurrogates(b.Text)})
		case ai.ImageContent:
			parts = append(parts, inputImagePart(b))
		}
	}
	if len(parts) == 0 {
		return nil, false
	}
	return wireEasyMessage{Role: "user", Content: parts}, true
}

func inputImagePart(b ai.ImageContent) wireInputImagePart {
	return wireInputImagePart{
		Type:     "input_image",
		Detail:   "auto",
		ImageURL: "data:" + b.MimeType + ";base64," + b.Data,
	}
}

// convertAssistantMessage builds the wire items replaying one assistant
// message's content blocks. msgIndex is the shared across-message-kind
// counter used for the msg_pi_<msgIndex> fallback text-block id (ported
// exactly from upstream's msgIndex, which only advances for messages that
// actually emitted an item -- see the caller's `continue` handling above).
func convertAssistantMessage(m *ai.AssistantMessage, msgIndex int, isDifferentModel bool) ([]wireInputItem, int) {
	var output []wireInputItem
	textBlockIndex := 0

	for _, block := range m.Content {
		switch b := block.(type) {
		case ai.ThinkingContent:
			if b.ThinkingSignature == "" {
				continue
			}
			if !json.Valid([]byte(b.ThinkingSignature)) {
				continue
			}
			output = append(output, json.RawMessage(b.ThinkingSignature))

		case ai.TextContent:
			sig := parseTextSignature(b.TextSignature)
			fallbackID := "msg_pi_" + itoa(msgIndex)
			if textBlockIndex > 0 {
				fallbackID = "msg_pi_" + itoa(msgIndex) + "_" + itoa(textBlockIndex)
			}
			textBlockIndex++
			msgID := fallbackID
			phase := ""
			if sig != nil {
				if sig.ID != "" {
					msgID = sig.ID
				}
				phase = sig.Phase
			}
			// OpenAI requires id to be max 64 characters.
			if len([]rune(msgID)) > 64 {
				msgID = "msg_" + ai.ShortHash(msgID)
			}
			output = append(output, wireOutputMessage{
				Type:    "message",
				Role:    "assistant",
				Content: []wireOutputContentPart{{Type: "output_text", Text: ai.SanitizeSurrogates(b.Text), Annotations: []any{}}},
				Status:  "completed",
				ID:      msgID,
				Phase:   phase,
			})

		case ai.ToolCall:
			callID, itemID := splitToolCallID(b.ID)
			if isDifferentModel && strings.HasPrefix(itemID, "fc_") {
				itemID = ""
			}
			args := b.Arguments
			if args == nil {
				args = map[string]any{}
			}
			argsJSON, _ := json.Marshal(args)
			output = append(output, wireFunctionCall{
				Type:      "function_call",
				ID:        itemID,
				CallID:    callID,
				Name:      b.Name,
				Arguments: string(argsJSON),
			})
		}
	}

	return output, textBlockIndex
}

func convertToolResultMessage(m *ai.ToolResultMessage, model *ai.Model) wireInputItem {
	var texts []string
	var images []ai.ImageContent
	for _, c := range m.Content {
		switch b := c.(type) {
		case ai.TextContent:
			texts = append(texts, b.Text)
		case ai.ImageContent:
			images = append(images, b)
		}
	}
	textResult := strings.Join(texts, "\n")
	hasText := textResult != ""
	hasImages := len(images) > 0
	callID, _ := splitToolCallID(m.ToolCallID)

	var output any
	if hasImages && model.SupportsImageInput() {
		var parts []wireFunctionCallOutputPart
		if hasText {
			parts = append(parts, wireFunctionCallOutputPart{Type: "input_text", Text: ai.SanitizeSurrogates(textResult)})
		}
		for _, img := range images {
			parts = append(parts, wireFunctionCallOutputPart{
				Type:     "input_image",
				Detail:   "auto",
				ImageURL: "data:" + img.MimeType + ";base64," + img.Data,
			})
		}
		output = parts
	} else {
		switch {
		case hasText:
			output = ai.SanitizeSurrogates(textResult)
		case hasImages:
			output = "(see attached image)"
		default:
			output = "(no tool output)"
		}
	}

	return wireFunctionCallOutput{Type: "function_call_output", CallID: callID, Output: output}
}

// splitToolCallID splits the `{callId}|{itemId}` id convention this family
// produces (see stream.go's ensureToolCallBlock); ids without a pipe return
// an empty itemID.
func splitToolCallID(id string) (callID, itemID string) {
	if idx := strings.IndexByte(id, '|'); idx >= 0 {
		return id[:idx], id[idx+1:]
	}
	return id, ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// normalizeIdPart replaces every character outside [a-zA-Z0-9_-] with '_',
// truncates to 64 chars, and strips trailing underscores. Ports the
// normalizeIdPart closure from convertResponsesMessages.
func normalizeIdPart(part string) string {
	var b strings.Builder
	for _, r := range part {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return strings.TrimRight(out, "_")
}

// buildForeignResponsesItemId ports buildForeignResponsesItemId: a
// deterministic, OpenAI-acceptable ("fc_"-prefixed) item id derived from a
// foreign provider's opaque item id.
func buildForeignResponsesItemID(itemID string) string {
	out := "fc_" + ai.ShortHash(itemID)
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// buildNormalizeToolCallID returns the apis.NormalizeToolCallID closure
// ported from convertResponsesMessages' normalizeToolCallId: ids from
// providers in allowedToolCallProviders keep their `{callId}|{itemId}` pipe
// structure (remapping a foreign item id to a synthetic "fc_"-prefixed one
// when the call crosses providers/apis); every other id is normalized
// without a pipe split.
func buildNormalizeToolCallID(allowedToolCallProviders map[string]bool) apis.NormalizeToolCallID {
	return func(id string, model *ai.Model, source *ai.AssistantMessage) string {
		if !allowedToolCallProviders[model.Provider] {
			return normalizeIdPart(id)
		}
		idx := strings.IndexByte(id, '|')
		if idx < 0 {
			return normalizeIdPart(id)
		}
		callID, itemID := id[:idx], id[idx+1:]
		normalizedCallID := normalizeIdPart(callID)
		isForeignToolCall := source.Provider != model.Provider || source.Api != model.Api
		var normalizedItemID string
		if isForeignToolCall {
			normalizedItemID = buildForeignResponsesItemID(itemID)
		} else {
			normalizedItemID = normalizeIdPart(itemID)
		}
		if !strings.HasPrefix(normalizedItemID, "fc_") {
			normalizedItemID = normalizeIdPart("fc_" + normalizedItemID)
		}
		return normalizedCallID + "|" + normalizedItemID
	}
}

// parsedTextSignature is the decoded form of TextContent.TextSignature.
type parsedTextSignature struct {
	ID    string
	Phase string
}

// parseTextSignature ports parseTextSignature: a JSON-object signature
// decodes to its structured id/phase (ai.TextSignatureV1); anything else
// (including a legacy plain-string signature) is treated as a bare id.
func parseTextSignature(signature string) *parsedTextSignature {
	if signature == "" {
		return nil
	}
	if strings.HasPrefix(signature, "{") {
		var v ai.TextSignatureV1
		if err := json.Unmarshal([]byte(signature), &v); err == nil && v.V == 1 && v.ID != "" {
			if v.Phase == "commentary" || v.Phase == "final_answer" {
				return &parsedTextSignature{ID: v.ID, Phase: v.Phase}
			}
			return &parsedTextSignature{ID: v.ID}
		}
	}
	return &parsedTextSignature{ID: signature}
}
