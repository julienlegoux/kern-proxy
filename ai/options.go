package ai

import (
	"context"
	"time"
)

// Ports: packages/ai/src/types.ts (StreamOptions, SimpleStreamOptions, ...)

// CacheRetention is the prompt-cache retention preference. Providers map it
// to their supported values. The zero value means the default ("short").
type CacheRetention string

const (
	CacheRetentionNone  CacheRetention = "none"
	CacheRetentionShort CacheRetention = "short"
	CacheRetentionLong  CacheRetention = "long"
)

// Transport is the preferred transport for providers that support several.
type Transport string

const (
	TransportSSE             Transport = "sse"
	TransportWebSocket       Transport = "websocket"
	TransportWebSocketCached Transport = "websocket-cached"
	TransportAuto            Transport = "auto"
)

// ProviderEnv holds provider-scoped environment overrides; values take
// precedence over the process environment.
type ProviderEnv = map[string]string

// ProviderHeaders holds custom HTTP headers. A nil value suppresses a
// provider/API default header with the same name (TS `null`).
type ProviderHeaders = map[string]*string

// HeaderValue is a convenience for building ProviderHeaders literals.
func HeaderValue(s string) *string { return &s }

// ProviderResponse is the response metadata passed to OnResponse.
type ProviderResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// AnthropicEffort selects the adaptive-thinking effort level (anthropic-messages
// only, `Model.Compat.ForceAdaptiveThinking` models). "max" is Opus 4.6 only;
// "xhigh" requires an explicit `Model.ThinkingLevelMap` entry.
type AnthropicEffort string

const (
	AnthropicEffortLow    AnthropicEffort = "low"
	AnthropicEffortMedium AnthropicEffort = "medium"
	AnthropicEffortHigh   AnthropicEffort = "high"
	AnthropicEffortXHigh  AnthropicEffort = "xhigh"
	AnthropicEffortMax    AnthropicEffort = "max"
)

// AnthropicThinkingDisplay controls how thinking content is returned
// (anthropic-messages only).
type AnthropicThinkingDisplay string

const (
	// AnthropicThinkingSummarized returns thinking blocks with summarized text.
	AnthropicThinkingSummarized AnthropicThinkingDisplay = "summarized"
	// AnthropicThinkingOmitted returns an empty thinking field; the encrypted
	// signature still travels back for multi-turn continuity.
	AnthropicThinkingOmitted AnthropicThinkingDisplay = "omitted"
)

// DefaultMaxRetryDelay caps server-requested retry waits (TS: 60000 ms).
const DefaultMaxRetryDelay = 60 * time.Second

// StreamOptions is the base option set shared by every API adapter.
// Cancellation is carried by the context.Context passed to Stream/Complete
// (the Go equivalent of TS AbortSignal).
type StreamOptions struct {
	// Temperature; nil leaves the provider default (0 is a valid value).
	Temperature *float64
	// MaxTokens; nil uses the model/provider default.
	MaxTokens *int
	// APIKey overrides resolved auth for this request.
	APIKey string
	// Transport preference for providers with multiple transports.
	Transport Transport
	// CacheRetention preference; zero value means "short".
	CacheRetention CacheRetention
	// SessionID enables session-based caching/routing on supporting providers.
	SessionID string
	// OnPayload can inspect or replace the provider payload before sending.
	// Return (nil, nil) to keep the payload unchanged.
	OnPayload func(ctx context.Context, payload any, model *Model) (any, error)
	// OnResponse is invoked after an HTTP response is received and before its
	// body stream is consumed.
	OnResponse func(ctx context.Context, response ProviderResponse, model *Model) error
	// Headers are merged over provider defaults; a nil value suppresses a
	// default header with the same name.
	Headers ProviderHeaders
	// Timeout is the HTTP request timeout (TS timeoutMs). Zero = default.
	Timeout time.Duration
	// WebsocketConnectTimeout covers the WebSocket open handshake only.
	WebsocketConnectTimeout time.Duration
	// MaxRetries is the client-side retry attempt cap; nil uses the adapter
	// default (2 for OpenAI/Anthropic-style clients).
	MaxRetries *int
	// MaxRetryDelay caps server-requested retry waits. nil = DefaultMaxRetryDelay;
	// a pointer to 0 disables the cap (TS maxRetryDelayMs: 0).
	MaxRetryDelay *time.Duration
	// Metadata is passed through; providers extract the fields they understand
	// (e.g. Anthropic user_id).
	Metadata map[string]any
	// Env holds provider-scoped environment values that take precedence over
	// the process environment.
	Env ProviderEnv

	// --- anthropic-messages only (mirrors upstream AnthropicOptions, merged
	// here since Go's fixed StreamFunc signature can't carry a per-adapter
	// options type the way TS's per-api function overloads do — the same
	// flat-merge deviation already applied to Compat) ---

	// ThinkingEnabled explicitly toggles extended thinking. nil means unset
	// (thinking is omitted from the request); StreamSimple always sets this
	// explicitly to true or false.
	ThinkingEnabled *bool
	// ThinkingBudgetTokens is the token budget for budget-based thinking
	// models. nil uses the adapter default (1024).
	ThinkingBudgetTokens *int
	// Effort selects the adaptive-thinking effort level; only read when the
	// model has Compat.ForceAdaptiveThinking true.
	Effort AnthropicEffort
	// ThinkingDisplay controls thinking-content verbosity. Empty uses the
	// adapter default (AnthropicThinkingSummarized).
	ThinkingDisplay AnthropicThinkingDisplay

	// --- openai-completions / openai-responses (same flat-merge deviation
	// as above; upstream's OpenAICompletionsOptions and
	// OpenAIResponsesOptions each extend StreamOptions with reasoningEffort)
	// ---

	// ReasoningEffort is the (already-clamped) abstract thinking level. For
	// openai-completions it's translated into the vendor-specific
	// reasoning/thinking request shape selected by Model.Compat.ThinkingFormat;
	// for openai-responses it maps directly to the `reasoning.effort` field
	// (via Model.ThinkingLevelMap when present). Empty means reasoning is off
	// or unrequested; StreamSimple resolves this from
	// SimpleStreamOptions.Reasoning via ai.ClampThinkingLevel.
	ReasoningEffort ThinkingLevel

	// --- openai-responses only ---

	// ReasoningSummary selects the reasoning summary verbosity
	// ("auto"|"detailed"|"concise"); empty defaults to "auto" whenever a
	// reasoning field is sent. A explicit "null" sentinel is not
	// representable in Go's string type, so upstream's `| null` is not
	// distinguished from "unset" here (both fall back to "auto").
	ReasoningSummary string
	// ServiceTier selects OpenAI's service tier ("auto"|"default"|"flex"|
	// "priority"); empty leaves the provider default.
	ServiceTier string

	// --- azure-openai-responses only (same flat-merge deviation as above;
	// upstream's AzureOpenAIResponsesOptions extends StreamOptions with these
	// four fields) ---

	// AzureAPIVersion overrides the Azure OpenAI API version; empty resolves
	// AZURE_OPENAI_API_VERSION then defaults to "v1".
	AzureAPIVersion string
	// AzureResourceName builds the default Azure base URL
	// (https://<name>.openai.azure.com/openai/v1) when AzureBaseURL is unset;
	// empty resolves AZURE_OPENAI_RESOURCE_NAME.
	AzureResourceName string
	// AzureBaseURL overrides the Azure endpoint directly; empty resolves
	// AZURE_OPENAI_BASE_URL, then AzureResourceName, then Model.BaseURL.
	AzureBaseURL string
	// AzureDeploymentName overrides the deployment name sent as the request's
	// model field; empty resolves AZURE_OPENAI_DEPLOYMENT_NAME_MAP (keyed by
	// model ID), then falls back to the model ID itself.
	AzureDeploymentName string

	// --- openai-codex-responses only (same flat-merge deviation as above;
	// upstream's OpenAICodexResponsesOptions extends StreamOptions with this
	// field) ---

	// TextVerbosity selects the response text verbosity
	// ("low"|"medium"|"high"); empty defaults to "low".
	TextVerbosity string

	// --- google-generative-ai only (same flat-merge deviation as above;
	// upstream's GoogleOptions extends StreamOptions with toolChoice and
	// thinking; thinking.enabled/budgetTokens reuse the ThinkingEnabled/
	// ThinkingBudgetTokens fields already defined above for anthropic-messages) ---

	// GoogleToolChoice selects Gemini's function-calling mode
	// ("auto"|"none"|"any"); empty behaves like "auto".
	GoogleToolChoice string
	// GoogleThinkingLevel selects Gemini 3's discrete thinking level
	// ("MINIMAL"|"LOW"|"MEDIUM"|"HIGH"), taking priority over
	// ThinkingBudgetTokens when set. Empty means unset.
	GoogleThinkingLevel GoogleThinkingLevel
}

// GoogleThinkingLevel mirrors Google's Gemini 3 ThinkingLevel enum values.
type GoogleThinkingLevel string

const (
	GoogleThinkingLevelUnspecified GoogleThinkingLevel = "THINKING_LEVEL_UNSPECIFIED"
	GoogleThinkingLevelMinimal     GoogleThinkingLevel = "MINIMAL"
	GoogleThinkingLevelLow         GoogleThinkingLevel = "LOW"
	GoogleThinkingLevelMedium      GoogleThinkingLevel = "MEDIUM"
	GoogleThinkingLevelHigh        GoogleThinkingLevel = "HIGH"
)

// EffectiveCacheRetention resolves the zero value to the "short" default.
func (o *StreamOptions) EffectiveCacheRetention() CacheRetention {
	if o == nil || o.CacheRetention == "" {
		return CacheRetentionShort
	}
	return o.CacheRetention
}

// EffectiveMaxRetryDelay resolves the retry-delay cap (0 = uncapped).
func (o *StreamOptions) EffectiveMaxRetryDelay() time.Duration {
	if o == nil || o.MaxRetryDelay == nil {
		return DefaultMaxRetryDelay
	}
	return *o.MaxRetryDelay
}

// SimpleStreamOptions adds the abstract reasoning knob translated per-provider
// via the model's ThinkingLevelMap.
type SimpleStreamOptions struct {
	StreamOptions
	// Reasoning selects a thinking level; empty = reasoning off/default.
	Reasoning ThinkingLevel
	// ThinkingBudgets customizes token budgets for token-based providers.
	ThinkingBudgets *ThinkingBudgets
}

// StreamFunc is the fundamental adapter signature: every API implementation
// exports a Stream and a StreamSimple of this shape. Once invoked, failures
// must be encoded in the returned stream (an in-band `error` event carrying an
// AssistantMessage with StopReason "error" or "aborted"), never returned as a
// Go error.
type StreamFunc func(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream

// SimpleStreamFunc is StreamFunc for the unified-reasoning entrypoint.
type SimpleStreamFunc func(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream

// ProviderStreams is the uniform dispatch contract of an API implementation.
type ProviderStreams interface {
	Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream
	StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream
}
