package ai

// Ports: packages/ai/src/types.ts

// Api identifies a wire protocol (not a vendor). Many providers share one Api.
type Api = string

// Known wire protocols.
const (
	ApiOpenAICompletions     Api = "openai-completions"
	ApiMistralConversations  Api = "mistral-conversations"
	ApiOpenAIResponses       Api = "openai-responses"
	ApiAzureOpenAIResponses  Api = "azure-openai-responses"
	ApiOpenAICodexResponses  Api = "openai-codex-responses"
	ApiAnthropicMessages     Api = "anthropic-messages"
	ApiBedrockConverseStream Api = "bedrock-converse-stream"
	ApiGoogleGenerativeAI    Api = "google-generative-ai"
	ApiGoogleVertex          Api = "google-vertex"
)

// ProviderId identifies a vendor/endpoint (e.g. "anthropic", "groq").
type ProviderId = string

// Role discriminates the Message union.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "toolResult"
)

// StopReason mirrors the TS StopReason union.
type StopReason string

const (
	StopReasonStop    StopReason = "stop"
	StopReasonLength  StopReason = "length"
	StopReasonToolUse StopReason = "toolUse"
	StopReasonError   StopReason = "error"
	StopReasonAborted StopReason = "aborted"
)

// ContentType discriminates content parts on the wire ("type" field).
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeThinking ContentType = "thinking"
	ContentTypeImage    ContentType = "image"
	ContentTypeToolCall ContentType = "toolCall"
)

// ContentPart is the closed union of all content block kinds.
type ContentPart interface {
	ContentType() ContentType
}

// AssistantContentPart is a content block allowed in AssistantMessage.Content:
// TextContent, ThinkingContent, or ToolCall.
type AssistantContentPart interface {
	ContentPart
	assistantContent()
}

// UserContentPart is a content block allowed in UserMessage.Content:
// TextContent or ImageContent. Tool results share the same block kinds.
type UserContentPart interface {
	ContentPart
	userContent()
}

// TextContent is a plain text block. TextSignature carries opaque provider
// metadata (e.g. OpenAI responses item id, or TextSignatureV1 JSON).
type TextContent struct {
	Text          string `json:"text"`
	TextSignature string `json:"textSignature,omitempty"`
}

func (TextContent) ContentType() ContentType { return ContentTypeText }
func (TextContent) assistantContent()        {}
func (TextContent) userContent()             {}

// TextSignatureV1 is the structured form of TextContent.TextSignature.
type TextSignatureV1 struct {
	V     int    `json:"v"`
	ID    string `json:"id"`
	Phase string `json:"phase,omitempty"` // "commentary" | "final_answer"
}

// ThinkingContent is a reasoning block. When Redacted is true the content was
// removed by safety filters and the opaque encrypted payload lives in
// ThinkingSignature for multi-turn replay.
type ThinkingContent struct {
	Thinking          string `json:"thinking"`
	ThinkingSignature string `json:"thinkingSignature,omitempty"`
	Redacted          bool   `json:"redacted,omitempty"`
}

func (ThinkingContent) ContentType() ContentType { return ContentTypeThinking }
func (ThinkingContent) assistantContent()        {}

// ImageContent is a base64-encoded image block.
type ImageContent struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (ImageContent) ContentType() ContentType { return ContentTypeImage }
func (ImageContent) userContent()             {}

// ToolCall is an assistant tool invocation. ThoughtSignature is
// Google-specific opaque metadata for reusing thought context.
type ToolCall struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Arguments        map[string]any `json:"arguments"`
	ThoughtSignature string         `json:"thoughtSignature,omitempty"`
}

func (ToolCall) ContentType() ContentType { return ContentTypeToolCall }
func (ToolCall) assistantContent()        {}

// UsageCost is the computed dollar cost breakdown of a Usage.
type UsageCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// Usage is the unified token accounting block.
//
// CacheWrite1h is the subset of CacheWrite written with 1h retention; only
// Anthropic reports this split. Reasoning is a subset of Output, set (possibly
// to 0) only by providers that expose a reasoning breakdown — nil means the
// provider does not report it.
type Usage struct {
	Input        int       `json:"input"`
	Output       int       `json:"output"`
	CacheRead    int       `json:"cacheRead"`
	CacheWrite   int       `json:"cacheWrite"`
	CacheWrite1h *int      `json:"cacheWrite1h,omitempty"`
	Reasoning    *int      `json:"reasoning,omitempty"`
	TotalTokens  int       `json:"totalTokens"`
	Cost         UsageCost `json:"cost"`
}

// Message is the closed union of conversation messages, discriminated on the
// wire by the "role" field.
type Message interface {
	MessageRole() Role
}

// UserContent models the TS `string | (TextContent | ImageContent)[]` shape.
// Exactly one of Plain or Blocks is meaningful; a Plain string round-trips as
// a JSON string, Blocks as an array.
type UserContent struct {
	Plain  *string
	Blocks []UserContentPart
}

// UserText builds a plain-string UserContent.
func UserText(text string) UserContent { return UserContent{Plain: &text} }

// UserBlocks builds a block-array UserContent.
func UserBlocks(blocks ...UserContentPart) UserContent { return UserContent{Blocks: blocks} }

// Text returns the concatenated text of the content (the plain string, or all
// text blocks joined).
func (c UserContent) Text() string {
	if c.Plain != nil {
		return *c.Plain
	}
	out := ""
	for _, b := range c.Blocks {
		if t, ok := b.(TextContent); ok {
			out += t.Text
		}
	}
	return out
}

// UserMessage is a user turn. Timestamp is Unix milliseconds.
type UserMessage struct {
	Content   UserContent `json:"content"`
	Timestamp int64       `json:"timestamp"`
}

func (*UserMessage) MessageRole() Role { return RoleUser }

// AssistantMessage is both the streaming accumulator and the final result of
// an assistant turn. Timestamp is Unix milliseconds.
type AssistantMessage struct {
	Content       []AssistantContentPart       `json:"content"`
	Api           Api                          `json:"api"`
	Provider      ProviderId                   `json:"provider"`
	Model         string                       `json:"model"`
	ResponseModel string                       `json:"responseModel,omitempty"`
	ResponseID    string                       `json:"responseId,omitempty"`
	Diagnostics   []AssistantMessageDiagnostic `json:"diagnostics,omitempty"`
	Usage         Usage                        `json:"usage"`
	StopReason    StopReason                   `json:"stopReason"`
	ErrorMessage  string                       `json:"errorMessage,omitempty"`
	Timestamp     int64                        `json:"timestamp"`
}

func (*AssistantMessage) MessageRole() Role { return RoleAssistant }

// Clone returns a snapshot copy safe to hand to a concurrent consumer: the
// struct, content slice, and diagnostics slice are copied. Content blocks are
// value types; adapters must replace (not mutate) ToolCall.Arguments maps so
// shared references stay read-only.
func (m *AssistantMessage) Clone() *AssistantMessage {
	if m == nil {
		return nil
	}
	out := *m
	out.Content = make([]AssistantContentPart, len(m.Content))
	copy(out.Content, m.Content)
	if m.Diagnostics != nil {
		out.Diagnostics = make([]AssistantMessageDiagnostic, len(m.Diagnostics))
		copy(out.Diagnostics, m.Diagnostics)
	}
	return &out
}

// ToolResultMessage carries a tool execution result back to the model.
// Timestamp is Unix milliseconds.
type ToolResultMessage struct {
	ToolCallID string            `json:"toolCallId"`
	ToolName   string            `json:"toolName"`
	Content    []UserContentPart `json:"content"`
	Details    any               `json:"details,omitempty"`
	IsError    bool              `json:"isError"`
	Timestamp  int64             `json:"timestamp"`
}

func (*ToolResultMessage) MessageRole() Role { return RoleToolResult }

// Tool describes a callable tool. Parameters is a JSON Schema document.
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  JSONSchema `json:"parameters"`
}

// Context is the full input of a request.
type Context struct {
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	Messages     []Message `json:"messages"`
	Tools        []Tool    `json:"tools,omitempty"`
}
