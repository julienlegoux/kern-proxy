// Package httpretry is the shared HTTP-request-level retry loop that every
// raw-net/http adapter under ai/apis uses to honor
// ai.StreamOptions.MaxRetries and ai.StreamOptions.MaxRetryDelay.
//
// The loop retries the request until it gets a 2xx *before* any streaming
// begins, and hands the still-unread response body back to the caller. That
// boundary is deliberate: once SSE events have flowed to the consumer, a retry
// cannot be made transparent, so a request that fails mid-stream is the
// caller's problem, not this package's. Adapters backed by a vendor SDK
// (bedrock) map MaxRetries onto the SDK's own retryer instead of using this.
//
// Generalized from the Codex adapter's doRequestWithRetry, which ported
// upstream's fetch-with-retry loop (packages/ai/src/api/openai-codex-responses.ts).
package httpretry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

// DefaultMaxRetries is the retry cap adapters pass as Config.DefaultMaxRetries
// when ai.StreamOptions.MaxRetries is nil. It matches the value documented on
// that field and upstream's OpenAI/Anthropic-style client default.
//
// Codex is the deliberate exception: it passes 0, mirroring upstream's
// DEFAULT_MAX_RETRIES for the Codex backend.
const DefaultMaxRetries = 2

// BaseDelay is the exponential-backoff base delay (ports BASE_DELAY_MS).
// A var, not a const, so tests can shrink it instead of waiting on real
// 1s/2s/4s sleeps.
var BaseDelay = 1000 * time.Millisecond

// maxErrorBodyBytes bounds how much of a non-2xx body is read for
// classification and error text.
const maxErrorBodyBytes = 1 << 20

// Request is the HTTP request to send, re-created from scratch on every
// attempt so Body is replayable.
type Request struct {
	// Method defaults to POST when empty.
	Method string
	URL    string
	Body   []byte
	// Headers are set (not added) on each attempt.
	Headers map[string]string
}

// Config carries the per-adapter knobs. The zero value is usable: it makes one
// attempt with a default client and returns response-body text as the error.
type Config struct {
	// Client sends the requests; nil uses a fresh &http.Client{}.
	Client *http.Client
	// Opts supplies MaxRetries, MaxRetryDelay, Timeout, and OnResponse. May be nil.
	Opts *ai.StreamOptions
	// Model is passed to Opts.OnResponse. May be nil.
	Model *ai.Model
	// DefaultMaxRetries applies when Opts.MaxRetries is nil. Adapters pass
	// DefaultMaxRetries; codex passes 0.
	DefaultMaxRetries int
	// ParseError turns a terminal non-2xx response into the error the adapter
	// wants to surface. nil returns the body text (or the status line).
	ParseError func(status int, statusText, body string) error
	// TimeoutError names the adapter in the header-timeout error. nil produces
	// a generic message.
	TimeoutError func(timeout time.Duration) error
}

// ErrAborted is returned when ctx is cancelled, matching the error text every
// adapter already surfaces for an aborted request.
var ErrAborted = errors.New("Request was aborted")

// Do sends req, retrying transient failures up to the effective retry cap with
// exponential backoff (or the server's Retry-After delay when present, clamped
// by Opts.MaxRetryDelay).
//
// On success it returns the 2xx response with its body unread — the caller
// owns closing it. On failure every response body has already been drained and
// closed.
func Do(ctx context.Context, req Request, cfg Config) (*http.Response, error) {
	maxRetries := cfg.DefaultMaxRetries
	if cfg.Opts != nil && cfg.Opts.MaxRetries != nil {
		maxRetries = *cfg.Opts.MaxRetries
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{}
	}
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}
	var headerTimeout time.Duration
	if cfg.Opts != nil {
		headerTimeout = cfg.Opts.Timeout
	}

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return nil, ErrAborted
		}

		httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, bytes.NewReader(req.Body))
		if err != nil {
			return nil, err
		}
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}

		resp, err := doWithHeaderTimeout(ctx, client, httpReq, headerTimeout, cfg.TimeoutError)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ErrAborted
			}
			if attempt < maxRetries {
				if sleepErr := Sleep(ctx, ExponentialDelay(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, err
		}

		if cfg.Opts != nil && cfg.Opts.OnResponse != nil {
			meta := ai.ProviderResponse{Status: resp.StatusCode, Headers: ai.HeadersToRecord(resp.Header)}
			if cbErr := cfg.Opts.OnResponse(ctx, meta, cfg.Model); cbErr != nil {
				_ = resp.Body.Close()
				return nil, cbErr
			}
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		_ = resp.Body.Close()
		errorText := string(errBody)

		if attempt < maxRetries && IsRetryable(resp.StatusCode, errorText) {
			delay := ExponentialDelay(attempt)
			if retryAfter, ok := RetryAfterDelay(resp.Header); ok {
				if resp.StatusCode == http.StatusTooManyRequests {
					delay = capRetryDelay(retryAfter, cfg.Opts)
				} else {
					delay = retryAfter
				}
			}
			if sleepErr := Sleep(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if cfg.ParseError != nil {
			return nil, cfg.ParseError(resp.StatusCode, resp.Status, errorText)
		}
		return nil, defaultParseError(resp.StatusCode, resp.Status, errorText)
	}
}

// doWithHeaderTimeout applies timeout only until response headers arrive: once
// the request succeeds the timer is stopped, so it never cancels the streaming
// body read that follows. A zero timeout disables it (the request then only
// respects ctx).
func doWithHeaderTimeout(
	ctx context.Context,
	client *http.Client,
	req *http.Request,
	timeout time.Duration,
	timeoutError func(time.Duration) error,
) (*http.Response, error) {
	if timeout <= 0 {
		return client.Do(req)
	}

	timeoutCtx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(timeout, cancel)
	resp, err := client.Do(req.WithContext(timeoutCtx))
	if err != nil {
		timer.Stop()
		if timeoutCtx.Err() != nil && ctx.Err() == nil {
			if timeoutError != nil {
				return nil, timeoutError(timeout)
			}
			return nil, fmt.Errorf("response headers timed out after %dms", timeout.Milliseconds())
		}
		return nil, err
	}
	timer.Stop()
	return resp, nil
}

// defaultParseError surfaces the response body, falling back to the status line.
func defaultParseError(_ int, statusText, body string) error {
	switch {
	case body != "":
		return errors.New(body)
	case statusText != "":
		return errors.New(statusText)
	default:
		return errors.New("Request failed")
	}
}

// Sleep waits for d, or returns ErrAborted as soon as ctx is cancelled.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		if ctx.Err() != nil {
			return ErrAborted
		}
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ErrAborted
	}
}

// ExponentialDelay ports `BASE_DELAY_MS * 2 ** attempt`.
func ExponentialDelay(attempt int) time.Duration {
	return BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
}

// IsRetryable classifies an HTTP error response. Provider text naming a
// subscription, quota, or billing limit is terminal at any status; the
// canonical transient statuses always retry; anything else retries only when
// its body text reads as a transient provider or transport failure.
//
// Both text classifications come from ai/retry.go, so the request-level retry
// loop and the assistant-turn-level one (ai.IsRetryableAssistantError) agree
// on what "transient" means. The text fallback uses the status-code-free
// subset: a raw error body is not a composed assistant message, so digits in
// it ("max_tokens: 15000") carry no meaning.
func IsRetryable(status int, errorText string) bool {
	if ai.IsNonRetryableProviderLimitError(errorText) {
		return false
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return ai.IsTransientProviderErrorText(errorText)
}

// RetryAfterDelay reads the server-requested delay (ports
// getRetryAfterDelayMs): `retry-after-ms` wins over `retry-after`, which is
// itself parsed as either seconds or an HTTP date.
func RetryAfterDelay(headers http.Header) (time.Duration, bool) {
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
