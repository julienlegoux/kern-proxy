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

// retryableStatusCodePatterns match the numeric status a composed assistant
// error message carries as its prefix (e.g. `429 {"type":"error",...}`). They
// are text patterns, so they only mean "transient" when the text they are
// matched against is known to *start* with a status code. Never apply them to
// a raw HTTP response body: a 400 rejecting `max_tokens: 15000` contains the
// digits "500" and is not remotely transient.
var retryableStatusCodePatterns = []string{
	"429",
	"500",
	"502",
	"503",
	"504",
	"524",
}

// transientProviderErrorPatterns match transient provider/transport failures by
// their wording alone, independent of any status code. Safe to apply to a raw
// error body.
var transientProviderErrorPatterns = []string{
	// Generic provider load and server-side transient failures.
	"overloaded",
	"rate.?limit",
	"too many requests",
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
}

// retryableProviderErrorPattern matches transient provider/transport failures
// in a composed assistant error message, whose leading status code is itself a
// signal.
var retryableProviderErrorPattern = buildProviderErrorPattern(
	append(append([]string{}, retryableStatusCodePatterns...), transientProviderErrorPatterns...),
)

// transientProviderErrorPattern is the status-code-free subset.
var transientProviderErrorPattern = buildProviderErrorPattern(transientProviderErrorPatterns)

// IsNonRetryableProviderLimitError reports whether raw provider error text
// names a subscription, quota, or billing limit — an account-level condition
// that will not clear by retrying. It short-circuits every retry policy in
// this repo, including the HTTP-request-level one adapters run before
// streaming begins (ai/apis/internal/httpretry).
func IsNonRetryableProviderLimitError(text string) bool {
	return nonRetryableProviderLimitErrorPattern.MatchString(text)
}

// IsTransientProviderErrorText reports whether raw provider error text reads
// as a transient provider or transport failure, judged on wording alone. It
// deliberately ignores the status-code patterns IsRetryableAssistantError
// uses: those match a composed message's leading status, and a raw HTTP error
// body full of token counts would trip them (see retryableStatusCodePatterns).
// Callers that already know the numeric status — the request-level retry loop
// in ai/apis/internal/httpretry — check it themselves and use this for the
// text fallback.
//
// Callers must consult IsNonRetryableProviderLimitError first: quota text such
// as "quota exceeded" also matches this pattern's "rate.?limit"-family
// entries, and the terminal classification wins.
func IsTransientProviderErrorText(text string) bool {
	return transientProviderErrorPattern.MatchString(text)
}

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
