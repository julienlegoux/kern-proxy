package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

// wireRequest is the logical shape of a ConverseStream request, exposed to
// StreamOptions.OnPayload exactly like the plain commandInput object upstream
// hands to its onPayload hook -- before it is converted into the AWS SDK's
// typed (and partly document.Interface-wrapped) ConverseStreamInput for the
// actual client call. Field names/shapes mirror the Converse API's wire JSON
// (see the AWS Bedrock Runtime ConverseStream API reference), which is also
// what upstream's commandInput literal produces.
type wireRequest struct {
	ModelID                      string            `json:"modelId"`
	Messages                     []wireMessage     `json:"messages"`
	System                       []wireSystemBlock `json:"system,omitempty"`
	InferenceConfig              wireInference     `json:"inferenceConfig"`
	ToolConfig                   *wireToolConfig   `json:"toolConfig,omitempty"`
	AdditionalModelRequestFields map[string]any    `json:"additionalModelRequestFields,omitempty"`
	RequestMetadata              map[string]string `json:"requestMetadata,omitempty"`
}

// wireMessage is one Converse message.
type wireMessage struct {
	Role    string             `json:"role"` // "user" | "assistant"
	Content []wireContentBlock `json:"content"`
}

// wireContentBlock is a Converse content block union, discriminated by which
// pointer/non-zero field is set (matching the wire shape, where the union
// member name is itself the JSON key: {"text": "..."} , {"toolUse": {...}},
// ...).
type wireContentBlock struct {
	Text             string                `json:"text,omitempty"`
	Image            *wireImageBlock       `json:"image,omitempty"`
	ToolUse          *wireToolUse          `json:"toolUse,omitempty"`
	ToolResult       *wireToolResult       `json:"toolResult,omitempty"`
	ReasoningContent *wireReasoningContent `json:"reasoningContent,omitempty"`
	CachePoint       *wireCachePoint       `json:"cachePoint,omitempty"`
}

type wireImageBlock struct {
	Format string          `json:"format"`
	Source wireImageSource `json:"source"`
}

type wireImageSource struct {
	Bytes []byte `json:"bytes"`
}

type wireToolUse struct {
	ToolUseID string         `json:"toolUseId"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
}

type wireToolResult struct {
	ToolUseID string                  `json:"toolUseId"`
	Content   []wireToolResultContent `json:"content"`
	Status    string                  `json:"status,omitempty"`
}

// wireToolResultContent is a tool-result content block union (text or
// image only; upstream's convertToolResultContent handles no other types).
type wireToolResultContent struct {
	Text  string          `json:"text,omitempty"`
	Image *wireImageBlock `json:"image,omitempty"`
}

type wireReasoningContent struct {
	ReasoningText *wireReasoningText `json:"reasoningText,omitempty"`
}

type wireReasoningText struct {
	Text      string `json:"text"`
	Signature string `json:"signature,omitempty"`
}

type wireCachePoint struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// wireSystemBlock is a system-prompt content block union (text or cachePoint).
type wireSystemBlock struct {
	Text       string          `json:"text,omitempty"`
	CachePoint *wireCachePoint `json:"cachePoint,omitempty"`
}

type wireInference struct {
	MaxTokens   *int     `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}

type wireToolConfig struct {
	Tools      []wireTool      `json:"tools"`
	ToolChoice *wireToolChoice `json:"toolChoice,omitempty"`
}

type wireTool struct {
	ToolSpec wireToolSpec `json:"toolSpec"`
}

type wireToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema wireInputSchema `json:"inputSchema"`
}

type wireInputSchema struct {
	JSON map[string]any `json:"json"`
}

// wireToolChoice is a tool-choice union: exactly one of Auto, Any, or Tool is
// set.
type wireToolChoice struct {
	Auto *wireEmpty          `json:"auto,omitempty"`
	Any  *wireEmpty          `json:"any,omitempty"`
	Tool *wireToolChoiceName `json:"tool,omitempty"`
}

type wireEmpty struct{}

type wireToolChoiceName struct {
	Name string `json:"name"`
}
