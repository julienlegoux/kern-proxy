package ai

// Ports: packages/ai/src/types.ts (Model, Compat, ThinkingLevel, routing)

// ThinkingLevel is the abstract reasoning-effort knob passed to StreamSimple.
type ThinkingLevel string

const (
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
)

// ModelThinkingLevel extends ThinkingLevel with "off".
type ModelThinkingLevel = ThinkingLevel

// ThinkingOff marks reasoning disabled in a ThinkingLevelMap.
const ThinkingOff ModelThinkingLevel = "off"

// ThinkingLevelMap maps pi thinking levels to provider/model-specific values.
// A missing key uses the provider default; an explicit nil marks the level as
// unsupported (TS `null`).
type ThinkingLevelMap map[ModelThinkingLevel]*string

// ThinkingBudgets holds custom token budgets per thinking level for
// token-budget-based providers.
type ThinkingBudgets struct {
	Minimal *int `json:"minimal,omitempty"`
	Low     *int `json:"low,omitempty"`
	Medium  *int `json:"medium,omitempty"`
	High    *int `json:"high,omitempty"`
}

// ModelCost is the model's price sheet in $/million tokens.
type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// Modality is an input/output modality of a model.
type Modality string

const (
	ModalityText  Modality = "text"
	ModalityImage Modality = "image"
)

// Model is the descriptor of one model: which vendor serves it (Provider),
// which wire protocol it speaks (Api), pricing, limits, and compat overrides.
type Model struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Api       Api        `json:"api"`
	Provider  ProviderId `json:"provider"`
	BaseURL   string     `json:"baseUrl"`
	Reasoning bool       `json:"reasoning"`
	// ThinkingLevelMap maps pi thinking levels to provider-specific values.
	ThinkingLevelMap ThinkingLevelMap  `json:"thinkingLevelMap,omitempty"`
	Input            []Modality        `json:"input"`
	Cost             ModelCost         `json:"cost"`
	ContextWindow    int               `json:"contextWindow"`
	MaxTokens        int               `json:"maxTokens"`
	Headers          map[string]string `json:"headers,omitempty"`
	// Compat holds per-api compatibility overrides. Which fields apply depends
	// on Api; unset fields fall back to baseUrl auto-detection or defaults.
	Compat *Compat `json:"compat,omitempty"`
}

// SupportsImageInput reports whether the model accepts image input.
func (m *Model) SupportsImageInput() bool {
	for _, mod := range m.Input {
		if mod == ModalityImage {
			return true
		}
	}
	return false
}

// ModelsAreEqual compares two models by id and provider.
func ModelsAreEqual(a, b *Model) bool {
	if a == nil || b == nil {
		return false
	}
	return a.ID == b.ID && a.Provider == b.Provider
}

// ThinkingFormat selects the reasoning parameter encoding for
// OpenAI-completions-compatible providers.
type ThinkingFormat string

const (
	ThinkingFormatOpenAI           ThinkingFormat = "openai"
	ThinkingFormatOpenRouter       ThinkingFormat = "openrouter"
	ThinkingFormatDeepseek         ThinkingFormat = "deepseek"
	ThinkingFormatTogether         ThinkingFormat = "together"
	ThinkingFormatZai              ThinkingFormat = "zai"
	ThinkingFormatQwen             ThinkingFormat = "qwen"
	ThinkingFormatChatTemplate     ThinkingFormat = "chat-template"
	ThinkingFormatQwenChatTemplate ThinkingFormat = "qwen-chat-template"
	ThinkingFormatStringThinking   ThinkingFormat = "string-thinking"
	ThinkingFormatAntLing          ThinkingFormat = "ant-ling"
)

// ChatTemplateKwargValue is a chat_template_kwargs value: a JSON scalar, or a
// {"$var": "thinking.enabled" | "thinking.effort"} placeholder object with an
// optional omitWhenOff flag. Stored as decoded JSON (any).
type ChatTemplateKwargValue = any

// Compat merges the three per-api compatibility structs from the TS package
// (OpenAICompletionsCompat, OpenAIResponsesCompat, AnthropicMessagesCompat)
// into one flat struct — the wire JSON is a flat object either way, and each
// adapter reads only the fields for its own Api. All flags are tri-state
// pointers: nil = auto-detect/default.
type Compat struct {
	// --- openai-completions ---
	SupportsStore                               *bool                             `json:"supportsStore,omitempty"`
	SupportsDeveloperRole                       *bool                             `json:"supportsDeveloperRole,omitempty"` // also openai-responses (default true there)
	SupportsReasoningEffort                     *bool                             `json:"supportsReasoningEffort,omitempty"`
	SupportsUsageInStreaming                    *bool                             `json:"supportsUsageInStreaming,omitempty"`
	MaxTokensField                              string                            `json:"maxTokensField,omitempty"` // "max_completion_tokens" | "max_tokens"
	RequiresToolResultName                      *bool                             `json:"requiresToolResultName,omitempty"`
	RequiresAssistantAfterToolResult            *bool                             `json:"requiresAssistantAfterToolResult,omitempty"`
	RequiresThinkingAsText                      *bool                             `json:"requiresThinkingAsText,omitempty"`
	RequiresReasoningContentOnAssistantMessages *bool                             `json:"requiresReasoningContentOnAssistantMessages,omitempty"`
	ThinkingFormat                              ThinkingFormat                    `json:"thinkingFormat,omitempty"`
	ChatTemplateKwargs                          map[string]ChatTemplateKwargValue `json:"chatTemplateKwargs,omitempty"`
	OpenRouterRouting                           *OpenRouterRouting                `json:"openRouterRouting,omitempty"`
	VercelGatewayRouting                        *VercelGatewayRouting             `json:"vercelGatewayRouting,omitempty"`
	ZaiToolStream                               *bool                             `json:"zaiToolStream,omitempty"`
	SupportsStrictMode                          *bool                             `json:"supportsStrictMode,omitempty"`
	CacheControlFormat                          string                            `json:"cacheControlFormat,omitempty"` // "anthropic"

	// --- shared: openai-completions / openai-responses / anthropic-messages ---
	SendSessionAffinityHeaders *bool `json:"sendSessionAffinityHeaders,omitempty"`
	SupportsLongCacheRetention *bool `json:"supportsLongCacheRetention,omitempty"`

	// --- openai-responses ---
	SendSessionIDHeader *bool `json:"sendSessionIdHeader,omitempty"`

	// --- anthropic-messages ---
	SupportsEagerToolInputStreaming *bool `json:"supportsEagerToolInputStreaming,omitempty"`
	SupportsCacheControlOnTools     *bool `json:"supportsCacheControlOnTools,omitempty"`
	SupportsTemperature             *bool `json:"supportsTemperature,omitempty"`
	ForceAdaptiveThinking           *bool `json:"forceAdaptiveThinking,omitempty"`
	AllowEmptySignature             *bool `json:"allowEmptySignature,omitempty"`
}

// OpenRouterRouting is sent as the OpenRouter `provider` request field.
// Field names mirror the OpenRouter API (snake_case).
type OpenRouterRouting struct {
	AllowFallbacks         *bool    `json:"allow_fallbacks,omitempty"`
	RequireParameters      *bool    `json:"require_parameters,omitempty"`
	DataCollection         string   `json:"data_collection,omitempty"` // "allow" | "deny"
	ZDR                    *bool    `json:"zdr,omitempty"`
	EnforceDistillableText *bool    `json:"enforce_distillable_text,omitempty"`
	Order                  []string `json:"order,omitempty"`
	Only                   []string `json:"only,omitempty"`
	Ignore                 []string `json:"ignore,omitempty"`
	Quantizations          []string `json:"quantizations,omitempty"`
	Sort                   any      `json:"sort,omitempty"` // string or {by, partition}
	MaxPrice               any      `json:"max_price,omitempty"`
	PreferredMinThroughput any      `json:"preferred_min_throughput,omitempty"`
	PreferredMaxLatency    any      `json:"preferred_max_latency,omitempty"`
}

// VercelGatewayRouting controls Vercel AI Gateway upstream routing.
type VercelGatewayRouting struct {
	Only  []string `json:"only,omitempty"`
	Order []string `json:"order,omitempty"`
}
