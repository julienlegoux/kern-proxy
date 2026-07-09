package ai

// Ports: packages/ai/src/types.ts (JSON codec for the ContentPart / Message
// discriminated unions — Go needs explicit (un)marshalers where upstream
// relies on structural typing)

import (
	"encoding/json"
	"fmt"
)

// JSONSchema is a raw JSON Schema document (the Go stand-in for a TypeBox
// schema value).
type JSONSchema = json.RawMessage

// --- content part marshaling -------------------------------------------------

func marshalWithType(t ContentType, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	tag, err := json.Marshal(struct {
		Type ContentType `json:"type"`
	}{t})
	if err != nil {
		return nil, err
	}
	if string(raw) == "{}" {
		return tag, nil
	}
	// Splice the type tag in front of the remaining fields.
	out := append(tag[:len(tag)-1], ',')
	out = append(out, raw[1:]...)
	return out, nil
}

func (c TextContent) MarshalJSON() ([]byte, error) {
	type alias TextContent
	return marshalWithType(ContentTypeText, alias(c))
}

func (c ThinkingContent) MarshalJSON() ([]byte, error) {
	type alias ThinkingContent
	return marshalWithType(ContentTypeThinking, alias(c))
}

func (c ImageContent) MarshalJSON() ([]byte, error) {
	type alias ImageContent
	return marshalWithType(ContentTypeImage, alias(c))
}

func (c ToolCall) MarshalJSON() ([]byte, error) {
	type alias ToolCall
	return marshalWithType(ContentTypeToolCall, alias(c))
}

// UnmarshalContentPart decodes one content block by its "type" discriminator.
func UnmarshalContentPart(data []byte) (ContentPart, error) {
	var probe struct {
		Type ContentType `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case ContentTypeText:
		var v TextContent
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case ContentTypeThinking:
		var v ThinkingContent
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case ContentTypeImage:
		var v ImageContent
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case ContentTypeToolCall:
		var v ToolCall
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("ai: unknown content type %q", probe.Type)
	}
}

func unmarshalContentList(data []byte) ([]ContentPart, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	parts := make([]ContentPart, 0, len(raws))
	for _, r := range raws {
		p, err := UnmarshalContentPart(r)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, nil
}

// assistantContentList is the JSON codec target for AssistantMessage.Content.
type assistantContentList []AssistantContentPart

func (l *assistantContentList) UnmarshalJSON(data []byte) error {
	parts, err := unmarshalContentList(data)
	if err != nil {
		return err
	}
	out := make([]AssistantContentPart, 0, len(parts))
	for _, p := range parts {
		a, ok := p.(AssistantContentPart)
		if !ok {
			return fmt.Errorf("ai: content type %q not allowed in assistant message", p.ContentType())
		}
		out = append(out, a)
	}
	*l = out
	return nil
}

// userContentList is the JSON codec target for tool-result / user block lists.
type userContentList []UserContentPart

func (l *userContentList) UnmarshalJSON(data []byte) error {
	parts, err := unmarshalContentList(data)
	if err != nil {
		return err
	}
	out := make([]UserContentPart, 0, len(parts))
	for _, p := range parts {
		u, ok := p.(UserContentPart)
		if !ok {
			return fmt.Errorf("ai: content type %q not allowed in user/tool-result content", p.ContentType())
		}
		out = append(out, u)
	}
	*l = out
	return nil
}

// --- UserContent (string | blocks) -------------------------------------------

func (c UserContent) MarshalJSON() ([]byte, error) {
	if c.Plain != nil {
		return json.Marshal(*c.Plain)
	}
	if c.Blocks == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(c.Blocks)
}

func (c *UserContent) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		c.Plain = &s
		c.Blocks = nil
		return nil
	}
	var blocks userContentList
	if err := blocks.UnmarshalJSON(data); err != nil {
		return err
	}
	c.Plain = nil
	c.Blocks = blocks
	return nil
}

// --- message union ------------------------------------------------------------

func marshalWithRole(role Role, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	tag, err := json.Marshal(struct {
		Role Role `json:"role"`
	}{role})
	if err != nil {
		return nil, err
	}
	if string(raw) == "{}" {
		return tag, nil
	}
	out := append(tag[:len(tag)-1], ',')
	out = append(out, raw[1:]...)
	return out, nil
}

func (m *UserMessage) MarshalJSON() ([]byte, error) {
	type alias UserMessage
	return marshalWithRole(RoleUser, (*alias)(m))
}

func (m *AssistantMessage) MarshalJSON() ([]byte, error) {
	type alias AssistantMessage
	return marshalWithRole(RoleAssistant, (*alias)(m))
}

func (m *ToolResultMessage) MarshalJSON() ([]byte, error) {
	type alias ToolResultMessage
	return marshalWithRole(RoleToolResult, (*alias)(m))
}

func (m *AssistantMessage) UnmarshalJSON(data []byte) error {
	type alias AssistantMessage
	var v struct {
		alias
		Content assistantContentList `json:"content"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = AssistantMessage(v.alias)
	m.Content = v.Content
	return nil
}

func (m *ToolResultMessage) UnmarshalJSON(data []byte) error {
	type alias ToolResultMessage
	var v struct {
		alias
		Content userContentList `json:"content"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = ToolResultMessage(v.alias)
	m.Content = v.Content
	return nil
}

// UnmarshalMessage decodes one message by its "role" discriminator.
func UnmarshalMessage(data []byte) (Message, error) {
	var probe struct {
		Role Role `json:"role"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	switch probe.Role {
	case RoleUser:
		var v UserMessage
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case RoleAssistant:
		var v AssistantMessage
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case RoleToolResult:
		var v ToolResultMessage
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	default:
		return nil, fmt.Errorf("ai: unknown message role %q", probe.Role)
	}
}

// Messages is a JSON-codable []Message.
type Messages []Message

func (ms *Messages) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}
	out := make(Messages, 0, len(raws))
	for _, r := range raws {
		m, err := UnmarshalMessage(r)
		if err != nil {
			return err
		}
		out = append(out, m)
	}
	*ms = out
	return nil
}

// UnmarshalJSON for Context decodes the polymorphic message list.
func (c *Context) UnmarshalJSON(data []byte) error {
	var v struct {
		SystemPrompt string   `json:"systemPrompt"`
		Messages     Messages `json:"messages"`
		Tools        []Tool   `json:"tools"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	c.SystemPrompt = v.SystemPrompt
	c.Messages = v.Messages
	c.Tools = v.Tools
	return nil
}
