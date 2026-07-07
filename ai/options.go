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
}

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
