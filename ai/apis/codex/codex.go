// Package codex implements the Codex request/auth layer over the shared
// openai-responses core (ai/apis/openairesponses): OAuth credential
// consumption (via the caller-resolved ai.StreamOptions.APIKey, same as every
// sibling adapter -- ai/resolve.go's OAuth refresh applies no special-cased
// margin for Codex, so no adapter-side handling is needed here), JWT
// accountId extraction, and Codex-specific request shaping.
//
// Two transports are implemented: a WebSocket transport (websocket.go, epic 7
// issue 04) tried first for any Transport other than "sse", and the plain
// HTTP/SSE transport (this file, epic 7 issue 03) used directly when
// Transport is "sse" and as the fallback when the WebSocket attempt fails
// before producing any output. The SSE path always zstd-compresses its
// request body (zstd.go, issue 04), matching the Codex backend's own
// zstd-compressed traffic. See websocket.go's package doc for what its
// connection-cache scope cut deliberately leaves unported.
//
// Ports: packages/ai/src/api/openai-codex-responses.ts
package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/openairesponses"
)

// defaultCodexBaseURL is Codex's default backend, used when the model
// supplies no baseUrl.
const defaultCodexBaseURL = "https://chatgpt.com/backend-api"

// defaultMaxRetries matches upstream's DEFAULT_MAX_RETRIES: Codex's own
// per-request HTTP retry cap defaults to zero (no retries), unlike the
// generic "2" default this repo's shared StreamOptions.MaxRetries doc
// describes for OpenAI/Anthropic-style clients -- no other adapter in this
// repo implements that generic default yet, so there is no conflict.
const defaultMaxRetries = 0

// baseRetryDelay is the exponential-backoff base delay (ports BASE_DELAY_MS).
// Declared as a var (not a const) so tests can shrink it to keep retry tests
// fast without waiting on real 1s/2s/4s sleeps.
var baseRetryDelay = 1000 * time.Millisecond

// Stream implements ai.StreamFunc for the Codex Responses HTTP/SSE wire
// protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc. Ports the streamSimple half of
// openai-codex-responses.ts: unlike Stream's raw reasoningEffort (which can
// carry the "off" sentinel through to applyReasoning's explicit-suppression
// path), streamSimple clears reasoning entirely when the clamped level is
// "off" -- ai.ClampThinkingLevel's "off" result is simply never copied into
// ReasoningEffort, so applyReasoning sees it unset and omits the field.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)
	if opts != nil && opts.Reasoning != "" {
		if clamped := ai.ClampThinkingLevel(model, opts.Reasoning); clamped != ai.ThinkingOff {
			base.ReasoningEffort = clamped
		}
	}
	return Stream(ctx, model, chat, &base)
}

func run(ctx context.Context, out *ai.Stream, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) {
	output := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        ai.ApiOpenAICodexResponses,
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  time.Now().UnixMilli(),
	}

	fail := func(err error) {
		reason := ai.StopReasonError
		if ctx.Err() != nil {
			reason = ai.StopReasonAborted
		}
		output.StopReason = reason
		output.ErrorMessage = err.Error()
		out.Push(ai.ErrorEvent{Reason: reason, Error: output})
	}

	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	if apiKey == "" {
		fail(fmt.Errorf("No API key for provider: %s", model.Provider))
		return
	}

	accountID, err := extractAccountID(apiKey)
	if err != nil {
		fail(err)
		return
	}

	body := buildRequestBody(model, chat, opts)
	var payload any = body
	if opts != nil && opts.OnPayload != nil {
		next, err := opts.OnPayload(ctx, payload, model)
		if err != nil {
			fail(err)
			return
		}
		if next != nil {
			payload = next
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		fail(err)
		return
	}

	transport := ai.TransportAuto
	if opts != nil && opts.Transport != "" {
		transport = opts.Transport
	}
	var sessionID string
	if opts != nil {
		sessionID = opts.SessionID
	}

	// WebSocket transport attempt (epic 7 issue 04): tried first unless the
	// caller explicitly asked for "sse", or this session has already fallen
	// back to SSE once before (per-session fallback memory -- ports
	// isWebSocketSseFallbackActive/recordWebSocketSseFallback).
	if transport != ai.TransportSSE {
		if codexWebSocketFallbackActive(sessionID) {
			codexRecordWebSocketSSEFallback(sessionID)
		} else if handled := attemptCodexWebSocketOrFallback(ctx, out, output, model, opts, bodyBytes, accountID, apiKey, transport, fail); handled {
			return
		}
	}

	// Plain HTTP/SSE transport (epic 7 issue 03), reached directly when
	// Transport is "sse", or as the WebSocket attempt's fallback.
	url := resolveCodexURL(model.BaseURL)
	headers := buildHeaders(model, opts, accountID, apiKey)

	sseBody := bodyBytes
	if compressed, cerr := compressRequestBodyZstd(bodyBytes); cerr == nil {
		sseBody = compressed
		headers["content-encoding"] = "zstd"
	}

	resp, err := doRequestWithRetry(ctx, url, sseBody, headers, model, opts)
	if err != nil {
		fail(err)
		return
	}
	defer resp.Body.Close()

	out.Push(ai.StartEvent{Partial: output.Clone()})

	var serviceTier string
	if opts != nil {
		serviceTier = opts.ServiceTier
	}
	decodeErr := openairesponses.DecodeStream(out, output, model, resp.Body, openairesponses.ServiceTierOptions{
		RequestServiceTier: serviceTier,
		ResolveServiceTier: resolveCodexServiceTier,
	})
	if decodeErr != nil {
		fail(decodeErr)
		return
	}

	finishCodexStream(ctx, out, output, fail)
}

// attemptCodexWebSocketOrFallback runs the WebSocket connection-limit-retry
// loop (attemptCodexWebSocket) and applies the same fallback-vs-propagate
// decision as upstream's stream() catch block: an aborted request or a
// non-transport error (an in-band Codex error/protocol error, unless it's the
// connection-limit case already retried) is final; anything else is a
// transport failure that falls back to SSE unless output had already started,
// in which case it's final too. Returns handled=true when the request is
// already fully resolved (succeeded, or failed terminally via fail) and no
// SSE fallback should be attempted; handled=false means "fall through to
// SSE".
func attemptCodexWebSocketOrFallback(
	ctx context.Context,
	out *ai.Stream,
	output *ai.AssistantMessage,
	model *ai.Model,
	opts *ai.StreamOptions,
	bodyBytes []byte,
	accountID, apiKey string,
	transport ai.Transport,
	fail func(error),
) (handled bool) {
	var sessionID string
	if opts != nil {
		sessionID = opts.SessionID
	}

	started, wsErr := attemptCodexWebSocket(ctx, out, output, model, opts, bodyBytes, accountID, apiKey)
	if wsErr == nil {
		finishCodexStream(ctx, out, output, fail)
		return true
	}

	if ctx.Err() != nil || isCodexNonTransportError(wsErr) {
		fail(wsErr)
		return true
	}

	ai.AppendDiagnostic(output, ai.NewAssistantMessageDiagnostic("provider_transport_failure", wsErr, map[string]any{
		"configuredTransport": string(transport),
		"eventsEmitted":       started,
		"requestBytes":        len(bodyBytes),
	}))
	codexRecordWebSocketFailure(sessionID, wsErr)
	if started {
		fail(wsErr)
		return true
	}
	codexRecordWebSocketSSEFallback(sessionID)
	return false
}

// finishCodexStream applies the shared terminal-state checks both transports
// finish through: an aborted context or an in-flight-marked abort/error
// StopReason becomes the final ErrorEvent, otherwise the DoneEvent is pushed.
func finishCodexStream(ctx context.Context, out *ai.Stream, output *ai.AssistantMessage, fail func(error)) {
	if ctx.Err() != nil {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonAborted {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonError {
		msg := output.ErrorMessage
		if msg == "" {
			msg = "An unknown error occurred"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// doRequestWithRetry sends the Codex SSE POST, retrying transient failures up
// to opts.MaxRetries times with exponential backoff (or the server's
// retry-after delay when present). Ports the fetch-with-retry loop in
// stream() (the SSE branch only). bodyBytes is whatever run() decided to
// send -- the zstd-compressed body in the normal case (see run()'s call
// site), or the raw JSON if compression somehow failed.
func doRequestWithRetry(
	ctx context.Context,
	url string,
	bodyBytes []byte,
	headers map[string]string,
	model *ai.Model,
	opts *ai.StreamOptions,
) (*http.Response, error) {
	maxRetries := defaultMaxRetries
	if opts != nil && opts.MaxRetries != nil {
		maxRetries = *opts.MaxRetries
	}
	var headerTimeout time.Duration
	if opts != nil {
		headerTimeout = opts.Timeout
	}
	client := &http.Client{}

	var lastErr error
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return nil, errors.New("Request was aborted")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := doWithHeaderTimeout(ctx, client, req, headerTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return nil, errors.New("Request was aborted")
			}
			lastErr = err
			if attempt < maxRetries {
				if sleepErr := sleepCtx(ctx, exponentialDelay(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, lastErr
		}

		if opts != nil && opts.OnResponse != nil {
			respMeta := ai.ProviderResponse{Status: resp.StatusCode, Headers: ai.HeadersToRecord(resp.Header)}
			if cbErr := opts.OnResponse(ctx, respMeta, model); cbErr != nil {
				resp.Body.Close()
				return nil, cbErr
			}
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		errorText := string(errBody)

		if attempt < maxRetries && isRetryableError(resp.StatusCode, errorText) {
			delay := exponentialDelay(attempt)
			if retryAfter, ok := getRetryAfterDelay(resp.Header); ok {
				if resp.StatusCode == http.StatusTooManyRequests {
					delay = capRetryDelay(retryAfter, opts)
				} else {
					delay = retryAfter
				}
			}
			if sleepErr := sleepCtx(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		return nil, parseErrorResponse(resp.StatusCode, resp.Status, errorText)
	}
}

// doWithHeaderTimeout applies timeout only until response headers arrive
// (ports the AbortSignal.timeout(httpTimeoutMs) combined into the SSE fetch
// call): once the request succeeds, the timer is stopped so it never cancels
// the body read that follows. A zero timeout disables this (the request then
// only respects ctx).
func doWithHeaderTimeout(ctx context.Context, client *http.Client, req *http.Request, timeout time.Duration) (*http.Response, error) {
	if timeout <= 0 {
		return client.Do(req)
	}

	timeoutCtx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(timeout, cancel)
	resp, err := client.Do(req.WithContext(timeoutCtx))
	if err != nil {
		timer.Stop()
		if timeoutCtx.Err() != nil && ctx.Err() == nil {
			return nil, fmt.Errorf("Codex SSE response headers timed out after %dms", timeout.Milliseconds())
		}
		return nil, err
	}
	timer.Stop()
	return resp, nil
}

// sleepCtx sleeps for d, or returns an aborted error if ctx is cancelled
// first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		if ctx.Err() != nil {
			return errors.New("Request was aborted")
		}
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return errors.New("Request was aborted")
	}
}

// exponentialDelay ports `BASE_DELAY_MS * 2 ** attempt`.
func exponentialDelay(attempt int) time.Duration {
	return baseRetryDelay * time.Duration(math.Pow(2, float64(attempt)))
}

// --- retry classification -----------------------------------------------

var (
	terminalRateLimitPattern = regexp.MustCompile(
		`(?i)GoUsageLimitError|FreeUsageLimitError|Monthly usage limit reached|available balance|insufficient_quota|out of budget|quota exceeded|billing`,
	)
	retryableTextPattern  = regexp.MustCompile(`(?i)rate.?limit|overloaded|service.?unavailable|upstream.?connect|connection.?refused`)
	usageLimitCodePattern = regexp.MustCompile(`(?i)usage_limit_reached|usage_not_included|rate_limit_exceeded`)
)

// isRetryableError ports isRetryableError: a 429 whose body text matches a
// terminal (subscription/quota) rate-limit pattern is never retried; other
// 429/5xx statuses, and any status whose body matches the generic transient
// pattern, are retryable.
func isRetryableError(status int, errorText string) bool {
	if status == http.StatusTooManyRequests && terminalRateLimitPattern.MatchString(errorText) {
		return false
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return retryableTextPattern.MatchString(errorText)
}

// getRetryAfterDelay ports getRetryAfterDelayMs: retry-after-ms wins over
// retry-after (itself parsed as either seconds or an HTTP date).
func getRetryAfterDelay(headers http.Header) (time.Duration, bool) {
	if raw := headers.Get("retry-after-ms"); raw != "" {
		if ms, err := strconv.ParseFloat(raw, 64); err == nil {
			if ms < 0 {
				ms = 0
			}
			return time.Duration(ms * float64(time.Millisecond)), true
		}
	}

	raw := headers.Get("retry-after")
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs * float64(time.Second)), true
	}
	if t, err := http.ParseTime(raw); err == nil {
		delay := time.Until(t)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

// capRetryDelay ports capRetryDelayMs: caps a server-requested 429 delay at
// opts.EffectiveMaxRetryDelay (0 disables the cap).
func capRetryDelay(delay time.Duration, opts *ai.StreamOptions) time.Duration {
	maxDelay := opts.EffectiveMaxRetryDelay()
	if maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}
	return delay
}

// parseErrorResponse ports parseErrorResponse: prefers a JSON error body's
// own message, upgrading to a friendly usage-limit message when the error
// code (or a 429 status) indicates a ChatGPT usage-limit rejection.
func parseErrorResponse(status int, statusText, raw string) error {
	message := raw
	if message == "" {
		message = statusText
	}
	if message == "" {
		message = "Request failed"
	}

	var parsed struct {
		Error *struct {
			Code     string  `json:"code"`
			Type     string  `json:"type"`
			Message  string  `json:"message"`
			PlanType string  `json:"plan_type"`
			ResetsAt float64 `json:"resets_at"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &parsed) != nil || parsed.Error == nil {
		return errors.New(message)
	}

	errInfo := parsed.Error
	code := errInfo.Code
	if code == "" {
		code = errInfo.Type
	}

	var friendly string
	if usageLimitCodePattern.MatchString(code) || status == http.StatusTooManyRequests {
		plan := ""
		if errInfo.PlanType != "" {
			plan = fmt.Sprintf(" (%s plan)", strings.ToLower(errInfo.PlanType))
		}
		when := ""
		if errInfo.ResetsAt > 0 {
			mins := int(math.Round((errInfo.ResetsAt*1000 - float64(time.Now().UnixMilli())) / 60000))
			if mins < 0 {
				mins = 0
			}
			when = fmt.Sprintf(" Try again in ~%d min.", mins)
		}
		friendly = strings.TrimSpace(fmt.Sprintf("You have hit your ChatGPT usage limit%s.%s", plan, when))
	}

	if errInfo.Message != "" {
		return errors.New(errInfo.Message)
	}
	if friendly != "" {
		return errors.New(friendly)
	}
	return errors.New(message)
}

// --- headers -----------------------------------------------------------

// buildHeaders builds the Codex SSE request headers. Ports
// buildBaseCodexHeaders + buildSSEHeaders (the WebSocket header variant is
// issue 04's scope).
func buildHeaders(model *ai.Model, opts *ai.StreamOptions, accountID, apiKey string) map[string]string {
	defaults := map[string]string{
		"authorization":      "Bearer " + apiKey,
		"chatgpt-account-id": accountID,
		"originator":         "pi",
		"user-agent":         fmt.Sprintf("pi (%s; %s)", runtime.GOOS, runtime.GOARCH),
		"openai-beta":        "responses=experimental",
		"accept":             "text/event-stream",
		"content-type":       "application/json",
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}

	var sessionID string
	if opts != nil {
		sessionID = opts.SessionID
	}
	if sessionID != "" {
		defaults["session-id"] = sessionID
		defaults["x-client-request-id"] = sessionID
	}

	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}
