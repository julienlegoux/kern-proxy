package ai

import (
	"regexp"
	"strings"
)

// Ports: packages/ai/src/utils/retry.ts

func buildProviderErrorPattern(patterns []string) *regexp.Regexp {
	return regexp.MustCompile("(?i)" + strings.Join(patterns, "|"))
}

// nonRetryableProviderLimitErrorPattern matches subscription/account limits —
// quota, billing, and budget exhaustion — that must not be retried.
var nonRetryableProviderLimitErrorPattern = buildProviderErrorPattern([]string{
	// OpenCode Go/free-tier limits returned as 429 JSON error types by OpenCode's
	// Zen API. These are subscription/account limits, not transient throttles.
	"GoUsageLimitError",
	"FreeUsageLimitError",

	// OpenCode Go subscription-limit text asks users to enable available-balance
	// usage after rolling/weekly/monthly limits are reached.
	"Monthly usage limit reached",
	"available balance",

	// Generic quota/budget/billing exhaustion. `insufficient_quota` is OpenAI's
	// quota/billing error code; the other strings cover common gateway wording.
	"insufficient_quota",
	"out of budget",
	"quota exceeded",
	"billing",
})

// retryableProviderErrorPattern matches transient provider/transport failures.
var retryableProviderErrorPattern = buildProviderErrorPattern([]string{
	// Generic provider load, HTTP status, and server-side transient failures.
	"overloaded",
	"rate.?limit",
	"too many requests",
	"429",
	"500",
	"502",
	"503",
	"504",
	"524",
	"service.?unavailable",
	"server.?error",
	"internal.?error",

	// Wrapper/provider text for transient upstream failures, including OpenRouter
	// "Provider returned error" responses.
	"provider.?returned.?error",

	// Network, proxy, and fetch transport failures.
	"network.?error",
	"connection.?error",
	"connection.?refused",
	"connection.?lost",
	"other side closed",
	"fetch failed",
	"upstream.?connect",
	"reset before headers",
	"socket hang up",
	"timed? out",
	"timeout",
	"terminated",

	// WebSocket transports can report close/error text instead of HTTP/fetch text.
	"websocket.?closed",
	"websocket.?error",

	// Premature stream endings from SDKs and transports.
	"ended without",
	"stream ended before message_stop",
	"http2 request did not get a response",

	// Provider-requested retry delay cap failures should flow through the outer
	// retry policy so callers can surface/abort the backoff.
	"retry delay",

	// Explicit retry guidance emitted mid-stream by providers.
	"you can retry your request",
	"try your request again",
	"please retry your request",
})

// IsRetryableAssistantError classifies whether a failed assistant message
// looks like a transient provider or transport error, so callers can decide if
// the last assistant turn should be restarted.
//
// This does not implement retry policy. Callers should first handle context
// overflow separately, then apply their own retry budget, backoff, and
// reporting before restarting the assistant turn.
func IsRetryableAssistantError(message *AssistantMessage) bool {
	if message.StopReason != StopReasonError || message.ErrorMessage == "" {
		return false
	}
	if nonRetryableProviderLimitErrorPattern.MatchString(message.ErrorMessage) {
		return false
	}
	return retryableProviderErrorPattern.MatchString(message.ErrorMessage)
}
